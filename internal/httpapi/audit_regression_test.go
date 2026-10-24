package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zekihan/watchledger/internal/auth"
	"github.com/zekihan/watchledger/internal/config"
	"github.com/zekihan/watchledger/internal/mapping"
	"github.com/zekihan/watchledger/internal/store"
)

type auditEnv struct {
	p       *pgxpool.Pool
	h       http.Handler
	users   [3]string
	cookies [3]string
}

func auditSetup(t *testing.T) auditEnv {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	p, e := store.Open(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(p.Close)
	if e = store.Migrate(ctx, p); e != nil {
		t.Fatal(e)
	}
	var issuer string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"issuer": issuer, "authorization_endpoint": issuer + "/authorize", "token_endpoint": issuer + "/token", "jwks_uri": issuer + "/keys", "id_token_signing_alg_values_supported": []string{"RS256"}})
	}))
	t.Cleanup(server.Close)
	issuer = server.URL
	identity, e := auth.New(ctx, p, config.Config{Issuer: issuer, PublicURL: "http://127.0.0.1:8080", ClientID: "test", SessionKey: base64.StdEncoding.EncodeToString(make([]byte, 32))})
	if e != nil {
		t.Fatal(e)
	}
	env := auditEnv{p: p, h: NewApplication("", p, identity, "")}
	for i := range 3 {
		subject := fmt.Sprintf("audit-%s-%d-%d", t.Name(), time.Now().UnixNano(), i)
		if e = p.QueryRow(ctx, "INSERT INTO users(issuer,subject,admin) VALUES('audit',$1,$2) RETURNING id", subject, i == 2).Scan(&env.users[i]); e != nil {
			t.Fatal(e)
		}
		env.cookies[i] = subject
		sum := fmt.Sprintf("%x", sha256.Sum256([]byte(subject)))
		if _, e = p.Exec(ctx, "INSERT INTO sessions(token_hash,user_id,token,expires_at) VALUES($1,$2,'{}',now()+interval '1 hour')", sum, env.users[i]); e != nil {
			t.Fatal(e)
		}
	}
	t.Cleanup(func() {
		for _, uid := range env.users {
			for _, table := range []string{"list_dependencies", "list_members", "lists", "alternate_history_views", "simkl_episode_links", "history", "history_tombstones", "library", "statuses", "imports"} {
				_, _ = p.Exec(ctx, "DELETE FROM "+table+" WHERE user_id=$1", uid)
			}
			_, _ = p.Exec(ctx, "DELETE FROM users WHERE id=$1", uid)
		}
	})

	return env
}
func (e auditEnv) req(t *testing.T, user int, method, path string, body any, want int) []byte {
	t.Helper()
	raw := []byte{}
	if body != nil {
		var err error
		raw, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	r := httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(string(raw)))
	r.Header.Set("Origin", "http://127.0.0.1:8080")
	r.AddCookie(&http.Cookie{Name: "wl_session", Value: e.cookies[user]})
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	if w.Code != want {
		t.Fatalf("%s %s: got %d want %d: %s", method, path, w.Code, want, w.Body.String())
	}
	return w.Body.Bytes()
}
func auditSnapshot(id string) map[string]any {
	return map[string]any{"version": 1, "created_at": "2026-01-01T00:00:00Z", "settings": map[string]any{},
		"catalog":  []any{map[string]any{"id": id, "source": "tmdb", "source_id": strings.Split(id, ":")[2], "kind": "movie", "title": "Synthetic audit movie", "metadata": map[string]any{"en": map[string]any{"title": "Synthetic audit movie"}}, "fetched_at": time.Now().UTC().Format(time.RFC3339)}},
		"episodes": []any{}, "statuses": []any{}, "library": []any{map[string]any{"catalog_id": id, "base": "watching", "position": 0, "added_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z"}},
		"history": []any{}, "lists": []any{}, "tombstones": []any{}}
}
func (e auditEnv) restore(t *testing.T, user int, s any) {
	t.Helper()
	if user != 2 {
		raw, _ := json.Marshal(s)
		var source map[string]any
		_ = json.Unmarshal(raw, &source)
		seed := map[string]any{"version": 1, "created_at": source["created_at"], "settings": map[string]any{}, "catalog": source["catalog"], "episodes": source["episodes"], "history": []any{}, "library": []any{}, "statuses": []any{}, "lists": []any{}, "tombstones": []any{}}
		e.restore(t, 2, seed)
	}
	e.req(t, user, "GET", "/api/export", nil, 200)
	raw := e.req(t, user, "POST", "/api/restore/preview", s, 200)
	var pre struct{ ID string }
	if err := json.Unmarshal(raw, &pre); err != nil {
		t.Fatal(err)
	}
	e.req(t, user, "POST", "/api/restore/"+pre.ID+"/apply", map[string]any{"trust_shared_metadata": user == 2}, 200)
}
func TestUnsignedRestoreCannotPoisonSharedCatalog(t *testing.T) {
	e := auditSetup(t)
	id := "tmdb:movie:881001"
	_, _ = e.p.Exec(context.Background(), "DELETE FROM catalog WHERE id=$1", id)
	s := auditSnapshot(id)
	for _, user := range []int{0, 2} {
		e.req(t, user, "GET", "/api/export", nil, 200)
		raw := e.req(t, user, "POST", "/api/restore/preview", s, 200)
		var preview struct {
			ID     string
			Shared bool `json:"shared_metadata_requires_approval"`
		}
		_ = json.Unmarshal(raw, &preview)
		if !preview.Shared {
			t.Fatal("approval warning missing")
		}
		e.req(t, user, "POST", "/api/restore/"+preview.ID+"/apply", map[string]any{}, 403)
		if user == 0 {
			e.req(t, user, "POST", "/api/restore/"+preview.ID+"/apply", map[string]any{"trust_shared_metadata": true}, 403)
		}
	}
	var exists bool
	if err := e.p.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM catalog WHERE id=$1)", id).Scan(&exists); err != nil || exists {
		t.Fatal("unauthorized cache write", err)
	}
	e.restore(t, 2, s)
	e.req(t, 1, "GET", "/api/catalog/movie/881001?language=en", nil, 200)
}
func TestRestoreRemapsCustomStatusPredicates(t *testing.T) {
	e := auditSetup(t)
	s := auditSnapshot("tmdb:movie:880002")
	st := "11111111-1111-4111-8111-111111111111"
	list := "22222222-2222-4222-8222-222222222222"
	s["statuses"] = []any{map[string]any{"id": st, "name": "Audit custom", "base": "watching", "used": true}}
	s["library"].([]any)[0].(map[string]any)["status_id"] = st
	s["lists"] = []any{map[string]any{"id": list, "name": "Audit status rule", "kind": "smart", "sort": "title", "rule": map[string]any{"op": "eq", "field": "status", "value": st}, "members": []any{}}}
	e.restore(t, 0, s)
	var statusID, ruleID string
	if err := e.p.QueryRow(context.Background(), "SELECT l.status_id::text,s.rule->>'value' FROM library l JOIN lists s ON s.user_id=l.user_id WHERE l.user_id=$1", e.users[0]).Scan(&statusID, &ruleID); err != nil {
		t.Fatal(err)
	}
	if statusID != ruleID {
		t.Fatal("status predicate not remapped")
	}
	var id string
	_ = e.p.QueryRow(context.Background(), "SELECT id FROM lists WHERE user_id=$1", e.users[0]).Scan(&id)
	raw := e.req(t, 0, "GET", "/api/lists/"+id+"/results", nil, 200)
	if !strings.Contains(string(raw), "\"count\":1") {
		t.Fatal(string(raw))
	}

}
func TestStatusDeletionBlocksDependentSmartLists(t *testing.T) {
	e := auditSetup(t)
	s := auditSnapshot("tmdb:movie:880003")
	e.restore(t, 0, s)
	raw := e.req(t, 0, "POST", "/api/statuses", map[string]any{"Name": "Audit old", "Base": "watching"}, 200)
	var st struct{ ID string }
	_ = json.Unmarshal(raw, &st)
	e.req(t, 0, "PUT", "/api/library/tmdb:movie:880003", map[string]any{"base": "watching", "status_id": st.ID}, 204)
	raw = e.req(t, 0, "POST", "/api/lists", map[string]any{"name": "Status filter", "kind": "smart", "sort": "title", "rule": map[string]any{"op": "eq", "field": "status", "value": st.ID}}, 200)
	var list struct{ ID string }
	_ = json.Unmarshal(raw, &list)
	before := e.req(t, 0, "GET", "/api/lists/"+list.ID+"/results", nil, 200)
	if !strings.Contains(string(before), "\"count\":1") {
		t.Fatal(string(before))
	}
	e.req(t, 0, "DELETE", "/api/statuses/"+st.ID, map[string]any{"replacement": "hold"}, 409)
	after := e.req(t, 0, "GET", "/api/lists/"+list.ID+"/results", nil, 200)
	if !strings.Contains(string(after), "\"count\":1") {
		t.Fatal(string(after))
	}
	e.req(t, 0, "POST", "/api/lists", map[string]any{"id": list.ID, "name": "Status filter", "kind": "smart", "sort": "title", "rule": map[string]any{"op": "eq", "field": "base", "value": "hold"}}, 200)
	e.req(t, 0, "DELETE", "/api/statuses/"+st.ID, map[string]any{"replacement": "hold"}, 204)
}
func TestPortableRoundtripPreservesIntentOrdering(t *testing.T) {
	e := auditSetup(t)
	ctx := context.Background()
	for _, v := range []struct{ id, source, kind string }{{"anidb:anime:880004", "anidb", "anime"}, {"tmdb:tv:880005", "tmdb", "tv"}} {
		_, err := e.p.Exec(ctx, "INSERT INTO catalog(id,source,source_id,kind,title) VALUES($1,$2,$1,$3,'Audit') ON CONFLICT(id) DO NOTHING", v.id, v.source, v.kind)
		if err != nil {
			t.Fatal(err)
		}
		_, err = e.p.Exec(ctx, "INSERT INTO episodes(id,catalog_id,source,source_id,number) VALUES($1,$2,$3,$1,1) ON CONFLICT(id) DO NOTHING", v.id+":ep", v.id, v.source)
		if err != nil {
			t.Fatal(err)
		}
		_, err = e.p.Exec(ctx, "INSERT INTO library(user_id,catalog_id) VALUES($1,$2)", e.users[0], v.id)
		if err != nil {
			t.Fatal(err)
		}
	}
	src := "anidb:anime:880004:ep"
	dst := "tmdb:tv:880005:ep"
	edges, rev, _, err := mapping.Canonical([]mapping.Edge{{From: src, To: dst}})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(edges)
	_, err = e.p.Exec(ctx, "INSERT INTO mappings(revision,provenance,relationships,requires_consent,reviewed) VALUES($1,'{}',$2,false,true)", rev, raw)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = e.p.Exec(ctx, "DELETE FROM mappings WHERE revision=$1", rev) })
	_, err = e.p.Exec(ctx, "INSERT INTO history(user_id,catalog_id,episode_id,local_action_at) VALUES($1,'anidb:anime:880004',$2,now())", e.users[0], src)
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.p.Exec(ctx, "INSERT INTO history_tombstones(user_id,catalog_id,episode_key,deleted_at) VALUES($1,'tmdb:tv:880005',$2,now()-interval '1 day')", e.users[0], dst)
	if err != nil {
		t.Fatal(err)
	}
	before, err := loadEffective(ctx, e.p, e.users[0], "tmdb:tv:880005")
	if err != nil || !before.States[dst].Derived {
		t.Fatal("setup", err, before.States[dst])
	}
	export := e.req(t, 0, "GET", "/api/export", nil, 200)
	var s map[string]any
	_ = json.Unmarshal(export, &s)
	e.restore(t, 1, s)
	after, err := loadEffective(ctx, e.p, e.users[1], "tmdb:tv:880005")
	if err != nil || !after.States[dst].Derived {
		t.Fatal("roundtrip changed effective progress", err)
	}
	secondExport := e.req(t, 1, "GET", "/api/export", nil, 200)
	var secondSnapshot map[string]any
	if err = json.Unmarshal(secondExport, &secondSnapshot); err != nil {
		t.Fatal(err)
	}
	e.restore(t, 2, secondSnapshot)
	secondRoundtrip, err := loadEffective(ctx, e.p, e.users[2], "tmdb:tv:880005")
	if err != nil || !secondRoundtrip.States[dst].Derived {
		t.Fatal("second roundtrip changed progress", err)
	}
	// Scope comparison remains stable under another backup generation.
	if !intentCanOverride([]string{"restore", "same"}, []string{"restore", "same"}) || intentCanOverride([]string{"restore"}, nil) || intentCanOverride([]string{"restore", "older"}, []string{"restore"}) {
		t.Fatal("intent scope precedence")
	}
	// An incoming watch must not undo an existing destination-local unwatch.
	_, err = e.p.Exec(ctx, "UPDATE history_tombstones SET intent_scope='{}' WHERE user_id=$1", e.users[1])
	if err != nil {
		t.Fatal(err)
	}
	guarded, err := loadEffective(ctx, e.p, e.users[1], "tmdb:tv:880005")
	if err != nil || guarded.States[dst].Derived {
		t.Fatal("imported intent overrode destination unwatch", err)
	}

}
func TestPortableExportAboveOldLimitCanRestore(t *testing.T) {
	e := auditSetup(t)
	ctx := context.Background()
	meta := map[string]any{"en": map[string]any{"overview": strings.Repeat("a", 33<<20)}}
	raw, _ := json.Marshal(meta)
	_, err := e.p.Exec(ctx, "INSERT INTO catalog(id,source,source_id,kind,title,metadata) VALUES('tmdb:movie:880006','tmdb','880006','movie','Large synthetic',$1) ON CONFLICT(id) DO UPDATE SET metadata=EXCLUDED.metadata", raw)
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.p.Exec(ctx, "INSERT INTO library(user_id,catalog_id) VALUES($1,'tmdb:movie:880006')", e.users[0])
	if err != nil {
		t.Fatal(err)
	}
	export := e.req(t, 0, "GET", "/api/export", nil, 200)
	if len(export) <= 32<<20 {
		t.Fatal("fixture too small")
	}
	e.req(t, 1, "POST", "/api/restore/preview", json.RawMessage(export), 200)
	// Successful downloads must fit the same cap; oversized libraries fail
	// before producing a misleading portable-backup attachment.
	_, err = e.p.Exec(ctx, "INSERT INTO catalog(id,source,source_id,kind,title,metadata) VALUES('tmdb:movie:880007','tmdb','880007','movie','Second large synthetic',$1) ON CONFLICT(id) DO UPDATE SET metadata=EXCLUDED.metadata", raw)
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.p.Exec(ctx, "INSERT INTO library(user_id,catalog_id) VALUES($1,'tmdb:movie:880007')", e.users[0])
	if err != nil {
		t.Fatal(err)
	}
	e.req(t, 0, "GET", "/api/export", nil, 413)

}

func TestSignedRestoreAllowsMissingCacheButRejectsMetadataTampering(t *testing.T) {
	e := auditSetup(t)
	ctx := context.Background()
	id := "tmdb:movie:881002"
	snapshot := auditSnapshot(id)
	e.restore(t, 0, snapshot)
	exported := e.req(t, 0, "GET", "/api/export", nil, 200)
	var backup map[string]any
	if err := json.Unmarshal(exported, &backup); err != nil {
		t.Fatal(err)
	}
	if backup["catalog_proof"] == "" {
		t.Fatal("missing proof")
	}
	if _, err := e.p.Exec(ctx, "DELETE FROM library WHERE user_id=$1 AND catalog_id=$2", e.users[0], id); err != nil {
		t.Fatal(err)
	}
	if _, err := e.p.Exec(ctx, "DELETE FROM catalog WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	backup["catalog"].([]any)[0].(map[string]any)["title"] = "Tampered shared title"
	e.req(t, 1, "GET", "/api/export", nil, 200)
	raw := e.req(t, 1, "POST", "/api/restore/preview", backup, 200)
	var preview struct{ ID string }
	_ = json.Unmarshal(raw, &preview)
	e.req(t, 1, "POST", "/api/restore/"+preview.ID+"/apply", map[string]any{}, 403)
	raw = e.req(t, 1, "POST", "/api/restore/preview", json.RawMessage(exported), 200)
	_ = json.Unmarshal(raw, &preview)
	e.req(t, 1, "POST", "/api/restore/"+preview.ID+"/apply", map[string]any{}, 200)
}
func TestIntentScopeComparisonPreservesEveryRestoreGeneration(t *testing.T) {
	cases := []struct {
		watch, unwatch []string
		want           bool
	}{
		{nil, nil, true}, {nil, []string{"old"}, true}, {[]string{"old"}, nil, false},
		{[]string{"a"}, []string{"b"}, false}, {[]string{"a"}, []string{"a"}, true},
		{[]string{"a", "b"}, []string{"a"}, false}, {[]string{"a"}, []string{"a", "b"}, true},
	}
	for _, tc := range cases {
		if intentCanOverride(tc.watch, tc.unwatch) != tc.want {
			t.Fatal(tc)
		}
		for range 3 {
			tc.watch = append([]string{"new-generation"}, tc.watch...)
			tc.unwatch = append([]string{"new-generation"}, tc.unwatch...)
			if intentCanOverride(tc.watch, tc.unwatch) != tc.want {
				t.Fatal("restore changed precedence", tc)
			}
		}
	}
}
