package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/zekihan/watchledger/internal/auth"
	"github.com/zekihan/watchledger/internal/catalog"
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

func TestDatasetBridgeUsesExternalIDsAndRevisionBoundConsent(t *testing.T) {
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

	const native = "anidb:anime:934000001"
	const e1 = "anidb:episode:934000002"
	const e2 = "anidb:episode:934000003"
	if _, e = p.Exec(ctx, `INSERT INTO catalog(id,source,source_id,kind,title) VALUES($1,'anidb','934000001','anime','Synthetic native') ON CONFLICT DO NOTHING`, native); e != nil {
		t.Fatal(e)
	}
	for _, eid := range []string{e1, e2} {
		if _, e = p.Exec(ctx, "INSERT INTO episodes(id,catalog_id,source,source_id,season,number) VALUES($1,$2,'anidb',$1,1,1) ON CONFLICT DO NOTHING", eid, native); e != nil {
			t.Fatal(e)
		}
		if _, e = p.Exec(ctx, "INSERT INTO history(user_id,catalog_id,episode_id) VALUES($1,$2,$3)", uid, native, eid); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = p.Exec(ctx, "INSERT INTO library(user_id,catalog_id,base) VALUES($1,$2,'watching')", uid, native); e != nil {
		t.Fatal(e)
	}
	if _, e = p.Exec(ctx, "DELETE FROM external_episode_links WHERE source_id IN(934000004,934000007)"); e != nil {
		t.Fatal(e)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/find/934000004":
			if r.URL.Query().Get("external_source") != "tvdb_id" {
				t.Error("wrong external source")
			}
			_, _ = fmt.Fprint(w, `{"tv_episode_results":[{"id":934000006,"show_id":934000005,"season_number":3,"episode_number":7}]}`)
		case "/find/934000007":
			_, _ = fmt.Fprint(w, `{"tv_episode_results":[{"id":934000008,"show_id":934000005,"season_number":3,"episode_number":8}]}`)
		case "/tv/934000005":
			_, _ = fmt.Fprint(w, `{"id":934000005,"name":"Synthetic target","overview":"Synthetic","seasons":[{"season_number":3}]}`)
		case "/tv/934000005/season/3":
			_, _ = fmt.Fprint(w, `{"episodes":[{"id":934000006,"episode_number":7},{"id":934000008,"episode_number":8}]}`)
		default:
			t.Error("guessed provider numbering", r.URL.Path)
			http.Error(w, "unexpected", 404)
		}
	}))
	defer upstream.Close()
	provider := catalog.New("synthetic")
	provider.BaseURL = upstream.URL
	app := &API{DB: p, Catalog: &catalog.Cache{DB: p, Provider: provider}}
	mux := http.NewServeMux()
	app.routes(mux)
	handler = identity.Middleware(mux)
	raw := `{"version":2,"generated_at":"2026-10-21T00:00:00Z","sources":{"mapping":{"revision":"synthetic"},"anidb":{"titles_dump":"2026-10-21"}},"attribution":{"license":"ODbL-1.0","license_url":"https://opendatacommons.org/licenses/odbl/1-0/","contents_license":"DbCL-1.0","contents_license_url":"https://opendatacommons.org/licenses/dbcl/1-0/","notice":"Synthetic"},"anime":[{"anidb_id":934000001,"type":"TV","tvdb_id":90,"last":{"regular":2},"episodes_listed":true,"episodes":[{"kind":"regular","start":1,"end":1,"anidb_episode_ids":[934000002],"tvdb":[{"season":9,"episode":20,"episode_ids":[934000004]}]},{"kind":"regular","start":2,"end":2,"anidb_episode_ids":[934000003],"tvdb":[{"season":9,"episode":20,"episode_ids":[934000004]}]}]}],"entries":[]}`
	sha := fmt.Sprintf("%x", sha256.Sum256([]byte(raw)))
	route := "/api/admin/mapping-datasets?version=vsynthetic&checksum=" + sha
	if w := request("POST", route, raw, true); w.Code != 403 {
		t.Fatal("nonadmin dataset import", w.Code)
	}
	if _, e = p.Exec(ctx, "UPDATE users SET admin=true WHERE id=$1", uid); e != nil {
		t.Fatal(e)
	}
	w := request("POST", route, raw, true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	type proposal struct {
		ID               string `json:"proposal_id"`
		ExistingID       string `json:"existing_id"`
		ExistingRevision string `json:"existing_revision"`
		Revision         string
		Relationships    []mapping.Edge
		Requires         bool `json:"requires_consent"`
	}
	resolve := func(checksum string) proposal {
		t.Helper()
		w := request("POST", "/api/admin/mapping-datasets/"+checksum+"/anime/934000001/resolve", "{}", true)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var v proposal
		if e = json.Unmarshal(w.Body.Bytes(), &v); e != nil {
			t.Fatal(e)
		}
		return v
	}
	first := resolve(sha)
	if !first.Requires || len(first.Relationships) != 2 || first.Relationships[0].To != "tmdb:episode:934000006" {
		t.Fatal("TVDB numbers treated as TMDB numbers or split lost", first)
	}
	save := func(v proposal) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]interface{}{"id": v.ExistingID, "expected_revision": v.ExistingRevision, "proposal_id": v.ID, "relationships": v.Relationships, "provenance": map[string]interface{}{"source": "animap", "version": "vsynthetic"}})
		return request("POST", "/api/admin/mappings", string(body), true)
	}
	w = save(first)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var saved struct{ ID, Revision string }
	if e = json.Unmarshal(w.Body.Bytes(), &saved); e != nil {
		t.Fatal(e)
	}
	if w = request("POST", "/api/mappings/"+saved.ID+"/confirm", fmt.Sprintf(`{"revision":%q}`, saved.Revision), true); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	changed := strings.ReplaceAll(raw, "934000004", "934000007")
	changedSHA := fmt.Sprintf("%x", sha256.Sum256([]byte(changed)))
	if w = request("POST", "/api/admin/mapping-datasets?version=vsynthetic2&checksum="+changedSHA, changed, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	second := resolve(changedSHA)
	if second.ExistingID != saved.ID || second.ExistingRevision != saved.Revision {
		t.Fatal("dataset update duplicated mapping", second)
	}
	if w = save(second); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var currentConsent string
	if e = p.QueryRow(ctx, "SELECT revision FROM mapping_consents WHERE user_id=$1 AND mapping_id=$2", uid, saved.ID).Scan(&currentConsent); e != nil || currentConsent == second.Revision {
		t.Fatal("changed relationships inherited consent", currentConsent, e)
	}
	var count int
	if e = p.QueryRow(ctx, "SELECT count(*) FROM history WHERE user_id=$1", uid).Scan(&count); e != nil || count != 2 {
		t.Fatal("mapping mutated history", count, e)
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM mapping_revision_history WHERE mapping_id=$1", saved.ID).Scan(&count); e != nil || count != 2 {
		t.Fatal("revision history missing", count, e)
	}
	if e = p.QueryRow(ctx, "SELECT count(*) FROM notices WHERE user_id=$1 AND kind='mapping_changed'", uid).Scan(&count); e != nil || count < 1 {
		t.Fatal("correction notice missing", count, e)
	}
	tampered := second
	tampered.Relationships = append([]mapping.Edge(nil), second.Relationships...)
	tampered.Relationships[0].To = "tmdb:episode:999"
	if w = save(tampered); w.Code != 409 {
		t.Fatal("forged dataset provenance accepted", w.Code, w.Body.String())
	}
	// An intentional local override takes precedence over later dataset proposals.
	localBody, _ := json.Marshal(map[string]interface{}{"id": saved.ID, "expected_revision": second.Revision, "relationships": second.Relationships, "provenance": map[string]interface{}{"source": "local-admin-override", "version": "local1"}})
	if w = request("POST", "/api/admin/mappings", string(localBody), true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = request("POST", "/api/admin/mapping-datasets/"+changedSHA+"/anime/934000001/resolve", "{}", true); w.Code != 409 {
		t.Fatal("dataset replaced local override", w.Code)
	}
	if _, e = p.Exec(ctx, "DELETE FROM mapping_consents WHERE mapping_id=$1", saved.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = p.Exec(ctx, "DELETE FROM mapping_revision_history WHERE mapping_id=$1", saved.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = p.Exec(ctx, "DELETE FROM mappings WHERE id=$1", saved.ID); e != nil {
		t.Fatal(e)
	}
	for _, checksum := range []string{sha, changedSHA} {
		_, _ = p.Exec(ctx, "DELETE FROM mapping_proposals WHERE dataset_checksum=$1", checksum)
		_, _ = p.Exec(ctx, "DELETE FROM mapping_dataset_anime WHERE dataset_checksum=$1", checksum)
		_, _ = p.Exec(ctx, "DELETE FROM mapping_datasets WHERE checksum=$1", checksum)
	}
}
