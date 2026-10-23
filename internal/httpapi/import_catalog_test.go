package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/zekihan/watchledger/internal/auth"
	"github.com/zekihan/watchledger/internal/catalog"
	"github.com/zekihan/watchledger/internal/config"
	"github.com/zekihan/watchledger/internal/store"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSIMKLLoadsExplicitProviderIdentityBeforeResolve(t *testing.T) {
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
	movieNumber := fmt.Sprint(time.Now().UnixNano())
	showNumber := fmt.Sprint(time.Now().UnixNano())
	episodeNumber := fmt.Sprint(time.Now().UnixNano())
	var offline atomic.Bool
	var calls, seasonCalls atomic.Int64
	offline.Store(true)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer synthetic-provider-token" {
			t.Error("missing synthetic provider auth")
		}
		if offline.Load() {
			http.Error(w, "synthetic outage", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/movie/" + movieNumber:
			_, _ = fmt.Fprintf(w, `{"id":%s,"title":"Synthetic movie","overview":"Synthetic overview"}`, movieNumber)
		case "/tv/" + showNumber:
			_, _ = fmt.Fprintf(w, `{"id":%s,"name":"Synthetic show","overview":"Synthetic overview","seasons":[{"season_number":2}]}`, showNumber)
		case "/tv/" + showNumber + "/season/2":
			seasonCalls.Add(1)
			_, _ = fmt.Fprintf(w, `{"episodes":[{"id":%s,"episode_number":7,"name":"Synthetic episode","air_date":"2020-01-01"}]}`, episodeNumber)
		default:
			t.Errorf("unexpected provider path: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	provider := catalog.New("synthetic-provider-token")
	provider.BaseURL = upstream.URL
	application := &API{DB: p, Catalog: &catalog.Cache{DB: p, Provider: provider}}
	mux := http.NewServeMux()
	application.routes(mux)
	handler = identity.Middleware(mux)
	raw := fmt.Sprintf(`{"movies":[{"movie":{"title":"Synthetic movie","ids":{"simkl":1,"tmdb":%q}},"status":"completed","last_watched_at":"2025-01-02","user_rating":7}],"shows":[{"show":{"title":"Synthetic show","ids":{"simkl":2,"tmdb":%q}},"status":"watching","seasons":[{"number":1,"episodes":[{"number":3}]}]}],"anime":[{"show":{"title":"Synthetic native entry","ids":{"simkl":3,"tmdb":%q}},"status":"watching"}]}`, movieNumber, showNumber, movieNumber)
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
	var movieReview, showReview, animeReview string
	for key, dest := range map[string]*string{"movies:1": &movieReview, "shows:2": &showReview, "anime:3": &animeReview} {
		if e = p.QueryRow(ctx, "SELECT id FROM import_reviews WHERE user_id=$1 AND import_id=$2 AND item->'source'->>'key'=$3", uid, preview.ID, key).Scan(dest); e != nil {
			t.Fatal(e)
		}
	}
	movieRoute := "/api/import-reviews/" + movieReview
	showRoute := "/api/import-reviews/" + showReview
	var count int
	if w = request("POST", movieRoute+"/catalog", "{}", true); w.Code != 502 {
		t.Fatal("provider outage hidden", w.Code, w.Body.String())
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM library WHERE user_id=$1", uid).Scan(&count); e != nil || count != 0 {
		t.Fatal("metadata failure imported library", count, e)
	}
	offline.Store(false)
	if w = request("POST", movieRoute+"/catalog", "{}", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM history WHERE user_id=$1", uid).Scan(&count); e != nil || count != 0 {
		t.Fatal("metadata loading imported history", count, e)
	}
	if w = request("POST", movieRoute+"/resolve", `{"confirm":true,"accept_existing":true,"links":[]}`, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var day string
	if e = p.QueryRow(ctx, "SELECT watched_date::text FROM history WHERE user_id=$1", uid).Scan(&day); e != nil || day != "2025-01-02" {
		t.Fatal("movie resolution lost recorded date", day, e)
	}
	if w = request("POST", showRoute+"/catalog", `{"season":1}`, true); w.Code != 400 || seasonCalls.Load() != 0 {
		t.Fatal("source season guessed as provider season", w.Code, w.Body.String())
	}
	if w = request("POST", showRoute+"/catalog", `{"season":2}`, true); w.Code != 200 || seasonCalls.Load() != 1 {
		t.Fatal("explicit season loading", w.Code, w.Body.String())
	}
	if w = request("GET", showRoute+"/candidates", "", true); w.Code != 200 || !strings.Contains(w.Body.String(), "tmdb:episode:"+episodeNumber) {
		t.Fatal("cached episode missing before library insertion", w.Code, w.Body.String())
	}
	link := fmt.Sprintf(`{"confirm":true,"accept_existing":true,"links":[{"source_key":"shows:2:s1:e3","episode_id":%q}]}`, "tmdb:episode:"+episodeNumber)
	if w = request("POST", showRoute+"/resolve", link, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM history WHERE user_id=$1", uid).Scan(&count); e != nil || count != 2 {
		t.Fatal("explicit episode not applied", count, e)
	}
	before := calls.Load()
	if w = request("POST", "/api/import-reviews/"+animeReview+"/catalog", "{}", true); w.Code != 409 || calls.Load() != before {
		t.Fatal("anime matched through shared TMDB ID", w.Code)
	}
	if _, e = p.Exec(ctx, "UPDATE sessions SET user_id=$2 WHERE token_hash=$1", sum, other); e != nil {
		t.Fatal(e)
	}
	if w = request("POST", movieRoute+"/catalog", "{}", true); w.Code != 404 || calls.Load() != before {
		t.Fatal("cross-user provider fetch", w.Code)
	}
}
