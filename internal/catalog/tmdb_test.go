package catalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
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
