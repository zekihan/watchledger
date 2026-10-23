package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/zekihan/watchledger/internal/store"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestPeopleDiscoveryFiltersKnownDataAndMultipleServices(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	ctx := context.Background()
	p, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if err = store.Migrate(ctx, p); err != nil {
		t.Fatal(err)
	}
	base := int(time.Now().UnixNano()%100000000) + 800000000
	ids := []int{base, base + 1, base + 2}
	for i, id := range ids {
		metadata := fmt.Sprintf(`{"en":{"id":%d,"name":"Synthetic candidate","episode_run_time":[%d],"genres":[{"id":%d}],"vote_count":100,"vote_average":8}}`, id, []int{20, 0, 35}[i], []int{1, 3, 2}[i])
		key := "tmdb:tv:" + strconv.Itoa(id)
		if _, err = p.Exec(ctx, "INSERT INTO catalog(id,source,source_id,kind,title,metadata) VALUES($1,'tmdb',$2,'tv','Synthetic',$3)", key, strconv.Itoa(id), metadata); err != nil {
			t.Fatal(err)
		}
		defer func(key string) {
			_, _ = p.Exec(ctx, "DELETE FROM availability WHERE catalog_id=$1", key)
			_, _ = p.Exec(ctx, "DELETE FROM catalog_localizations WHERE catalog_id=$1", key)
			_, _ = p.Exec(ctx, "DELETE FROM catalog WHERE id=$1", key)
		}(key)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/person/1/tv_credits" {
			_, _ = fmt.Fprintf(w, `{"cast":[{"id":%d},{"id":%d},{"id":%d},{"id":%d}],"crew":[]}`, ids[0], ids[1], ids[2], ids[0])
			return
		}
		for i, id := range ids {
			if r.URL.Path == fmt.Sprintf("/tv/%d/watch/providers", id) {
				if i == 0 {
					_, _ = fmt.Fprint(w, `{"results":{"TR":{"link":"https://example.test/offers","flatrate":[{"provider_id":8}],"rent":[{"provider_id":9}]}}}`)
				} else {
					_, _ = fmt.Fprintf(w, `{"results":{"TR":{"flatrate":[{"provider_id":%d}]}}}`, []int{8, 8, 9}[i])
				}
				return
			}
		}
		t.Errorf("unexpected request %s", r.URL.Path)
		http.NotFound(w, r)
	}))
	defer server.Close()
	provider := New("synthetic")
	provider.BaseURL = server.URL
	c := Cache{DB: p, Provider: provider}
	for _, test := range []struct {
		name   string
		params url.Values
		want   []int
	}{
		{"OR providers with known runtime", url.Values{"with_watch_providers": {"8|9"}, "with_watch_monetization_types": {"flatrate"}, "with_runtime.lte": {"30"}}, []int{ids[0]}},
		{"AND providers limited to subscriptions", url.Values{"with_watch_providers": {"8,9"}, "with_watch_monetization_types": {"flatrate"}}, nil},
		{"AND providers across offer categories", url.Values{"with_watch_providers": {"8,9"}}, []int{ids[0]}},
		{"rental offers", url.Values{"with_watch_providers": {"8|9"}, "with_watch_monetization_types": {"rent"}}, []int{ids[0]}},
		{"OR genres", url.Values{"with_genres": {"1|2"}}, []int{ids[0], ids[2]}},
		{"AND genres", url.Values{"with_genres": {"1,2"}}, nil},
		{"large page does not overflow", url.Values{"page": {"9223372036854775807"}}, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.params.Set("language", "en")
			test.params.Set("watch_region", "TR")
			raw, err := c.PeopleTV(ctx, "1", test.params)
			if err != nil {
				t.Fatal(err)
			}
			var result struct {
				Results []struct {
					ID int `json:"id"`
				} `json:"results"`
				Exhaustive bool `json:"exhaustive"`
				Candidates int  `json:"candidate_count"`
				Pages      int  `json:"total_pages"`
			}
			if err = json.Unmarshal(raw, &result); err != nil {
				t.Fatal(err)
			}
			got := []int{}
			for _, item := range result.Results {
				got = append(got, item.ID)
			}
			if fmt.Sprint(got) != fmt.Sprint(test.want) || result.Exhaustive || result.Candidates != 3 || result.Pages != 1 {
				t.Fatal(string(raw), test.want)
			}
		})
	}
}
func TestDiscoverySetRejectsAmbiguousOrInvalidExpressions(t *testing.T) {
	for _, raw := range []string{"1|2,3", "1||2", "0", "-1", "one"} {
		if _, err := parseDiscoverySet(raw, positiveID); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}
