package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/zekihan/watchledger/internal/auth"
	"github.com/zekihan/watchledger/internal/config"
	"github.com/zekihan/watchledger/internal/store"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSIMKLEpisodeResolutionPreservesIdentity(t *testing.T) {
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
	id := "tmdb:tv:" + suffix
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
	raw := fmt.Sprintf(`{"shows":[{"show":{"title":"Synthetic gaps","ids":{"simkl":123,"tmdb":%q}},"status":"watching","seasons":[{"number":1,"episodes":[{"number":1,"watched_at":"2025-02-03"},{"number":3}]}]}]}`, suffix)
	w := request("POST", "/api/simkl/preview", raw, true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var preview struct {
		ID string `json:"id"`
	}
	if e = json.Unmarshal(w.Body.Bytes(), &preview); e != nil {
		t.Fatal(e)
	}
	if w = request("GET", "/api/export", "", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = request("POST", "/api/simkl/"+preview.ID+"/apply", "{}", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var reviewID string
	if e = p.QueryRow(ctx, "SELECT id FROM import_reviews WHERE user_id=$1 AND import_id=$2", uid, preview.ID).Scan(&reviewID); e != nil {
		t.Fatal(e)
	}
	route := "/api/import-reviews/" + reviewID
	if w = request("GET", route+"/candidates", "", true); w.Code != 200 || !strings.Contains(w.Body.String(), "shows:123:s1:e3") {
		t.Fatal(w.Code, w.Body.String())
	}
	correct := fmt.Sprintf(`{"confirm":true,"accept_existing":true,"links":[{"source_key":"shows:123:s1:e1","episode_id":%q},{"source_key":"shows:123:s1:e3","episode_id":%q}]}`, id+"1", id+"2")
	for _, bad := range []string{strings.Replace(correct, `"confirm":true`, `"confirm":false`, 1), strings.Replace(correct, id+"2", id+"1", 1), strings.Replace(correct, "shows:123:s1:e3", "shows:123:s1:e9", 1)} {
		if w = request("POST", route+"/resolve", bad, true); w.Code != 400 {
			t.Fatal("invalid mapping accepted", w.Code, w.Body.String())
		}
	}
	bad := strings.Replace(correct, id+"2", "foreign-episode", 1)
	if w = request("POST", route+"/resolve", bad, true); w.Code != 409 {
		t.Fatal("foreign episode accepted", w.Code, w.Body.String())
	}
	var count int
	if e = p.QueryRow(ctx, "SELECT count(*) FROM simkl_episode_links WHERE user_id=$1", uid).Scan(&count); e != nil || count != 0 {
		t.Fatal("failed resolution partially applied", count, e)
	}
	for n := 0; n < 2; n++ {
		if w = request("POST", route+"/resolve", correct, true); w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM history WHERE user_id=$1", uid).Scan(&count); e != nil || count != 2 {
		t.Fatal("history duplication", count, e)
	}
	var day *string
	if e = p.QueryRow(ctx, "SELECT watched_date::text FROM history WHERE user_id=$1 AND episode_id=$2", uid, id+"1").Scan(&day); e != nil || day == nil || *day != "2025-02-03" {
		t.Fatal("precise date lost", e)
	}
	if e = p.QueryRow(ctx, "SELECT watched_date::text FROM history WHERE user_id=$1 AND episode_id=$2", uid, id+"2").Scan(&day); e != nil || day != nil {
		t.Fatal("unknown date fabricated", e)
	}

	// A concurrent tracking request must wait for the same per-user lock held by
	// import/restore. Observe actual PostgreSQL blocking, not an arbitrary sleep.
	lockTx, e := p.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = lockTx.Rollback(ctx) }()
	if _, e = lockTx.Exec(ctx, "SELECT id FROM users WHERE id=$1 FOR UPDATE", uid); e != nil {
		t.Fatal(e)
	}
	var holderPID int
	if e = lockTx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&holderPID); e != nil {
		t.Fatal(e)
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- request("POST", "/api/library/"+id+"/track", fmt.Sprintf(`{"watched":false,"episode_id":%q,"date":{}}`, id+"1"), true)
	}()
	deadline := time.After(3 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	blocked := false
	for !blocked {
		select {
		case response := <-done:
			t.Fatalf("tracking bypassed import lock: %d", response.Code)
		case <-deadline:
			t.Fatal("tracking did not reach account lock")
		case <-ticker.C:
			e = p.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))", holderPID).Scan(&blocked)
			if e != nil {
				t.Fatal(e)
			}
		}
	}
	if e = lockTx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	select {
	case w = <-done:
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	case <-time.After(3 * time.Second):
		t.Fatal("tracking stayed blocked after import commit")
	}

	if w = request("POST", "/api/simkl/preview", raw, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = request("POST", "/api/simkl/"+preview.ID+"/apply", "{}", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	swapped := fmt.Sprintf(`{"confirm":true,"accept_existing":true,"links":[{"source_key":"shows:123:s1:e1","episode_id":%q},{"source_key":"shows:123:s1:e3","episode_id":%q}]}`, id+"2", id+"1")
	if w = request("POST", route+"/resolve", swapped, true); w.Code != 409 {
		t.Fatal("identity silently moved", w.Code, w.Body.String())
	}
	if w = request("POST", route+"/resolve", correct, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM history WHERE user_id=$1", uid).Scan(&count); e != nil || count != 1 {
		t.Fatal("unwatch resurrected", count, e)
	}
	// Confirmed source identities and tombstones survive portable restoration.
	w = request("GET", "/api/export", "", true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "simkl_links") {
		t.Fatal("missing portable identities", w.Code)
	}
	snapshot := w.Body.String()
	if _, e = p.Exec(ctx, "UPDATE sessions SET user_id=$2 WHERE token_hash=$1", sum, other); e != nil {
		t.Fatal(e)
	}
	if w = request("GET", route+"/candidates", "", true); w.Code != 404 {
		t.Fatal("cross-account candidates", w.Code)
	}
	if w = request("POST", route+"/resolve", correct, true); w.Code != 404 {
		t.Fatal("cross-account resolve", w.Code)
	}
	if w = request("GET", "/api/export", "", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request("POST", "/api/restore/preview", snapshot, true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if e = json.Unmarshal(w.Body.Bytes(), &preview); e != nil {
		t.Fatal(e)
	}
	if w = request("POST", "/api/restore/"+preview.ID+"/apply", "{}", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM simkl_episode_links WHERE user_id=$1", other).Scan(&count); e != nil || count != 2 {
		t.Fatal("portable identity loss", count, e)
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM history_tombstones WHERE user_id=$1", other).Scan(&count); e != nil || count != 1 {
		t.Fatal("portable unwatch loss", count, e)
	}
}
