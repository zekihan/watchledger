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

func TestStatisticsCanonicalNativeUnitsAndPartialProgress(t *testing.T) {
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

	if _, e = p.Exec(ctx, `UPDATE users SET settings='{"timezone":"Europe/Istanbul","metadata_language":"tr"}' WHERE id=$1`, uid); e != nil {
		t.Fatal(e)
	}
	if _, e = p.Exec(ctx, `UPDATE catalog SET metadata='{"en":{"runtime":999,"genres":[{"id":1,"name":"Drama"}]},"tr":{"runtime":888,"genres":[{"id":1,"name":"Dram"}]}}' WHERE id=$1`, id); e != nil {
		t.Fatal(e)
	}
	if _, e = p.Exec(ctx, `UPDATE episodes SET metadata='{"runtime":24}' WHERE id=$1`, id+"1"); e != nil {
		t.Fatal(e)
	}

	exec := func(query string, args ...interface{}) {
		t.Helper()
		if _, err := p.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	native := id + ":native"
	exec(`INSERT INTO catalog(id,source,source_id,kind,title,metadata) VALUES($1,'anidb',$1,'anime','Synthetic native','{"":{"genres":[{"name":"Native genre"}]}}')`, native)
	exec(`INSERT INTO episodes(id,catalog_id,source,source_id,season,number,metadata) VALUES($1,$2,'anidb',$1,1,1,'{"runtime":24}')`, native+"1", native)
	exec("INSERT INTO library(user_id,catalog_id,base) VALUES($1,$2,'watching')", uid, native)
	edges, revision, _, _ := mapping.Canonical([]mapping.Edge{{From: native + "1", To: id + "1"}, {From: native + "1", To: id + "2"}})
	raw, _ := json.Marshal(edges)
	var mid string
	if err := p.QueryRow(ctx, `INSERT INTO mappings(revision,provenance,relationships,requires_consent,reviewed) VALUES($1,'{"source":"synthetic"}',$2,true,true) RETURNING id`, revision, raw).Scan(&mid); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = p.Exec(ctx, "DELETE FROM mapping_consents WHERE mapping_id=$1", mid)
		_, _ = p.Exec(ctx, "DELETE FROM mappings WHERE id=$1", mid)
	}()
	exec("INSERT INTO mapping_consents(user_id,mapping_id,revision) VALUES($1,$2,$3)", uid, mid, revision)
	exec("INSERT INTO history(user_id,catalog_id,episode_id,watched_date) VALUES($1,$2,$3,'2026-01-31')", uid, id, id+"1")
	read := func() statisticsResult {
		t.Helper()
		w := request("GET", "/api/statistics", "", true)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var result statisticsResult
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	got := read()
	if got.Episodes != 0 || got.PartialNativeEpisodes != 1 || got.PartialSourceRecords != 1 || got.EstimatedMinutes != 0 || len(got.Monthly) != 0 {
		t.Fatal("partial native counted as complete", got)
	}
	exec("INSERT INTO history(user_id,catalog_id,episode_id) VALUES($1,$2,$3)", uid, id, id+"2")
	got = read()
	if got.Episodes != 1 || got.PartialNativeEpisodes != 0 || got.EstimatedMinutes != 24 || len(got.Monthly) != 0 || got.DerivedDateUnits != 0 {
		t.Fatal("unknown part date became precise", got)
	}
	exec("UPDATE history SET watched_time='2026-01-31T22:30:00Z' WHERE user_id=$1 AND episode_id=$2", uid, id+"2")
	exec(`UPDATE users SET settings='{"timezone":"Europe/Istanbul","anime_view":"show"}' WHERE id=$1`, uid)
	got = read()
	if got.Episodes != 1 || len(got.Monthly) != 1 || got.Monthly[0] != (statisticMonth{Month: "2026-02", Count: 1}) || got.DerivedDateUnits != 1 {
		t.Fatal("native completion month", got)
	}
	if len(got.Genres) != 1 || got.Genres[0].Name != "Native genre" || got.Genres[0].Count != 1 {
		t.Fatal("duplicate source genre", got.Genres)
	}
	exec(`UPDATE users SET settings=jsonb_set(settings,'{anime_view}','"native"') WHERE id=$1`, uid)
	if nativeView := read(); nativeView.Episodes != got.Episodes || nativeView.EstimatedMinutes != got.EstimatedMinutes {
		t.Fatal("structure inflated totals", nativeView, got)
	}
	// A direct native fact wins even when its date is unknown, and duplicate
	// source-representation facts must not inflate count/runtime/genre totals.
	exec("INSERT INTO history(user_id,catalog_id,episode_id) VALUES($1,$2,$3)", uid, native, native+"1")
	got = read()
	if got.Episodes != 1 || got.EstimatedMinutes != 24 || len(got.Monthly) != 0 || got.DerivedDateUnits != 0 {
		t.Fatal("original native date overwritten in statistics", got)
	}
	var count int
	if err := p.QueryRow(ctx, "SELECT count(*) FROM history WHERE user_id=$1", uid).Scan(&count); err != nil || count != 3 {
		t.Fatal("statistics rewrote history", count, err)
	}
	exec("DELETE FROM mapping_consents WHERE user_id=$1 AND mapping_id=$2", uid, mid)
	got = read()
	if got.Episodes != 3 || got.UnresolvedLinkedRecords != 2 {
		t.Fatal("unconfirmed deduplication was silently applied", got)
	}
	exec("UPDATE sessions SET user_id=$2 WHERE token_hash=$1", sum, other)
	if got = read(); got.Episodes != 0 || got.PartialNativeEpisodes != 0 || len(got.Monthly) != 0 {
		t.Fatal("statistics leaked another account", got)
	}
	exec("UPDATE sessions SET user_id=$2 WHERE token_hash=$1", sum, uid)
	exec("INSERT INTO mapping_consents(user_id,mapping_id,revision) VALUES($1,$2,$3)", uid, mid, revision)
	exec("INSERT INTO episodes(id,catalog_id,source,source_id,season,number) VALUES($1,$2,'fixture',$1,1,3)", id+"3", id)
	combined := []mapping.Edge{}
	for n := 2; n <= 3; n++ {
		eid := native + fmt.Sprint(n)
		exec("INSERT INTO episodes(id,catalog_id,source,source_id,season,number) VALUES($1,$2,'anidb',$1,1,$3)", eid, native, n)
		combined = append(combined, mapping.Edge{From: eid, To: id + "3"})
	}
	combined, combinedRev, _, _ := mapping.Canonical(combined)
	combinedRaw, _ := json.Marshal(combined)
	var combinedID string
	if err := p.QueryRow(ctx, `INSERT INTO mappings(revision,provenance,relationships,requires_consent,reviewed) VALUES($1,'{"source":"synthetic"}',$2,true,true) RETURNING id`, combinedRev, combinedRaw).Scan(&combinedID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = p.Exec(ctx, "DELETE FROM mapping_consents WHERE mapping_id=$1", combinedID)
		_, _ = p.Exec(ctx, "DELETE FROM mappings WHERE id=$1", combinedID)
	}()
	exec("INSERT INTO mapping_consents(user_id,mapping_id,revision) VALUES($1,$2,$3)", uid, combinedID, combinedRev)
	exec("INSERT INTO history(user_id,catalog_id,episode_id,watched_date) VALUES($1,$2,$3,'2026-04-03')", uid, id, id+"3")
	got = read()
	if got.Episodes != 3 || got.UnknownRuntime != 2 || got.EstimatedMinutes != 24 || len(got.Monthly) != 1 || got.Monthly[0] != (statisticMonth{Month: "2026-04", Count: 2}) {
		t.Fatal("combined show event did not count native units", got)
	}

}
