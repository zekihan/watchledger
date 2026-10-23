package simkl

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestReadOnlyListFetchPreservesPagesWithoutCredentials(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "GET" || r.URL.Path != "/lists/42" || r.Header.Get("Authorization") != "Bearer synthetic-private-token" || r.URL.Query().Get("client_id") != "synthetic-client" || r.URL.Query().Get("app-name") != "watchledger" {
			t.Error("unexpected request", r.Method, r.URL.Path)
		}
		if strings.Contains(r.URL.RawQuery, "synthetic-private-token") {
			t.Error("token placed in URL")
		}
		switch r.URL.Query().Get("page") {
		case "1":
			_, _ = fmt.Fprint(w, listFixture(1, 2, 1, `[{"title":"A","ids":{"simkl":1,"slug":"synthetic-a"}}]`))
		case "2":
			_, _ = fmt.Fprint(w, listFixture(2, 2, 1, `[{"title":"B","ids":{"simkl":2}}]`))
		default:
			t.Error("unexpected page")
		}
	}))
	defer upstream.Close()
	client := NewReadOnlyClient()
	client.BaseURL = upstream.URL
	client.interval = 0
	fetched, err := client.FetchList(context.Background(), "synthetic-client", "synthetic-private-token", "42")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseCustomList(fetched.Raw)
	if err != nil || calls.Load() != 2 || len(parsed.Members) != 2 || parsed.Members[0].SourceURL != "https://simkl.com/movies/1/synthetic-a" {
		t.Fatal("wrong paginated import", calls.Load(), parsed, err)
	}
	if strings.Contains(string(fetched.Raw), "synthetic-private-token") || strings.Contains(string(fetched.Raw), "synthetic-client") {
		t.Fatal("credentials entered source archive")
	}
}
func TestReadOnlyListFetchRejectsErrorsAndNeverFollowsRedirect(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"premium", 200, `{"error":"premium_only","message":"synthetic-private-token"}`},
		{"auth v1", 403, `{"error":"oauth2_token_required"}`},
		{"rate", 429, `{"message":"synthetic-private-token"}`},
		{"unexpected auth", 200, `{"access_token":"synthetic-private-token","items":[]}`},
		{"wrong list", 200, strings.Replace(listFixture(1, 1, 1, `[{"title":"A","ids":{"simkl":1}}]`), `"id":42`, `"id":43`, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(tc.status); _, _ = fmt.Fprint(w, tc.body) }))
			defer server.Close()
			client := NewReadOnlyClient()
			client.BaseURL = server.URL
			_, err := client.FetchList(context.Background(), "synthetic-client", "synthetic-private-token", "42")
			if err == nil || strings.Contains(err.Error(), "synthetic-private-token") {
				t.Fatal("unsafe error", err)
			}
		})
	}
	var leaked atomic.Bool
	destination := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { leaked.Store(true) }))
	defer destination.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, http.StatusFound) }))
	defer redirect.Close()
	client := NewReadOnlyClient()
	client.BaseURL = redirect.URL
	if _, err := client.FetchList(context.Background(), "synthetic-client", "synthetic-private-token", "42"); err == nil || leaked.Load() {
		t.Fatal("followed credential-bearing redirect", err, leaked.Load())
	}
	if _, err := client.FetchList(context.Background(), "synthetic-client", "synthetic-private-token", "../42"); err == nil {
		t.Fatal("accepted arbitrary upstream path")
	}
}
func TestReadOnlyListFetchRejectsChangingSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "1" {
			_, _ = fmt.Fprint(w, listFixture(1, 2, 1, `[{"title":"A","ids":{"simkl":1}}]`))
		} else {
			_, _ = fmt.Fprint(w, strings.Replace(listFixture(2, 2, 1, `[{"title":"B","ids":{"simkl":2}}]`), "2026-01-01", "2026-01-02", 1))
		}
	}))
	defer server.Close()
	client := NewReadOnlyClient()
	client.BaseURL = server.URL
	client.interval = 0
	if result, err := client.FetchList(context.Background(), "synthetic-client", "synthetic-private-token", "42"); err == nil || len(result.Raw) != 0 {
		t.Fatal("accepted torn list snapshot", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.FetchList(ctx, "synthetic-client", "synthetic-private-token", "42"); err == nil {
		t.Fatal("cancelled read succeeded")
	}
}
