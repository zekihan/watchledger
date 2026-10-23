package anidb

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zekihan/watchledger/internal/store"
)

const hotFixture = `<hotanime><anime id="41" restricted="false"><title xml:lang="en" type="official">Synthetic hot title</title><startdate>2026-01-02</startdate><picture>123.jpg</picture><ratings><permanent count="10">8.1</permanent></ratings></anime><anime id="42" restricted="true"><title xml:lang="en">Restricted fixture</title></anime></hotanime>`

func TestHotFeedValidation(t *testing.T) {
	items, err := parseHot([]byte(hotFixture))
	if err != nil || len(items) != 1 || items[0].ID != "41" {
		t.Fatal(items, err)
	}
	for _, raw := range []string{`<anime/>`, `<hotanime><anime id="../1"><title>x</title></anime></hotanime>`, `<hotanime><anime id="1"><title>x</title></anime><anime id="1"><title>y</title></anime></hotanime>`, `<hotanime/><hotanime/>`, `<error>Banned</error>`} {
		if _, err = parseHot([]byte(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
func TestHotFeedDailyCacheGlobalBanAndNoCatalogMirror(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	ctx := context.Background()
	admin, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := pgx.Identifier{fmt.Sprintf("hot_test_%d", time.Now().UnixNano())}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE") }()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = store.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	var banned atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Query().Get("request") != "hotanime" || r.URL.Query().Get("aid") != "" || r.URL.Query().Get("client") != "synthetic" {
			t.Error("wrong feed request", r.URL.RawQuery)
		}
		if banned.Load() {
			_, _ = fmt.Fprint(w, `<error>Banned</error>`)
			return
		}
		_, _ = fmt.Fprint(w, hotFixture)
	}))
	defer server.Close()
	provider := New(Config{Client: "synthetic", Version: 1})
	provider.URL = server.URL
	c := Cache{DB: pool, Provider: provider}
	first, err := c.Hot(ctx)
	if err != nil || first.Stale || len(first.Items) != 1 {
		t.Fatal(first, err)
	}
	second, err := c.Hot(ctx)
	if err != nil || !second.FetchedAt.Equal(first.FetchedAt) || calls.Load() != 1 {
		t.Fatal(second, err, calls.Load())
	}
	var n int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM catalog").Scan(&n); err != nil || n != 0 {
		t.Fatal("feed mirrored into catalog", n, err)
	}
	if _, err = pool.Exec(ctx, "UPDATE anidb_feeds SET fetched_at=now()-interval '2 days'"); err != nil {
		t.Fatal(err)
	}
	stale, err := c.Hot(ctx)
	if err != nil || !stale.Stale || calls.Load() != 1 {
		t.Fatal("daily limit bypassed", stale, err, calls.Load())
	}
	if _, err = pool.Exec(ctx, "UPDATE provider_request_limits SET next_allowed_at=now()-interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	banned.Store(true)
	fallback, err := c.Hot(ctx)
	if err != nil || !fallback.Stale || len(fallback.Items) != 1 || calls.Load() != 2 {
		t.Fatal(fallback, err, calls.Load())
	}
	var globalBlocked bool
	if err = pool.QueryRow(ctx, "SELECT next_allowed_at>now()+interval '23 hours' FROM provider_request_limits WHERE key='anidb:global'").Scan(&globalBlocked); err != nil || !globalBlocked {
		t.Fatal("ban not persisted", globalBlocked, err)
	}
	if _, err = c.Hot(ctx); err != nil || calls.Load() != 2 {
		t.Fatal("ban request repeated", err, calls.Load())
	}
	provider.Config.Client = ""
	if fallback, err = c.Hot(ctx); err != nil || !fallback.Stale || len(fallback.Items) != 1 {
		t.Fatal("disabled provider lost cached feed", fallback, err)
	}
}
