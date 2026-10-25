package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDecodeRequiresOneCompleteJSONValue(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       bool
	}{
		{"single", `{"name":"synthetic"}`, true},
		{"whitespace", "{\"name\":\"synthetic\"} \n\t", true},
		{"second object", `{"name":"synthetic"}{"name":"ignored"}`, false},
		{"trailing garbage", `{"name":"synthetic"}x`, false},
		{"trailing null", `{"name":"synthetic"}null`, false},
		{"oversize trailing whitespace", `{"name":"synthetic"}` + strings.Repeat(" ", 2<<20), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var in struct {
				Name string `json:"name"`
			}
			w := httptest.NewRecorder()
			r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(tc.body))
			if got := decode(w, r, &in); got != tc.want {
				t.Fatalf("decode = %v, want %v (HTTP %d)", got, tc.want, w.Code)
			}
			if !tc.want && w.Code != http.StatusBadRequest {
				t.Fatalf("HTTP %d, want 400", w.Code)
			}
		})
	}
}

func TestLibraryPatchClearsExplicitStructureOverride(t *testing.T) {
	e := thirdSetup(t)
	thirdMovie(t, e, "992007")
	e.req(t, 0, "PATCH", "/api/library/tmdb:movie:992007", map[string]any{"anime_view": "show", "rating": 8}, 204)
	e.req(t, 0, "PATCH", "/api/library/tmdb:movie:992007", map[string]any{"base": "hold"}, 204)
	var view *string
	var rating int
	check := func(want *string) {
		t.Helper()
		err := e.p.QueryRow(context.Background(), "SELECT anime_view,rating FROM library WHERE user_id=$1 AND catalog_id='tmdb:movie:992007'", e.users[0]).Scan(&view, &rating)
		if err != nil || rating != 8 || (view == nil) != (want == nil) || (view != nil && *view != *want) {
			t.Fatalf("view=%v rating=%d err=%v", view, rating, err)
		}
	}
	show := "show"
	check(&show)
	e.req(t, 0, "PATCH", "/api/library/tmdb:movie:992007", json.RawMessage(`{"anime_view":null}`), 204)
	check(nil)
	e.req(t, 1, "PATCH", "/api/library/tmdb:movie:992007", map[string]any{"anime_view": "native"}, 404)
	check(nil)
}

func TestTrailingJSONDoesNotCreateStatusOrRestorePreview(t *testing.T) {
	e := auditSetup(t)
	backup := string(e.req(t, 0, "GET", "/api/export", nil, 200))
	for _, tc := range []struct{ path, body string }{
		{"/api/statuses", `{"Name":"Rejected","Base":"watching"} {}`},
		{"/api/restore/preview", backup + `{}`},
		{"/api/restore/preview", backup + `invalid`},
	} {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, tc.path, strings.NewReader(tc.body))
		r.Header.Set("Origin", "http://127.0.0.1:8080")
		r.AddCookie(&http.Cookie{Name: "wl_session", Value: e.cookies[0]})
		w := httptest.NewRecorder()
		e.h.ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s returned %d: %s", tc.path, w.Code, w.Body.String())
		}
	}
	for _, table := range []string{"statuses", "imports"} {
		var count int
		if err := e.p.QueryRow(context.Background(), "SELECT count(*) FROM "+table+" WHERE user_id=$1", e.users[0]).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s has %d records: %v", table, count, err)
		}
	}
	// A valid complete document remains accepted after the rejected attempts.
	e.req(t, 0, "POST", "/api/restore/preview", json.RawMessage(backup), 200)
}

func TestStructureChangeHonorsAccountWriteLock(t *testing.T) {
	e := thirdSetup(t)
	thirdMovie(t, e, "992007")
	tx, err := e.p.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(context.Background(), "SELECT id FROM users WHERE id=$1 FOR UPDATE", e.users[0]); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	r := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/library/tmdb:movie:992007/structure", strings.NewReader(`{"view":"show"}`))
	r.Header.Set("Origin", "http://127.0.0.1:8080")
	r.AddCookie(&http.Cookie{Name: "wl_session", Value: e.cookies[0]})
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("structure write bypassed account lock: %d", w.Code)
	}
	if err = tx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	var view *string
	if err = e.p.QueryRow(context.Background(), "SELECT anime_view FROM library WHERE user_id=$1 AND catalog_id='tmdb:movie:992007'", e.users[0]).Scan(&view); err != nil || view != nil {
		t.Fatalf("cancelled write changed override: %v %v", view, err)
	}
	e.req(t, 0, "PUT", "/api/library/tmdb:movie:992007/structure", map[string]any{"view": "show"}, 204)
	if err = e.p.QueryRow(context.Background(), "SELECT anime_view FROM library WHERE user_id=$1 AND catalog_id='tmdb:movie:992007'", e.users[0]).Scan(&view); err != nil || view == nil || *view != "show" {
		t.Fatalf("released write failed: %v %v", view, err)
	}
}
