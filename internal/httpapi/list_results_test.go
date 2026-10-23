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

func TestSavedListResultsAndEditedDependencyCycles(t *testing.T) {
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
	movieID := "fixture-movie:" + suffix
	if _, e = p.Exec(ctx, `INSERT INTO catalog(id,source,source_id,kind,title,metadata) VALUES($1,'fixture',$2,'movie','English movie','{"tr":{"title":"Yerel film","genres":[{"id":35,"name":"Komedi"}]},"en":{"title":"English movie","genres":[{"id":18,"name":"Drama"}]}}')`, movieID, suffix); e != nil {
		t.Fatal(e)
	}
	if _, e = p.Exec(ctx, `UPDATE users SET settings='{"metadata_language":"tr","country":"TR"}' WHERE id=$1`, uid); e != nil {
		t.Fatal(e)
	}
	for _, catalogID := range []string{id, movieID} {
		if _, e = p.Exec(ctx, "INSERT INTO library(user_id,catalog_id,base) VALUES($1,$2,'watching')", uid, catalogID); e != nil {
			t.Fatal(e)
		}
	}
	manual := fmt.Sprintf(`{"name":"Synthetic manual","kind":"manual","sort":"title","members":[%q,%q]}`, movieID, id)
	w := request("POST", "/api/lists", manual, true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var a, b struct {
		ID string `json:"id"`
	}
	if e = json.Unmarshal(w.Body.Bytes(), &a); e != nil {
		t.Fatal(e)
	}
	if w = request("GET", "/api/lists/"+a.ID+"/results", "", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result struct {
		Count   int                          `json:"count"`
		Results []struct{ ID, Title string } `json:"results"`
	}
	if e = json.Unmarshal(w.Body.Bytes(), &result); e != nil || len(result.Results) != 2 || result.Results[0].ID != movieID || result.Results[0].Title != "Yerel film" {
		t.Fatal("manual order/localization", e, w.Body.String())
	}
	rule := `{"op":"and","children":[{"op":"eq","field":"base","value":"watching"},{"op":"not","children":[{"op":"eq","field":"service","value":"8"}]}]}`
	smartBody := fmt.Sprintf(`{"name":"Synthetic smart","kind":"smart","sort":"title","rule":%s,"members":[]}`, rule)
	if w = request("POST", "/api/lists", smartBody, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if e = json.Unmarshal(w.Body.Bytes(), &b); e != nil {
		t.Fatal(e)
	}
	if w = request("GET", "/api/lists/"+b.ID+"/results", "", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if e = json.Unmarshal(w.Body.Bytes(), &result); e != nil || result.Count != 0 {
		t.Fatal("unknown availability matched NOT", e, w.Body.String())
	}
	if _, e = p.Exec(ctx, `INSERT INTO availability(catalog_id,country,offers) VALUES($1,'TR','{"flatrate":[]}')`, movieID); e != nil {
		t.Fatal(e)
	}
	if w = request("GET", "/api/lists/"+b.ID+"/results", "", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if e = json.Unmarshal(w.Body.Bytes(), &result); e != nil || result.Count != 1 || result.Results[0].ID != movieID {
		t.Fatal("known empty availability did not match", e, w.Body.String())
	}
	// Metadata filters use the requested language deterministically.
	for n := 0; n < 10; n++ {
		w = request("POST", "/api/lists/preview", `{"rule":{"op":"eq","field":"genre","value":"35"},"sort":"title"}`, true)
		if e = json.Unmarshal(w.Body.Bytes(), &result); e != nil || w.Code != 200 || result.Count != 1 {
			t.Fatal("nondeterministic localized metadata", e, w.Body.String())
		}
	}

	if _, e = p.Exec(ctx, "UPDATE library SET rating=CASE WHEN catalog_id=$2 THEN 9 ELSE 1 END,added_at=CASE WHEN catalog_id=$2 THEN '2020-01-01'::timestamptz ELSE '2021-01-01'::timestamptz END WHERE user_id=$1", uid, movieID); e != nil {
		t.Fatal(e)
	}
	if _, e = p.Exec(ctx, `UPDATE catalog SET metadata=metadata||jsonb_build_object('tr',coalesce(metadata->'tr','{}'::jsonb)||'{"release_date":"2023-01-01"}'::jsonb) WHERE id=$1`, movieID); e != nil {
		t.Fatal(e)
	}
	if _, e = p.Exec(ctx, `UPDATE catalog SET metadata=metadata||jsonb_build_object('tr',coalesce(metadata->'tr','{}'::jsonb)||'{"first_air_date":"2024-01-01"}'::jsonb) WHERE id=$1`, id); e != nil {
		t.Fatal(e)
	}
	for order, first := range map[string]string{"title": id, "rating": movieID, "release": id, "added": id} {
		body := fmt.Sprintf(`{"id":%q,"name":"Synthetic smart","kind":"smart","sort":%q,"rule":{"op":"eq","field":"base","value":"watching"},"members":[]}`, b.ID, order)
		if w = request("POST", "/api/lists", body, true); w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		if w = request("GET", "/api/lists/"+b.ID+"/results", "", true); w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		if e = json.Unmarshal(w.Body.Bytes(), &result); e != nil || len(result.Results) != 2 || result.Results[0].ID != first {
			t.Fatal("saved sort", order, e, w.Body.String())
		}
	}
	reference := fmt.Sprintf(`{"id":%q,"name":"Synthetic smart","kind":"smart","sort":"title","rule":{"op":"eq","field":"list","value":%q},"members":[]}`, b.ID, a.ID)
	if w = request("POST", "/api/lists", reference, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	cycle := fmt.Sprintf(`{"id":%q,"rule":{"op":"eq","field":"list","value":%q},"sort":"title"}`, a.ID, b.ID)
	if w = request("POST", "/api/lists/preview", cycle, true); w.Code != 409 {
		t.Fatal("edited dependency cycle missed", w.Code, w.Body.String())
	}
	if w = request("GET", "/api/lists/"+a.ID+"/results", "", false); w.Code != 401 {
		t.Fatal("unauthenticated result", w.Code)
	}
	if _, e = p.Exec(ctx, "UPDATE sessions SET user_id=$2 WHERE token_hash=$1", sum, other); e != nil {
		t.Fatal(e)
	}
	for _, listID := range []string{a.ID, b.ID} {
		if w = request("GET", "/api/lists/"+listID+"/results", "", true); w.Code != 404 {
			t.Fatal("private list result leaked", w.Code, w.Body.String())
		}
	}
}
