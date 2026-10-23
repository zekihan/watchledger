package httpapi

import (
	"encoding/json"
	"net/http"

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
	rows, err := a.DB.Query(r.Context(), `SELECT jsonb_build_object('id',e.id,'season',e.season,'number',e.number,'retired',e.retired,'metadata',e.metadata) FROM episodes e JOIN library l ON l.catalog_id=e.catalog_id AND l.user_id=$1 WHERE e.catalog_id=$2 ORDER BY e.season,e.number,e.id`, uid, review.Source.Target())
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
	writeJSON(w, map[string]interface{}{"source": review.Source, "target": review.Source.Target(), "episodes": candidates, "links": links})
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
	if resolved {
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
	var review sourceReview
	if json.Unmarshal(payload, &review) != nil || len(review.Source.Issues) > 0 || review.Source.Target() == "" {
		http.Error(w, "source identity or validation issue must be resolved first", http.StatusConflict)
		return
	}
	target := review.Source.Target()
	var exists bool
	err = tx.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM library WHERE user_id=$1 AND catalog_id=$2)", uid, target).Scan(&exists)
	if fail(err) {
		return
	}
	if !exists {
		http.Error(w, "load catalog details and re-preview the source backup first", http.StatusConflict)
		return
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
	// Library status/rating and all pre-existing watch decisions intentionally remain untouched.
	_, err = tx.Exec(r.Context(), "UPDATE import_reviews SET resolved=true WHERE user_id=$1 AND id=$2", uid, r.PathValue("id"))
	if fail(err) || fail(tx.Commit(r.Context())) {
		return
	}
	writeJSON(w, map[string]interface{}{"status": "resolved", "history_added": applied, "app_decisions_preserved": preserved})
}
