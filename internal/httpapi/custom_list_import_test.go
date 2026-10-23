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

func TestCustomListImportPreservesAppEditsAndPortableSource(t *testing.T) {
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
	firstNumber := fmt.Sprint(time.Now().UnixNano())
	secondNumber := fmt.Sprint(time.Now().UnixNano())
	first, second := "tmdb:movie:"+firstNumber, "tmdb:movie:"+secondNumber
	for _, number := range []string{firstNumber, secondNumber} {
		if _, e = p.Exec(ctx, `INSERT INTO catalog(id,source,source_id,kind,title) VALUES($1,'tmdb',$2,'movie','Synthetic movie')`, "tmdb:movie:"+number, number); e != nil {
			t.Fatal(e)
		}
		if _, e = p.Exec(ctx, "INSERT INTO library(user_id,catalog_id,base) VALUES($1,$2,'planned')", uid, "tmdb:movie:"+number); e != nil {
			t.Fatal(e)
		}
	}
	// An item without a TMDB ID may use this account's explicit backup identity, never a title match.
	backup := fmt.Sprintf(`{"movies":[{"movie":{"title":"Synthetic B","ids":{"simkl":202,"tmdb":%q}},"status":"plantowatch"}]}`, secondNumber)
	if w := request("POST", "/api/simkl/preview", backup, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	raw := fmt.Sprintf(`{"id":42,"name":%q,"media_type":"movies","type":"auto","pagination":{"page":1,"limit":50,"total_items":2,"total_pages":1},"items":[{"title":"Synthetic A","position":9,"ids":{"simkl_id":101,"tmdb":%q}},{"title":"Synthetic B","position":1,"ids":{"simkl_id":202}}],"untranslated_rule":{"synthetic":true}}`, strings.Repeat("ö", 75), firstNumber)
	w := request("POST", "/api/simkl/custom-lists/preview", raw, true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var preview struct {
		ID         string `json:"id"`
		Unresolved int    `json:"unresolved"`
		Action     string `json:"action"`
	}
	if e = json.Unmarshal(w.Body.Bytes(), &preview); e != nil || preview.Unresolved != 0 || preview.Action != "create_manual_snapshot" {
		t.Fatal(e, w.Body.String())
	}
	route := "/api/simkl/custom-lists/" + preview.ID + "/apply"
	if w = request("POST", route, "{}", true); w.Code != 428 {
		t.Fatal("backup not required", w.Code, w.Body.String())
	}
	if w = request("GET", "/api/export", "", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	// A preview cannot silently change from complete to partial after a library edit.
	if _, e = p.Exec(ctx, "DELETE FROM library WHERE user_id=$1 AND catalog_id=$2", uid, first); e != nil {
		t.Fatal(e)
	}
	if w = request("POST", route, "{}", true); w.Code != 409 {
		t.Fatal("changed membership was applied", w.Code, w.Body.String())
	}
	if _, e = p.Exec(ctx, "INSERT INTO library(user_id,catalog_id,base) VALUES($1,$2,'planned')", uid, first); e != nil {
		t.Fatal(e)
	}
	for n := 0; n < 2; n++ {
		if w = request("POST", route, "{}", true); w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	var listID, kind string
	if e = p.QueryRow(ctx, "SELECT id,kind FROM lists WHERE user_id=$1 AND source_id='42'", uid).Scan(&listID, &kind); e != nil || kind != "manual" {
		t.Fatal("source rule executed or list missing", kind, e)
	}
	var members []string
	if e = p.QueryRow(ctx, "SELECT array_agg(catalog_id ORDER BY position) FROM list_members WHERE user_id=$1 AND list_id=$2", uid, listID).Scan(&members); e != nil || len(members) != 2 || members[0] != first || members[1] != second {
		t.Fatal("source response order changed", members, e)
	}
	var count int
	if e = p.QueryRow(ctx, "SELECT count(*) FROM history WHERE user_id=$1", uid).Scan(&count); e != nil || count != 0 {
		t.Fatal("membership invented watch history", count, e)
	}
	if w = request("GET", "/api/imports/"+preview.ID+"/archive", "", true); w.Code != 200 || !strings.Contains(w.Body.String(), "untranslated_rule") {
		t.Fatal("original rule archive lost", w.Code, w.Body.String())
	}
	edit := fmt.Sprintf(`{"id":%q,"name":"App edited","kind":"manual","sort":"title","members":[%q]}`, listID, second)
	if w = request("POST", "/api/lists", edit, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = request("POST", "/api/simkl/custom-lists/preview", raw, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = request("POST", route, "{}", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var name string
	if e = p.QueryRow(ctx, "SELECT name FROM lists WHERE id=$1", listID).Scan(&name); e != nil || name != "App edited" {
		t.Fatal("app name overwritten", name, e)
	}
	if e = p.QueryRow(ctx, "SELECT array_agg(catalog_id ORDER BY position) FROM list_members WHERE user_id=$1 AND list_id=$2", uid, listID).Scan(&members); e != nil || len(members) != 1 || members[0] != second {
		t.Fatal("removed member resurrected", members, e)
	}
	// Unresolved lists are archived atomically; no partial empty list is created.
	unresolved := `{"id":43,"name":"Unresolved","media_type":"movies","pagination":{"page":1,"limit":50,"total_items":1,"total_pages":1},"items":[{"title":"Unknown source","ids":{"simkl_id":999}}]}`
	if w = request("POST", "/api/simkl/custom-lists/preview", unresolved, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var pending struct {
		ID string `json:"id"`
	}
	if e = json.Unmarshal(w.Body.Bytes(), &pending); e != nil {
		t.Fatal(e)
	}
	if w = request("POST", "/api/simkl/custom-lists/"+pending.ID+"/apply", "{}", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM lists WHERE user_id=$1", uid).Scan(&count); e != nil || count != 1 {
		t.Fatal("partial list fabricated", count, e)
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM import_reviews WHERE user_id=$1 AND import_id=$2 AND NOT resolved", uid, pending.ID).Scan(&count); e != nil || count != 1 {
		t.Fatal("unresolved source lost", count, e)
	}

	emptySource := `{"id":44,"name":"Deleted source","media_type":"movies","pagination":{"page":1,"limit":50,"total_items":0,"total_pages":1},"items":[]}`
	w = request("POST", "/api/simkl/custom-lists/preview", emptySource, true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var emptyPreview struct {
		ID     string `json:"id"`
		Action string `json:"action"`
	}
	if e = json.Unmarshal(w.Body.Bytes(), &emptyPreview); e != nil {
		t.Fatal(e)
	}
	if w = request("POST", "/api/simkl/custom-lists/"+emptyPreview.ID+"/apply", "{}", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var deletedList string
	if e = p.QueryRow(ctx, "SELECT id FROM lists WHERE user_id=$1 AND source_id='44'", uid).Scan(&deletedList); e != nil {
		t.Fatal(e)
	}
	if w = request("DELETE", "/api/lists/"+deletedList, "{}", true); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request("POST", "/api/simkl/custom-lists/preview", emptySource, true)
	if e = json.Unmarshal(w.Body.Bytes(), &emptyPreview); e != nil || emptyPreview.Action != "preserve_existing_app_deletion" {
		t.Fatal("deleted list missing from preview policy", e, w.Body.String())
	}
	if w = request("POST", "/api/simkl/custom-lists/"+emptyPreview.ID+"/apply", "{}", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM lists WHERE user_id=$1 AND source_id='44'", uid).Scan(&count); e != nil || count != 0 {
		t.Fatal("deleted list resurrected", count, e)
	}
	// Export/restore keeps source identity so reimport cannot duplicate a renamed app list.
	w = request("GET", "/api/export", "", true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	snapshot := w.Body.String()
	if _, e = p.Exec(ctx, "UPDATE sessions SET user_id=$2 WHERE token_hash=$1", sum, other); e != nil {
		t.Fatal(e)
	}
	if w = request("POST", route, "{}", true); w.Code != 404 {
		t.Fatal("cross-account list apply", w.Code)
	}
	if w = request("GET", "/api/imports/"+preview.ID+"/archive", "", true); w.Code != 404 {
		t.Fatal("cross-account archive", w.Code)
	}
	// Neither the first account's library nor its saved SIMKL aliases are shared.
	w = request("POST", "/api/simkl/custom-lists/preview", raw, true)
	var privatePreview struct {
		Members []listResolution `json:"members"`
	}
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if e = json.Unmarshal(w.Body.Bytes(), &privatePreview); e != nil || len(privatePreview.Members) != 2 {
		t.Fatal(e, w.Body.String())
	}
	if privatePreview.Members[0].Reason != "add_source_title_to_private_library_first" || privatePreview.Members[1].Reason != "source_identity_unresolved" {
		t.Fatal("other account's private identity leaked", privatePreview)
	}
	if w = request("GET", "/api/export", "", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request("POST", "/api/restore/preview", snapshot, true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var restore struct {
		ID string `json:"id"`
	}
	if e = json.Unmarshal(w.Body.Bytes(), &restore); e != nil {
		t.Fatal(e)
	}
	if w = request("POST", "/api/restore/"+restore.ID+"/apply", "{}", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request("POST", "/api/simkl/custom-lists/preview", raw, true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if e = json.Unmarshal(w.Body.Bytes(), &preview); e != nil || preview.Action != "preserve_existing_app_list" {
		t.Fatal("restored list lost source identity", e, w.Body.String())
	}
	if w = request("POST", "/api/simkl/custom-lists/"+preview.ID+"/apply", "{}", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM lists WHERE user_id=$1", other).Scan(&count); e != nil || count != 1 {
		t.Fatal("reimport duplicated restored list", count, e)
	}
	w = request("POST", "/api/simkl/custom-lists/preview", emptySource, true)
	if e = json.Unmarshal(w.Body.Bytes(), &emptyPreview); e != nil || emptyPreview.Action != "preserve_existing_app_deletion" {
		t.Fatal("portable source deletion lost", e, w.Body.String())
	}
	if w = request("POST", "/api/simkl/custom-lists/"+emptyPreview.ID+"/apply", "{}", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM lists WHERE user_id=$1 AND source_id='44'", other).Scan(&count); e != nil || count != 0 {
		t.Fatal("restored source deletion ignored", count, e)
	}

}
