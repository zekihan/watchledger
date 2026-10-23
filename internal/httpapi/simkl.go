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
	type candidate struct {
		simkl.Item
		Target string `json:"target"`
		Action string `json:"action"`
	}
	candidates := make([]candidate, 0, len(backup.Items))
	additions, conflicts, unresolved := 0, 0, 0
	for _, item := range backup.Items {
		c := candidate{Item: item, Target: item.Target(), Action: "review"}
		if len(item.Issues) == 0 && c.Target != "" {
			var cached, exists bool
			err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM catalog WHERE id=$2),EXISTS(SELECT 1 FROM library WHERE user_id=$1 AND catalog_id=$2)`, uid, c.Target).Scan(&cached, &exists)
			if err != nil {
				http.Error(w, "preview unavailable", http.StatusServiceUnavailable)
				return
			}
			if cached {
				if exists {
					c.Action = "preserve_existing"
					conflicts++
				} else {
					c.Action = "add"
					additions++
				}
			} else {
				c.Warnings = append(c.Warnings, "catalog_not_cached")
			}
		}
		if c.Action == "review" || len(item.Episodes) > 0 {
			unresolved++
		}
		candidates = append(candidates, c)
	}
	summary := map[string]interface{}{"additions": additions, "conflicts": conflicts, "unresolved": unresolved, "items": candidates, "warnings": backup.Warnings, "backup_required": true, "existing_app_edits_win": true, "episode_mapping_policy": "explicit review required; no number-based inference", "custom_lists_supported": false}
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
	summary["id"] = id
	writeJSON(w, summary)
}

func (a *API) simklApply(w http.ResponseWriter, r *http.Request) {
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
	added, reviewed := 0, 0
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
				reasons = append(reasons, "existing_app_library_preserved")
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
					reasons = append(reasons, "existing_app_watch_decision_preserved")
				}
			}
			if len(item.Episodes) > 0 {
				reasons = append(reasons, "episode_identity_review_required")
			}
		}
		reviewID := scopedID(uid, "simkl-review", r.PathValue("id")+"/"+item.Key)
		if len(reasons) > 0 {
			payload, e := json.Marshal(map[string]interface{}{"source": item, "target": target, "reasons": reasons})
			if fail(e) {
				return
			}
			_, err = tx.Exec(r.Context(), "INSERT INTO import_reviews(id,user_id,import_id,item,reason) VALUES($1,$2,$3,$4,'simkl_source_review') ON CONFLICT(id) DO UPDATE SET item=EXCLUDED.item,resolved=false", reviewID, uid, r.PathValue("id"), payload)
			if fail(err) {
				return
			}
			reviewed++
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
	writeJSON(w, map[string]interface{}{"status": "applied", "additions": added, "reviews": reviewed})
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
