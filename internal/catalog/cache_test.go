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
	"strconv"
	"strings"
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
		parts := strings.Split(r.URL.Path, "/")
		sourceID, _ := strconv.ParseInt(parts[len(parts)-1], 10, 64)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": sourceID, "title": "Synthetic cache test", "overview": "Test fixture"})
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
	fallback, err := c.Detail(ctx, "movie", id, "tr", true)
	if err != nil || !fallback.Stale || fallback.Language != "en" {
		t.Fatal("cached language fallback unavailable", err)
	}
	if _, e = c.Detail(ctx, "movie", "99999999", "", true); e == nil {
		t.Fatal("uncached outage silently succeeded")
	}
}

func TestBackgroundAvailabilityDoesNotExtendRecentUse(t *testing.T) {
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
	id := fmt.Sprint(time.Now().UnixNano())
	key := "tmdb:movie:" + id
	if _, e = p.Exec(ctx, "INSERT INTO catalog(id,source,source_id,kind,title) VALUES($1,'tmdb',$2,'movie','Synthetic availability')", key, id); e != nil {
		t.Fatal(e)
	}
	defer func() {
		_, _ = p.Exec(ctx, "DELETE FROM availability WHERE catalog_id=$1", key)
		_, _ = p.Exec(ctx, "DELETE FROM catalog WHERE id=$1", key)
	}()
	if _, e = p.Exec(ctx, `INSERT INTO availability(catalog_id,country,offers,fetched_at,used_at) VALUES($1,'TR','null',now()-interval '2 days',now()-interval '10 days')`, key); e != nil {
		t.Fatal(e)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":{"TR":{"flatrate":[{"provider_id":8,"provider_name":"Synthetic provider"}]}}}`))
	}))
	defer server.Close()
	provider := New("fixture")
	provider.BaseURL = server.URL
	c := Cache{p, provider}
	if _, e = c.Availability(ctx, "movie", id, "TR", true); e != nil {
		t.Fatal(e)
	}
	var old bool
	if e = p.QueryRow(ctx, "SELECT used_at<now()-interval '9 days' FROM availability WHERE catalog_id=$1", key).Scan(&old); e != nil || !old {
		t.Fatal("background refresh extended usage", e)
	}
	if _, e = c.Availability(ctx, "movie", id, "TR", false); e != nil {
		t.Fatal(e)
	}
	if e = p.QueryRow(ctx, "SELECT used_at>now()-interval '1 minute' FROM availability WHERE catalog_id=$1", key).Scan(&old); e != nil || !old {
		t.Fatal("foreground use not recorded", e)
	}
}

func TestConcurrentLanguageRefreshKeepsNewerVariant(t *testing.T) {
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
	id := fmt.Sprint(time.Now().UnixNano())
	numeric, _ := strconv.ParseInt(id, 10, 64)
	requested := make(chan struct{})
	release := make(chan struct{})
	var version atomic.Int32
	version.Store(1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("language") == "tr" {
			close(requested)
			<-release
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": numeric, "title": fmt.Sprintf("Synthetic version %d", version.Load()), "overview": "Synthetic synopsis"})
	}))
	defer server.Close()
	provider := New("fixture")
	provider.BaseURL = server.URL
	c := Cache{p, provider}
	if _, e = c.Detail(ctx, "movie", id, "en", true); e != nil {
		t.Fatal(e)
	}
	defer func() {
		_, _ = p.Exec(ctx, "DELETE FROM catalog_localizations WHERE catalog_id=$1", "tmdb:movie:"+id)
		_, _ = p.Exec(ctx, "DELETE FROM catalog WHERE id=$1", "tmdb:movie:"+id)
	}()
	done := make(chan error, 1)
	go func() { _, err := c.Detail(ctx, "movie", id, "tr", true); done <- err }()
	<-requested
	version.Store(2)
	_, e = c.Detail(ctx, "movie", id, "en", true)
	close(release)
	if e != nil {
		t.Fatal(e)
	}
	if e = <-done; e != nil {
		t.Fatal(e)
	}
	latest, e := c.Detail(ctx, "movie", id, "en", false)
	if e != nil {
		t.Fatal(e)
	}
	var meta struct {
		Title string `json:"title"`
	}
	if e = json.Unmarshal(latest.Metadata, &meta); e != nil || meta.Title != "Synthetic version 2" {
		t.Fatal("stale translation replaced newer metadata", meta.Title, e)
	}
}
