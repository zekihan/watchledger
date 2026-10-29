package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/zekihan/watchledger/internal/store"
)

func TestDetailFallbackRequiresMatchingIdentity(t *testing.T) {
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
	for _, kind := range []string{"movie", "tv"} {
		for _, cached := range []bool{false, true} {
			for _, tc := range []struct {
				name     string
				identity string
				valid    bool
			}{
				{"matching", `"id":881234,`, true},
				{"different", `"id":881235,`, false},
				{"missing", "", false},
				{"zero", `"id":0,`, false},
				{"null", `"id":null,`, false},
				{"wrong_type", `"id":"881234",`, false},
			} {
				t.Run(fmt.Sprintf("%s/cached_%t/%s", kind, cached, tc.name), func(t *testing.T) {
					key := "tmdb:" + kind + ":881234"
					t.Cleanup(func() {
						if _, err := p.Exec(ctx, "DELETE FROM catalog_localizations WHERE catalog_id=$1", key); err != nil {
							t.Error(err)
						}
						if _, err := p.Exec(ctx, "DELETE FROM catalog WHERE id=$1", key); err != nil {
							t.Error(err)
						}
					})
					if cached {
						_, err = p.Exec(ctx, `INSERT INTO catalog(id,source,source_id,kind,title,metadata) VALUES($1,'tmdb','881234',$2,'Existing','{"":{"id":881234,"overview":"Existing default"}}')`, key, kind)
						if err != nil {
							t.Fatal(err)
						}
					}
					var requests atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						requests.Add(1)
						if r.URL.Query().Get("language") == "tr" {
							_, _ = w.Write([]byte(`{"id":881234,"title":"","name":"","overview":""}`))
							return
						}
						_, _ = fmt.Fprintf(w, `{%s"title":"Default title","name":"Default name","overview":"Default overview"}`, tc.identity)
					}))
					defer server.Close()
					provider := New("synthetic")
					provider.BaseURL = server.URL
					cache := Cache{DB: p, Provider: provider}
					detail, err := cache.Detail(ctx, kind, "881234", "tr", true)
					if err != nil {
						t.Fatal(err)
					}
					assertFields := func(raw json.RawMessage) {
						t.Helper()
						var fields map[string]interface{}
						if err := json.Unmarshal(raw, &fields); err != nil {
							t.Fatal(err)
						}
						for _, field := range []string{"title", "name", "overview"} {
							want := ""
							if tc.valid {
								want = "Default " + field
							}
							if fields[field] != want {
								t.Errorf("%s = %v, want %q", field, fields[field], want)
							}
						}
					}
					assertFields(detail.Metadata)
					var stored map[string]json.RawMessage
					if err = p.QueryRow(ctx, "SELECT metadata FROM catalog WHERE id=$1", key).Scan(&stored); err != nil {
						t.Fatal(err)
					}
					assertFields(stored["tr"])
					var defaultMeta struct {
						Overview string `json:"overview"`
					}
					if raw, ok := stored[""]; ok {
						if err = json.Unmarshal(raw, &defaultMeta); err != nil {
							t.Fatal(err)
						}
					} else if cached || tc.valid {
						t.Fatal("missing default cache")
					}
					wantDefault := ""
					if tc.valid {
						wantDefault = "Default overview"
					} else if cached {
						wantDefault = "Existing default"
					}
					if defaultMeta.Overview != wantDefault {
						t.Errorf("default overview = %q, want %q", defaultMeta.Overview, wantDefault)
					}
					cachedDetail, err := cache.Detail(ctx, kind, "881234", "tr", false)
					if err != nil {
						t.Fatal(err)
					}
					assertFields(cachedDetail.Metadata)
					if requests.Load() != 2 {
						t.Errorf("provider requests = %d, want 2", requests.Load())
					}
				})
			}
		}
	}
}
