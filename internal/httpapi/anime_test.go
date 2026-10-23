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

func TestNativeCatalogAndTrackingUseAniDBIdentity(t *testing.T) {
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

	const aid = "928400001"
	const key = "anidb:anime:" + aid
	const episode = "anidb:episode:928400002"
	raw := `<anime id="928400001"><type>TV Series</type><episodecount>1</episodecount><enddate>2026-01-01</enddate><titles><title xml:lang="en" type="official">Synthetic native</title></titles><episodes><episode id="928400002"><epno type="1">1</epno><length>24</length><airdate>2026-01-01</airdate><title xml:lang="en">Native first</title></episode></episodes></anime>`
	if _, e = p.Exec(ctx, "INSERT INTO anidb_documents(aid,document) VALUES($1,$2) ON CONFLICT(aid) DO UPDATE SET document=EXCLUDED.document,fetched_at=now()", aid, []byte(raw)); e != nil {
		t.Fatal(e)
	}
	defer func() { _, _ = p.Exec(ctx, "DELETE FROM anidb_documents WHERE aid=$1", aid) }()
	if _, e = p.Exec(ctx, `INSERT INTO catalog(id,source,source_id,kind,title,metadata) VALUES($1,'anidb',$2,'anime','Synthetic native','{"en":{"id":928400001,"name":"Synthetic native","status":"Ended","number_of_episodes":1,"titles":[{"language":"en","text":"Synthetic native"}]}}') ON CONFLICT(id) DO NOTHING`, key, aid); e != nil {
		t.Fatal(e)
	}
	if _, e = p.Exec(ctx, `INSERT INTO episodes(id,catalog_id,source,source_id,season,number,kind,air_date) VALUES($1,$2,'anidb','928400002',1,1,'regular','2026-01-01') ON CONFLICT(id) DO NOTHING`, episode, key); e != nil {
		t.Fatal(e)
	}
	w := request("GET", "/api/catalog/anime/"+aid+"?language=en", "", true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Native first") {
		t.Fatal("native cached details", w.Code, w.Body.String())
	}
	if w = request("GET", "/api/search?kind=anime&query=Synthetic%20native", "", true); w.Code != 200 || !strings.Contains(w.Body.String(), "cached_titles_only") {
		t.Fatal("cached scope misrepresented", w.Code, w.Body.String())
	}
	if w = request("GET", "/api/search?kind=anime&query="+aid, "", true); w.Code != 200 || !strings.Contains(w.Body.String(), "exact_source_id") {
		t.Fatal("exact native search", w.Code, w.Body.String())
	}
	if w = request("PUT", "/api/library/"+key, `{"base":"watching","rating":8}`, true); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	body := fmt.Sprintf(`{"episode_id":%q,"watched":true,"date":{"day":"2026-01-02"}}`, episode)
	if w = request("POST", "/api/library/"+key+"/track", body, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var storedKey, storedEpisode, day string
	if e = p.QueryRow(ctx, "SELECT catalog_id,episode_id,watched_date::text FROM history WHERE user_id=$1", uid).Scan(&storedKey, &storedEpisode, &day); e != nil || storedKey != key || storedEpisode != episode || day != "2026-01-02" {
		t.Fatal("native identity/date lost", e, storedKey, storedEpisode, day)
	}
	if _, e = p.Exec(ctx, "UPDATE sessions SET user_id=$2 WHERE token_hash=$1", sum, other); e != nil {
		t.Fatal(e)
	}
	if w = request("POST", "/api/library/"+key+"/track", body, true); w.Code != 404 {
		t.Fatal("cross-account native tracking", w.Code)
	}
	if w = request("GET", "/api/catalog/anime/"+aid, "", false); w.Code != 401 {
		t.Fatal("anonymous metadata", w.Code)
	}
}
