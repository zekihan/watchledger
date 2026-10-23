package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/zekihan/watchledger/internal/auth"
	"github.com/zekihan/watchledger/internal/config"
	"github.com/zekihan/watchledger/internal/simkl"
	"github.com/zekihan/watchledger/internal/store"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSIMKLReadOnlyFetchCachesPrivatelyAndNeverStoresToken(t *testing.T) {
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
	number := fmt.Sprint(time.Now().UnixNano())
	movie := "tmdb:movie:" + number
	exec("INSERT INTO catalog(id,source,source_id,kind,title) VALUES($1,'tmdb',$2,'movie','Synthetic movie')", movie, number)
	exec("INSERT INTO library(user_id,catalog_id,base) VALUES($1,$2,'planned')", uid, movie)
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "GET" || r.URL.Path != "/lists/42" {
			t.Error("unexpected source request", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer synthetic-private-token" {
			http.Error(w, "secret should not escape", http.StatusUnauthorized)
			return
		}
		_, _ = fmt.Fprintf(w, `{"id":42,"name":"Synthetic API list","media_type":"movies","type":"auto","pagination":{"page":1,"limit":500,"total_items":1,"total_pages":1},"items":[{"title":"Synthetic movie","ids":{"simkl_id":101,"tmdb":%q,"slug":"synthetic-movie"}}]}`, number)
	}))
	defer upstream.Close()
	source := simkl.NewReadOnlyClient()
	source.BaseURL = upstream.URL
	app := &API{DB: p, SIMKL: source}
	mux := http.NewServeMux()
	app.routes(mux)
	handler = identity.Middleware(mux)
	body := `{"client_id":"synthetic-client","access_token":"synthetic-private-token","list_id":"42"}`
	results := make(chan *httptest.ResponseRecorder, 2)
	for range 2 {
		go func() { results <- request("POST", "/api/simkl/custom-lists/fetch", body, true) }()
	}
	first, second := <-results, <-results
	if second.Code != 200 || calls.Load() != 1 {
		t.Fatal("concurrent auto-list fetch was not coalesced", second.Code, calls.Load())
	}
	if first.Code != 200 {
		t.Fatal(first.Code, first.Body.String())
	}
	var preview struct {
		ID         string
		Unresolved int
		FetchedAt  string `json:"api_fetched_at"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &preview); err != nil || preview.Unresolved != 0 || preview.FetchedAt == "" {
		t.Fatal("missing complete preview", preview, err)
	}
	var count int
	if err := p.QueryRow(ctx, "SELECT count(*) FROM lists WHERE user_id=$1", uid).Scan(&count); err != nil || count != 0 {
		t.Fatal("fetch applied list without approval", count, err)
	}
	if err := p.QueryRow(ctx, "SELECT count(*) FROM imports WHERE user_id=$1", uid).Scan(&count); err != nil || count != 1 {
		t.Fatal("cache read duplicated source archive", count, err)
	}
	var leaked bool
	if err := p.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM imports WHERE user_id=$1 AND to_jsonb(imports)::text LIKE '%'||$2||'%')", uid, "synthetic-private-token").Scan(&leaked); err != nil || leaked {
		t.Fatal("token entered persisted imports", leaked, err)
	}
	exec("UPDATE imports SET api_fetched_at=now()-interval '23 hours' WHERE id=$1", preview.ID)
	if w := request("POST", "/api/simkl/custom-lists/fetch", body, true); w.Code != 200 || calls.Load() != 1 {
		t.Fatal("auto list fetched again within a day", w.Code, calls.Load())
	}
	var unchanged bool
	if err := p.QueryRow(ctx, "SELECT api_fetched_at<now()-interval '22 hours' FROM imports WHERE id=$1", preview.ID).Scan(&unchanged); err != nil || !unchanged {
		t.Fatal("cache read extended provider freshness", unchanged, err)
	}
	exec("UPDATE sessions SET user_id=$2 WHERE token_hash=$1", sum, other)
	badBody := strings.ReplaceAll(body, "synthetic-private-token", "invalid-token")
	if w := request("POST", "/api/simkl/custom-lists/fetch", badBody, true); w.Code != 502 || strings.Contains(w.Body.String(), "secret should not escape") || calls.Load() != 2 {
		t.Fatal("private cache exposed or upstream body reflected", w.Code, w.Body.String(), calls.Load())
	}
	if w := request("POST", "/api/simkl/custom-lists/fetch", body, false); w.Code != 401 || calls.Load() != 2 {
		t.Fatal("anonymous upstream access", w.Code, calls.Load())
	}
	exec("UPDATE sessions SET user_id=$2 WHERE token_hash=$1", sum, uid)
	if w := request("POST", "/api/simkl/custom-lists/"+preview.ID+"/apply", "{}", true); w.Code != 428 {
		t.Fatal("API import skipped backup prerequisite", w.Code)
	}
	if w := request("GET", "/api/export", "", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("POST", "/api/simkl/custom-lists/"+preview.ID+"/apply", "{}", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := p.QueryRow(ctx, "SELECT count(*) FROM lists WHERE user_id=$1", uid).Scan(&count); err != nil || count != 1 {
		t.Fatal("explicit API list apply failed", count, err)
	}
	exec("UPDATE imports SET api_fetched_at=now()-interval '25 hours' WHERE id=$1", preview.ID)
	if w := request("POST", "/api/simkl/custom-lists/fetch", body, true); w.Code != 200 || calls.Load() != 3 {
		t.Fatal("explicit expired-cache retry failed", w.Code, w.Body.String(), calls.Load())
	}
	if err := p.QueryRow(ctx, "SELECT count(*) FROM history WHERE user_id=$1", uid).Scan(&count); err != nil || count != 0 {
		t.Fatal("list-only API changed viewing history", count, err)
	}
}
