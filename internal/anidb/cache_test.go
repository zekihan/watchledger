package anidb

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/zekihan/watchledger/internal/store"
)

func TestPersistentCacheThrottleAndEpisodeHistory(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	ctx := context.Background()
	pool, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = store.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	const aid = "734200001"
	const eid = "734200002"
	const special = "734200003"
	body := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(fixture, `id="42"`, `id="`+aid+`"`), `id="501"`, `id="`+eid+`"`), `id="502"`, `id="`+special+`"`)
	_, err = pool.Exec(ctx, "DELETE FROM provider_request_limits WHERE key='anidb:global' OR key=$1", "anidb:anime:"+aid)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		mu.Lock()
		defer mu.Unlock()
		_, _ = fmt.Fprint(w, body)
	}))
	defer server.Close()
	provider := New(Config{Client: "synthetic", Version: 1})
	provider.URL = server.URL
	cache := Cache{DB: pool, Provider: provider}
	var uid string
	if err = pool.QueryRow(ctx, "INSERT INTO users(issuer,subject) VALUES('test','anidb-cache') ON CONFLICT(issuer,subject) DO UPDATE SET name='' RETURNING id").Scan(&uid); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id=$1", uid)
		_, _ = pool.Exec(ctx, "DELETE FROM episodes WHERE catalog_id=$1", "anidb:anime:"+aid)
		_, _ = pool.Exec(ctx, "DELETE FROM catalog WHERE id=$1", "anidb:anime:"+aid)
		_, _ = pool.Exec(ctx, "DELETE FROM anidb_documents WHERE aid=$1", aid)
		_, _ = pool.Exec(ctx, "DELETE FROM provider_request_limits WHERE key='anidb:global' OR key=$1", "anidb:anime:"+aid)
	}()
	d, err := cache.Detail(ctx, aid, "en", "translated")
	if err != nil || d.Stale {
		t.Fatal(d, err)
	}
	other := Cache{DB: pool, Provider: provider}
	if _, err = other.Detail(ctx, aid, "ja", "native"); err != nil || calls.Load() != 1 {
		t.Fatal("language or new process bypassed shared cache", err, calls.Load())
	}
	if err = other.reserve(ctx, aid); err == nil {
		t.Fatal("per-dataset request cooldown lost")
	}
	if _, err = pool.Exec(ctx, "INSERT INTO history(user_id,catalog_id,episode_id,watched_date) VALUES($1,$2,$3,'2026-01-02')", uid, d.ID, "anidb:episode:"+eid); err != nil {
		t.Fatal(err)
	}
	// Synthetic aging is confined to this disposable fixture database.
	expire := func() {
		t.Helper()
		_, e := pool.Exec(ctx, "UPDATE anidb_documents SET fetched_at=now()-interval '2 days' WHERE aid=$1", aid)
		if e != nil {
			t.Fatal(e)
		}
		_, e = pool.Exec(ctx, "UPDATE provider_request_limits SET next_allowed_at=now()-interval '1 second' WHERE key='anidb:global' OR key=$1", "anidb:anime:"+aid)
		if e != nil {
			t.Fatal(e)
		}
	}
	expire()
	mu.Lock()
	body = strings.ReplaceAll(body, `<epno type="1">1</epno>`, `<epno type="1">2</epno>`)
	mu.Unlock()
	if _, err = cache.Detail(ctx, aid, "en", "translated"); err != nil {
		t.Fatal(err)
	}
	var day string
	if err = pool.QueryRow(ctx, "SELECT watched_date::text FROM history WHERE user_id=$1 AND episode_id=$2", uid, "anidb:episode:"+eid).Scan(&day); err != nil || day != "2026-01-02" {
		t.Fatal("metadata refresh changed source history", day, err)
	}
	var notices int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM notices WHERE user_id=$1 AND kind='episode_changed'", uid).Scan(&notices); err != nil || notices < 1 {
		t.Fatal("reordering was not reported", notices, err)
	}
	expire()
	mu.Lock()
	body = "<error>Banned</error>"
	mu.Unlock()
	d, err = cache.Detail(ctx, aid, "en", "translated")
	if err != nil || !d.Stale || d.Warning == "" {
		t.Fatal("outage lost cached metadata", d, err)
	}
	var blocked bool
	if err = pool.QueryRow(ctx, "SELECT next_allowed_at>now()+interval '23 hours' FROM provider_request_limits WHERE key='anidb:global'").Scan(&blocked); err != nil || !blocked {
		t.Fatal("ban did not stop provider requests", err)
	}
}
