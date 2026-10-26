package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/zekihan/watchledger/internal/auth"
	"github.com/zekihan/watchledger/internal/simkl"
)

type sourceReview struct {
	Source  simkl.Item `json:"source"`
	Target  string     `json:"target"`
	Reasons []string   `json:"reasons"`
}
type episodeLink struct {
	SourceKey string `json:"source_key"`
	EpisodeID string `json:"episode_id"`
}

func (a *API) reviewCandidates(w http.ResponseWriter, r *http.Request) {
	uid := auth.Current(r.Context()).ID
	var payload []byte
	if err := a.DB.QueryRow(r.Context(), "SELECT item FROM import_reviews WHERE user_id=$1 AND id=$2 AND reason='simkl_source_review'", uid, r.PathValue("id")).Scan(&payload); err != nil {
		http.Error(w, "review not found", http.StatusNotFound)
		return
	}
	var review sourceReview
	if json.Unmarshal(payload, &review) != nil {
		http.Error(w, "invalid review", http.StatusConflict)
		return
	}
	rows, err := a.DB.Query(r.Context(), `SELECT jsonb_build_object('id',e.id,'season',e.season,'number',e.number,'retired',e.retired,'metadata',e.metadata,'watched',h.id IS NOT NULL,'watched_date',h.watched_date,'watched_time',h.watched_time,'explicit_unwatch',t.episode_key IS NOT NULL) FROM episodes e LEFT JOIN history h ON h.user_id=$1 AND h.episode_id=e.id LEFT JOIN history_tombstones t ON t.user_id=$1 AND t.catalog_id=e.catalog_id AND t.episode_key=e.id WHERE e.catalog_id=$2 ORDER BY e.season,e.number,e.id`, uid, review.Source.Target())
	if err != nil {
		http.Error(w, "candidates unavailable", http.StatusServiceUnavailable)
		return
	}
	candidates := []json.RawMessage{}
	for rows.Next() {
		var b json.RawMessage
		if err = rows.Scan(&b); err != nil {
			rows.Close()
			http.Error(w, "candidates unavailable", http.StatusServiceUnavailable)
			return
		}
		candidates = append(candidates, b)
	}
	rows.Close()
	if rows.Err() != nil {
		http.Error(w, "candidates unavailable", http.StatusServiceUnavailable)
		return
	}
	keys := []string{}
	for _, ep := range review.Source.Episodes {
		keys = append(keys, ep.Key)
	}
	rows, err = a.DB.Query(r.Context(), "SELECT source_key,episode_id FROM simkl_episode_links WHERE user_id=$1 AND source_key=ANY($2::text[])", uid, keys)
	if err != nil {
		http.Error(w, "links unavailable", http.StatusServiceUnavailable)
		return
	}
	defer rows.Close()
	links := []episodeLink{}
	for rows.Next() {
		var link episodeLink
		if err = rows.Scan(&link.SourceKey, &link.EpisodeID); err != nil {
			http.Error(w, "links unavailable", http.StatusServiceUnavailable)
			return
		}
		links = append(links, link)
	}
	if rows.Err() != nil {
		http.Error(w, "links unavailable", http.StatusServiceUnavailable)
		return
	}
	var current json.RawMessage
	err = a.DB.QueryRow(r.Context(), `SELECT coalesce((SELECT jsonb_build_object('base',l.base,'rating',l.rating,'status_name',s.name,'watched',h.id IS NOT NULL,'watched_date',h.watched_date,'watched_time',h.watched_time,'explicit_unwatch',t.episode_key IS NOT NULL) FROM library l LEFT JOIN statuses s ON s.user_id=l.user_id AND s.id=l.status_id LEFT JOIN history h ON h.user_id=l.user_id AND h.catalog_id=l.catalog_id AND h.episode_id IS NULL LEFT JOIN history_tombstones t ON t.user_id=l.user_id AND t.catalog_id=l.catalog_id AND t.episode_key='' WHERE l.user_id=$1 AND l.catalog_id=$2),'null'::jsonb)`, uid, review.Source.Target()).Scan(&current)
	if err != nil {
		http.Error(w, "current app state unavailable", http.StatusServiceUnavailable)
		return
	}
	var cached bool
	var variants []byte
	err = a.DB.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM catalog WHERE id=$1),coalesce((SELECT metadata FROM catalog WHERE id=$1),'{}'::jsonb)", review.Source.Target()).Scan(&cached, &variants)
	if err != nil {
		http.Error(w, "catalog state unavailable", http.StatusServiceUnavailable)
		return
	}
	var translations map[string]json.RawMessage
	_ = json.Unmarshal(variants, &translations)
	seasonSet := map[int]bool{}
	seasons := []int{}
	for _, raw := range translations {
		var metadata struct {
			Seasons []struct {
				Number int `json:"season_number"`
			} `json:"seasons"`
		}
		if json.Unmarshal(raw, &metadata) != nil {
			continue
		}
		for _, season := range metadata.Seasons {
			if season.Number >= 0 && !seasonSet[season.Number] {
				seasonSet[season.Number] = true
				seasons = append(seasons, season.Number)
			}
		}
	}
	sort.Ints(seasons)
	writeJSON(w, map[string]interface{}{"source": review.Source, "target": review.Source.Target(), "episodes": candidates, "links": links, "current": current, "catalog_cached": cached, "seasons": seasons})
}

func (a *API) resolveImportReview(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Confirm        bool          `json:"confirm"`
		AcceptExisting bool          `json:"accept_existing"`
		Links          []episodeLink `json:"links"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !in.Confirm || !in.AcceptExisting {
		http.Error(w, "explicit mapping confirmation and preservation of app edits required", http.StatusBadRequest)
		return
	}
	uid := auth.Current(r.Context()).ID
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		http.Error(w, "review unavailable", http.StatusServiceUnavailable)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	fail := func(e error) bool {
		if e != nil {
			http.Error(w, "review could not be applied; no changes were made", http.StatusConflict)
			return true
		}
		return false
	}
	_, err = tx.Exec(r.Context(), "SELECT id FROM users WHERE id=$1 FOR UPDATE", uid)
	if fail(err) {
		return
	}
	var payload []byte
	var resolved bool
	var importID string
	err = tx.QueryRow(r.Context(), "SELECT item,resolved,import_id FROM import_reviews WHERE user_id=$1 AND id=$2 AND reason='simkl_source_review' FOR UPDATE", uid, r.PathValue("id")).Scan(&payload, &resolved, &importID)
	if err != nil {
		http.Error(w, "review not found", http.StatusNotFound)
		return
	}
	var review sourceReview
	if json.Unmarshal(payload, &review) != nil || len(review.Source.Issues) > 0 || review.Source.Target() == "" {
		http.Error(w, "source identity or validation issue must be resolved first", http.StatusConflict)
		return
	}
	if resolved {
		if len(in.Links) != len(review.Source.Episodes) {
			http.Error(w, "explicit mapping required for every source episode", http.StatusBadRequest)
			return
		}
		remaining := map[string]bool{}
		for _, ep := range review.Source.Episodes {
			remaining[ep.Key] = true
		}
		for _, link := range in.Links {
			var matches bool
			err = tx.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM simkl_episode_links WHERE user_id=$1 AND source_key=$2 AND catalog_id=$3 AND episode_id=$4)", uid, link.SourceKey, review.Source.Target(), link.EpisodeID).Scan(&matches)
			if fail(err) {
				return
			}
			if !remaining[link.SourceKey] || !matches {
				http.Error(w, "source episode already has a different confirmed identity; history was not moved", http.StatusConflict)
				return
			}
			delete(remaining, link.SourceKey)
		}
		writeJSON(w, map[string]string{"status": "already_resolved"})
		return
	}
	var backed bool
	err = tx.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM export_receipts WHERE user_id=$1 AND exported_at>now()-interval '24 hours')", uid).Scan(&backed)
	if fail(err) {
		return
	}
	if !backed {
		http.Error(w, "download a current export before resolving import", http.StatusPreconditionRequired)
		return
	}
	target := review.Source.Target()
	var exists bool
	err = tx.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM library WHERE user_id=$1 AND catalog_id=$2)", uid, target).Scan(&exists)
	if fail(err) {
		return
	}
	if !exists {
		var cached bool
		err = tx.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM catalog WHERE id=$1)", target).Scan(&cached)
		if fail(err) {
			return
		}
		if !cached {
			http.Error(w, "load the explicitly identified catalog title first", http.StatusConflict)
			return
		}
		_, err = tx.Exec(r.Context(), "INSERT INTO library(user_id,catalog_id,base,rating,added_at) VALUES($1,$2,$3,$4,coalesce($5::timestamptz,now())) ON CONFLICT(user_id,catalog_id) DO NOTHING", uid, target, review.Source.Base, review.Source.Rating, review.Source.AddedAt)
		if fail(err) {
			return
		}
	}
	if len(in.Links) != len(review.Source.Episodes) {
		http.Error(w, "explicit mapping required for every source episode", http.StatusBadRequest)
		return
	}
	sources := map[string]simkl.Episode{}
	for _, ep := range review.Source.Episodes {
		sources[ep.Key] = ep
	}
	seenSource, seenTarget := map[string]bool{}, map[string]bool{}
	applied, preserved := 0, 0
	for _, link := range in.Links {
		ep, ok := sources[link.SourceKey]
		if !ok || seenSource[link.SourceKey] || seenTarget[link.EpisodeID] {
			http.Error(w, "invalid, duplicate, or combined episode mapping", http.StatusBadRequest)
			return
		}
		seenSource[link.SourceKey] = true
		seenTarget[link.EpisodeID] = true
		var owner string
		var retired bool
		err = tx.QueryRow(r.Context(), "SELECT catalog_id,retired FROM episodes WHERE id=$1", link.EpisodeID).Scan(&owner, &retired)
		if err != nil || owner != target {
			http.Error(w, "target episode does not belong to the source title", http.StatusConflict)
			return
		}
		var previous string
		err = tx.QueryRow(r.Context(), "SELECT coalesce((SELECT episode_id FROM simkl_episode_links WHERE user_id=$1 AND source_key=$2),'')", uid, link.SourceKey).Scan(&previous)
		if fail(err) {
			return
		}
		if previous != "" && previous != link.EpisodeID {
			http.Error(w, "source episode already has a different confirmed identity; history was not moved", http.StatusConflict)
			return
		}
		if retired && previous == "" {
			http.Error(w, "new mappings cannot target retired episodes", http.StatusConflict)
			return
		}
		_, err = tx.Exec(r.Context(), "INSERT INTO simkl_episode_links(user_id,source_key,catalog_id,episode_id) VALUES($1,$2,$3,$4) ON CONFLICT(user_id,source_key) DO NOTHING", uid, link.SourceKey, target, link.EpisodeID)
		if fail(err) {
			return
		}
		provenance, e := json.Marshal(map[string]string{"source": "simkl", "source_key": link.SourceKey, "import_id": importID, "mapping": "user_confirmed"})
		if fail(e) {
			return
		}
		var date interface{}
		if ep.Date.Day != "" {
			date = ep.Date.Day
		}
		tag, e := tx.Exec(r.Context(), `INSERT INTO history(user_id,catalog_id,episode_id,watched_date,watched_time,provenance) SELECT $1,$2,$3,$4,$5,$6 WHERE NOT EXISTS(SELECT 1 FROM history_tombstones WHERE user_id=$1 AND catalog_id=$2 AND episode_key=$3) ON CONFLICT(user_id,catalog_id,episode_id) DO NOTHING`, uid, target, link.EpisodeID, date, ep.Date.Timestamp, provenance)
		if fail(e) {
			return
		}
		if tag.RowsAffected() > 0 {
			applied++
		} else {
			preserved++
		}
	}

	if review.Source.Kind == "movies" && review.Source.MovieWatched {
		var date interface{}
		if review.Source.MovieDate.Day != "" {
			date = review.Source.MovieDate.Day
		}
		provenance, e := json.Marshal(map[string]string{"source": "simkl", "source_key": review.Source.Key, "import_id": importID})
		if fail(e) {
			return
		}
		tag, e := tx.Exec(r.Context(), `INSERT INTO history(user_id,catalog_id,watched_date,watched_time,provenance) SELECT $1,$2,$3,$4,$5 WHERE NOT EXISTS(SELECT 1 FROM history_tombstones WHERE user_id=$1 AND catalog_id=$2 AND episode_key='') ON CONFLICT(user_id,catalog_id,episode_id) DO NOTHING`, uid, target, date, review.Source.MovieDate.Timestamp, provenance)
		if fail(e) {
			return
		}
		if tag.RowsAffected() > 0 {
			applied++
		} else {
			preserved++
		}
	}
	// Library status/rating and all pre-existing watch decisions intentionally remain untouched.
	_, err = tx.Exec(r.Context(), "UPDATE import_reviews SET resolved=true WHERE user_id=$1 AND id=$2", uid, r.PathValue("id"))
	if fail(err) || fail(tx.Commit(r.Context())) {
		return
	}
	writeJSON(w, map[string]interface{}{"status": "resolved", "history_added": applied, "app_decisions_preserved": preserved})
}

// loadReviewCatalog requests only the public provider identity explicitly present
// in this account's source record. Loading metadata never applies private history.
func (a *API) loadReviewCatalog(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Season *int `json:"season,omitempty"`
	}
	if !decode(w, r, &in) {
		return
	}
	uid := auth.Current(r.Context()).ID
	var payload []byte
	if err := a.DB.QueryRow(r.Context(), "SELECT item FROM import_reviews WHERE user_id=$1 AND id=$2 AND reason='simkl_source_review'", uid, r.PathValue("id")).Scan(&payload); err != nil {
		http.Error(w, "review not found", http.StatusNotFound)
		return
	}
	var review sourceReview
	if json.Unmarshal(payload, &review) != nil || len(review.Source.Issues) > 0 {
		http.Error(w, "source validation must be resolved first", http.StatusConflict)
		return
	}
	parts := strings.Split(review.Source.Target(), ":")
	if len(parts) == 3 && parts[0] == "anidb" && parts[1] == "anime" {
		if in.Season != nil {
			http.Error(w, "native episodes load with the source entry", http.StatusBadRequest)
			return
		}
		if a.Anime == nil {
			http.Error(w, "AniDB source unavailable", http.StatusServiceUnavailable)
			return
		}
		var settings struct {
			Language string `json:"metadata_language"`
			Style    string `json:"anime_title"`
		}
		_ = json.Unmarshal(auth.Current(r.Context()).Settings, &settings)
		detail, err := a.Anime.Detail(r.Context(), parts[2], settings.Language, settings.Style)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, map[string]interface{}{"status": "catalog_loaded", "target": review.Source.Target(), "history_applied": false, "stale": detail.Stale, "warning": detail.Warning})
		return
	}
	if len(parts) != 3 || parts[0] != "tmdb" || (parts[1] != "movie" && parts[1] != "tv") {
		http.Error(w, "no supported explicit TMDB identity; no match was guessed", http.StatusConflict)
		return
	}
	if in.Season != nil && (parts[1] != "tv" || *in.Season < 0) {
		http.Error(w, "invalid season request", http.StatusBadRequest)
		return
	}
	var settings struct {
		Language string `json:"metadata_language"`
	}
	_ = json.Unmarshal(auth.Current(r.Context()).Settings, &settings)
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	detail, err := a.Catalog.Detail(ctx, parts[1], parts[2], settings.Language, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if in.Season != nil {
		var metadata struct {
			Seasons []struct {
				Number int `json:"season_number"`
			} `json:"seasons"`
		}
		if json.Unmarshal(detail.Metadata, &metadata) != nil {
			http.Error(w, "invalid season metadata", http.StatusBadGateway)
			return
		}
		found := false
		for _, season := range metadata.Seasons {
			if season.Number == *in.Season {
				found = true
				break
			}
		}
		if !found {
			http.Error(w, "season is not listed by the provider", http.StatusBadRequest)
			return
		}
		seasonData, seasonErr := a.Catalog.Season(ctx, parts[2], *in.Season, settings.Language)
		if err = seasonErr; err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		var freshness struct {
			Stale   bool   `json:"stale"`
			Warning string `json:"warning"`
		}
		if json.Unmarshal(seasonData, &freshness) == nil && freshness.Stale {
			detail.Stale = true
			detail.Warning = freshness.Warning
		}
	}
	writeJSON(w, map[string]interface{}{"status": "catalog_loaded", "target": review.Source.Target(), "history_applied": false, "stale": detail.Stale, "warning": detail.Warning})
}
