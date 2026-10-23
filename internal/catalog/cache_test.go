package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/zekihan/watchledger/internal/store"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

func TestCachedDetailsSurviveProviderOutage(t *testing.T) {
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
	var failing atomic.Bool
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failing.Load() {
			http.Error(w, "synthetic outage", http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": 991234, "title": "Synthetic cache test", "overview": "Test fixture"})
	}))
	defer s.Close()
	provider := New("fixture")
	provider.BaseURL = s.URL
	c := Cache{p, provider}
	id := fmt.Sprint(time.Now().UnixNano())
	d, e := c.Detail(ctx, "movie", id, "en", true)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _, _ = p.Exec(ctx, "DELETE FROM catalog WHERE id=$1", d.ID) }()
	failing.Store(true)
	cached, e := c.Detail(ctx, "movie", id, "en", true)
	var a, b interface{}
	_ = json.Unmarshal(cached.Metadata, &a)
	_ = json.Unmarshal(d.Metadata, &b)
	if e != nil || !cached.Stale || !reflect.DeepEqual(a, b) {
		t.Fatal("cache unavailable", e)
	}
	if _, e = c.Detail(ctx, "movie", "99999999", "", true); e == nil {
		t.Fatal("uncached outage silently succeeded")
	}
}
