package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/zekihan/watchledger/internal/catalog"
	"github.com/zekihan/watchledger/internal/portable"
	"github.com/zekihan/watchledger/internal/store"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func thirdMovie(t *testing.T, e auditEnv, id string) {
	audit2Exec(t, e, `INSERT INTO catalog(id,source,source_id,kind,title,metadata) VALUES($1,'tmdb',$2,'movie','Synthetic',jsonb_build_object('en',jsonb_build_object('id',$2::text::bigint,'title','Synthetic','genres','[]'::jsonb)))`, "tmdb:movie:"+id, id)
	e.req(t, 0, "PUT", "/api/library/tmdb:movie:"+id, map[string]any{"base": "watching"}, 204)
}
func TestThirdRegressionStatusNameCollision(t *testing.T) {
	e := thirdSetup(t)
	ids := []string{}
	for _, u := range []int{0, 1} {
		ids = append(ids, audit2ID(e.req(t, u, "POST", "/api/statuses", map[string]any{"Name": "My watching", "Base": "watching"}, 200)))
	}
	thirdMovie(t, e, "992003")
	e.req(t, 0, "PATCH", "/api/library/tmdb:movie:992003", map[string]any{"status_id": ids[0]}, 204)
	backup := json.RawMessage(e.req(t, 0, "GET", "/api/export", nil, 200))
	e.req(t, 1, "GET", "/api/export", nil, 200)
	p := audit2ID(e.req(t, 1, "POST", "/api/restore/preview", backup, 200))
	r := e.req(t, 1, "POST", "/api/restore/"+p+"/apply", nil, 200)
	var actual string
	if err := e.p.QueryRow(context.Background(), "SELECT status_id::text FROM library WHERE user_id=$1 AND catalog_id='tmdb:movie:992003'", e.users[1]).Scan(&actual); err != nil || actual != ids[1] {
		t.Fatal(actual, err)
	}
	t.Logf("Compatible status name and incoming reference reused: %s", r)
}
func TestThirdRegressionSmartKnownEmpty(t *testing.T) {
	e := thirdSetup(t)
	thirdMovie(t, e, "992001")
	audit2Exec(t, e, `INSERT INTO availability(catalog_id,country,offers) VALUES('tmdb:movie:992001','TR','null')`)
	for _, field := range []string{"service", "genre"} {
		value := "8"
		if field == "genre" {
			value = "18"
		}
		rule := map[string]any{"op": "not", "children": []any{map[string]any{"op": "eq", "field": field, "value": value}}}
		raw := e.req(t, 0, "POST", "/api/lists/preview", map[string]any{"rule": rule, "sort": "title"}, 200)
		var v struct{ Count int }
		_ = json.Unmarshal(raw, &v)
		if v.Count != 1 {
			t.Fatalf("regression: %s", raw)
		}
		t.Logf("Known-empty %s included in negative condition: %s", field, raw)
	}
}
func TestThirdRegressionFreeAdsOffers(t *testing.T) {
	e := thirdSetup(t)
	thirdMovie(t, e, "992002")
	audit2Exec(t, e, `INSERT INTO availability(catalog_id,country,offers) VALUES('tmdb:movie:992002','TR','{"free":[{"provider_id":8}]}')`)
	raw := e.req(t, 0, "POST", "/api/lists/preview", map[string]any{"rule": map[string]any{"op": "eq", "field": "service", "value": "8"}}, 200)
	var v struct{ Count int }
	_ = json.Unmarshal(raw, &v)
	if v.Count != 1 {
		t.Fatalf("regression: %s", raw)
	}
	t.Logf("Free offer included in service filter: %s", raw)
}
func TestThirdRegressionAvailabilityMalformed(t *testing.T) {
	e := thirdSetup(t)
	thirdMovie(t, e, "992004")
	audit2Exec(t, e, `INSERT INTO availability(catalog_id,country,offers) VALUES('tmdb:movie:992004','TR','{"flatrate":[{"provider_id":8}]}')`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, `{}`) }))
	defer srv.Close()
	p := catalog.New("fixture")
	p.BaseURL = srv.URL
	c := catalog.Cache{DB: e.p, Provider: p}
	v, err := c.Availability(context.Background(), "movie", "992004", "TR", true)
	if err != nil || !strings.Contains(string(v.Offers), "8") || !v.Stale || v.Warning == "" {
		t.Fatalf("regression %+v %v", v, err)
	}
	t.Log("Malformed availability retains cached offers with warning")
}
func TestThirdRegressionRetiredSeasonFallback(t *testing.T) {
	e := thirdSetup(t)
	audit2Exec(t, e, `INSERT INTO catalog(id,source,source_id,kind,title,metadata) VALUES('tmdb:tv:992005','tmdb','992005','tv','TV','{"en":{"id":992005,"name":"TV"}}')`)
	audit2Exec(t, e, `INSERT INTO episodes(id,catalog_id,source,source_id,season,number,retired,metadata) VALUES('tmdb:episode:992005','tmdb:tv:992005','tmdb','992005',1,1,true,'{"id":992005,"episode_number":1,"name":"Retired English"}')`)
	audit2Exec(t, e, `INSERT INTO episodes(id,catalog_id,source,source_id,season,number,metadata) VALUES('tmdb:episode:992008','tmdb:tv:992005','tmdb','992008',1,2,'{"id":992008,"episode_number":2,"name":"Active"}')`)
	c := catalog.Cache{DB: e.p, Provider: catalog.New("")}
	raw, err := c.Season(context.Background(), "992005", 1, "tr")
	if err != nil || strings.Contains(string(raw), "Retired English") || !strings.Contains(string(raw), "Active") {
		t.Fatalf("regression %s %v", raw, err)
	}
	t.Logf("Retired episode excluded from outage fallback: %s", raw)
}
func TestThirdRegressionConcurrentEdits(t *testing.T) {
	e := thirdSetup(t)
	thirdMovie(t, e, "992006")
	e.req(t, 0, "PUT", "/api/library/tmdb:movie:992006", map[string]any{"base": "watching", "rating": 3}, 204)
	e.req(t, 0, "PUT", "/api/library/tmdb:movie:992006", map[string]any{"base": "watching", "rating": 8}, 204)
	e.req(t, 0, "PATCH", "/api/library/tmdb:movie:992006", map[string]any{"base": "hold", "status_id": nil}, 204)
	var rating int
	err := e.p.QueryRow(context.Background(), "SELECT rating FROM library WHERE user_id=$1 AND catalog_id='tmdb:movie:992006'", e.users[0]).Scan(&rating)
	if err != nil || rating != 8 {
		t.Fatal(rating, err)
	}
	t.Log("Status-only patch preserves other tab rating8")
}

func thirdSetup(t *testing.T) (e auditEnv) {
	t.Cleanup(func() {
		if os.Getenv("TEST_DATABASE_URL") == "" {
			return
		}
		p, err := store.Open(context.Background(), os.Getenv("TEST_DATABASE_URL"))
		if err != nil {
			return
		}
		defer p.Close()
		for _, table := range []string{"availability", "catalog_localizations", "season_cache"} {
			_, _ = p.Exec(context.Background(), "DELETE FROM "+table+" WHERE catalog_id IN ('tmdb:movie:992001','tmdb:movie:992002','tmdb:movie:992003','tmdb:movie:992004','tmdb:tv:992005','tmdb:movie:992006','tmdb:movie:992007')")
		}
		_, _ = p.Exec(context.Background(), "DELETE FROM episodes WHERE source_id IN ('992005','992008')")
		_, _ = p.Exec(context.Background(), "DELETE FROM catalog WHERE source_id=ANY($1::text[])", []string{"992001", "992002", "992003", "992004", "992005", "992006", "992007"})
	})
	return auditSetup(t)
}
func TestThirdRegressionStatusBaseCollisionPreview(t *testing.T) {
	e := thirdSetup(t)
	e.req(t, 0, "POST", "/api/statuses", map[string]any{"Name": "Collision", "Base": "watching"}, 200)
	e.req(t, 1, "POST", "/api/statuses", map[string]any{"Name": "Collision", "Base": "hold"}, 200)
	backup := json.RawMessage(e.req(t, 0, "GET", "/api/export", nil, 200))
	e.req(t, 1, "GET", "/api/export", nil, 200)
	raw := e.req(t, 1, "POST", "/api/restore/preview", backup, 200)
	var v struct {
		ID        string
		Conflicts []string `json:"status_conflicts"`
	}
	_ = json.Unmarshal(raw, &v)
	if len(v.Conflicts) != 1 {
		t.Fatal(string(raw))
	}
	e.req(t, 1, "POST", "/api/restore/"+v.ID+"/apply", nil, 409)
}
func TestThirdRegressionPatchPrivacyAndNull(t *testing.T) {
	e := thirdSetup(t)
	thirdMovie(t, e, "992007")
	e.req(t, 0, "PATCH", "/api/library/tmdb:movie:992007", map[string]any{"rating": 8}, 204)
	e.req(t, 1, "PATCH", "/api/library/tmdb:movie:992007", map[string]any{"rating": 1}, 404)
	e.req(t, 0, "PATCH", "/api/library/tmdb:movie:992007", map[string]any{"rating": nil}, 204)
	e.req(t, 0, "PATCH", "/api/library/tmdb:movie:992007", map[string]any{"rating": 11}, 400)
	e.req(t, 0, "PATCH", "/api/library/tmdb:movie:992007", map[string]any{"unexpected": true}, 400)
	var rating *int
	if err := e.p.QueryRow(context.Background(), "SELECT rating FROM library WHERE user_id=$1 AND catalog_id='tmdb:movie:992007'", e.users[0]).Scan(&rating); err != nil || rating != nil {
		t.Fatal(rating, err)
	}
}
func TestThirdRegressionCompactLibraryAndBulkPreview(t *testing.T) {
	e := thirdSetup(t)
	thirdMovie(t, e, "992003")
	audit2Exec(t, e, `UPDATE catalog SET metadata=jsonb_build_object('en',jsonb_build_object('title','Synthetic','poster_path','/poster.jpg','genres','[]'::jsonb,'overview',repeat('x',100000),'credits',jsonb_build_object('cast',jsonb_build_array(1,2,3)))) WHERE id='tmdb:movie:992003'`)
	raw := e.req(t, 0, "GET", "/api/library", nil, 200)
	if len(raw) > 2000 || strings.Contains(string(raw), "credits") || !strings.Contains(string(raw), "/poster.jpg") {
		t.Fatalf("not compact: %d", len(raw))
	}
	entries := make([]portable.Library, 10000)
	for i := range entries {
		entries[i].CatalogID = fmt.Sprintf("tmdb:movie:%d", i)
	}
	entries[0].CatalogID = "tmdb:movie:992003"
	start := time.Now()
	counter := &countedRestoreReader{queryReader: e.p}
	conflicts, adds, err := restoreLibrarySummary(context.Background(), counter, e.users[0], entries)
	if err != nil || len(conflicts) != 1 || adds != 9999 || counter.calls != 1 {
		t.Fatal(len(conflicts), adds, err)
	}
	t.Logf("10,000 candidate set lookup: %s", time.Since(start))
	other, adds, err := restoreLibrarySummary(context.Background(), e.p, e.users[1], entries)
	if err != nil || len(other) != 0 || adds != 10000 {
		t.Fatal(other, adds, err)
	}
}
func TestThirdRegressionRequestBudgets(t *testing.T) {
	cancelled := make(chan struct{})
	h := requestBudget(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done(); close(cancelled) }), time.Millisecond, 50*time.Millisecond)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), "GET", "/api/library", nil))
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
	<-cancelled
	h = requestBudget(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(10 * time.Millisecond):
			w.WriteHeader(204)
		case <-r.Context().Done():
		}
	}), time.Millisecond, time.Second)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), "POST", "/api/restore/preview", nil))
	if w.Code != 204 {
		t.Fatal(w.Code)
	}
}

func TestThirdRegressionRestorePreservesRenamedStatus(t *testing.T) {
	e := thirdSetup(t)
	id := audit2ID(e.req(t, 0, "POST", "/api/statuses", map[string]any{"Name": "Original name", "Base": "watching"}, 200))
	backup := json.RawMessage(e.req(t, 0, "GET", "/api/export", nil, 200))
	e.req(t, 0, "POST", "/api/statuses", map[string]any{"ID": id, "Name": "Renamed locally", "Base": "watching"}, 200)
	e.req(t, 0, "POST", "/api/statuses", map[string]any{"Name": "Original name", "Base": "hold"}, 200)
	pre := audit2ID(e.req(t, 0, "POST", "/api/restore/preview", backup, 200))
	e.req(t, 0, "POST", "/api/restore/"+pre+"/apply", nil, 200)
	var name string
	if err := e.p.QueryRow(context.Background(), "SELECT name FROM statuses WHERE id=$1", id).Scan(&name); err != nil || name != "Renamed locally" {
		t.Fatal(name, err)
	}
}

type countedRestoreReader struct {
	queryReader
	calls int
}

func (c *countedRestoreReader) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	c.calls++
	return c.queryReader.Query(ctx, sql, args...)
}
