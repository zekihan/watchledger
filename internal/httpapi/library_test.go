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

func TestPrivateTrackingAndStatusLocks(t *testing.T) {
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
	if w := request("GET", "/api/library", "", false); w.Code != 401 {
		t.Fatal("unauthenticated read", w.Code)
	}
	if w := request("PUT", "/api/library/"+id, `{"base":"watching"}`, true); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("POST", "/api/library/"+id+"/track", fmt.Sprintf(`{"episode_id":%q,"watched":true,"date":{"day":"2026-01-02"}}`, id+"2"), true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var count int
	if e = p.QueryRow(ctx, "SELECT count(*) FROM history WHERE user_id=$1", uid).Scan(&count); e != nil || count != 1 {
		t.Fatal("default marked earlier", count, e)
	}
	if w := request("POST", "/api/library/"+id+"/track", `{"whole":true,"watched":true,"date":{}}`, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var day, base string
	if e = p.QueryRow(ctx, "SELECT watched_date::text FROM history WHERE user_id=$1 AND episode_id=$2", uid, id+"2").Scan(&day); e != nil || day != "2026-01-02" {
		t.Fatal("date overwritten", day, e)
	}
	if e = p.QueryRow(ctx, "SELECT base FROM library WHERE user_id=$1 AND catalog_id=$2", uid, id).Scan(&base); e != nil || base != "completed" {
		t.Fatal(base, e)
	}
	if w := request("POST", "/api/library/"+id+"/track", fmt.Sprintf(`{"episode_id":%q,"watched":false,"date":{}}`, id+"2"), true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if e = p.QueryRow(ctx, "SELECT base FROM library WHERE user_id=$1 AND catalog_id=$2", uid, id).Scan(&base); e != nil || base != "completed" {
		t.Fatal("unwatch silently changed status", base, e)
	}
	var foreign string
	if e = p.QueryRow(ctx, "INSERT INTO statuses(user_id,name,base) VALUES($1,'Private','watching') RETURNING id", other).Scan(&foreign); e != nil {
		t.Fatal(e)
	}
	if w := request("PUT", "/api/library/"+id, fmt.Sprintf(`{"base":"watching","status_id":%q}`, foreign), true); w.Code != 404 {
		t.Fatal("foreign status accepted", w.Code)
	}
	w := request("POST", "/api/lists", `{"name":"Private smart","kind":"smart","sort":"title","rule":{"op":"not","children":[{"op":"eq","field":"service","value":"8"}]},"members":[]}`, true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request("POST", "/api/lists/preview", `{"rule":{"op":"not","children":[{"op":"eq","field":"service","value":"8"}]},"sort":"title"}`, true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"count":0`) {
		t.Fatal("unknown availability matched", w.Code, w.Body.String())
	}
	w = request("POST", "/api/lists", `{"name":"Foreign dependency","kind":"smart","sort":"title","rule":{"op":"eq","field":"list","value":"00000000-0000-0000-0000-000000000001"},"members":[]}`, true)
	if w.Code != 409 {
		t.Fatal("missing dependency accepted", w.Code)
	}

	backup := request("GET", "/api/export", "", true)
	if backup.Code != 200 {
		t.Fatal(backup.Code, backup.Body.String())
	}
	preview := request("POST", "/api/restore/preview", backup.Body.String(), true)
	if preview.Code != 200 {
		t.Fatal(preview.Code, preview.Body.String())
	}
	var previewData struct {
		ID string `json:"id"`
	}
	if e = json.Unmarshal(preview.Body.Bytes(), &previewData); e != nil {
		t.Fatal(e)
	}
	for range 2 {
		applied := request("POST", "/api/restore/"+previewData.ID+"/apply", `{}`, true)
		if applied.Code != 200 {
			t.Fatal("restore", applied.Code, applied.Body.String())
		}
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM history WHERE user_id=$1", uid).Scan(&count); e != nil || count != 1 {
		t.Fatal("restore duplicated history", count, e)
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM history WHERE user_id=$1", other).Scan(&count); e != nil || count != 0 {
		t.Fatal("restore escaped account", count, e)
	}
	if w := request("POST", "/api/admin/mappings", `{}`, true); w.Code != 403 {
		t.Fatal("non-admin mapping access", w.Code)
	}

}
