package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zekihan/watchledger/internal/auth"
	"github.com/zekihan/watchledger/internal/catalog"
	"github.com/zekihan/watchledger/internal/config"
)

type simklImportFixture struct {
	auditEnv
	show, movie string
	calls       *atomic.Int32
	source      map[string]any
	provider    *catalog.TMDB
}

func newSimklImportFixture(t *testing.T) simklImportFixture {
	t.Helper()
	e := auditSetup(t)
	ctx := context.Background()
	show := strconv.FormatInt(time.Now().UnixNano()%1_000_000_000_000, 10)
	movie := show + "1"
	calls := &atomic.Int32{}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/tv/"+show+"/season/1" {
			_, _ = fmt.Fprintf(w, `{"episodes":[{"id":%s1,"episode_number":1,"season_number":1,"name":"First episode","air_date":"2020-01-01"},{"id":%s3,"episode_number":3,"season_number":1,"name":"Third episode","air_date":"2020-01-03"}]}`, show, show)
			return
		}
		id := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/movie/"), "/tv/")
		number, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			http.Error(w, "unavailable", 404)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": number, "title": "Synthetic title", "name": "Synthetic show", "overview": "Synthetic metadata", "seasons": []any{map[string]any{"season_number": 1}}})
	}))
	t.Cleanup(provider.Close)
	var issuer string
	oidc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"issuer": issuer, "authorization_endpoint": issuer + "/authorize", "token_endpoint": issuer + "/token", "jwks_uri": issuer + "/keys", "id_token_signing_alg_values_supported": []string{"RS256"}})
	}))
	t.Cleanup(oidc.Close)
	issuer = oidc.URL
	identity, err := auth.New(ctx, e.p, config.Config{Issuer: issuer, PublicURL: "http://127.0.0.1:8080", ClientID: "test", SessionKey: base64.StdEncoding.EncodeToString(make([]byte, 32))})
	if err != nil {
		t.Fatal(err)
	}
	tmdb := catalog.New("synthetic-provider-token")
	tmdb.BaseURL = provider.URL
	app := &API{DB: e.p, Identity: identity, Catalog: &catalog.Cache{DB: e.p, Provider: tmdb}}
	mux := http.NewServeMux()
	app.routes(mux)
	e.h = identity.Middleware(mux)
	raw := fmt.Sprintf(`{"movies":[{"movie":{"title":"Synthetic movie","ids":{"simkl":11,"tmdb":%q}},"status":"completed","last_watched_at":"2025-02-01"}],"shows":[{"show":{"title":"Synthetic show","ids":{"simkl":22,"tmdb":%q}},"status":"watching","user_rating":7,"seasons":[{"number":1,"episodes":[{"number":1,"watched_at":"2025-02-03"},{"number":3}]}]}]}`, movie, show)
	var source map[string]any
	if err = json.Unmarshal([]byte(raw), &source); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = e.p.Exec(ctx, "DELETE FROM catalog WHERE id=ANY($1::text[])", []string{"tmdb:tv:" + show, "tmdb:movie:" + movie})
	})
	return simklImportFixture{auditEnv: e, show: show, movie: movie, calls: calls, source: source, provider: tmdb}
}
func (f simklImportFixture) prepare(t *testing.T) simklSummary {
	t.Helper()
	var initial simklSummary
	if err := json.Unmarshal(f.req(t, 0, "POST", "/api/simkl/preview", f.source, 200), &initial); err != nil {
		t.Fatal(err)
	}
	if initial.ID == "" {
		t.Fatal("missing saved import")
	}
	next := 0
	for next < initial.PreparationTotal {
		var result struct {
			Next    int
			Paused  bool
			Preview simklSummary
		}
		if err := json.Unmarshal(f.req(t, 0, "POST", "/api/simkl/"+initial.ID+"/prepare", map[string]any{"offset": next}, 200), &result); err != nil {
			t.Fatal(err)
		}
		if result.Paused || result.Next <= next {
			t.Fatal("preparation did not progress")
		}
		next = result.Next
	}
	var ready simklSummary
	if err := json.Unmarshal(f.req(t, 0, "GET", "/api/simkl/"+initial.ID+"/preview", nil, 200), &ready); err != nil {
		t.Fatal(err)
	}
	return ready
}
func TestSIMKLBulkPreparationResumesWithoutApplyingPrivateData(t *testing.T) {
	f := newSimklImportFixture(t)
	ready := f.prepare(t)
	if ready.Additions != 2 || ready.EpisodeMatches != 2 || ready.EpisodeExceptions != 0 || ready.Unresolved != 0 {
		t.Fatalf("unexpected preview: %+v", ready)
	}
	calls := f.calls.Load()
	f.req(t, 1, "GET", "/api/simkl/"+ready.ID+"/preview", nil, 404)
	f.req(t, 1, "POST", "/api/simkl/"+ready.ID+"/prepare", map[string]any{"offset": 0}, 404)
	f.req(t, 0, "POST", "/api/simkl/"+ready.ID+"/prepare", map[string]any{"offset": -1}, 400)
	f.req(t, 0, "POST", "/api/simkl/"+ready.ID+"/prepare", map[string]any{"offset": 0}, 200)
	if f.calls.Load() != calls {
		t.Fatal("cached/resumed or foreign requests fetched provider metadata")
	}
	for _, table := range []string{"library", "history", "simkl_episode_links", "import_reviews"} {
		var count int
		if err := f.p.QueryRow(context.Background(), "SELECT count(*) FROM "+table+" WHERE user_id=$1", f.users[0]).Scan(&count); err != nil || count != 0 {
			t.Fatal("preparation changed private data", table, count, err)
		}
	}
	f.req(t, 0, "POST", "/api/simkl/"+ready.ID+"/apply", map[string]any{"confirm_episode_matches": true, "episode_match_digest": ready.EpisodeMatchDigest}, 428)
}
func TestSIMKLBulkEpisodeMatchesRequireExplicitConsent(t *testing.T) {
	for _, confirm := range []bool{false, true} {
		t.Run(fmt.Sprint(confirm), func(t *testing.T) {
			f := newSimklImportFixture(t)
			ready := f.prepare(t)
			f.req(t, 0, "GET", "/api/export", nil, 200)
			body := map[string]any{"confirm_episode_matches": confirm, "episode_match_digest": ready.EpisodeMatchDigest}
			f.req(t, 0, "POST", "/api/simkl/"+ready.ID+"/apply", body, 200)
			f.req(t, 0, "POST", "/api/simkl/"+ready.ID+"/apply", body, 200)
			var links int
			_ = f.p.QueryRow(context.Background(), "SELECT count(*) FROM simkl_episode_links WHERE user_id=$1", f.users[0]).Scan(&links)
			expected := 0
			if confirm {
				expected = 2
			}
			if links != expected {
				t.Fatalf("confirmation %v: got %d episode mappings", confirm, links)
			}
			if confirm {
				var known, unknown *string
				if err := f.p.QueryRow(context.Background(), "SELECT watched_date::text FROM history WHERE user_id=$1 AND episode_id=$2", f.users[0], "tmdb:episode:"+f.show+"1").Scan(&known); err != nil || known == nil || *known != "2025-02-03" {
					t.Fatal("source date not retained", err)
				}
				if err := f.p.QueryRow(context.Background(), "SELECT watched_date::text FROM history WHERE user_id=$1 AND episode_id=$2", f.users[0], "tmdb:episode:"+f.show+"3").Scan(&unknown); err != nil || unknown != nil {
					t.Fatal("unknown date fabricated", err)
				}
				var gap bool
				_ = f.p.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM history WHERE user_id=$1 AND episode_id=$2)", f.users[0], "tmdb:episode:"+f.show+"2").Scan(&gap)
				if gap {
					t.Fatal("unrecorded episode inferred")
				}
			}
		})
	}
}
func TestSIMKLBulkApprovalRejectsChangedEpisodesAtomically(t *testing.T) {
	f := newSimklImportFixture(t)
	ready := f.prepare(t)
	ctx := context.Background()
	f.req(t, 0, "GET", "/api/export", nil, 200)
	if _, err := f.p.Exec(ctx, "UPDATE episodes SET retired=true WHERE id=$1", "tmdb:episode:"+f.show+"1"); err != nil {
		t.Fatal(err)
	}
	f.req(t, 0, "POST", "/api/simkl/"+ready.ID+"/apply", map[string]any{"confirm_episode_matches": true, "episode_match_digest": ready.EpisodeMatchDigest}, 409)
	var count int
	_ = f.p.QueryRow(ctx, "SELECT count(*) FROM library WHERE user_id=$1", f.users[0]).Scan(&count)
	if count != 0 {
		t.Fatal("stale approval partially applied")
	}
	var current simklSummary
	_ = json.Unmarshal(f.req(t, 0, "GET", "/api/simkl/"+ready.ID+"/preview", nil, 200), &current)
	if current.EpisodeMatches != 1 || current.EpisodeExceptions != 1 || current.Unresolved != 1 {
		t.Fatal("retired exception missing", current)
	}
}
func TestSIMKLBulkImportPreservesExistingDatesRatingsAndUnwatch(t *testing.T) {
	f := newSimklImportFixture(t)
	ready := f.prepare(t)
	ctx := context.Background()
	target := "tmdb:tv:" + f.show
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{"INSERT INTO library(user_id,catalog_id,base,rating) VALUES($1,$2,'completed',9)", []any{f.users[0], target}},
		{"INSERT INTO history(user_id,catalog_id,episode_id,watched_date) VALUES($1,$2,$3,'2020-01-01')", []any{f.users[0], target, "tmdb:episode:" + f.show + "1"}},
		{"INSERT INTO history_tombstones(user_id,catalog_id,episode_key) VALUES($1,$2,$3)", []any{f.users[0], target, "tmdb:episode:" + f.show + "3"}},
	} {
		if _, err := f.p.Exec(ctx, statement.query, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
	_ = json.Unmarshal(f.req(t, 0, "GET", "/api/simkl/"+ready.ID+"/preview", nil, 200), &ready)
	f.req(t, 0, "GET", "/api/export", nil, 200)
	f.req(t, 0, "POST", "/api/simkl/"+ready.ID+"/apply", map[string]any{"confirm_episode_matches": true, "episode_match_digest": ready.EpisodeMatchDigest}, 200)
	var state, date string
	var rating, count int
	_ = f.p.QueryRow(ctx, "SELECT base,rating FROM library WHERE user_id=$1 AND catalog_id=$2", f.users[0], target).Scan(&state, &rating)
	_ = f.p.QueryRow(ctx, "SELECT watched_date::text FROM history WHERE user_id=$1 AND episode_id=$2", f.users[0], "tmdb:episode:"+f.show+"1").Scan(&date)
	_ = f.p.QueryRow(ctx, "SELECT count(*) FROM history WHERE user_id=$1 AND episode_id=$2", f.users[0], "tmdb:episode:"+f.show+"3").Scan(&count)
	if state != "completed" || rating != 9 || date != "2020-01-01" || count != 0 {
		t.Fatal("existing decisions changed", state, rating, date, count)
	}
}

func TestSIMKLBulkPreparationHonorsProviderCooldown(t *testing.T) {
	f := newSimklImportFixture(t)
	t.Cleanup(func() {
		simklPreparationGate <- struct{}{}
		simklPreparationRetryAt = time.Time{}
		<-simklPreparationGate
	})
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "60")
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	t.Cleanup(provider.Close)
	f.provider.BaseURL = provider.URL
	var initial simklSummary
	_ = json.Unmarshal(f.req(t, 0, "POST", "/api/simkl/preview", f.source, 200), &initial)
	for range 2 {
		var result struct {
			Paused     bool
			Next       int
			RetryAfter int `json:"retry_after"`
		}
		_ = json.Unmarshal(f.req(t, 0, "POST", "/api/simkl/"+initial.ID+"/prepare", map[string]any{"offset": 0}, 200), &result)
		if !result.Paused || result.Next != 0 || result.RetryAfter < 50 {
			t.Fatal("provider cooldown not surfaced", result)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("cooldown still hit provider", calls.Load())
	}
}
func TestSIMKLBulkMatchingLeavesDuplicateTitlesAndSpecialsForReview(t *testing.T) {
	f := newSimklImportFixture(t)
	_ = f.prepare(t)
	// Distinct SIMKL entries sharing a TMDB show are not silently merged.
	raw, _ := json.Marshal(f.source)
	var duplicate map[string]any
	_ = json.Unmarshal(raw, &duplicate)
	shows := duplicate["shows"].([]any)
	copyRaw, _ := json.Marshal(shows[0])
	var second map[string]any
	_ = json.Unmarshal(copyRaw, &second)
	second["show"].(map[string]any)["ids"].(map[string]any)["simkl"] = 23
	duplicate["shows"] = append(shows, second)
	var preview simklSummary
	_ = json.Unmarshal(f.req(t, 0, "POST", "/api/simkl/preview", duplicate, 200), &preview)
	if preview.EpisodeMatches != 0 || preview.EpisodeExceptions != 4 {
		t.Fatal("duplicate source titles were guessed", preview)
	}
	shows = f.source["shows"].([]any)
	show := shows[0].(map[string]any)
	show["seasons"] = append(show["seasons"].([]any), map[string]any{"number": 0, "episodes": []any{map[string]any{"number": 1}}})
	_ = json.Unmarshal(f.req(t, 0, "POST", "/api/simkl/preview", f.source, 200), &preview)
	if preview.EpisodeMatches != 2 || preview.EpisodeExceptions != 1 {
		t.Fatal("special episode was guessed", preview)
	}
}
func TestSIMKLIdenticalBulkReimportDoesNotCreateReviews(t *testing.T) {
	f := newSimklImportFixture(t)
	for range 2 {
		ready := f.prepare(t)
		f.req(t, 0, "GET", "/api/export", nil, 200)
		f.req(t, 0, "POST", "/api/simkl/"+ready.ID+"/apply", map[string]any{"confirm_episode_matches": true, "episode_match_digest": ready.EpisodeMatchDigest}, 200)
	}
	var count int
	if err := f.p.QueryRow(context.Background(), "SELECT count(*) FROM import_reviews WHERE user_id=$1 AND NOT resolved", f.users[0]).Scan(&count); err != nil || count != 0 {
		t.Fatal("identical import required review", count, err)
	}
}
