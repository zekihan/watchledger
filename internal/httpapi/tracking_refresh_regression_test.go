package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zekihan/watchledger/internal/catalog"
	"github.com/zekihan/watchledger/internal/mapping"
	"github.com/zekihan/watchledger/internal/store"
)

func refreshRegressionSetup(t *testing.T, ids ...string) auditEnv {
	t.Helper()
	t.Cleanup(func() {
		if os.Getenv("TEST_DATABASE_URL") == "" {
			return
		}
		p, err := store.Open(context.Background(), os.Getenv("TEST_DATABASE_URL"))
		if err != nil {
			t.Error(err)
			return
		}
		defer p.Close()
		for _, table := range []string{"availability", "catalog_localizations", "season_cache", "episodes"} {
			if _, err = p.Exec(context.Background(), "DELETE FROM "+table+" WHERE catalog_id=ANY($1::text[])", ids); err != nil {
				t.Error(err)
			}
		}
		if _, err = p.Exec(context.Background(), "DELETE FROM catalog WHERE id=ANY($1::text[])", ids); err != nil {
			t.Error(err)
		}
	})
	return auditSetup(t)
}

func TestTrackingUsesLatestSourceEpisodeCount(t *testing.T) {
	id := "tmdb:tv:987601"
	e := refreshRegressionSetup(t, id)
	audit2Exec(t, e, `INSERT INTO catalog(id,source,source_id,kind,title,metadata) VALUES($1,'tmdb','987601','tv','Audit','{"en":{"status":"Ended","number_of_episodes":3},"tr":{"status":"Ended","number_of_episodes":2}}')`, id)
	audit2Exec(t, e, `INSERT INTO catalog_localizations(catalog_id,language,metadata,fetched_at) VALUES($1,'en','{"status":"Ended","number_of_episodes":3}',now()-interval '1 day'),($1,'tr','{"status":"Ended","number_of_episodes":2}',now())`, id)
	for n := 1; n <= 2; n++ {
		audit2Exec(t, e, `INSERT INTO episodes(id,catalog_id,source,source_id,season,number,air_date) VALUES($1,$2,'tmdb',$1,1,$3,'2020-01-01')`, fmt.Sprintf("tmdb:episode:98760%d", n), id, n)
	}
	e.req(t, 0, "PUT", "/api/library/"+id, map[string]any{"base": "watching"}, 204)
	for n := 1; n <= 2; n++ {
		e.req(t, 0, "POST", "/api/library/"+id+"/track", map[string]any{"episode_id": fmt.Sprintf("tmdb:episode:98760%d", n), "watched": true, "date": map[string]any{}}, 200)
	}
	var base string
	if err := e.p.QueryRow(t.Context(), "SELECT base FROM library WHERE user_id=$1 AND catalog_id=$2", e.users[0], id).Scan(&base); err != nil || base != "completed" {
		t.Fatal(base, err)
	}
	e.req(t, 0, "POST", "/api/library/"+id+"/track", map[string]any{"whole": true, "watched": true, "date": map[string]any{}}, 200)
	raw := e.req(t, 0, "GET", "/api/library/watched", nil, 200)
	if !strings.Contains(string(raw), id) {
		t.Fatal("latest source count not used for watched exclusion", string(raw))
	}
}

func TestAlternateUpNextUsesDisplayedEpisodeOrder(t *testing.T) {
	e := refreshRegressionSetup(t, "anidb:anime:987630", "tmdb:tv:987630")
	audit2Exec(t, e, `INSERT INTO catalog(id,source,source_id,kind,title,metadata) VALUES('anidb:anime:987630','anidb','987630','anime','Native','{"en":{"number_of_episodes":2}}'),('tmdb:tv:987630','tmdb','987630','tv','Show','{"en":{"number_of_episodes":2}}')`)
	for n := 1; n <= 2; n++ {
		for _, prefix := range []string{"anidb", "tmdb"} {
			kind := "tv"
			if prefix == "anidb" {
				kind = "anime"
			}
			audit2Exec(t, e, `INSERT INTO episodes(id,catalog_id,source,source_id,season,number,air_date) VALUES($1,$2,$3,$4,1,$5,'2020-01-01')`, fmt.Sprintf("%s:episode:98763%d", prefix, n), prefix+":"+kind+":987630", prefix, fmt.Sprint(987630+n), n)
		}
	}
	e.req(t, 0, "PUT", "/api/library/anidb:anime:987630", map[string]any{"base": "watching", "anime_view": "show"}, 204)
	edges, revision, _, err := mapping.Canonical([]mapping.Edge{{From: "anidb:episode:987631", To: "tmdb:episode:987632"}, {From: "anidb:episode:987632", To: "tmdb:episode:987631"}})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(edges)
	if err != nil {
		t.Fatal(err)
	}
	var mappingID string
	if err = e.p.QueryRow(t.Context(), `INSERT INTO mappings(revision,relationships,provenance,reviewed,requires_consent) VALUES($1,$2,'{}',true,false) RETURNING id`, revision, raw).Scan(&mappingID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { audit2Exec(t, e, "DELETE FROM mappings WHERE id=$1", mappingID) })
	cards, err := buildUpNext(t.Context(), e.p, e.users[0], "UTC")
	if err != nil || len(cards) != 1 || cards[0].Number != 1 || cards[0].CatalogID != "tmdb:tv:987630" {
		t.Fatalf("cards=%+v err=%v", cards, err)
	}
	e.req(t, 0, "PUT", "/api/library/anidb:anime:987630/structure", map[string]any{"view": "native"}, 204)
	cards, err = buildUpNext(t.Context(), e.p, e.users[0], "UTC")
	if err != nil || len(cards) != 1 || cards[0].EpisodeID != "anidb:episode:987631" {
		t.Fatalf("native cards=%+v err=%v", cards, err)
	}
}

func TestSeasonReconciliationInvalidatesOtherLanguagesAndNotifies(t *testing.T) {
	e := refreshRegressionSetup(t, "tmdb:tv:987650")
	audit2Exec(t, e, `INSERT INTO catalog(id,source,source_id,kind,title,metadata) VALUES('tmdb:tv:987650','tmdb','987650','tv','Audit','{"en":{"id":987650},"tr":{"id":987650}}')`)
	audit2Exec(t, e, `INSERT INTO episodes(id,catalog_id,source,source_id,season,number,metadata,air_date) VALUES('tmdb:episode:987651','tmdb:tv:987650','tmdb','987651',1,1,'{"id":987651}','2020-01-01')`)
	audit2Exec(t, e, `INSERT INTO history(user_id,catalog_id,episode_id,watched_date) VALUES($1,'tmdb:tv:987650','tmdb:episode:987651','2026-01-02')`, e.users[0])
	audit2Exec(t, e, `INSERT INTO season_cache(catalog_id,season,language,metadata) VALUES('tmdb:tv:987650',1,'en','{"episodes":[{"id":987651,"episode_number":1}]}')`)
	var number atomic.Int32
	number.Store(2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if number.Load() == 0 {
			_, _ = fmt.Fprint(w, `{"episodes":[]}`)
		} else {
			_, _ = fmt.Fprintf(w, `{"episodes":[{"id":987651,"episode_number":%d,"air_date":"2020-01-01"}]}`, number.Load())
		}
	}))
	defer srv.Close()
	provider := catalog.New("synthetic")
	provider.BaseURL = srv.URL
	cache := catalog.Cache{DB: e.p, Provider: provider}
	for _, expected := range []int{2, 0} {
		number.Store(int32(expected))
		if _, err := cache.Season(t.Context(), "987650", 1, "tr", true); err != nil {
			t.Fatal(err)
		}
		raw, err := cache.Season(t.Context(), "987650", 1, "en")
		if err != nil {
			t.Fatal(err)
		}
		var data struct {
			Episodes []struct {
				Number int `json:"episode_number"`
			} `json:"episodes"`
		}
		if err = json.Unmarshal(raw, &data); err != nil || (expected == 0 && len(data.Episodes) != 0) || (expected == 2 && (len(data.Episodes) != 1 || data.Episodes[0].Number != 2)) {
			t.Fatalf("stale localized season: %s %v", raw, err)
		}
	}
	var notices, history int
	var day string
	if err := e.p.QueryRow(t.Context(), "SELECT count(*) FROM notices WHERE user_id=$1 AND kind='episode_reconciliation'", e.users[0]).Scan(&notices); err != nil || notices != 2 {
		t.Fatal(notices, err)
	}
	if err := e.p.QueryRow(t.Context(), "SELECT count(*),min(watched_date)::text FROM history WHERE user_id=$1", e.users[0]).Scan(&history, &day); err != nil || history != 1 || day != "2026-01-02" {
		t.Fatal(history, day, err)
	}
}

func TestManualTVRefreshForcesSeasonRefreshAndPreservesOutageWarning(t *testing.T) {
	e := refreshRegressionSetup(t, "tmdb:tv:987660")
	audit2Exec(t, e, `INSERT INTO catalog(id,source,source_id,kind,title,metadata) VALUES('tmdb:tv:987660','tmdb','987660','tv','Audit','{"en":{"id":987660,"seasons":[{"season_number":1}]}}')`)
	audit2Exec(t, e, `INSERT INTO season_cache(catalog_id,season,language,metadata) VALUES('tmdb:tv:987660',1,'en','{"episodes":[]}')`)
	var failing atomic.Bool
	var seasonRequests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/season/") {
			seasonRequests.Add(1)
			if failing.Load() {
				http.Error(w, "synthetic outage", http.StatusServiceUnavailable)
				return
			}
			_, _ = fmt.Fprint(w, `{"episodes":[{"id":987661,"episode_number":1,"air_date":"2020-01-01"}]}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"id":987660,"name":"Audit","overview":"Synthetic","seasons":[{"season_number":1}]}`)
	}))
	defer srv.Close()
	provider := catalog.New("synthetic")
	provider.BaseURL = srv.URL
	api := API{DB: e.p, Catalog: &catalog.Cache{DB: e.p, Provider: provider}}
	refresh := func() catalog.Detail {
		t.Helper()
		r := httptest.NewRequestWithContext(t.Context(), "POST", "/api/catalog/tv/987660/refresh?language=en", nil)
		r.SetPathValue("kind", "tv")
		r.SetPathValue("id", "987660")
		w := httptest.NewRecorder()
		api.refresh(w, r)
		var result catalog.Detail
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || w.Code != 200 {
			t.Fatal(w.Code, w.Body.String(), err)
		}
		return result
	}
	if result := refresh(); result.Stale || seasonRequests.Load() != 1 {
		t.Fatal("fresh season cache prevented manual refresh", result, seasonRequests.Load())
	}
	var count int
	if err := e.p.QueryRow(t.Context(), "SELECT count(*) FROM episodes WHERE catalog_id='tmdb:tv:987660' AND NOT retired").Scan(&count); err != nil || count != 1 {
		t.Fatal("manual refresh omitted episodes", count, err)
	}
	failing.Store(true)
	if result := refresh(); !result.Stale || result.Warning == "" || seasonRequests.Load() != 2 {
		t.Fatal("season failure was hidden", result, seasonRequests.Load())
	}
}

func TestSeasonReconciliationDoesNotBlockTrackingForeignKeys(t *testing.T) {
	e := refreshRegressionSetup(t, "tmdb:tv:987670")
	audit2Exec(t, e, `INSERT INTO catalog(id,source,source_id,kind,title,metadata) VALUES('tmdb:tv:987670','tmdb','987670','tv','Audit','{"en":{"id":987670}}')`)
	for n := 1; n <= 2; n++ {
		audit2Exec(t, e, `INSERT INTO episodes(id,catalog_id,source,source_id,season,number) VALUES($1,'tmdb:tv:987670','tmdb',$2,1,$3)`, fmt.Sprintf("tmdb:episode:98767%d", n), fmt.Sprint(987670+n), n)
	}
	audit2Exec(t, e, `INSERT INTO history(user_id,catalog_id,episode_id) VALUES($1,'tmdb:tv:987670','tmdb:episode:987671')`, e.users[0])
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	tx, err := e.p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, "SELECT id FROM users WHERE id=$1 FOR UPDATE", e.users[0]); err != nil {
		t.Fatal(err)
	}
	var pid int
	if err = tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"episodes":[{"id":987671,"episode_number":3},{"id":987672,"episode_number":2}]}`)
	}))
	defer srv.Close()
	provider := catalog.New("synthetic")
	provider.BaseURL = srv.URL
	cache := catalog.Cache{DB: e.p, Provider: provider}
	result := make(chan error, 1)
	go func() { _, refreshErr := cache.Season(ctx, "987670", 1, "en"); result <- refreshErr }()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for blocked := false; !blocked; {
		select {
		case err = <-result:
			t.Fatalf("refresh did not wait for the account transaction: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
			if err = e.p.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))", pid).Scan(&blocked); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO history(user_id,catalog_id,episode_id) VALUES($1,'tmdb:tv:987670','tmdb:episode:987672')`, e.users[0]); err != nil {
		t.Fatal("metadata locks prevented concurrent tracking", err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-result; err != nil {
		t.Fatal(err)
	}
}
