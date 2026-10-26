package httpapi

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/zekihan/watchledger/internal/auth"
	"github.com/zekihan/watchledger/internal/simkl"
)

// SIMKL archives and source snapshots remain private to their importing account.
// Episode-number pairs are never silently converted into another provider's IDs.
func (a *API) simklPreview(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 32<<20))
	if err != nil {
		http.Error(w, "backup exceeds 32 MiB or cannot be read", http.StatusBadRequest)
		return
	}
	backup, err := simkl.Parse(raw)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	uid := auth.Current(r.Context()).ID
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		http.Error(w, "preview unavailable", http.StatusServiceUnavailable)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	summary, err := a.simklSummary(r.Context(), tx, uid, backup)
	if err != nil {
		http.Error(w, "preview unavailable", http.StatusServiceUnavailable)
		return
	}
	preview, err := json.Marshal(summary)
	if err != nil {
		http.Error(w, "preview unavailable", http.StatusServiceUnavailable)
		return
	}
	checksum := fmt.Sprintf("%x", sha256.Sum256(raw))
	var id string
	err = tx.QueryRow(r.Context(), `INSERT INTO imports(user_id,kind,checksum,archive,preview) VALUES($1,'simkl',$2,$3,$4) ON CONFLICT(user_id,checksum) DO UPDATE SET preview=EXCLUDED.preview,state='preview' WHERE imports.kind='simkl' RETURNING id`, uid, checksum, raw, preview).Scan(&id)
	if err != nil {
		http.Error(w, "preview unavailable", http.StatusServiceUnavailable)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		http.Error(w, "preview unavailable", http.StatusServiceUnavailable)
		return
	}
	summary.ID = id
	writeJSON(w, summary)
}

func (a *API) simklApply(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ConfirmEpisodeMatches bool   `json:"confirm_episode_matches"`
		EpisodeMatchDigest    string `json:"episode_match_digest"`
	}
	if r.ContentLength != 0 && !decode(w, r, &in) {
		return
	}
	uid := auth.Current(r.Context()).ID
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		http.Error(w, "import unavailable", http.StatusServiceUnavailable)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	fail := func(e error) bool {
		if e != nil {
			http.Error(w, "import could not be applied; no changes were made", http.StatusConflict)
			return true
		}
		return false
	}
	_, err = tx.Exec(r.Context(), "SELECT id FROM users WHERE id=$1 FOR UPDATE", uid)
	if fail(err) {
		return
	}
	var raw []byte
	var state string
	err = tx.QueryRow(r.Context(), "SELECT archive,state FROM imports WHERE user_id=$1 AND id=$2 AND kind='simkl' FOR UPDATE", uid, r.PathValue("id")).Scan(&raw, &state)
	if err != nil {
		http.Error(w, "preview not found", http.StatusNotFound)
		return
	}
	if state == "applied" {
		writeJSON(w, map[string]string{"status": "already_applied"})
		return
	}
	var backed bool
	err = tx.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM export_receipts WHERE user_id=$1 AND exported_at>now()-interval '24 hours')", uid).Scan(&backed)
	if fail(err) {
		return
	}
	if !backed {
		http.Error(w, "download a current export before applying import", http.StatusPreconditionRequired)
		return
	}
	backup, err := simkl.Parse(raw)
	if fail(err) {
		return
	}
	var approved map[string][]simklEpisodeMatch
	if in.ConfirmEpisodeMatches {
		// Season reconciliation takes this same catalog lock before changing episodes.
		targets := simklTargets(backup)
		rows, e := tx.Query(r.Context(), "SELECT id FROM catalog WHERE id=ANY($1::text[]) ORDER BY id FOR SHARE", targets)
		if fail(e) {
			return
		}
		for rows.Next() {
			var locked string
			if fail(rows.Scan(&locked)) {
				rows.Close()
				return
			}
		}
		rows.Close()
		if fail(rows.Err()) {
			return
		}
		summary, e := a.simklSummary(r.Context(), tx, uid, backup)
		if fail(e) {
			return
		}
		if in.EpisodeMatchDigest == "" || in.EpisodeMatchDigest != summary.EpisodeMatchDigest {
			http.Error(w, "episode proposals changed; refresh the preview before applying", http.StatusConflict)
			return
		}
		approved = make(map[string][]simklEpisodeMatch)
		for _, item := range summary.Items {
			approved[item.Key] = item.Matches
		}
	}
	added, reviewed, episodesAdded := 0, 0, 0
	for _, item := range backup.Items {
		target := item.Target()
		reasons := append([]string{}, item.Issues...)
		var cached bool
		if target != "" && len(item.Issues) == 0 {
			err = tx.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM catalog WHERE id=$1)", target).Scan(&cached)
			if fail(err) {
				return
			}
		}
		if !cached {
			reasons = append(reasons, "unresolved_catalog_identity")
		} else {
			tag, e := tx.Exec(r.Context(), `INSERT INTO library(user_id,catalog_id,base,rating,added_at) VALUES($1,$2,$3,$4,coalesce($5::timestamptz,now())) ON CONFLICT(user_id,catalog_id) DO NOTHING`, uid, target, item.Base, item.Rating, item.AddedAt)
			if fail(e) {
				return
			}
			if tag.RowsAffected() > 0 {
				added++
			} else {
				var conflict bool
				err = tx.QueryRow(r.Context(), "SELECT base<>$3 OR ($4::int IS NOT NULL AND rating IS DISTINCT FROM $4::int) FROM library WHERE user_id=$1 AND catalog_id=$2", uid, target, item.Base, item.Rating).Scan(&conflict)
				if fail(err) {
					return
				}
				if conflict {
					reasons = append(reasons, "existing_app_library_preserved")
				}
			}
			if item.Kind == "movies" && item.MovieWatched {
				var date interface{}
				if item.MovieDate.Day != "" {
					date = item.MovieDate.Day
				}
				provenance, e := json.Marshal(map[string]string{"source": "simkl", "source_key": item.Key, "import_id": r.PathValue("id")})
				if fail(e) {
					return
				}
				tag, e = tx.Exec(r.Context(), `INSERT INTO history(user_id,catalog_id,watched_date,watched_time,provenance) SELECT $1,$2,$3,$4,$5 WHERE NOT EXISTS(SELECT 1 FROM history_tombstones WHERE user_id=$1 AND catalog_id=$2 AND episode_key='') ON CONFLICT(user_id,catalog_id,episode_id) DO NOTHING`, uid, target, date, item.MovieDate.Timestamp, provenance)
				if fail(e) {
					return
				}
				if tag.RowsAffected() == 0 {
					conflict, e := simklWatchConflict(r.Context(), tx, uid, target, "", date, item.MovieDate.Timestamp)
					if fail(e) {
						return
					}
					if conflict {
						reasons = append(reasons, "existing_app_watch_decision_preserved")
					}
				}
			}
			if len(item.Episodes) > 0 {
				matches := approved[item.Key]
				if len(matches) > 0 {
					n, conflict, e := applySIMKLProposals(r.Context(), tx, uid, r.PathValue("id"), item, matches)
					if fail(e) {
						return
					}
					episodesAdded += n
					if conflict {
						reasons = append(reasons, "existing_app_watch_decision_preserved")
					}
				}
				if len(matches) != len(item.Episodes) {
					reasons = append(reasons, "episode_identity_review_required")
				}
			}
		}
		reviewID := scopedID(uid, "simkl-review", r.PathValue("id")+"/"+item.Key)
		if len(reasons) > 0 {
			payload, e := json.Marshal(map[string]interface{}{"source": item, "target": target, "reasons": reasons})
			if fail(e) {
				return
			}
			var resolved bool
			err = tx.QueryRow(r.Context(), "INSERT INTO import_reviews(id,user_id,import_id,item,reason) VALUES($1,$2,$3,$4,'simkl_source_review') ON CONFLICT(id) DO UPDATE SET item=EXCLUDED.item,resolved=import_reviews.resolved AND import_reviews.item->'source'=EXCLUDED.item->'source' AND import_reviews.item->'target'=EXCLUDED.item->'target' RETURNING resolved", reviewID, uid, r.PathValue("id"), payload).Scan(&resolved)
			if fail(err) {
				return
			}
			if !resolved {
				reviewed++
			}
		} else {
			_, err = tx.Exec(r.Context(), "UPDATE import_reviews SET resolved=true WHERE id=$1 AND user_id=$2", reviewID, uid)
			if fail(err) {
				return
			}
		}
	}
	if backup.CustomListsPresent {
		_, err = tx.Exec(r.Context(), `INSERT INTO import_reviews(id,user_id,import_id,item,reason) VALUES($1,$2,$3,'{}','unsupported_custom_lists_retained_in_archive') ON CONFLICT(id) DO NOTHING`, scopedID(uid, "simkl-list-review", r.PathValue("id")), uid, r.PathValue("id"))
		if fail(err) {
			return
		}
		reviewed++
	}
	_, err = tx.Exec(r.Context(), "UPDATE imports SET state='applied' WHERE user_id=$1 AND id=$2", uid, r.PathValue("id"))
	if fail(err) || fail(tx.Commit(r.Context())) {
		return
	}
	writeJSON(w, map[string]interface{}{"status": "applied", "additions": added, "reviews": reviewed, "episodes_added": episodesAdded})
}
func (a *API) importReviews(w http.ResponseWriter, r *http.Request) {
	rows, e := a.DB.Query(r.Context(), `SELECT jsonb_build_object('id',id,'import_id',import_id,'item',item,'reason',reason,'resolved',resolved) FROM import_reviews WHERE user_id=$1 AND NOT resolved ORDER BY id`, auth.Current(r.Context()).ID)
	if e != nil {
		http.Error(w, "reviews unavailable", http.StatusServiceUnavailable)
		return
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var b json.RawMessage
		if e = rows.Scan(&b); e != nil {
			http.Error(w, "reviews unavailable", http.StatusServiceUnavailable)
			return
		}
		out = append(out, b)
	}
	if rows.Err() != nil {
		http.Error(w, "reviews unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, out)
}
func (a *API) importArchive(w http.ResponseWriter, r *http.Request) {
	var raw json.RawMessage
	err := a.DB.QueryRow(r.Context(), "SELECT archive FROM imports WHERE user_id=$1 AND id=$2", auth.Current(r.Context()).ID, r.PathValue("id")).Scan(&raw)
	if err != nil {
		http.Error(w, "archive not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="source-import.json"`)
	writeJSON(w, raw)
}
