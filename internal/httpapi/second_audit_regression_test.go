package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/zekihan/watchledger/internal/anidb"
	"github.com/zekihan/watchledger/internal/catalog"
	"github.com/zekihan/watchledger/internal/store"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

func audit2Exec(t *testing.T, e auditEnv, q string, args ...any) {
	t.Helper()
	if _, err := e.p.Exec(context.Background(), q, args...); err != nil {
		t.Fatal(err)
	}
}
func audit2ID(raw []byte) string { var v struct{ ID string }; _ = json.Unmarshal(raw, &v); return v.ID }
func audit2Movie(t *testing.T, e auditEnv, id string) {
	t.Helper()
	audit2Exec(t, e, `INSERT INTO catalog(id,source,source_id,kind,title,metadata) VALUES($1,'tmdb',$2,'movie','Audit','{"en":{"title":"Audit"}}')`, "tmdb:movie:"+id, id)
	e.req(t, 0, "PUT", "/api/library/tmdb:movie:"+id, map[string]any{"base": "watching"}, 204)
}

func TestRegressionNativeFallback(t *testing.T) {
	e := audit2Setup(t)
	id := "anidb:anime:991001"
	audit2Exec(t, e, `INSERT INTO catalog(id,source,source_id,kind,title,metadata) VALUES($1,'anidb','991001','anime','Native','{"en":{"number_of_episodes":1}}')`, id)
	// Exactly the episode metadata shape written by anidb.Cache.Detail, not a presentation fixture.
	audit2Exec(t, e, `INSERT INTO episodes(id,catalog_id,source,source_id,season,number,kind,metadata,air_date) VALUES('anidb:episode:991002',$1,'anidb','991002',1,1,'regular','{"name":"Episode","titles":[],"runtime":24,"native_number":"1","native_type":"1"}','2020-01-01')`, id)
	c := anidb.Cache{DB: e.p, Provider: anidb.New(anidb.Config{})}
	v, err := c.Detail(context.Background(), "991001", "en", "translated")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(v.Metadata["episodes"])
	var eps []map[string]any
	_ = json.Unmarshal(raw, &eps)
	if len(eps) != 1 || eps[0]["id"] != float64(991002) || eps[0]["season_number"] != float64(1) {
		t.Fatalf("regression: %s", raw)
	}
	t.Logf("Restored native episode retains presentation identity: %s", raw)
}
func TestRegressionDeletedListRestore(t *testing.T) {
	e := audit2Setup(t)
	audit2Movie(t, e, "991011")
	id := audit2ID(e.req(t, 0, "POST", "/api/lists", map[string]any{"name": "Deleted locally", "kind": "manual", "sort": "title", "members": []string{"tmdb:movie:991011"}}, 200))
	backup := json.RawMessage(e.req(t, 0, "GET", "/api/export", nil, 200))
	e.req(t, 0, "DELETE", "/api/lists/"+id, nil, 204)
	pre := audit2ID(e.req(t, 0, "POST", "/api/restore/preview", backup, 200))
	e.req(t, 0, "POST", "/api/restore/"+pre+"/apply", nil, 200)
	var count int
	if err := e.p.QueryRow(context.Background(), "SELECT count(*) FROM lists WHERE user_id=$1", e.users[0]).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("probe changed count %d", count)
	}
	t.Log("Local list deletion preserved")
}

func TestRegressionCalendarLanguage(t *testing.T) {
	e := audit2Setup(t)
	audit2Exec(t, e, `INSERT INTO catalog(id,source,source_id,kind,title,metadata) VALUES('tmdb:tv:991041','tmdb','991041','tv','English title','{"en":{"name":"English title"},"tr":{"name":"Türkçe başlık"}}')`)
	audit2Exec(t, e, `INSERT INTO episodes(id,catalog_id,source,source_id,season,number,air_date) VALUES('tmdb:episode:991042','tmdb:tv:991041','tmdb','991042',1,1,'2026-10-25')`)
	e.req(t, 0, "PUT", "/api/library/tmdb:tv:991041", map[string]any{"base": "watching"}, 204)
	audit2Exec(t, e, `UPDATE users SET settings='{"language":"tr","metadata_language":"tr"}' WHERE id=$1`, e.users[0])
	lib := e.req(t, 0, "GET", "/api/library", nil, 200)
	cal := e.req(t, 0, "GET", "/api/calendar?month=2026-10", nil, 200)
	if !strings.Contains(string(lib), "Türkçe başlık") || !strings.Contains(string(cal), "Türkçe başlık") {
		t.Fatalf("regression: %s %s", lib, cal)
	}
	t.Log("Calendar honors the account language")
}
func TestRegressionPeopleFilters(t *testing.T) {
	e := audit2Setup(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "tv_credits") {
			_, _ = fmt.Fprint(w, `{"id":1,"cast":[{"id":991051,"vote_average":1},{"id":991052,"vote_average":9}],"crew":[]}`)
			return
		}
		id := 991051
		score := 1
		if strings.HasSuffix(r.URL.Path, "991052") {
			id = 991052
			score = 9
		}
		_, _ = fmt.Fprintf(w, `{"id":%d,"name":"TV","overview":"OK","genres":[{"id":18}],"vote_average":%d,"first_air_date":"2020-01-01"}`, id, score)
	}))
	defer server.Close()
	p := catalog.New("fixture")
	p.BaseURL = server.URL
	c := catalog.Cache{DB: e.p, Provider: p}
	raw, err := c.PeopleTV(context.Background(), "1", url.Values{"without_genres": {"18"}, "sort_by": {"vote_average.desc"}})
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Results []struct {
			ID int `json:"id"`
		}
	}
	_ = json.Unmarshal(raw, &v)
	if len(v.Results) != 0 {
		t.Fatalf("regression: %s", raw)
	}
	raw, err = c.PeopleTV(context.Background(), "1", url.Values{"sort_by": {"vote_average.desc"}})
	if err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(raw, &v)
	if len(v.Results) != 2 || v.Results[0].ID != 991052 {
		t.Fatalf("descending score sort ignored: %s", raw)
	}
}
func TestRegressionMalformedDetailFallback(t *testing.T) {
	e := audit2Setup(t)
	audit2Movie(t, e, "991061")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, `{"id":991061,"title":{}}`) }))
	defer server.Close()
	p := catalog.New("fixture")
	p.BaseURL = server.URL
	c := catalog.Cache{DB: e.p, Provider: p}
	result, err := c.Detail(context.Background(), "movie", "991061", "en", true)
	if err != nil || !result.Stale || result.Warning == "" {
		t.Fatal("cached fallback lost", result, err)
	}
	t.Logf("Malformed provider response preserves stale cache: %v", err)
}
func TestRegressionTimezoneLocal(t *testing.T) {
	e := audit2Setup(t)
	audit2Movie(t, e, "991071")
	e.req(t, 0, "PUT", "/api/settings", map[string]any{"language": "en", "metadata_language": "en", "country": "TR", "timezone": "Local", "anime_view": "native", "anime_title": "romaji"}, 400)
	// Go accepts Local, PostgreSQL and browser Intl do not share that pseudo-zone.
	result := e.req(t, 0, "GET", "/api/dashboard", nil, 200)
	t.Logf("Invalid timezone rejected, dashboard remains usable: %s", result)
}

func TestRegressionStaleStatusRule(t *testing.T) {
	e := audit2Setup(t)
	audit2Movie(t, e, "991081")
	sid := audit2ID(e.req(t, 0, "POST", "/api/statuses", map[string]any{"Name": "Old custom", "Base": "watching"}, 200))
	// Another tab deletes the status after the editor fetched it, before the first save.
	e.req(t, 0, "DELETE", "/api/statuses/"+sid, map[string]any{"replacement": "watching"}, 204)
	rule := map[string]any{"op": "eq", "field": "status", "value": sid}
	e.req(t, 0, "POST", "/api/lists", map[string]any{"name": "Stale editor", "kind": "smart", "sort": "title", "rule": rule, "members": []string{}}, 409)
	e.req(t, 0, "POST", "/api/lists/preview", map[string]any{"rule": rule, "sort": "title"}, 409)
}

func audit2Setup(t *testing.T) (e auditEnv) {
	t.Cleanup(func() {
		if e.p == nil {
			return
		}
		// auditSetup closes its pool before this outer cleanup; use a new connection.
		p, err := store.Open(context.Background(), os.Getenv("TEST_DATABASE_URL"))
		if err != nil {
			return
		}
		defer p.Close()
		_, _ = p.Exec(context.Background(), "DELETE FROM episodes WHERE catalog_id IN ('anidb:anime:991001','tmdb:tv:991041')")
		_, _ = p.Exec(context.Background(), "DELETE FROM catalog_localizations WHERE catalog_id IN ('tmdb:tv:991051','tmdb:tv:991052')")
		_, _ = p.Exec(context.Background(), "DELETE FROM catalog WHERE id=ANY($1::text[])", []string{"anidb:anime:991001", "tmdb:movie:991011", "tmdb:tv:991041", "tmdb:tv:991051", "tmdb:tv:991052", "tmdb:movie:991061", "tmdb:movie:991071", "tmdb:movie:991081", "tmdb:movie:991111"})
	})
	return auditSetup(t)
}

func TestSignedDecimalBackupRestoresMissingCache(t *testing.T) {
	e := audit2Setup(t)
	audit2Movie(t, e, "991111")
	audit2Exec(t, e, `UPDATE catalog SET metadata='{"en":{"id":991111,"title":"Audit","vote_average":8.0}}' WHERE id='tmdb:movie:991111'`)
	backup := json.RawMessage(e.req(t, 0, "GET", "/api/export", nil, 200))
	if !strings.Contains(string(backup), "8.0") {
		t.Fatal("fixture lost decimal encoding")
	}
	audit2Exec(t, e, "DELETE FROM library WHERE user_id=$1", e.users[0])
	audit2Exec(t, e, "DELETE FROM catalog WHERE id='tmdb:movie:991111'")
	raw := e.req(t, 0, "POST", "/api/restore/preview", backup, 200)
	var pre struct {
		ID       string
		Approval bool `json:"shared_metadata_requires_approval"`
	}
	if err := json.Unmarshal(raw, &pre); err != nil {
		t.Fatal(err)
	}
	if pre.Approval {
		t.Fatal("own unchanged export requires admin")
	}
	e.req(t, 0, "POST", "/api/restore/"+pre.ID+"/apply", nil, 200)
}

func TestListDeletionRoundTripAndDependentRules(t *testing.T) {
	e := audit2Setup(t)
	manual := audit2ID(e.req(t, 0, "POST", "/api/lists", map[string]any{"name": "Manual", "kind": "manual", "sort": "title", "members": []string{}}, 200))
	dependent := audit2ID(e.req(t, 0, "POST", "/api/lists", map[string]any{"name": "Dependent", "kind": "smart", "sort": "title", "rule": map[string]any{"op": "eq", "field": "list", "value": manual}}, 200))
	old := json.RawMessage(e.req(t, 0, "GET", "/api/export", nil, 200))
	e.req(t, 0, "DELETE", "/api/lists/"+dependent, nil, 204)
	e.req(t, 0, "DELETE", "/api/lists/"+manual, nil, 204)
	deletionBackup := json.RawMessage(e.req(t, 0, "GET", "/api/export", nil, 200))
	// A fresh account restores deletion facts, then an older backup with both lists.
	e.req(t, 1, "GET", "/api/export", nil, 200)
	pre := audit2ID(e.req(t, 1, "POST", "/api/restore/preview", deletionBackup, 200))
	e.req(t, 1, "POST", "/api/restore/"+pre+"/apply", nil, 200)
	raw := e.req(t, 1, "POST", "/api/restore/preview", old, 200)
	if !strings.Contains(string(raw), "Manual") || !strings.Contains(string(raw), "Dependent") {
		t.Fatalf("deletion preview missing: %s", raw)
	}
	e.req(t, 1, "POST", "/api/restore/"+audit2ID(raw)+"/apply", nil, 200)
	if raw = e.req(t, 1, "GET", "/api/lists", nil, 200); strings.TrimSpace(string(raw)) != "[]" {
		t.Fatalf("deleted lists resurrected: %s", raw)
	}
	// New dependent rules on a deleted list are skipped rather than becoming dangling.
	var snapshot map[string]any
	_ = json.Unmarshal(old, &snapshot)
	lists := snapshot["lists"].([]any)
	for _, value := range lists {
		list := value.(map[string]any)
		if list["name"] == "Dependent" {
			list["id"] = "new-dependent"
			list["name"] = "New dependent"
		}
	}
	raw = e.req(t, 1, "POST", "/api/restore/preview", snapshot, 200)
	if !strings.Contains(string(raw), "New dependent") {
		t.Fatal("transitive skip not previewed")
	}
	e.req(t, 1, "POST", "/api/restore/"+audit2ID(raw)+"/apply", nil, 200)
}
