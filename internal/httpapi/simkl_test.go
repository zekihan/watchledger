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

func TestSIMKLPrivatePreviewAndApply(t *testing.T) {
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
	movieID := "tmdb:movie:" + suffix
	if _, e = p.Exec(ctx, `INSERT INTO catalog(id,source,source_id,kind,title) VALUES($1,'tmdb',$2,'movie','Synthetic imported movie')`, movieID, suffix); e != nil {
		t.Fatal(e)
	}
	raw := fmt.Sprintf(`{"movies":[{"movie":{"title":"Synthetic movie","ids":{"simkl":1,"tmdb":%q}},"status":"completed","user_rating":7,"last_watched_at":"2025-01-02","unsupported":{"keep":true}}],"anime":[{"show":{"title":"Synthetic native anime","ids":{"simkl":2,"tmdb":%q}},"status":"watching","seasons":[{"number":1,"episodes":[{"number":3}]}]}]}`, suffix, suffix)
	if w := request("POST", "/api/simkl/preview", raw, false); w.Code != 401 {
		t.Fatal("unauthenticated preview", w.Code)
	}
	w := request("POST", "/api/simkl/preview", raw, true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var preview struct {
		ID         string `json:"id"`
		Additions  int    `json:"additions"`
		Unresolved int    `json:"unresolved"`
	}
	if e = json.Unmarshal(w.Body.Bytes(), &preview); e != nil || preview.Additions != 1 || preview.Unresolved != 1 {
		t.Fatal(e, w.Body.String())
	}
	path := "/api/simkl/" + preview.ID + "/apply"
	if w = request("POST", path, "{}", true); w.Code != 428 {
		t.Fatal("backup prerequisite", w.Code, w.Body.String())
	}
	if w = request("GET", "/api/export", "", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	for n := 0; n < 2; n++ {
		if w = request("POST", path, "{}", true); w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	var count int
	if e = p.QueryRow(ctx, "SELECT count(*) FROM history WHERE user_id=$1", uid).Scan(&count); e != nil || count != 1 {
		t.Fatal("history duplication", count, e)
	}
	var day string
	if e = p.QueryRow(ctx, "SELECT watched_date::text FROM history WHERE user_id=$1", uid).Scan(&day); e != nil || day != "2025-01-02" {
		t.Fatal("watch precision", day, e)
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM import_reviews WHERE user_id=$1", uid).Scan(&count); e != nil || count != 1 {
		t.Fatal("review duplication", count, e)
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM library WHERE user_id=$1", uid).Scan(&count); e != nil || count != 1 {
		t.Fatal("anime incorrectly merged", count, e)
	}
	if w = request("GET", "/api/imports/"+preview.ID+"/archive", "", true); w.Code != 200 || !strings.Contains(w.Body.String(), "unsupported") {
		t.Fatal("archive lost", w.Code, w.Body.String())
	}

	// Re-previewing the same source permits safe retries without duplicate review rows.
	for n := 0; n < 2; n++ {
		if w = request("POST", "/api/simkl/preview", raw, true); w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		if w = request("POST", path, "{}", true); w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM import_reviews WHERE user_id=$1", uid).Scan(&count); e != nil || count != 2 {
		t.Fatal("repreview duplicated reviews", count, e)
	}
	// A later app edit and explicit unwatch win over a different manual reimport.
	if _, e = p.Exec(ctx, "UPDATE library SET rating=9 WHERE user_id=$1", uid); e != nil {
		t.Fatal(e)
	}
	if w = request("POST", "/api/library/"+movieID+"/track", `{"watched":false,"whole":true,"date":{}}`, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	raw = strings.Replace(raw, "\"user_rating\":7", "\"user_rating\":8", 1)
	if w = request("POST", "/api/simkl/preview", raw, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if e = json.Unmarshal(w.Body.Bytes(), &preview); e != nil {
		t.Fatal(e)
	}
	if w = request("POST", "/api/simkl/"+preview.ID+"/apply", "{}", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var rating int
	if e = p.QueryRow(ctx, "SELECT rating FROM library WHERE user_id=$1", uid).Scan(&rating); e != nil || rating != 9 {
		t.Fatal("app rating overwritten", rating, e)
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM history WHERE user_id=$1", uid).Scan(&count); e != nil || count != 0 {
		t.Fatal("explicit unwatch resurrected", count, e)
	}

	uncachedID := fmt.Sprint(time.Now().UnixNano())
	uncachedRaw := fmt.Sprintf(`{"movies":[{"movie":{"title":"Synthetic uncached","ids":{"simkl":99,"tmdb":%q}},"status":"plantowatch"}]}`, uncachedID)
	if w = request("POST", "/api/simkl/preview", uncachedRaw, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var retry struct {
		ID string `json:"id"`
	}
	if e = json.Unmarshal(w.Body.Bytes(), &retry); e != nil {
		t.Fatal(e)
	}
	if w = request("POST", "/api/simkl/"+retry.ID+"/apply", "{}", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, e = p.Exec(ctx, `INSERT INTO catalog(id,source,source_id,kind,title) VALUES($1,'tmdb',$2,'movie','Synthetic uncached')`, "tmdb:movie:"+uncachedID, uncachedID); e != nil {
		t.Fatal(e)
	}
	if w = request("POST", "/api/simkl/preview", uncachedRaw, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = request("POST", "/api/simkl/"+retry.ID+"/apply", "{}", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM library WHERE user_id=$1 AND catalog_id=$2", uid, "tmdb:movie:"+uncachedID).Scan(&count); e != nil || count != 1 {
		t.Fatal("new cached identity not applied", count, e)
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM import_reviews WHERE user_id=$1 AND import_id=$2 AND NOT resolved", uid, retry.ID).Scan(&count); e != nil || count != 0 {
		t.Fatal("resolved identity left unresolved", count, e)
	}
	// Switch the same synthetic session to the other account, then attempt access.
	if _, e = p.Exec(ctx, "UPDATE sessions SET user_id=$2 WHERE token_hash=$1", sum, other); e != nil {
		t.Fatal(e)
	}
	if w = request("GET", "/api/imports/"+preview.ID+"/archive", "", true); w.Code != 404 {
		t.Fatal("cross-user source archive", w.Code)
	}
	if w = request("POST", "/api/simkl/"+preview.ID+"/apply", "{}", true); w.Code != 404 {
		t.Fatal("cross-user apply", w.Code)
	}
	for _, route := range []string{"/api/imports", "/api/import-reviews"} {
		if w = request("GET", route, "", true); w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
			t.Fatal("private import data leaked", route, w.Code, w.Body.String())
		}
	}
}
