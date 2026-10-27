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
	"github.com/zekihan/watchledger/internal/portable"
	"github.com/zekihan/watchledger/internal/store"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestUpNextUsesEffectiveProgressAndPrivateStructureOrder(t *testing.T) {
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
	if _, e = p.Exec(ctx, `INSERT INTO catalog(id,source,source_id,kind,title,metadata) VALUES($1,'fixture',$2,'tv','Synthetic','{"":{"status":"Ended","number_of_episodes":2,"poster_path":"/native.jpg"}}')`, id, suffix); e != nil {
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
	exec("UPDATE catalog SET kind='anime' WHERE id=$1", id)
	exec(`UPDATE users SET settings='{"anime_view":"show"}' WHERE id=$1`, uid)
	exec("INSERT INTO library(user_id,catalog_id,base) VALUES($1,$2,'watching')", uid, id)
	target := id + ":target"
	exec(`INSERT INTO catalog(id,source,source_id,kind,title,metadata) VALUES($1,'tmdb',$1,'tv','Synthetic show','{"en":{"poster_path":"/show.jpg"}}')`, target)
	edges := []mapping.Edge{}
	for n := 1; n <= 2; n++ {
		eid := target + fmt.Sprint(n)
		exec("INSERT INTO episodes(id,catalog_id,source,source_id,season,number,air_date) VALUES($1,$2,'tmdb',$1,2,$3,'2020-01-01')", eid, target, n+5)
		edges = append(edges, mapping.Edge{From: id + fmt.Sprint(n), To: eid})
	}
	edges, revision, _, _ := mapping.Canonical(edges)
	raw, _ := json.Marshal(edges)
	var mid string
	if err := p.QueryRow(ctx, `INSERT INTO mappings(revision,provenance,relationships,requires_consent,reviewed) VALUES($1,'{"source":"synthetic"}',$2,false,true) RETURNING id`, revision, raw).Scan(&mid); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = p.Exec(ctx, "DELETE FROM mappings WHERE id=$1", mid) }()
	// The only recorded fact is on the alternate representation.
	exec("INSERT INTO history(user_id,catalog_id,episode_id) VALUES($1,$2,$3)", uid, target, target+"1")
	cards := func() []nextCard {
		t.Helper()
		w := request("GET", "/api/dashboard", "", true)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var d struct {
			Cards []nextCard `json:"up_next"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
			t.Fatal(err)
		}
		return d.Cards
	}
	check := func(cid, eid, anchor string, incomplete bool) {
		t.Helper()
		got := cards()
		if len(got) != 1 || got[0].CatalogID != cid || got[0].EpisodeID != eid || got[0].ViewAnchor != anchor || got[0].Incomplete != incomplete {
			t.Fatalf("unexpected cards: %+v", got)
		}
	}
	check(target, target+"2", id, false)
	if got := cards()[0].PosterPath; got != "/show.jpg" {
		t.Fatalf("projected card poster = %q", got)
	}
	// Ordering a displayed aggregate is valid without creating private membership.
	if w := request("PUT", "/api/up-next/order", fmt.Sprintf(`{"ids":[%q]}`, target), true); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	var count int
	if err := p.QueryRow(ctx, "SELECT count(*) FROM library WHERE user_id=$1", uid).Scan(&count); err != nil || count != 1 {
		t.Fatal("invented library membership", count, err)
	}
	if w := request("PUT", "/api/up-next/order", `{"ids":["foreign:invisible"]}`, true); w.Code != 400 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("PUT", "/api/settings", `{"anime_view":"show","timezone":"UTC","language":"en","metadata_language":"en","country":"TR","anime_title":"romaji"}`, true); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	var order string
	if err := p.QueryRow(ctx, "SELECT settings->'up_next_order'->>0 FROM users WHERE id=$1", uid).Scan(&order); err != nil || order != target {
		t.Fatal("settings lost card order", order, err)
	}
	exec("UPDATE library SET anime_view='native' WHERE user_id=$1", uid)
	check(id, id+"2", "", false)
	if got := cards()[0].PosterPath; got != "/native.jpg" {
		t.Fatalf("native card poster = %q", got)
	}
	exec("UPDATE library SET anime_view=NULL WHERE user_id=$1", uid)
	// Unknown/unaired/special counterparts never become a playable Up next card.
	for _, update := range []string{"air_date=NULL", "air_date='2999-01-01'", "air_date='2020-01-01',kind='special'"} {
		exec("UPDATE episodes SET "+update+" WHERE id=$1", target+"2")
		check(id, id+"2", "", true)
	}
	exec("UPDATE episodes SET kind='regular' WHERE id=$1", target+"2")
	exec("UPDATE mappings SET requires_consent=true WHERE id=$1", mid)
	// No consent means no derived skip; first native episode remains actionable.
	check(id, id+"1", "", true)
	exec("UPDATE mappings SET requires_consent=false WHERE id=$1", mid)
	exec("INSERT INTO history(user_id,catalog_id,episode_id) VALUES($1,$2,$3)", uid, id, id+"2")
	if got := cards(); len(got) != 0 {
		t.Fatal("caught-up source still in Up next", got)
	}
	otherCards, err := buildUpNext(ctx, p, other, "UTC")
	if err != nil || len(otherCards) != 0 {
		t.Fatal("cross-user cards", otherCards, err)
	}
	if err = p.QueryRow(ctx, "SELECT count(*) FROM history WHERE user_id=$1", uid).Scan(&count); err != nil || count != 2 {
		t.Fatal("read changed facts", count, err)
	}
	exec("DELETE FROM history WHERE user_id=$1", uid)
	exec("UPDATE library SET base='watching',rating=8 WHERE user_id=$1 AND catalog_id=$2", uid, id)
	trackVirtual := func(eid string, watch bool, extra string) *httptest.ResponseRecorder {
		return request("POST", "/api/library/"+target+"/track", fmt.Sprintf(`{"view_anchor":%q,"episode_id":%q,"watched":%t,"date":{"day":"2026-02-03"}%s}`, id, eid, watch, extra), true)
	}
	if w := trackVirtual(target+"2", true, ",\"preview\":true"); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := trackVirtual(target+"2", true, ""); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := trackVirtual(target+"1", true, ""); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}

	exec(`UPDATE catalog SET metadata='{"":{"number_of_episodes":2,"status":"Ended"}}' WHERE id=$1`, target)
	for _, scope := range []string{`"whole":true`, `"season":2`} {
		w := request("POST", "/api/library/"+target+"/track", fmt.Sprintf(`{"view_anchor":%q,%s,"watched":true,"date":{}}`, id, scope), true)
		if w.Code != 200 {
			t.Fatal("virtual bulk tracking", w.Code, w.Body.String())
		}
	}
	var recordedDay string
	if err = p.QueryRow(ctx, "SELECT watched_date::text FROM history WHERE user_id=$1 AND episode_id=$2", uid, target+"2").Scan(&recordedDay); err != nil || recordedDay != "2026-02-03" {
		t.Fatal("bulk overwrote original alternate date", recordedDay, err)
	}
	var base string
	var rating int
	if err = p.QueryRow(ctx, "SELECT base,rating FROM library WHERE user_id=$1 AND catalog_id=$2", uid, id).Scan(&base, &rating); err != nil || base != "completed" || rating != 8 {
		t.Fatal("native status/rating", base, rating, err)
	}
	if err = p.QueryRow(ctx, "SELECT count(*) FROM library WHERE user_id=$1", uid).Scan(&count); err != nil || count != 1 {
		t.Fatal("alternate invented membership", count, err)
	}
	exported := request("GET", "/api/export", "", true)
	savedBackup := exported.Body.String()
	var backup portable.Snapshot
	if exported.Code != 200 {
		t.Fatal(exported.Code, exported.Body.String())
	}
	if err = json.Unmarshal(exported.Body.Bytes(), &backup); err != nil {
		t.Fatal(err)
	}
	if err = backup.Validate(); err != nil {
		t.Fatal("virtual history export cannot restore", err)
	}
	if len(backup.AlternateViews) != 1 || len(backup.History) != 2 {
		t.Fatal("alternate export missing records", backup.AlternateViews, len(backup.History))
	}
	if w := trackVirtual(target+"2", false, ",\"return_watching\":true"); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err = p.QueryRow(ctx, "SELECT base FROM library WHERE user_id=$1 AND catalog_id=$2", uid, id).Scan(&base); err != nil || base != "watching" {
		t.Fatal("native unwatch status", base, err)
	}
	exec("UPDATE mappings SET requires_consent=true WHERE id=$1", mid)
	if w := trackVirtual(target+"2", true, ""); w.Code != 409 {
		t.Fatal("unconfirmed alternate write", w.Code, w.Body.String())
	}
	exec("UPDATE sessions SET user_id=$2 WHERE token_hash=$1", sum, other)
	if w := trackVirtual(target+"2", true, ""); w.Code != 404 {
		t.Fatal("foreign anchor authorized alternate write", w.Code, w.Body.String())
	}

	exec("UPDATE sessions SET user_id=$2 WHERE token_hash=$1", sum, uid)
	exec("DELETE FROM mappings WHERE id=$1", mid)
	view := request("GET", "/api/library/"+id+"/alternate", "", true)
	if view.Code != 200 || !strings.Contains(view.Body.String(), target+"1") {
		t.Fatal("mapping correction hid recorded alternate history", view.Code, view.Body.String())
	}
	if w := trackVirtual(target+"1", false, ""); w.Code != 200 {
		t.Fatal("cannot remove retained own history after correction", w.Code, w.Body.String())
	}
	exported = request("GET", "/api/export", "", true)
	if err = json.Unmarshal(exported.Body.Bytes(), &backup); err != nil {
		t.Fatal(err)
	}
	if err = backup.Validate(); err != nil {
		t.Fatal("alternate tombstones not portable", err)
	}

	exec("UPDATE sessions SET user_id=$2 WHERE token_hash=$1", sum, other)
	if w := request("GET", "/api/export", "", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	preview := request("POST", "/api/restore/preview", savedBackup, true)
	var restore struct {
		ID string `json:"id"`
	}
	if preview.Code != 200 {
		t.Fatal(preview.Code, preview.Body.String())
	}
	if err = json.Unmarshal(preview.Body.Bytes(), &restore); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if w := request("POST", "/api/restore/"+restore.ID+"/apply", "{}", true); w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if err = p.QueryRow(ctx, "SELECT count(*) FROM history WHERE user_id=$1", other).Scan(&count); err != nil || count != 2 {
		t.Fatal("alternate restore history", count, err)
	}
	if err = p.QueryRow(ctx, "SELECT count(*) FROM library WHERE user_id=$1", other).Scan(&count); err != nil || count != 1 {
		t.Fatal("alternate restore fabricated library rows", count, err)
	}

}
