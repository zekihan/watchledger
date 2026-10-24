package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/zekihan/watchledger/internal/auth"
	"github.com/zekihan/watchledger/internal/config"
	"github.com/zekihan/watchledger/internal/mapping"
	"github.com/zekihan/watchledger/internal/store"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLinkedTrackingPreservesFactsAndRequiresFreshConfirmation(t *testing.T) {
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

	target := id + ":target"
	te := target + ":episode"
	if _, e = p.Exec(ctx, "INSERT INTO catalog(id,source,source_id,kind,title) VALUES($1,'fixture',$1,'tv','Synthetic target')", target); e != nil {
		t.Fatal(e)
	}
	if _, e = p.Exec(ctx, "INSERT INTO episodes(id,catalog_id,source,source_id,season,number,air_date) VALUES($1,$2,'fixture',$1,2,7,'2020-01-01')", te, target); e != nil {
		t.Fatal(e)
	}
	for _, cid := range []string{id, target} {
		if _, e = p.Exec(ctx, "INSERT INTO library(user_id,catalog_id,base) VALUES($1,$2,'watching')", uid, cid); e != nil {
			t.Fatal(e)
		}
	}
	edges, revision, _, _ := mapping.Canonical([]mapping.Edge{{From: id + "1", To: te}, {From: id + "2", To: te}})
	raw, _ := json.Marshal(edges)
	var mid string
	if e = p.QueryRow(ctx, `INSERT INTO mappings(revision,provenance,relationships,requires_consent,reviewed) VALUES($1,'{"source":"synthetic","version":"1"}',$2,true,true) RETURNING id`, revision, raw).Scan(&mid); e != nil {
		t.Fatal(e)
	}
	defer func() {
		_, _ = p.Exec(ctx, "DELETE FROM mapping_consents WHERE mapping_id=$1", mid)
		_, _ = p.Exec(ctx, "DELETE FROM mappings WHERE id=$1", mid)
	}()
	insertSources := func() {
		t.Helper()
		for _, eid := range []string{id + "1", id + "2"} {
			if _, e = p.Exec(ctx, `INSERT INTO history(user_id,catalog_id,episode_id,watched_date,provenance) VALUES($1,$2,$3,'2026-01-02','{"source":"simkl"}')`, uid, id, eid); e != nil {
				t.Fatal(e)
			}
		}
	}
	insertSources()
	state := func() mapping.State {
		t.Helper()
		w := request("GET", "/api/library/"+target+"/effective", "", true)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var states map[string]mapping.State
		if e = json.Unmarshal(w.Body.Bytes(), &states); e != nil {
			t.Fatal(e)
		}
		return states[te]
	}
	if state().Derived {
		t.Fatal("unconfirmed source propagated")
	}
	if w := request("POST", "/api/mappings/"+mid+"/confirm", fmt.Sprintf(`{"revision":%q}`, revision), true); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	if !state().Derived {
		t.Fatal("confirmed view missing")
	}
	watch := fmt.Sprintf(`{"episode_id":%q,"watched":true,"date":{"day":"2026-02-03"}}`, te)
	if w := request("POST", "/api/library/"+target+"/track", watch, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	countHistory := func(want int) {
		t.Helper()
		var count int
		if e = p.QueryRow(ctx, "SELECT count(*) FROM history WHERE user_id=$1", uid).Scan(&count); e != nil || count != want {
			t.Fatal("source history count", count, want, e)
		}
	}
	countHistory(2)
	var known string
	if e = p.QueryRow(ctx, "SELECT watched_date::text FROM history WHERE user_id=$1 AND episode_id=$2", uid, id+"1").Scan(&known); e != nil || known != "2026-01-02" {
		t.Fatal("linked watch overwrote original date", known, e)
	}
	type plan struct {
		Token   string         `json:"linked_plan"`
		Changes []recordedLink `json:"linked_changes"`
	}
	preview := func() plan {
		t.Helper()
		w := request("POST", "/api/library/"+target+"/track", fmt.Sprintf(`{"episode_id":%q,"watched":false,"preview":true,"date":{}}`, te), true)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var result plan
		if e = json.Unmarshal(w.Body.Bytes(), &result); e != nil || len(result.Changes) != 2 {
			t.Fatal(result, e)
		}
		return result
	}
	first := preview()
	apply := func(token string) *httptest.ResponseRecorder {
		return request("POST", "/api/library/"+target+"/track", fmt.Sprintf(`{"episode_id":%q,"watched":false,"include_linked":true,"linked_plan":%q,"date":{}}`, te, token), true)
	}
	if w := request("POST", "/api/library/"+target+"/track", fmt.Sprintf(`{"episode_id":%q,"watched":false,"date":{}}`, te), true); w.Code != 409 {
		t.Fatal("linked unwatch did not require confirmation", w.Code)
	}
	countHistory(2)
	if _, e = p.Exec(ctx, "DELETE FROM history WHERE user_id=$1 AND catalog_id=$2", uid, id); e != nil {
		t.Fatal(e)
	}
	insertSources()
	if w := apply(first.Token); w.Code != 409 {
		t.Fatal("replaced source records reused stale confirmation", w.Code)
	}
	fresh := preview()
	if w := apply(fresh.Token); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	countHistory(0)
	// Even newly imported counterparts must not resurrect a local unwatch.
	if _, e = p.Exec(ctx, "DELETE FROM history_tombstones WHERE user_id=$1 AND catalog_id=$2", uid, id); e != nil {
		t.Fatal(e)
	}
	insertSources()
	if s := state(); s.Derived || !s.ExplicitUnwatch {
		t.Fatal("import overrode explicit cross-view unwatch", s)
	}
	// Explicitly marking this view restores the linked flag without copying dates.
	if w := request("POST", "/api/library/"+target+"/track", watch, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	countHistory(2)
	if !state().Derived {
		t.Fatal("explicit watch could not restore linked view")
	}
	fresh = preview()
	if w := apply(fresh.Token); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, eid := range []string{id + "1", id + "2"} {
		if w := request("POST", "/api/library/"+id+"/track", fmt.Sprintf(`{"episode_id":%q,"watched":true,"date":{}}`, eid), true); w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if s := state(); !s.Derived || s.ExplicitUnwatch {
		t.Fatal("new local watch did not update linked view", s)
	}
	countHistory(2)
	w := request("GET", "/api/export", "", true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "intent_scope") {
		t.Fatal("portable intent scopes missing", w.Code)
	}
	if _, e = p.Exec(ctx, "UPDATE sessions SET user_id=$2 WHERE token_hash=$1", sum, other); e != nil {
		t.Fatal(e)
	}
	if w = request("GET", "/api/library/"+target+"/effective", "", true); w.Code != 404 {
		t.Fatal("cross-account effective history", w.Code)
	}
	if w = apply(fresh.Token); w.Code != 404 {
		t.Fatal("cross-account linked mutation", w.Code)
	}
}
