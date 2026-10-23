package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/zekihan/watchledger/internal/auth"
	"github.com/zekihan/watchledger/internal/config"
	"github.com/zekihan/watchledger/internal/portable"
	"github.com/zekihan/watchledger/internal/store"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMovieAliasesTrackOneWorkWithoutEpisodeCopies(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	ctx := context.Background()
	p, e := store.Open(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	if e = store.Migrate(ctx, p); e != nil {
		t.Fatal(e)
	}
	var issuer string
	oidc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"issuer": issuer, "authorization_endpoint": issuer + "/authorize", "token_endpoint": issuer + "/token", "jwks_uri": issuer + "/keys", "id_token_signing_alg_values_supported": []string{"RS256"}})
	}))
	defer oidc.Close()
	issuer = oidc.URL
	identity, e := auth.New(ctx, p, config.Config{Issuer: issuer, PublicURL: "http://127.0.0.1:8080", ClientID: "test", SessionKey: base64.StdEncoding.EncodeToString(make([]byte, 32))})
	if e != nil {
		t.Fatal(e)
	}
	suffix := fmt.Sprint(time.Now().UnixNano())
	var uid, other string
	for i, dest := range []*string{&uid, &other} {
		if e = p.QueryRow(ctx, "INSERT INTO users(issuer,subject) VALUES('test',$1) RETURNING id", suffix+fmt.Sprint(i)).Scan(dest); e != nil {
			t.Fatal(e)
		}
		defer func(id string) { _, _ = p.Exec(ctx, "DELETE FROM users WHERE id=$1", id) }(*dest)
	}
	sid := "synthetic-" + suffix
	sum := fmt.Sprintf("%x", sha256.Sum256([]byte(sid)))
	if _, e = p.Exec(ctx, "INSERT INTO sessions(token_hash,user_id,token,expires_at) VALUES($1,$2,'{}',now()+interval '1 hour')", sum, uid); e != nil {
		t.Fatal(e)
	}
	id := "fixture:" + suffix
	if _, e = p.Exec(ctx, `INSERT INTO catalog(id,source,source_id,kind,title,metadata) VALUES($1,'fixture',$2,'tv','Synthetic','{"":{"status":"Ended","number_of_episodes":2}}')`, id, suffix); e != nil {
		t.Fatal(e)
	}
	for n := 1; n <= 2; n++ {
		if _, e = p.Exec(ctx, `INSERT INTO episodes(id,catalog_id,source,source_id,season,number,air_date) VALUES($1,$2,'fixture',$1,1,$3,'2020-01-01')`, id+fmt.Sprint(n), id, n); e != nil {
			t.Fatal(e)
		}
	}
	handler := NewApplication("", p, identity, "")
	request := func(method, path, body string, authenticated bool) *httptest.ResponseRecorder {
		r := httptest.NewRequestWithContext(ctx, method, path, strings.NewReader(body))
		r.Header.Set("Origin", "http://127.0.0.1:8080")
		if authenticated {
			r.AddCookie(&http.Cookie{Name: "wl_session", Value: sid})
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}

	exec := func(query string, args ...interface{}) {
		t.Helper()
		if _, err := p.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("UPDATE users SET admin=true WHERE id=$1", uid)
	exec(`UPDATE catalog SET source='anidb',kind='anime',metadata='{"":{"status":"Ended","number_of_episodes":2,"anime_type":"Movie"}}' WHERE id=$1`, id)
	exec(`UPDATE episodes SET source='anidb',metadata='{"runtime":45}' WHERE catalog_id=$1`, id)
	exec("INSERT INTO library(user_id,catalog_id,base,rating) VALUES($1,$2,'watching',8)", uid, id)
	movieA, movieB := "tmdb:movie:"+suffix, "tmdb:movie:"+suffix+"2"
	for _, movie := range []string{movieA, movieB} {
		exec(`INSERT INTO catalog(id,source,source_id,kind,title,metadata) VALUES($1,'tmdb',$1,'movie','Synthetic movie alias','{"":{"runtime":90}}')`, movie)
	}
	saved := request("POST", "/api/admin/mappings", fmt.Sprintf(`{"kind":"movie-aliases","relationships":[{"from":%q,"to":%q},{"from":%q,"to":%q},{"from":%q,"to":%q},{"from":%q,"to":%q}],"provenance":{"source":"local-admin-override","version":"1"}}`, id+"1", movieA, id+"1", movieB, id+"2", movieA, id+"2", movieB), true)
	if saved.Code != 200 {
		t.Fatal(saved.Code, saved.Body.String())
	}
	var definition struct{ ID, Revision string }
	if err := json.Unmarshal(saved.Body.Bytes(), &definition); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = p.Exec(ctx, "DELETE FROM mapping_consents WHERE mapping_id=$1", definition.ID)
		_, _ = p.Exec(ctx, "DELETE FROM mapping_revision_history WHERE mapping_id=$1", definition.ID)
		_, _ = p.Exec(ctx, "DELETE FROM mappings WHERE id=$1", definition.ID)
	}()
	watch := func(movie string, watched bool, extra string) *httptest.ResponseRecorder {
		return request("POST", "/api/library/"+movie+"/track", fmt.Sprintf(`{"view_anchor":%q,"whole":true,"watched":%t,"date":{"day":"2026-03-04"}%s}`, id, watched, extra), true)
	}
	dashboard := request("GET", "/api/dashboard", "", true)
	var home struct {
		Next []nextCard `json:"up_next"`
	}
	if dashboard.Code != 200 {
		t.Fatal(dashboard.Code, dashboard.Body.String())
	}
	if err := json.Unmarshal(dashboard.Body.Bytes(), &home); err != nil {
		t.Fatal(err)
	}
	if len(home.Next) != 0 {
		t.Fatal("native movie advertised as a next TV episode", home.Next)
	}
	if w := watch(movieA, true, ""); w.Code != 409 {
		t.Fatal("movie mapping bypassed consent", w.Code, w.Body.String())
	}
	if w := request("POST", "/api/mappings/"+definition.ID+"/confirm", fmt.Sprintf(`{"revision":%q}`, definition.Revision), true); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	part := request("POST", "/api/library/"+id+"/track", fmt.Sprintf(`{"episode_id":%q,"watched":true,"date":{"day":"2026-03-01"}}`, id+"1"), true)
	if part.Code != 200 {
		t.Fatal(part.Code, part.Body.String())
	}
	partial, err := loadEffective(ctx, p, uid, id)
	if err != nil || partial.States[movieA].Derived || partial.States[id+"2"].Derived {
		t.Fatal("one native part completed the movie", partial, err)
	}
	partialStats, err := buildStatistics(ctx, p, uid, "UTC")
	if err != nil || partialStats.Movies != 0 || partialStats.Episodes != 0 || partialStats.PartialMovies != 1 {
		t.Fatal("partial film inflated totals", partialStats, err)
	}
	if w := watch(movieA, true, ""); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := watch(movieB, true, ""); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var count int
	var base string
	if err := p.QueryRow(ctx, "SELECT count(*) FROM history WHERE user_id=$1", uid).Scan(&count); err != nil || count != 2 {
		t.Fatal("copied alias history", count, err)
	}
	if err := p.QueryRow(ctx, "SELECT base FROM library WHERE user_id=$1 AND catalog_id=$2", uid, id).Scan(&base); err != nil || base != "completed" {
		t.Fatal("native movie progress", base, err)
	}
	var stats statisticsResult
	w := request("GET", "/api/statistics", "", true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatal(err)
	}
	if stats.Movies != 1 || stats.Episodes != 0 || stats.EstimatedMinutes != 90 {
		t.Fatal("movie aliases inflated statistics", stats)
	}
	exported := request("GET", "/api/export", "", true)
	var backup portable.Snapshot
	if err := json.Unmarshal(exported.Body.Bytes(), &backup); err != nil {
		t.Fatal(err)
	}
	if err := backup.Validate(); err != nil {
		t.Fatal("movie alias export", err)
	}
	movieEvents := 0
	for _, event := range backup.History {
		if event.EpisodeID == nil {
			movieEvents++
		}
	}
	if len(backup.History) != 2 || movieEvents != 1 || len(backup.Library) != 1 {
		t.Fatal("movie manufactured episodes/library", backup.History, backup.Library)
	}
	preview := request("POST", "/api/library/"+id+"/track", fmt.Sprintf(`{"episode_id":%q,"watched":false,"preview":true,"date":{}}`, id+"1"), true)
	var plan struct {
		Token   string         `json:"linked_plan"`
		Changes []recordedLink `json:"linked_changes"`
	}
	if err := json.Unmarshal(preview.Body.Bytes(), &plan); err != nil || len(plan.Changes) != 1 || !plan.Changes[0].Movie {
		t.Fatal("movie source removal not disclosed", preview.Code, preview.Body.String(), err)
	}
	if w := request("POST", "/api/library/"+id+"/track", fmt.Sprintf(`{"episode_id":%q,"watched":false,"date":{}}`, id+"1"), true); w.Code != 409 {
		t.Fatal("movie history removed without confirmation", w.Code)
	}
	w = request("POST", "/api/library/"+id+"/track", fmt.Sprintf(`{"episode_id":%q,"watched":false,"date":{},"include_linked":true,"linked_plan":%q,"return_watching":true}`, id+"1", plan.Token), true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := p.QueryRow(ctx, "SELECT count(*) FROM history WHERE user_id=$1", uid).Scan(&count); err != nil || count != 0 {
		t.Fatal("NULL episode movie record survived linked unwatch", count, err)
	}
	// An older backup containing a native part must not resurrect a film
	// explicitly unwatched through its alternate identity.
	oldBackup := backup
	part2 := id + "2"
	oldDay := "2026-02-01"
	oldBackup.History = append(oldBackup.History, portable.History{CatalogID: id, EpisodeID: &part2, WatchedDate: &oldDay, Provenance: json.RawMessage(`{"source":"synthetic-old-backup"}`), CreatedAt: time.Now().Add(-24 * time.Hour), UpdatedAt: time.Now().Add(-24 * time.Hour)})
	oldRaw, _ := json.Marshal(oldBackup)
	oldPreview := request("POST", "/api/restore/preview", string(oldRaw), true)
	var oldImport struct{ ID string }
	if oldPreview.Code != 200 {
		t.Fatal(oldPreview.Code, oldPreview.Body.String())
	}
	if err = json.Unmarshal(oldPreview.Body.Bytes(), &oldImport); err != nil {
		t.Fatal(err)
	}
	if w := request("POST", "/api/restore/"+oldImport.ID+"/apply", "{}", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err = p.QueryRow(ctx, "SELECT count(*) FROM history WHERE user_id=$1", uid).Scan(&count); err != nil || count != 0 {
		t.Fatal("old native part resurrected an explicit alternate unwatch", count, err)
	}
	if w := watch(movieB, true, ""); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	effective, err := loadEffective(ctx, p, uid, id)
	if err != nil || !effective.States[id+"1"].Derived {
		t.Fatal("new alias action did not supersede old unwatch", effective, err)
	}
	exec("UPDATE sessions SET user_id=$2 WHERE token_hash=$1", sum, other)
	if w := request("GET", "/api/export", "", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	restore := request("POST", "/api/restore/preview", exported.Body.String(), true)
	var imp struct{ ID string }
	if restore.Code != 200 {
		t.Fatal(restore.Code, restore.Body.String())
	}
	if err = json.Unmarshal(restore.Body.Bytes(), &imp); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if w := request("POST", "/api/restore/"+imp.ID+"/apply", "{}", true); w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if err = p.QueryRow(ctx, "SELECT count(*) FROM history WHERE user_id=$1 AND episode_id IS NULL", other).Scan(&count); err != nil || count != 1 {
		t.Fatal("movie restore duplicated event", count, err)
	}
	effective, err = loadEffective(ctx, p, other, id)
	if err != nil || effective.States[id+"1"].Derived {
		t.Fatal("restore inherited movie mapping consent", effective, err)
	}
}
