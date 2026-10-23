package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/zekihan/watchledger/internal/auth"
	"github.com/zekihan/watchledger/internal/simkl"
)

type queryRows interface {
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
}
type listResolution struct {
	SourceURL string `json:"source_url,omitempty"`
	Key       string `json:"key"`
	Title     string `json:"title"`
	CatalogID string `json:"catalog_id"`
	Reason    string `json:"reason,omitempty"`
}

func resolveCustomMembers(ctx context.Context, db queryRows, uid string, source simkl.CustomList) ([]listResolution, error) {
	targets := map[string]map[string]bool{}
	keys := []string{}
	for _, member := range source.Members {
		keys = append(keys, member.Key)
		targets[member.Key] = map[string]bool{}
		if member.Target != "" {
			targets[member.Key][member.Target] = true
		}
	}
	rows, err := db.Query(ctx, `SELECT DISTINCT member->>'key',member->>'target' FROM imports i CROSS JOIN LATERAL jsonb_array_elements(CASE WHEN jsonb_typeof(i.preview->'items')='array' THEN i.preview->'items' ELSE '[]'::jsonb END) member WHERE i.user_id=$1 AND i.kind='simkl' AND member->>'key'=ANY($2::text[]) AND member->'issues'='[]'::jsonb AND member->>'target'<>''`, uid, keys)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var key, target string
		if err = rows.Scan(&key, &target); err != nil {
			rows.Close()
			return nil, err
		}
		targets[key][target] = true
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	ids := []string{}
	for _, set := range targets {
		for id := range set {
			ids = append(ids, id)
		}
	}
	rows, err = db.Query(ctx, "SELECT catalog_id FROM library WHERE user_id=$1 AND catalog_id=ANY($2::text[])", uid, ids)
	if err != nil {
		return nil, err
	}
	owned := map[string]bool{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		owned[id] = true
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	result := make([]listResolution, 0, len(source.Members))
	for _, member := range source.Members {
		resolved := listResolution{Key: member.Key, Title: member.Title, SourceURL: member.SourceURL}
		set := targets[member.Key]
		if len(set) > 1 {
			resolved.Reason = "conflicting_source_identity"
		} else if len(set) == 0 {
			resolved.Reason = "source_identity_unresolved"
		} else {
			for target := range set {
				if owned[target] {
					resolved.CatalogID = target
				} else {
					resolved.Reason = "add_source_title_to_private_library_first"
				}
			}
		}
		result = append(result, resolved)
	}

	seenTargets := map[string]int{}
	for i := range result {
		id := result[i].CatalogID
		if id == "" {
			continue
		}
		if previous, exists := seenTargets[id]; exists {
			result[previous].CatalogID = ""
			result[previous].Reason = "multiple_source_members_share_target"
			result[i].CatalogID = ""
			result[i].Reason = "multiple_source_members_share_target"
		} else {
			seenTargets[id] = i
		}
	}
	return result, nil
}
func (a *API) customListPreview(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 32<<20))
	if err != nil {
		http.Error(w, "backup exceeds 32 MiB or cannot be read", http.StatusBadRequest)
		return
	}
	a.previewCustomList(w, r, raw, nil, "")
}
func (a *API) previewCustomList(w http.ResponseWriter, r *http.Request, raw []byte, fetchedAt *time.Time, listType string, cachedImport ...string) {
	source, err := simkl.ParseCustomList(raw)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	uid := auth.Current(r.Context()).ID
	members, err := resolveCustomMembers(r.Context(), a.DB, uid, source)
	if err != nil {
		http.Error(w, "list preview unavailable", http.StatusServiceUnavailable)
		return
	}
	unresolved := 0
	for _, member := range members {
		if member.CatalogID == "" {
			unresolved++
		}
	}
	var existing string
	err = a.DB.QueryRow(r.Context(), "SELECT coalesce((SELECT id::text FROM lists WHERE user_id=$1 AND source='simkl-custom-list' AND source_id=$2),'')", uid, source.ID).Scan(&existing)
	if err != nil {
		http.Error(w, "list preview unavailable", http.StatusServiceUnavailable)
		return
	}

	var deleted bool
	err = a.DB.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM list_source_tombstones WHERE user_id=$1 AND source='simkl-custom-list' AND source_id=$2)", uid, source.ID).Scan(&deleted)
	if err != nil {
		http.Error(w, "list preview unavailable", http.StatusServiceUnavailable)
		return
	}
	action := "create_manual_snapshot"
	if unresolved > 0 {
		action = "archive_for_review"
	}
	if existing != "" {
		action = "preserve_existing_app_list"
	}
	if deleted {
		action = "preserve_existing_app_deletion"
	}
	preview := map[string]interface{}{"type": "simkl-custom-list", "source": source, "members": members, "unresolved": unresolved, "existing_list_id": existing, "action": action, "backup_required": true}
	summary, err := json.Marshal(preview)
	if err != nil {
		http.Error(w, "list preview unavailable", http.StatusServiceUnavailable)
		return
	}
	var id string
	if len(cachedImport) > 0 {
		err = a.DB.QueryRow(r.Context(), "UPDATE imports SET preview=$3,state='preview' WHERE user_id=$1 AND id=$2 AND kind='simkl' RETURNING id", uid, cachedImport[0], summary).Scan(&id)
	} else {
		err = a.DB.QueryRow(r.Context(), `INSERT INTO imports(user_id,kind,checksum,archive,preview,api_fetched_at,api_list_type) VALUES($1,'simkl',$2,$3,$4,$5,$6) ON CONFLICT(user_id,checksum) DO UPDATE SET preview=EXCLUDED.preview,state='preview',api_fetched_at=coalesce(EXCLUDED.api_fetched_at,imports.api_fetched_at),api_list_type=CASE WHEN EXCLUDED.api_fetched_at IS NOT NULL THEN EXCLUDED.api_list_type ELSE imports.api_list_type END WHERE imports.kind='simkl' RETURNING id`, uid, fmt.Sprintf("%x", sha256.Sum256(raw)), raw, summary, fetchedAt, listType).Scan(&id)
	}
	if err != nil {
		http.Error(w, "list preview unavailable", http.StatusServiceUnavailable)
		return
	}
	preview["id"] = id
	if fetchedAt != nil {
		preview["api_fetched_at"] = fetchedAt
	}
	writeJSON(w, preview)
}
func (a *API) customListApply(w http.ResponseWriter, r *http.Request) {
	uid := auth.Current(r.Context()).ID
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		http.Error(w, "list import unavailable", http.StatusServiceUnavailable)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	fail := func(e error) bool {
		if e != nil {
			http.Error(w, "list import conflict: no changes were applied", http.StatusConflict)
			return true
		}
		return false
	}
	_, err = tx.Exec(r.Context(), "SELECT id FROM users WHERE id=$1 FOR UPDATE", uid)
	if fail(err) {
		return
	}
	var raw, summary []byte
	var state string
	err = tx.QueryRow(r.Context(), "SELECT archive,preview,state FROM imports WHERE user_id=$1 AND id=$2 AND kind='simkl' FOR UPDATE", uid, r.PathValue("id")).Scan(&raw, &summary, &state)
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
	source, err := simkl.ParseCustomList(raw)
	if fail(err) {
		return
	}
	members, err := resolveCustomMembers(r.Context(), tx, uid, source)
	if fail(err) {
		return
	}
	var saved struct {
		Type    string           `json:"type"`
		Members []listResolution `json:"members"`
	}
	if json.Unmarshal(summary, &saved) != nil || saved.Type != "simkl-custom-list" {
		http.Error(w, "invalid saved list preview", http.StatusConflict)
		return
	}
	if len(saved.Members) != len(members) {
		http.Error(w, "membership resolution changed; preview again", http.StatusConflict)
		return
	}
	for i, member := range members {
		if member.Key != saved.Members[i].Key || member.CatalogID != saved.Members[i].CatalogID || member.Reason != saved.Members[i].Reason {
			http.Error(w, "membership resolution changed; preview again", http.StatusConflict)
			return
		}
	}
	var existing string
	err = tx.QueryRow(r.Context(), "SELECT coalesce((SELECT id::text FROM lists WHERE user_id=$1 AND source='simkl-custom-list' AND source_id=$2),'')", uid, source.ID).Scan(&existing)
	if fail(err) {
		return
	}
	reasons := []string{}
	for _, member := range members {
		if member.Reason != "" {
			reasons = append(reasons, member.Key+":"+member.Reason)
		}
	}
	if existing != "" {
		reasons = append(reasons, "existing_app_list_preserved")
	}

	var deleted bool
	err = tx.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM list_source_tombstones WHERE user_id=$1 AND source='simkl-custom-list' AND source_id=$2)", uid, source.ID).Scan(&deleted)
	if fail(err) {
		return
	}
	if deleted {
		reasons = append(reasons, "existing_app_list_deletion_preserved")
	}
	reviewID := scopedID(uid, "simkl-custom-list-review", r.PathValue("id"))
	status := "created"
	listID := existing
	if len(reasons) > 0 {
		status = "retained_for_review"
		sort.Strings(reasons)
		payload, e := json.Marshal(map[string]interface{}{"source": source, "members": members, "reasons": reasons, "list_id": existing})
		if fail(e) {
			return
		}
		_, err = tx.Exec(r.Context(), `INSERT INTO import_reviews(id,user_id,import_id,item,reason) VALUES($1,$2,$3,$4,'simkl_custom_list_review') ON CONFLICT(id) DO UPDATE SET item=EXCLUDED.item,resolved=false`, reviewID, uid, r.PathValue("id"), payload)
		if fail(err) {
			return
		}
	} else {
		listID = scopedID(uid, "simkl-custom-list", source.ID)
		_, err = tx.Exec(r.Context(), "INSERT INTO lists(id,user_id,name,kind,sort,source,source_id) VALUES($1,$2,$3,'manual','title','simkl-custom-list',$4)", listID, uid, source.Name, source.ID)
		if fail(err) {
			return
		}
		for position, member := range members {
			_, err = tx.Exec(r.Context(), "INSERT INTO list_members(user_id,list_id,catalog_id,position) VALUES($1,$2,$3,$4)", uid, listID, member.CatalogID, position)
			if fail(err) {
				return
			}
		}
		_, err = tx.Exec(r.Context(), "UPDATE import_reviews SET resolved=true WHERE id=$1 AND user_id=$2", reviewID, uid)
		if fail(err) {
			return
		}
	}
	_, err = tx.Exec(r.Context(), "UPDATE imports SET state='applied' WHERE user_id=$1 AND id=$2", uid, r.PathValue("id"))
	if fail(err) || fail(tx.Commit(r.Context())) {
		return
	}
	writeJSON(w, map[string]string{"status": status, "list_id": listID})
}
