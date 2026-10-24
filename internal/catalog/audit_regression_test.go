package catalog

import (
	"context"
	"github.com/zekihan/watchledger/internal/store"
	"net/http"
	"net/http/httptest"
	"os"

	"testing"
)

func TestPersonCreditsAcceptsNumericRootIdentity(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"id":1,"cast":[],"crew":[]}`)) }))
	defer s.Close()
	p := New("synthetic")
	p.BaseURL = s.URL
	c := Cache{Provider: p}
	_, err := c.PeopleTV(context.Background(), "1", nil)
	if err != nil {
		t.Fatal(err)
	}

}
func TestSeasonMoveReconcilesEpisodeKind(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	p, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if err = store.Migrate(ctx, p); err != nil {
		t.Fatal(err)
	}
	_, err = p.Exec(ctx, `INSERT INTO catalog(id,source,source_id,kind,title,metadata) VALUES('tmdb:tv:880010','tmdb','880010','tv','Audit','{"en":{"id":880010,"name":"Audit"}}') ON CONFLICT(id) DO UPDATE SET fetched_at=now(); INSERT INTO episodes(id,catalog_id,source,source_id,season,number,kind) VALUES('tmdb:episode:880011','tmdb:tv:880010','tmdb','880011',1,1,'regular') ON CONFLICT(id) DO UPDATE SET kind='regular',season=1`)
	if err != nil {
		t.Fatal(err)
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"episodes":[{"id":880011,"season_number":0,"episode_number":1,"air_date":"2020-01-01"}]}`))
	}))
	defer s.Close()
	_, _ = p.Exec(ctx, "DELETE FROM season_cache WHERE catalog_id='tmdb:tv:880010'")
	provider := New("synthetic")
	provider.BaseURL = s.URL
	c := Cache{DB: p, Provider: provider}
	if _, err = c.Season(ctx, "880010", 0, "en"); err != nil {
		t.Fatal(err)
	}
	var season int
	var kind string
	if err = p.QueryRow(ctx, "SELECT season,kind FROM episodes WHERE id='tmdb:episode:880011'").Scan(&season, &kind); err != nil {
		t.Fatal(err)
	}
	if season != 0 || kind != "special" {
		t.Fatal(season, kind)
	}

}
