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

func TestStatisticsPrecisionAndLocale(t *testing.T) {
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
	if _, e = p.Exec(ctx, `INSERT INTO library(user_id,catalog_id,base) VALUES($1,$2,'watching')`, uid, id); e != nil {
		t.Fatal(e)
	}
	if _, e = p.Exec(ctx, `INSERT INTO history(user_id,catalog_id,episode_id,watched_time) VALUES($1,$2,$3,'2026-01-31T22:30:00Z')`, uid, id, id+"1"); e != nil {
		t.Fatal(e)
	}
	if _, e = p.Exec(ctx, `INSERT INTO history(user_id,catalog_id,episode_id) VALUES($1,$2,$3)`, uid, id, id+"2"); e != nil {
		t.Fatal(e)
	}
	movie := id + ":movie"
	if _, e = p.Exec(ctx, `INSERT INTO catalog(id,source,source_id,kind,title,metadata) VALUES($1,'fixture',$1,'movie','Synthetic movie','{"en":{"runtime":100,"genres":[{"id":2,"name":"Comedy"}]},"tr":{"runtime":101,"genres":[{"id":2,"name":"Komedi"}]}}')`, movie); e != nil {
		t.Fatal(e)
	}
	if _, e = p.Exec(ctx, `INSERT INTO history(user_id,catalog_id,watched_date) VALUES($1,$2,'2026-01-31')`, uid, movie); e != nil {
		t.Fatal(e)
	}
	// Another account's event must not affect any metric.
	if _, e = p.Exec(ctx, `INSERT INTO history(user_id,catalog_id,watched_date) VALUES($1,$2,'2025-01-01')`, other, movie); e != nil {
		t.Fatal(e)
	}
	type month struct {
		Month string
		Count int
	}
	var stats struct {
		Movies, Episodes int
		Estimated        int `json:"estimated_minutes"`
		Unknown          int `json:"unknown_runtime"`
		Monthly          []month
		Genres           []struct {
			Name  string
			Count int
		}
	}
	w := request("GET", "/api/statistics", "", true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if e = json.Unmarshal(w.Body.Bytes(), &stats); e != nil {
		t.Fatal(e)
	}
	if stats.Movies != 1 || stats.Episodes != 2 || stats.Estimated != 125 || stats.Unknown != 1 {
		t.Fatal("incorrect private counts or episode runtime", stats)
	}
	if len(stats.Monthly) != 2 || stats.Monthly[0] != (month{"2026-01", 1}) || stats.Monthly[1] != (month{"2026-02", 1}) {
		t.Fatal("timestamp/day/unknown precision changed", stats.Monthly)
	}
	if len(stats.Genres) != 2 || stats.Genres[0].Name != "Dram" || stats.Genres[0].Count != 2 || stats.Genres[1].Name != "Komedi" {
		t.Fatal("wrong metadata locale", stats.Genres)
	}
	if _, e = p.Exec(ctx, `UPDATE episodes SET retired=true,air_date='2099-01-01' WHERE id=$1`, id+"2"); e != nil {
		t.Fatal(e)
	}
	w = request("GET", "/api/dashboard", "", true)
	var dashboard struct {
		Statistics struct{ Monthly []month }
		Upcoming   []json.RawMessage
	}
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if e = json.Unmarshal(w.Body.Bytes(), &dashboard); e != nil {
		t.Fatal(e)
	}
	if len(dashboard.Statistics.Monthly) != 2 || dashboard.Statistics.Monthly[1] != (month{"2026-02", 1}) {
		t.Fatal("dashboard disagrees on local month", dashboard)
	}
	if len(dashboard.Upcoming) != 0 {
		t.Fatal("retired episode advertised as upcoming")
	}
	if w = request("GET", "/api/statistics", "", false); w.Code != 401 {
		t.Fatal("anonymous statistics", w.Code)
	}
	// Date-only releases group by local day; timestamped releases retain their
	// actual chronological order, rather than sorting after every date-only row.
	rows := []struct {
		key        string
		day, stamp interface{}
	}{
		{"late-day", "2099-01-03", nil},
		{"later-time", nil, "2099-01-02T00:30:00Z"},
		{"earlier-time", nil, "2099-01-01T23:30:00Z"},
		{"date-only", "2099-01-02", nil},
	}
	for n, row := range rows {
		if _, e = p.Exec(ctx, `INSERT INTO episodes(id,catalog_id,source,source_id,season,number,air_date,air_time) VALUES($1,$2,'fixture',$1,2,$3,$4,$5)`, id+":"+row.key, id, n+1, row.day, row.stamp); e != nil {
			t.Fatal(e)
		}
	}
	expected := []string{id + ":date-only", id + ":earlier-time", id + ":later-time", id + ":late-day"}
	for _, path := range []string{"/api/calendar?month=2099-01", "/api/dashboard"} {
		w = request("GET", path, "", true)
		if w.Code != 200 {
			t.Fatal(path, w.Code, w.Body.String())
		}
		var releases []struct {
			ID      string
			AirDate *string    `json:"air_date"`
			AirTime *time.Time `json:"air_time"`
		}
		raw := w.Body.Bytes()
		if path == "/api/dashboard" {
			var data map[string]json.RawMessage
			if e = json.Unmarshal(raw, &data); e != nil {
				t.Fatal(e)
			}
			raw = data["upcoming"]
		}
		if e = json.Unmarshal(raw, &releases); e != nil || len(releases) != 4 {
			t.Fatal(path, string(raw), e)
		}
		for n, row := range releases {
			if row.ID != expected[n] {
				t.Fatal("releases not in local chronological order", path, releases)
			}
		}
		if releases[0].AirDate == nil || releases[0].AirTime != nil || releases[1].AirDate != nil || releases[1].AirTime == nil {
			t.Fatal("ordering fabricated release precision", releases)
		}
	}

}
