package jobs

import (
	"context"
	"github.com/zekihan/watchledger/internal/catalog"
	"github.com/zekihan/watchledger/internal/store"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestRetryRetainsCachedData(t *testing.T) {
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "synthetic outage", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	provider := catalog.New("fixture")
	provider.BaseURL = server.URL
	r := Runner{Cache: &catalog.Cache{DB: p, Provider: provider}}
	id := "fixture-job-" + time.Now().Format("150405.000000000")
	if _, e = p.Exec(ctx, `INSERT INTO jobs(id,kind,payload) VALUES($1,'metadata','{"kind":"movie","source_id":"99999999"}')`, id); e != nil {
		t.Fatal(e)
	}
	defer func() { _, _ = p.Exec(ctx, "DELETE FROM jobs WHERE id=$1", id) }()
	if e = r.one(ctx); e != nil {
		t.Fatal(e)
	}
	var attempts int
	var future bool
	e = p.QueryRow(ctx, "SELECT attempts,run_at>now() FROM jobs WHERE id=$1", id).Scan(&attempts, &future)
	if e != nil || attempts != 1 || !future {
		t.Fatal(attempts, future, e)
	}
}
