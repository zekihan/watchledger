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

func TestPrivateAlternateProjectionConsentAndCorrections(t *testing.T) {
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

	if _, e = p.Exec(ctx, "INSERT INTO library(user_id,catalog_id,base,rating) VALUES($1,$2,'watching',8)", uid, id); e != nil {
		t.Fatal(e)
	}
	target := id + ":target"
	if _, e = p.Exec(ctx, "INSERT INTO catalog(id,source,source_id,kind,title) VALUES($1,'fixture',$1,'tv','Alternate show')", target); e != nil {
		t.Fatal(e)
	}
	if _, e = p.Exec(ctx, "INSERT INTO episodes(id,catalog_id,source,source_id,season,number) VALUES($1,$2,'fixture',$1,2,5)", target+"ep", target); e != nil {
		t.Fatal(e)
	}
	edges, rev, _, e := mapping.Canonical([]mapping.Edge{{From: id + "1", To: target + "ep"}, {From: id + "2", To: target + "ep"}})
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(edges)
	var mid string
	if e = p.QueryRow(ctx, `INSERT INTO mappings(revision,provenance,relationships,requires_consent,reviewed) VALUES($1,'{"source":"synthetic","version":"1"}',$2,true,true) RETURNING id`, rev, raw).Scan(&mid); e != nil {
		t.Fatal(e)
	}
	defer func() {
		_, _ = p.Exec(ctx, "DELETE FROM mapping_consents WHERE mapping_id=$1", mid)
		_, _ = p.Exec(ctx, "DELETE FROM mappings WHERE id=$1", mid)
	}()
	if _, e = p.Exec(ctx, "INSERT INTO history(user_id,catalog_id,episode_id) VALUES($1,$2,$3),($1,$2,$4)", uid, id, id+"1", id+"2"); e != nil {
		t.Fatal(e)
	}
	type view struct {
		Titles []struct {
			ID       string
			Rating   *int
			Episodes []struct {
				ID    string
				State mapping.State
			}
			Watched int
		}
	}
	read := func() view {
		t.Helper()
		w := request("GET", "/api/library/"+id+"/alternate", "", true)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var v view
		if e = json.Unmarshal(w.Body.Bytes(), &v); e != nil {
			t.Fatal(e)
		}
		return v
	}
	targetState := func(v view) mapping.State {
		for _, title := range v.Titles {
			if title.ID == target {
				if title.Rating != nil {
					t.Fatal("fabricated aggregate rating")
				}
				return title.Episodes[0].State
			}
		}
		t.Fatal("target missing")
		return mapping.State{}
	}
	if state := targetState(read()); state.Derived || !state.Blocked {
		t.Fatal("unconfirmed cross-view propagation", state)
	}
	if w := request("POST", "/api/mappings/"+mid+"/confirm", fmt.Sprintf(`{"revision":%q}`, rev), true); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	if state := targetState(read()); !state.Derived || state.Blocked {
		t.Fatal("confirmed projection absent", state)
	}
	var count int
	if e = p.QueryRow(ctx, "SELECT count(*) FROM history WHERE user_id=$1", uid).Scan(&count); e != nil || count != 2 {
		t.Fatal("projection copied history", count, e)
	}
	// Source removal recomputes the view; no copied target event can survive.
	if _, e = p.Exec(ctx, "DELETE FROM history WHERE user_id=$1 AND episode_id=$2", uid, id+"2"); e != nil {
		t.Fatal(e)
	}
	if targetState(read()).Derived {
		t.Fatal("partial combined progress")
	}
	// Retiring a mapped source blocks projection without deleting its event.
	if _, e = p.Exec(ctx, "UPDATE episodes SET retired=true WHERE id=$1", id+"1"); e != nil {
		t.Fatal(e)
	}
	if !targetState(read()).Blocked {
		t.Fatal("retired source still propagates")
	}
	if _, e = p.Exec(ctx, "UPDATE sessions SET user_id=$2 WHERE token_hash=$1", sum, other); e != nil {
		t.Fatal(e)
	}
	if w := request("GET", "/api/library/"+id+"/alternate", "", true); w.Code != 404 {
		t.Fatal("other library exposed", w.Code)
	}
	if w := request("GET", "/api/library/"+id+"/alternate", "", false); w.Code != 401 {
		t.Fatal("anonymous projection", w.Code)
	}
}
