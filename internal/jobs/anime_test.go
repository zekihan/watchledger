package jobs

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zekihan/watchledger/internal/anidb"
	"github.com/zekihan/watchledger/internal/catalog"
	"github.com/zekihan/watchledger/internal/store"
)

// Separate schemas keep provider-wide cooldown tests independent of concurrently
// running adapter tests, without changing production rate-limit behavior.
func jobDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	ctx := context.Background()
	admin, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := pgx.Identifier{fmt.Sprintf("jobs_test_%d", time.Now().UnixNano())}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); admin.Close() })
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err = store.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

func TestSavedAnimeRefreshCadenceAndDisabledProviders(t *testing.T) {
	p := jobDatabase(t)
	ctx := context.Background()
	var uid string
	if err := p.QueryRow(ctx, "INSERT INTO users(issuer,subject) VALUES('fixture','cadence') RETURNING id").Scan(&uid); err != nil {
		t.Fatal(err)
	}
	for i, fixture := range []struct {
		source, kind, base, age string
		saved                   bool
	}{
		{"anidb", "anime", "watching", "2 days", true},
		{"anidb", "anime", "planned", "2 days", true},
		{"anidb", "anime", "completed", "8 days", true},
		{"anidb", "anime", "watching", "2 hours", true},
		{"anidb", "anime", "watching", "30 days", false},
		{"tmdb", "tv", "watching", "2 days", true},
	} {
		id := fmt.Sprintf("%s:%s:%d", fixture.source, fixture.kind, i+1)
		_, err := p.Exec(ctx, "INSERT INTO catalog(id,source,source_id,kind,title,fetched_at) VALUES($1,$2,$3,$4,'Synthetic',now()-$5::interval)", id, fixture.source, fmt.Sprint(i+1), fixture.kind, fixture.age)
		if err != nil {
			t.Fatal(err)
		}
		if fixture.saved {
			if _, err = p.Exec(ctx, "INSERT INTO library(user_id,catalog_id,base) VALUES($1,$2,$3)", uid, id, fixture.base); err != nil {
				t.Fatal(err)
			}
		}
	}
	r := Runner{Cache: &catalog.Cache{DB: p, Provider: catalog.New("")}, Anime: &anidb.Cache{DB: p, Provider: anidb.New(anidb.Config{Client: "synthetic", Version: 1})}}
	if err := r.enqueue(ctx); err != nil {
		t.Fatal(err)
	}
	var ids []string
	if err := p.QueryRow(ctx, "SELECT array_agg(id ORDER BY id) FROM jobs").Scan(&ids); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(ids) != "[metadata:anidb:anime:1 metadata:anidb:anime:3]" {
		t.Fatal(ids)
	}
	// Repeated passes are idempotent and a missing TMDB token does not disable anime.
	if err := r.enqueue(ctx); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := p.QueryRow(ctx, "SELECT count(*) FROM jobs").Scan(&n); err != nil || n != 2 {
		t.Fatal(n, err)
	}
	r.Anime.Provider.Config.Client = ""
	if err := r.one(ctx); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("disabled source must not run", err)
	}
}

func TestAnimeRefreshPreservesHistoryAndRespectsCooldown(t *testing.T) {
	p := jobDatabase(t)
	ctx := context.Background()
	var calls atomic.Int32
	var failing atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if failing.Load() {
			http.Error(w, "synthetic outage", http.StatusServiceUnavailable)
			return
		}
		_, _ = fmt.Fprint(w, `<anime id="71"><type>TV Series</type><episodecount>1</episodecount><titles><title xml:lang="en" type="official">New title</title></titles><episodes><episode id="711"><epno type="1">2</epno><length>24</length><airdate>2026-01-02</airdate><title xml:lang="en">Renumbered</title></episode></episodes></anime>`)
	}))
	defer server.Close()
	provider := anidb.New(anidb.Config{Client: "synthetic", Version: 1})
	provider.URL = server.URL
	r := Runner{Cache: &catalog.Cache{DB: p, Provider: catalog.New("")}, Anime: &anidb.Cache{DB: p, Provider: provider}}
	old := []byte(`<anime id="71"><type>TV Series</type><episodecount>1</episodecount><titles><title xml:lang="en" type="official">Old title</title></titles><episodes><episode id="711"><epno type="1">1</epno><length>24</length><title xml:lang="en">Old</title></episode></episodes></anime>`)
	_, err := p.Exec(ctx, `INSERT INTO catalog(id,source,source_id,kind,title,fetched_at) VALUES('anidb:anime:71','anidb','71','anime','Old title',now()-interval '2 days');
 INSERT INTO episodes(id,catalog_id,source,source_id,season,number) VALUES('anidb:episode:711','anidb:anime:71','anidb','711',1,1);
 INSERT INTO users(issuer,subject) VALUES('fixture','refresh');
 INSERT INTO library(user_id,catalog_id,base) SELECT id,'anidb:anime:71','watching' FROM users;
 INSERT INTO history(user_id,catalog_id,episode_id,watched_date) SELECT id,'anidb:anime:71','anidb:episode:711','2026-01-03' FROM users;`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.Exec(ctx, "INSERT INTO anidb_documents(aid,document,fetched_at) VALUES('71',$1,now()-interval '2 days')", old); err != nil {
		t.Fatal(err)
	}
	if err = r.enqueue(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = p.Exec(ctx, "INSERT INTO provider_request_limits(key,next_allowed_at) VALUES('anidb:global',now()+interval '2 seconds')"); err != nil {
		t.Fatal(err)
	}
	if err = r.one(ctx); !errors.Is(err, pgx.ErrNoRows) || calls.Load() != 0 {
		t.Fatal("cooldown was bypassed", err, calls.Load())
	}
	if _, err = p.Exec(ctx, "UPDATE provider_request_limits SET next_allowed_at=now()-interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	if err = r.one(ctx); err != nil {
		t.Fatal(err)
	}
	var number, count int
	var date, title string
	if err = p.QueryRow(ctx, "SELECT e.number,h.watched_date::text,c.title FROM history h JOIN episodes e ON e.id=h.episode_id JOIN catalog c ON c.id=h.catalog_id").Scan(&number, &date, &title); err != nil || number != 2 || date != "2026-01-03" || title != "New title" {
		t.Fatal(number, date, title, err)
	}
	if err = p.QueryRow(ctx, "SELECT count(*) FROM jobs").Scan(&count); err != nil || count != 0 || calls.Load() != 1 {
		t.Fatal(count, calls.Load(), err)
	}
	// A later failed refresh retains both cached content and history, and retries
	// no earlier than the provider's persisted one-day dataset reservation.
	failing.Store(true)
	if _, err = p.Exec(ctx, "UPDATE catalog SET fetched_at=now()-interval '2 days'; UPDATE anidb_documents SET fetched_at=now()-interval '2 days'; UPDATE provider_request_limits SET next_allowed_at=now()-interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	if err = r.enqueue(ctx); err != nil {
		t.Fatal(err)
	}
	if err = r.one(ctx); err != nil {
		t.Fatal(err)
	}
	var delayed bool
	var attempts int
	var message string
	if err = p.QueryRow(ctx, "SELECT attempts,run_at>= (SELECT next_allowed_at FROM provider_request_limits WHERE key='anidb:anime:71'),last_error FROM jobs").Scan(&attempts, &delayed, &message); err != nil || attempts != 1 || !delayed || message == "" {
		t.Fatal(attempts, delayed, message, err)
	}
	if err = r.one(ctx); !errors.Is(err, pgx.ErrNoRows) || calls.Load() != 2 {
		t.Fatal(err, calls.Load())
	}
	if err = p.QueryRow(ctx, "SELECT count(*) FROM history WHERE watched_date='2026-01-03'").Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
}

func TestStaleSeasonRefreshRemainsRetryable(t *testing.T) {
	p := jobDatabase(t)
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tv/91" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"id":91,"name":"Synthetic","seasons":[{"season_number":1}]}`)
			return
		}
		http.Error(w, "synthetic season outage", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	provider := catalog.New("synthetic")
	provider.BaseURL = server.URL
	r := Runner{Cache: &catalog.Cache{DB: p, Provider: provider}}
	_, err := p.Exec(ctx, `INSERT INTO catalog(id,source,source_id,kind,title) VALUES('tmdb:tv:91','tmdb','91','tv','Synthetic');
 INSERT INTO episodes(id,catalog_id,source,source_id,season,number,metadata) VALUES('tmdb:episode:911','tmdb:tv:91','tmdb','911',1,1,'{"id":911,"episode_number":1,"season_number":1,"name":"Cached episode"}');
 INSERT INTO jobs(id,kind,payload) VALUES('metadata:tmdb:tv:91','metadata','{"kind":"tv","source_id":"91"}');`)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.one(ctx); err != nil {
		t.Fatal(err)
	}
	var attempts int
	var message string
	if err = p.QueryRow(ctx, "SELECT attempts,last_error FROM jobs WHERE id='metadata:tmdb:tv:91'").Scan(&attempts, &message); err != nil || attempts != 1 || message != "season refresh failed; cached episodes retained" {
		t.Fatal(attempts, message, err)
	}
	var count int
	if err = p.QueryRow(ctx, "SELECT count(*) FROM episodes WHERE id='tmdb:episode:911'").Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
}
