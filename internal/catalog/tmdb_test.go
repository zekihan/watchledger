package catalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestRealAdapterProtocol(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Error("missing authorization")
		}
		if r.URL.Query().Get("query") != "a & b" {
			t.Error("query not encoded")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer s.Close()
	p := New("fixture-token")
	p.BaseURL = s.URL
	if _, e := p.Get(context.Background(), "/search/movie", url.Values{"query": {"a & b"}}); e != nil {
		t.Fatal(e)
	}
	p.Token = ""
	if _, e := p.Get(context.Background(), "/search/movie", nil); e == nil {
		t.Fatal("unconfigured provider silently succeeded")
	}
}
func TestIdentityAndDiscovery(t *testing.T) {
	if _, e := Identity("tv", "../x"); e == nil {
		t.Fatal("unsafe id")
	}
	if _, e := Discovery("tv", url.Values{"with_cast": {"1"}}); e == nil {
		t.Fatal("unsupported TV cast filter accepted")
	}
	q, e := Discovery("movie", url.Values{"with_cast": {"1"}, "api_key": {"secret"}})
	if e != nil || q.Get("with_cast") != "1" || q.Get("api_key") != "" {
		t.Fatal(q, e)
	}
}

// A valid JSON prefix must not hide an oversized or malformed response tail.
func TestTMDBResponseSizeBoundary(t *testing.T) {
	const limit = 8 << 20
	prefix := `{"id":42,"overview":"` + strings.Repeat("x", limit-len(`{"id":42,"overview":""}`)) + `"}`
	for _, tc := range []struct {
		name      string
		body      string
		wantError bool
	}{
		{"small", `{"id":42}`, false},
		{"exact limit", prefix, false},
		{"oversized valid JSON", prefix + " ", true},
		{"invalid tail beyond limit", prefix + "x", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				// Exercise streamed responses without relying on Content-Length.
				w.(http.Flusher).Flush()
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			provider := New("synthetic-token")
			provider.BaseURL = server.URL
			raw, err := provider.Get(context.Background(), "/movie/42", nil)
			if tc.wantError {
				if err == nil || len(raw) != 0 {
					t.Fatalf("oversized response accepted: bytes=%d, error=%v", len(raw), err)
				}
			} else if err != nil || string(raw) != tc.body {
				t.Fatalf("bounded response changed: bytes=%d, error=%v", len(raw), err)
			}
		})
	}
}
