package httpapi

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/zekihan/watchledger/internal/auth"
	"github.com/zekihan/watchledger/internal/portable"
	"github.com/zekihan/watchledger/internal/smart"
	"net/http"
)

func (a *API) portableRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/export", a.export)
	m.HandleFunc("POST /api/restore/preview", a.restorePreview)
	m.HandleFunc("POST /api/restore/{id}/apply", a.restoreApply)
	m.HandleFunc("GET /api/imports", a.imports)
	m.HandleFunc("POST /api/simkl/custom-lists/preview", a.customListPreview)
	m.HandleFunc("POST /api/simkl/custom-lists/{id}/apply", a.customListApply)
	m.HandleFunc("POST /api/simkl/preview", a.simklPreview)
	m.HandleFunc("POST /api/simkl/{id}/apply", a.simklApply)
	m.HandleFunc("GET /api/import-reviews", a.importReviews)
	m.HandleFunc("GET /api/import-reviews/{id}/candidates", a.reviewCandidates)
	m.HandleFunc("POST /api/import-reviews/{id}/catalog", a.loadReviewCatalog)
	m.HandleFunc("POST /api/import-reviews/{id}/resolve", a.resolveImportReview)
	m.HandleFunc("GET /api/imports/{id}/archive", a.importArchive)
}
func (a *API) export(w http.ResponseWriter, r *http.Request) {
	uid := auth.Current(r.Context()).ID
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		http.Error(w, "export unavailable", http.StatusServiceUnavailable)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	if _, e = tx.Exec(r.Context(), "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ"); e != nil {
		http.Error(w, "export unavailable", http.StatusServiceUnavailable)
		return
	}
	var data json.RawMessage
	e = tx.QueryRow(r.Context(), `SELECT jsonb_build_object('version',1,'created_at',now(),'settings',u.settings,
 'catalog',coalesce((SELECT jsonb_agg(to_jsonb(c)) FROM catalog c JOIN library l ON l.catalog_id=c.id WHERE l.user_id=u.id),'[]'),
 'episodes',coalesce((SELECT jsonb_agg(to_jsonb(e)) FROM episodes e JOIN library l ON l.catalog_id=e.catalog_id WHERE l.user_id=u.id),'[]'),
 'statuses',coalesce((SELECT jsonb_agg(to_jsonb(s)-'user_id') FROM statuses s WHERE s.user_id=u.id),'[]'),
 'library',coalesce((SELECT jsonb_agg(to_jsonb(l)-'user_id') FROM library l WHERE l.user_id=u.id),'[]'),
 'list_source_tombstones',coalesce((SELECT jsonb_agg(to_jsonb(t)-'user_id') FROM list_source_tombstones t WHERE t.user_id=u.id),'[]'),
 'simkl_links',coalesce((SELECT jsonb_agg(to_jsonb(k)-'user_id') FROM simkl_episode_links k WHERE k.user_id=u.id),'[]'),
 'tombstones',coalesce((SELECT jsonb_agg(to_jsonb(t)-'user_id') FROM history_tombstones t WHERE t.user_id=u.id),'[]'),
 'history',coalesce((SELECT jsonb_agg(to_jsonb(h)-'user_id'-'local_action_at') FROM history h WHERE h.user_id=u.id),'[]'),
 'lists',coalesce((SELECT jsonb_agg((to_jsonb(l)-'user_id') || jsonb_build_object('members',coalesce((SELECT jsonb_agg(catalog_id ORDER BY position) FROM list_members m WHERE m.user_id=u.id AND m.list_id=l.id),'[]'))) FROM lists l WHERE l.user_id=u.id),'[]')) FROM users u WHERE u.id=$1`, uid).Scan(&data)
	if e != nil {
		http.Error(w, "export unavailable", http.StatusServiceUnavailable)
		return
	}
	if _, e = tx.Exec(r.Context(), "INSERT INTO export_receipts(user_id) VALUES($1) ON CONFLICT(user_id) DO UPDATE SET exported_at=now()", uid); e != nil {
		http.Error(w, "export unavailable", http.StatusServiceUnavailable)
		return
	}
	if e = tx.Commit(r.Context()); e != nil {
		http.Error(w, "export unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="watchledger-backup.json"`)
	writeJSON(w, data)
}
func (a *API) restorePreview(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
	var raw json.RawMessage
	if e := json.NewDecoder(r.Body).Decode(&raw); e != nil {
		http.Error(w, "invalid JSON backup", http.StatusBadRequest)
		return
	}
	var s portable.Snapshot
	if e := json.Unmarshal(raw, &s); e != nil {
		http.Error(w, "invalid backup", http.StatusBadRequest)
		return
	}
	if e := s.Validate(); e != nil {
		http.Error(w, e.Error(), http.StatusBadRequest)
		return
	}
	uid := auth.Current(r.Context()).ID
	conflicts := []string{}
	additions := 0
	for _, l := range s.Library {
		var exists bool
		if e := a.DB.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM library WHERE user_id=$1 AND catalog_id=$2)", uid, l.CatalogID).Scan(&exists); e != nil {
			http.Error(w, "preview unavailable", http.StatusServiceUnavailable)
			return
		}
		if exists {
			conflicts = append(conflicts, l.CatalogID)
		} else {
			additions++
		}
	}
	preview := map[string]interface{}{"additions": additions, "conflicts": conflicts, "history_candidates": len(s.History), "existing_app_edits_win": true, "backup_required": true, "settings_policy": "restore only when current settings are empty", "mapping_consents_restored": false}
	summary, e := json.Marshal(preview)
	if e != nil {
		http.Error(w, "preview unavailable", http.StatusServiceUnavailable)
		return
	}
	checksum := fmt.Sprintf("%x", sha256.Sum256(raw))
	var id string
	e = a.DB.QueryRow(r.Context(), `INSERT INTO imports(user_id,kind,checksum,archive,preview) VALUES($1,'restore',$2,$3,$4) ON CONFLICT(user_id,checksum) DO UPDATE SET preview=EXCLUDED.preview RETURNING id`, uid, checksum, raw, summary).Scan(&id)
	if e != nil {
		http.Error(w, "preview unavailable", http.StatusServiceUnavailable)
		return
	}
	preview["id"] = id
	writeJSON(w, preview)
}
func scopedID(uid, kind, id string) string {
	x := sha256.Sum256([]byte(uid + "/" + kind + "/" + id))
	x[6] = (x[6] & 15) | 80
	x[8] = (x[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", x[0:4], x[4:6], x[6:8], x[8:10], x[10:16])
}
func remapRule(r smart.Rule, ids map[string]string) smart.Rule {
	if r.Field == "list" {
		r.Value = ids[r.Value]
	}
	for i, c := range r.Children {
		r.Children[i] = remapRule(c, ids)
	}
	return r
}
func (a *API) restoreApply(w http.ResponseWriter, r *http.Request) {
	uid := auth.Current(r.Context()).ID
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		http.Error(w, "restore unavailable", http.StatusServiceUnavailable)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	if _, e = tx.Exec(r.Context(), "SELECT id FROM users WHERE id=$1 FOR UPDATE", uid); e != nil {
		http.Error(w, "restore unavailable", http.StatusServiceUnavailable)
		return
	}
	var backed bool
	if e = tx.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM export_receipts WHERE user_id=$1 AND exported_at>now()-interval '24 hours')", uid).Scan(&backed); e != nil || !backed {
		http.Error(w, "download a current export before applying restore", http.StatusPreconditionRequired)
		return
	}
	var raw []byte
	var state string
	if e = tx.QueryRow(r.Context(), "SELECT archive,state FROM imports WHERE user_id=$1 AND id=$2 AND kind='restore' FOR UPDATE", uid, r.PathValue("id")).Scan(&raw, &state); e != nil {
		http.Error(w, "preview not found", http.StatusNotFound)
		return
	}
	if state == "applied" {
		writeJSON(w, map[string]string{"status": "already_applied"})
		return
	}
	var s portable.Snapshot
	if json.Unmarshal(raw, &s) != nil || s.Validate() != nil {
		http.Error(w, "invalid saved backup", http.StatusBadRequest)
		return
	}
	fail := func(err error) bool {
		if err != nil {
			http.Error(w, "restore conflict: no changes were applied", http.StatusConflict)
			return true
		}
		return false
	}
	for _, c := range s.Catalog {
		_, e = tx.Exec(r.Context(), "INSERT INTO catalog(id,source,source_id,kind,title,metadata,fetched_at) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(id) DO NOTHING", c.ID, c.Source, c.SourceID, c.Kind, c.Title, c.Metadata, c.FetchedAt)
		if fail(e) {
			return
		}
		var source, sourceID, kind string
		e = tx.QueryRow(r.Context(), "SELECT source,source_id,kind FROM catalog WHERE id=$1", c.ID).Scan(&source, &sourceID, &kind)
		if fail(e) {
			return
		}
		if source != c.Source || sourceID != c.SourceID || kind != c.Kind {
			http.Error(w, "catalog identity conflict", http.StatusConflict)
			return
		}
	}
	for _, ep := range s.Episodes {
		_, e = tx.Exec(r.Context(), `INSERT INTO episodes(id,catalog_id,source,source_id,season,number,kind,metadata,air_date,air_time,retired) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT(id) DO NOTHING`, ep.ID, ep.CatalogID, ep.Source, ep.SourceID, ep.Season, ep.Number, ep.Kind, ep.Metadata, ep.AirDate, ep.AirTime, ep.Retired)
		if fail(e) {
			return
		}
		var catalogID string
		e = tx.QueryRow(r.Context(), "SELECT catalog_id FROM episodes WHERE id=$1", ep.ID).Scan(&catalogID)
		if fail(e) {
			return
		}
		if catalogID != ep.CatalogID {
			http.Error(w, "episode identity conflict", http.StatusConflict)
			return
		}
	}
	statusIDs := map[string]string{}
	for _, st := range s.Statuses {
		var existing string
		err := tx.QueryRow(r.Context(), "SELECT id FROM statuses WHERE user_id=$1 AND id=$2", uid, st.ID).Scan(&existing)
		id := scopedID(uid, "status", st.ID)
		if err == nil {
			id = existing
		}
		statusIDs[st.ID] = id
		_, e = tx.Exec(r.Context(), `INSERT INTO statuses(id,user_id,name,base,used) VALUES($1,$2,$3,$4,$5) ON CONFLICT(id) DO NOTHING`, id, uid, st.Name, st.Base, st.Used)
		if fail(e) {
			return
		}
	}
	for _, l := range s.Library {
		var sid interface{}
		if l.StatusID != nil {
			sid = statusIDs[*l.StatusID]
		}
		_, e = tx.Exec(r.Context(), `INSERT INTO library(user_id,catalog_id,base,status_id,rating,position,anime_view,added_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(user_id,catalog_id) DO NOTHING`, uid, l.CatalogID, l.Base, sid, l.Rating, l.Position, l.AnimeView, l.AddedAt, l.UpdatedAt)
		if fail(e) {
			return
		}
	}

	for _, link := range s.SIMKLLinks {
		_, e = tx.Exec(r.Context(), "INSERT INTO simkl_episode_links(user_id,source_key,catalog_id,episode_id) VALUES($1,$2,$3,$4) ON CONFLICT(user_id,source_key) DO NOTHING", uid, link.SourceKey, link.CatalogID, link.EpisodeID)
		if fail(e) {
			return
		}
		var existing string
		e = tx.QueryRow(r.Context(), "SELECT episode_id FROM simkl_episode_links WHERE user_id=$1 AND source_key=$2", uid, link.SourceKey).Scan(&existing)
		if fail(e) {
			return
		}
		if existing != link.EpisodeID {
			http.Error(w, "confirmed SIMKL identity conflict; existing history was not changed", http.StatusConflict)
			return
		}
	}
	for _, h := range s.History {
		episodeKey := ""
		if h.EpisodeID != nil {
			episodeKey = *h.EpisodeID
		}
		var deleted bool
		e = tx.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM history_tombstones WHERE user_id=$1 AND catalog_id=$2 AND episode_key=$3)", uid, h.CatalogID, episodeKey).Scan(&deleted)
		if fail(e) {
			return
		}
		if deleted {
			continue
		}

		_, e = tx.Exec(r.Context(), `INSERT INTO history(user_id,catalog_id,episode_id,watched_date,watched_time,provenance,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(user_id,catalog_id,episode_id) DO NOTHING`, uid, h.CatalogID, h.EpisodeID, h.WatchedDate, h.WatchedTime, h.Provenance, h.CreatedAt, h.UpdatedAt)
		if fail(e) {
			return
		}
	}
	for _, t := range s.Tombstones {
		_, e = tx.Exec(r.Context(), "INSERT INTO history_tombstones(user_id,catalog_id,episode_key,deleted_at) SELECT $1,$2,$3,$4 WHERE NOT EXISTS(SELECT 1 FROM history WHERE user_id=$1 AND catalog_id=$2 AND coalesce(episode_id,'')=$3) ON CONFLICT DO NOTHING", uid, t.CatalogID, t.EpisodeKey, t.DeletedAt)
		if fail(e) {
			return
		}
	}

	for _, deleted := range s.ListSourceTombstones {
		_, e = tx.Exec(r.Context(), `INSERT INTO list_source_tombstones(user_id,source,source_id,deleted_at) SELECT $1,$2,$3,$4 WHERE NOT EXISTS(SELECT 1 FROM lists WHERE user_id=$1 AND source=$2 AND source_id=$3) ON CONFLICT DO NOTHING`, uid, deleted.Source, deleted.SourceID, deleted.DeletedAt)
		if fail(e) {
			return
		}
	}
	listIDs := map[string]string{}
	newLists := map[string]bool{}
	for _, l := range s.Lists {
		var original bool
		e = tx.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM lists WHERE user_id=$1 AND id=$2)", uid, l.ID).Scan(&original)
		if fail(e) {
			return
		}
		id := scopedID(uid, "list", l.ID)
		if original {
			id = l.ID
		}

		if l.Source != nil {
			var deleted bool
			e = tx.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM list_source_tombstones WHERE user_id=$1 AND source=$2 AND source_id=$3)", uid, *l.Source, *l.SourceID).Scan(&deleted)
			if fail(e) {
				return
			}
			if deleted {
				http.Error(w, "existing app list deletion conflicts with restore; no changes applied", http.StatusConflict)
				return
			}
			var sourceID string
			e = tx.QueryRow(r.Context(), "SELECT coalesce((SELECT id::text FROM lists WHERE user_id=$1 AND source=$2 AND source_id=$3),'')", uid, *l.Source, *l.SourceID).Scan(&sourceID)
			if fail(e) {
				return
			}
			if sourceID != "" {
				id = sourceID
			}
		}
		listIDs[l.ID] = id
		tag, err := tx.Exec(r.Context(), "INSERT INTO lists(id,user_id,name,kind,sort,source,source_id) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(id) DO NOTHING", id, uid, l.Name, l.Kind, l.Sort, l.Source, l.SourceID)
		if fail(err) {
			return
		}

		if l.Source != nil {
			var existingSource, existingSourceID string
			e = tx.QueryRow(r.Context(), "SELECT coalesce(source,''),coalesce(source_id,'') FROM lists WHERE user_id=$1 AND id=$2", uid, id).Scan(&existingSource, &existingSourceID)
			if fail(e) {
				return
			}
			if existingSource != *l.Source || existingSourceID != *l.SourceID {
				http.Error(w, "list source identity conflict", http.StatusConflict)
				return
			}
		}
		newLists[l.ID] = tag.RowsAffected() > 0
	}
	for _, l := range s.Lists {
		if !newLists[l.ID] {
			continue
		}
		id := listIDs[l.ID]
		if l.Rule != nil {
			rule := remapRule(*l.Rule, listIDs)
			b, err := json.Marshal(rule)
			if fail(err) {
				return
			}
			_, e = tx.Exec(r.Context(), "UPDATE lists SET rule=$3 WHERE user_id=$1 AND id=$2", uid, id, b)
			if fail(e) {
				return
			}
			for _, ref := range smart.References(rule) {
				_, e = tx.Exec(r.Context(), "INSERT INTO list_dependencies(user_id,list_id,depends_on) VALUES($1,$2,$3) ON CONFLICT DO NOTHING", uid, id, ref)
				if fail(e) {
					return
				}
			}
		}
		for n, member := range l.Members {
			_, e = tx.Exec(r.Context(), "INSERT INTO list_members(user_id,list_id,catalog_id,position) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING", uid, id, member, n)
			if fail(e) {
				return
			}
		}
	}
	if len(s.Settings) > 0 {
		_, e = tx.Exec(r.Context(), "UPDATE users SET settings=$2 WHERE id=$1 AND settings='{}'::jsonb", uid, s.Settings)
		if fail(e) {
			return
		}
	}
	_, e = tx.Exec(r.Context(), "UPDATE imports SET state='applied' WHERE user_id=$1 AND id=$2", uid, r.PathValue("id"))
	if fail(e) {
		return
	}
	if fail(tx.Commit(r.Context())) {
		return
	}
	writeJSON(w, map[string]string{"status": "applied"})
}
func (a *API) imports(w http.ResponseWriter, r *http.Request) {
	rows, e := a.DB.Query(r.Context(), "SELECT jsonb_build_object('id',id,'kind',kind,'preview',preview,'state',state,'created_at',created_at) FROM imports WHERE user_id=$1 ORDER BY created_at DESC", auth.Current(r.Context()).ID)
	if e != nil {
		http.Error(w, "imports unavailable", http.StatusServiceUnavailable)
		return
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var b json.RawMessage
		if e = rows.Scan(&b); e != nil {
			http.Error(w, "imports unavailable", http.StatusServiceUnavailable)
			return
		}
		out = append(out, b)
	}
	if rows.Err() != nil {
		http.Error(w, "imports unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, out)
}
