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

	labels := request("GET", "/api/mappings", "", true)
	if labels.Code != 200 {
		t.Fatal(labels.Code, labels.Body.String())
	}
	var listed []struct {
		ID       string `json:"id"`
		Episodes map[string]struct {
			Title  string `json:"title"`
			Season int    `json:"season"`
			Number int    `json:"number"`
		} `json:"episodes"`
	}
	if err := json.Unmarshal(labels.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range listed {
		if m.ID == mid {
			found = true
			label := m.Episodes[target+"ep"]
			if label.Title != "Alternate show" || label.Season != 2 || label.Number != 5 {
				t.Fatal("missing human-readable episode labels", label)
			}
		}
	}
	if !found {
		t.Fatal("mapping absent from review")
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
	if w := request("PUT", "/api/library/"+id+"/structure", `{"view":"show"}`, true); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("PUT", "/api/library/"+id, `{"base":"watching","rating":9}`, true); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	var viewSetting *string
	if e = p.QueryRow(ctx, "SELECT anime_view FROM library WHERE user_id=$1 AND catalog_id=$2", uid, id).Scan(&viewSetting); e != nil || viewSetting == nil || *viewSetting != "show" {
		t.Fatal("rating update erased structure override", viewSetting, e)
	}
	if w := request("PUT", "/api/library/"+id+"/structure", `{"view":"default"}`, true); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	if e = p.QueryRow(ctx, "SELECT anime_view FROM library WHERE user_id=$1 AND catalog_id=$2", uid, id).Scan(&viewSetting); e != nil || viewSetting != nil {
		t.Fatal("default structure not restored", e)
	}

	// Separate cours linked to distinct episodes of the same show remain
	// separate native entries, while the show aggregates both sets of progress.
	exec := func(query string, args ...interface{}) {
		t.Helper()
		if _, err := p.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	cour := id + ":cour2"
	exec("INSERT INTO catalog(id,source,source_id,kind,title) VALUES($1,'fixture',$1,'anime','Second cour')", cour)
	exec("INSERT INTO episodes(id,catalog_id,source,source_id,season,number) VALUES($1,$2,'fixture',$1,1,1)", cour+"e", cour)
	exec("INSERT INTO episodes(id,catalog_id,source,source_id,season,number) VALUES($1,$2,'fixture',$1,3,1)", target+"second", target)
	secondEdges, secondRev, _, _ := mapping.Canonical([]mapping.Edge{{From: cour + "e", To: target + "second"}})
	secondRaw, _ := json.Marshal(secondEdges)
	var secondID string
	if err := p.QueryRow(ctx, `INSERT INTO mappings(revision,provenance,relationships,requires_consent,reviewed) VALUES($1,'{"source":"synthetic","version":"2"}',$2,false,true) RETURNING id`, secondRev, secondRaw).Scan(&secondID); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = p.Exec(ctx, "DELETE FROM mappings WHERE id=$1", secondID) }()
	exec("INSERT INTO library(user_id,catalog_id,base,rating) VALUES($1,$2,'completed',10)", other, cour)
	checkCour := func(wantRating *int, wantWatched int) {
		t.Helper()
		seen := false
		for _, title := range read().Titles {
			if title.ID == cour {
				seen = true
				if (title.Rating == nil) != (wantRating == nil) || (wantRating != nil && *title.Rating != *wantRating) || title.Watched != wantWatched {
					t.Fatal("cour privacy/progress", title)
				}
			}
			if title.ID == target && title.Watched != wantWatched {
				t.Fatal("show did not aggregate separate cour", title)
			}
		}
		if !seen {
			t.Fatal("second cour omitted")
		}
	}
	checkCour(nil, 0)
	exec("INSERT INTO library(user_id,catalog_id,base,rating) VALUES($1,$2,'completed',7)", uid, cour)
	exec("INSERT INTO history(user_id,catalog_id,episode_id) VALUES($1,$2,$3)", uid, cour, cour+"e")
	seven := 7
	checkCour(&seven, 1)
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
