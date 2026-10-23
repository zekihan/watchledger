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

func TestPrivateTrackingAndStatusLocks(t *testing.T) {
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
	if w := request("GET", "/api/library", "", false); w.Code != 401 {
		t.Fatal("unauthenticated read", w.Code)
	}
	if w := request("PUT", "/api/library/"+id, `{"base":"watching"}`, true); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("POST", "/api/library/"+id+"/track", fmt.Sprintf(`{"episode_id":%q,"watched":true,"date":{"day":"2026-01-02"}}`, id+"2"), true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var count int
	if e = p.QueryRow(ctx, "SELECT count(*) FROM history WHERE user_id=$1", uid).Scan(&count); e != nil || count != 1 {
		t.Fatal("default marked earlier", count, e)
	}
	if w := request("POST", "/api/library/"+id+"/track", `{"whole":true,"watched":true,"date":{}}`, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var day, base string
	if e = p.QueryRow(ctx, "SELECT watched_date::text FROM history WHERE user_id=$1 AND episode_id=$2", uid, id+"2").Scan(&day); e != nil || day != "2026-01-02" {
		t.Fatal("date overwritten", day, e)
	}
	if e = p.QueryRow(ctx, "SELECT base FROM library WHERE user_id=$1 AND catalog_id=$2", uid, id).Scan(&base); e != nil || base != "completed" {
		t.Fatal(base, e)
	}
	if w := request("POST", "/api/library/"+id+"/track", fmt.Sprintf(`{"episode_id":%q,"watched":false,"date":{}}`, id+"2"), true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if e = p.QueryRow(ctx, "SELECT base FROM library WHERE user_id=$1 AND catalog_id=$2", uid, id).Scan(&base); e != nil || base != "completed" {
		t.Fatal("unwatch silently changed status", base, e)
	}
	var foreign string
	if e = p.QueryRow(ctx, "INSERT INTO statuses(user_id,name,base) VALUES($1,'Private','watching') RETURNING id", other).Scan(&foreign); e != nil {
		t.Fatal(e)
	}
	if w := request("PUT", "/api/library/"+id, fmt.Sprintf(`{"base":"watching","status_id":%q}`, foreign), true); w.Code != 404 {
		t.Fatal("foreign status accepted", w.Code)
	}
	w := request("POST", "/api/lists", `{"name":"Private smart","kind":"smart","sort":"title","rule":{"op":"not","children":[{"op":"eq","field":"service","value":"8"}]},"members":[]}`, true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request("POST", "/api/lists/preview", `{"rule":{"op":"not","children":[{"op":"eq","field":"service","value":"8"}]},"sort":"title"}`, true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"count":0`) {
		t.Fatal("unknown availability matched", w.Code, w.Body.String())
	}
	w = request("POST", "/api/lists", `{"name":"Foreign dependency","kind":"smart","sort":"title","rule":{"op":"eq","field":"list","value":"00000000-0000-0000-0000-000000000001"},"members":[]}`, true)
	if w.Code != 409 {
		t.Fatal("missing dependency accepted", w.Code)
	}

	backup := request("GET", "/api/export", "", true)
	if backup.Code != 200 {
		t.Fatal(backup.Code, backup.Body.String())
	}
	preview := request("POST", "/api/restore/preview", backup.Body.String(), true)
	if preview.Code != 200 {
		t.Fatal(preview.Code, preview.Body.String())
	}
	var previewData struct {
		ID string `json:"id"`
	}
	if e = json.Unmarshal(preview.Body.Bytes(), &previewData); e != nil {
		t.Fatal(e)
	}
	for range 2 {
		applied := request("POST", "/api/restore/"+previewData.ID+"/apply", `{}`, true)
		if applied.Code != 200 {
			t.Fatal("restore", applied.Code, applied.Body.String())
		}
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM history WHERE user_id=$1", uid).Scan(&count); e != nil || count != 1 {
		t.Fatal("restore duplicated history", count, e)
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM history WHERE user_id=$1", other).Scan(&count); e != nil || count != 0 {
		t.Fatal("restore escaped account", count, e)
	}
	if w := request("POST", "/api/admin/mappings", `{}`, true); w.Code != 403 {
		t.Fatal("non-admin mapping access", w.Code)
	}

	for _, path := range []string{"/api/dashboard", "/api/statistics", "/api/calendar?month=2020-01"} {
		w := request("GET", path, "", true)
		if w.Code != 200 {
			t.Fatal(path, w.Code, w.Body.String())
		}
	}

	created := request("POST", "/api/statuses", `{"Name":"Focused","Base":"watching"}`, true)
	if created.Code != 200 {
		t.Fatal(created.Code, created.Body.String())
	}
	var custom struct {
		ID string `json:"id"`
	}
	if e = json.Unmarshal(created.Body.Bytes(), &custom); e != nil {
		t.Fatal(e)
	}
	if w := request("PUT", "/api/library/"+id, fmt.Sprintf(`{"base":"watching","status_id":%q,"rating":8}`, custom.ID), true); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("POST", "/api/statuses", fmt.Sprintf(`{"ID":%q,"Name":"Focused","Base":"completed"}`, custom.ID), true); w.Code != 409 {
		t.Fatal("used mapping changed", w.Code)
	}
	if w := request("POST", "/api/statuses", fmt.Sprintf(`{"ID":%q,"Name":"Renamed","Base":"watching"}`, custom.ID), true); w.Code != 200 {
		t.Fatal("rename broke identity", w.Code)
	}
	if w := request("POST", "/api/library/"+id+"/track", fmt.Sprintf(`{"episode_id":%q,"watched":true,"date":{}}`, id+"1"), true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var retained string
	if e = p.QueryRow(ctx, "SELECT status_id::text FROM library WHERE user_id=$1 AND catalog_id=$2", uid, id).Scan(&retained); e != nil || retained != custom.ID {
		t.Fatal("same-base custom status lost", retained, e)
	}
	if w := request("PUT", "/api/library/"+id, `{"base":"watching","rating":0}`, true); w.Code != 400 {
		t.Fatal("invalid personal rating accepted", w.Code)
	}
	if w := request("DELETE", "/api/statuses/"+custom.ID, `{"replacement":"planned"}`, true); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	if e = p.QueryRow(ctx, "SELECT base FROM library WHERE user_id=$1 AND catalog_id=$2", uid, id).Scan(&base); e != nil || base != "planned" {
		t.Fatal("status replacement failed", base, e)
	}
	manual := request("POST", "/api/lists", fmt.Sprintf(`{"name":"Manual","kind":"manual","sort":"title","members":[%q]}`, id), true)
	if manual.Code != 200 {
		t.Fatal(manual.Code, manual.Body.String())
	}
	var manualID struct {
		ID string `json:"id"`
	}
	if e = json.Unmarshal(manual.Body.Bytes(), &manualID); e != nil {
		t.Fatal(e)
	}
	dependent := request("POST", "/api/lists", fmt.Sprintf(`{"name":"Dependent","kind":"smart","sort":"rating","rule":{"op":"eq","field":"list","value":%q},"members":[]}`, manualID.ID), true)
	if dependent.Code != 200 {
		t.Fatal(dependent.Code, dependent.Body.String())
	}
	if w := request("DELETE", "/api/lists/"+manualID.ID, `{}`, true); w.Code != 409 {
		t.Fatal("referenced list deleted", w.Code)
	}
	renamed := request("POST", "/api/lists", fmt.Sprintf(`{"id":%q,"name":"Renamed list","kind":"manual","sort":"title","members":[%q]}`, manualID.ID, id), true)
	if renamed.Code != 200 {
		t.Fatal(renamed.Code, renamed.Body.String())
	}
	matching := request("POST", "/api/lists/preview", fmt.Sprintf(`{"rule":{"op":"eq","field":"list","value":%q},"sort":"title"}`, manualID.ID), true)
	if matching.Code != 200 || !strings.Contains(matching.Body.String(), `"count":1`) {
		t.Fatal("rename broke references", matching.Code, matching.Body.String())
	}
	forgedRequest := httptest.NewRequestWithContext(ctx, "PUT", "/api/library/"+id, strings.NewReader(`{"base":"dropped"}`))
	forgedRequest.Header.Set("Origin", "https://evil.example")
	forgedRequest.AddCookie(&http.Cookie{Name: "wl_session", Value: sid})
	forgedResponse := httptest.NewRecorder()
	handler.ServeHTTP(forgedResponse, forgedRequest)
	if forgedResponse.Code != 403 {
		t.Fatal("cross-site mutation accepted")
	}

	if w := request("POST", "/api/library/"+id+"/track", `{"season":1,"watched":true,"date":{}}`, true); w.Code != 200 {
		t.Fatal("season tracking", w.Code, w.Body.String())
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM history WHERE user_id=$1 AND catalog_id=$2", uid, id).Scan(&count); e != nil || count != 2 {
		t.Fatal("season progress", count, e)
	}
	if w := request("POST", "/api/library/"+id+"/track", `{"whole":true,"watched":false,"date":{}}`, true); w.Code != 200 {
		t.Fatal("whole unwatch", w.Code, w.Body.String())
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM history WHERE user_id=$1 AND catalog_id=$2", uid, id).Scan(&count); e != nil || count != 0 {
		t.Fatal("whole unwatch preserved flags", count, e)
	}
	if w := request("PUT", "/api/settings", `{"language":"en","country":"TR","timezone":"Europe/Istanbul","anime_view":"native"}`, true); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, e = p.Exec(ctx, "UPDATE episodes SET air_date=NULL,air_time='2020-01-31T23:30:00Z' WHERE id=$1", id+"1"); e != nil {
		t.Fatal(e)
	}
	january := request("GET", "/api/calendar?month=2020-01", "", true)
	february := request("GET", "/api/calendar?month=2020-02", "", true)
	if january.Code != 200 || february.Code != 200 || strings.Contains(january.Body.String(), `"id":"`+id+`1"`) || !strings.Contains(february.Body.String(), `"id":"`+id+`1"`) {
		t.Fatal("timezone month boundary", january.Body.String(), february.Body.String())
	}

	if w := request("POST", "/api/library/"+id+"/track", `{"whole":true,"date":{}}`, true); w.Code != 400 {
		t.Fatal("missing watched flag accepted", w.Code)
	}

	var ownNotice, foreignNotice string
	for _, fixture := range []struct {
		user   string
		target *string
	}{{uid, &ownNotice}, {other, &foreignNotice}} {
		if e = p.QueryRow(ctx, "INSERT INTO notices(user_id,kind,data) VALUES($1,'mapping_changed','{}') RETURNING id", fixture.user).Scan(fixture.target); e != nil {
			t.Fatal(e)
		}
	}
	if w := request("GET", "/api/notices", "", true); w.Code != 200 || !strings.Contains(w.Body.String(), ownNotice) || strings.Contains(w.Body.String(), foreignNotice) {
		t.Fatal("notice isolation", w.Code, w.Body.String())
	}
	if w := request("POST", "/api/notices/"+foreignNotice+"/read", "{}", true); w.Code != 404 {
		t.Fatal("foreign notice acknowledged", w.Code)
	}
	if w := request("POST", "/api/notices/invalid/read", "{}", true); w.Code != 400 {
		t.Fatal("invalid notice", w.Code)
	}
	if w := request("POST", "/api/notices/"+ownNotice+"/read", "{}", false); w.Code != 401 {
		t.Fatal("unauthenticated notice", w.Code)
	}
	for range 2 {
		if w := request("POST", "/api/notices/"+ownNotice+"/read", "{}", true); w.Code != 204 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if w := request("GET", "/api/notices", "", true); w.Code != 200 || strings.Contains(w.Body.String(), ownNotice) {
		t.Fatal("read notice still unread", w.Code, w.Body.String())
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM notices WHERE id=$1 AND read_at IS NOT NULL", ownNotice).Scan(&count); e != nil || count != 1 {
		t.Fatal("notice deleted rather than acknowledged", count, e)
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM notices WHERE id=$1 AND read_at IS NULL", foreignNotice).Scan(&count); e != nil || count != 1 {
		t.Fatal("foreign notice changed", count, e)
	}

}
