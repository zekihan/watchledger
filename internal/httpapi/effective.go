package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/zekihan/watchledger/internal/auth"
	"github.com/zekihan/watchledger/internal/mapping"
)

type recordedLink struct {
	RecordID      string     `json:"record_id"`
	LocalActionAt *time.Time `json:"-"`
	EpisodeID     string     `json:"episode_id"`
	CatalogID     string     `json:"catalog_id"`
	Title         string     `json:"title"`
}
type effectiveSnapshot struct {
	States    map[string]mapping.State
	RawStates map[string]mapping.State
	Direct    map[string]recordedLink
}

// Tracking callers lock the mapping table before their account lock, preventing
// revision changes during apply without reversing the mapping-notice lock order.
func loadEffective(ctx context.Context, db queryRows, uid, catalogID string, extraCatalogs ...string) (effectiveSnapshot, error) {
	out := effectiveSnapshot{Direct: map[string]recordedLink{}}
	query := `WITH related AS (
 SELECT DISTINCT m.id FROM mappings m CROSS JOIN LATERAL jsonb_array_elements(m.relationships) edge JOIN episodes e ON e.id=edge->>'from' OR e.id=edge->>'to' WHERE e.catalog_id=$2 OR e.catalog_id=ANY($3::text[]) OR ($2='' AND EXISTS(SELECT 1 FROM library l WHERE l.user_id=$1 AND l.catalog_id=e.catalog_id))
 ), vertices AS (
 SELECT edge->>'from' AS id FROM mappings m JOIN related r ON r.id=m.id CROSS JOIN LATERAL jsonb_array_elements(m.relationships) edge
 UNION SELECT edge->>'to' FROM mappings m JOIN related r ON r.id=m.id CROSS JOIN LATERAL jsonb_array_elements(m.relationships) edge
 )
 SELECT m.id,m.revision,m.relationships,(m.reviewed AND NOT EXISTS(SELECT 1 FROM jsonb_array_elements(m.relationships) edge LEFT JOIN episodes src ON src.id=edge->>'from' LEFT JOIN episodes dst ON dst.id=edge->>'to' WHERE src.id IS NULL OR dst.id IS NULL OR src.retired OR dst.retired)),m.requires_consent,coalesce(c.revision,'') FROM mappings m LEFT JOIN mapping_consents c ON c.mapping_id=m.id AND c.user_id=$1 WHERE m.id IN(SELECT id FROM related) OR EXISTS(SELECT 1 FROM jsonb_array_elements(m.relationships) edge WHERE edge->>'from' IN(SELECT id FROM vertices) OR edge->>'to' IN(SELECT id FROM vertices)) ORDER BY m.id`
	rows, err := db.Query(ctx, query, uid, catalogID, extraCatalogs)
	if err != nil {
		return out, err
	}
	definitions := []mapping.Definition{}
	for rows.Next() {
		var d mapping.Definition
		var raw []byte
		if err = rows.Scan(&d.ID, &d.Revision, &raw, &d.Reviewed, &d.RequiresConsent, &d.Consented); err != nil {
			rows.Close()
			return out, err
		}
		if err = json.Unmarshal(raw, &d.Edges); err != nil {
			rows.Close()
			return out, err
		}
		definitions = append(definitions, d)
	}
	rows.Close()
	if rows.Err() != nil {
		return out, rows.Err()
	}
	needed := []string{}
	for _, d := range definitions {
		for _, edge := range d.Edges {
			needed = append(needed, edge.From, edge.To)
		}
	}
	rows, err = db.Query(ctx, "SELECT h.id::text,h.episode_id,h.catalog_id,c.title,h.local_action_at FROM history h JOIN catalog c ON c.id=h.catalog_id WHERE h.user_id=$1 AND h.episode_id IS NOT NULL AND ($2='' OR h.catalog_id=$2 OR h.episode_id=ANY($3::text[]))", uid, catalogID, needed)
	if err != nil {
		return out, err
	}
	watched := map[string]bool{}
	for rows.Next() {
		var link recordedLink
		if err = rows.Scan(&link.RecordID, &link.EpisodeID, &link.CatalogID, &link.Title, &link.LocalActionAt); err != nil {
			rows.Close()
			return out, err
		}
		out.Direct[link.EpisodeID] = link
		watched[link.EpisodeID] = true
	}
	rows.Close()
	if rows.Err() != nil {
		return out, rows.Err()
	}
	out.RawStates = mapping.Project(definitions, watched)
	out.States = map[string]mapping.State{}
	for id, state := range out.RawStates {
		out.States[id] = state
	}
	rows, err = db.Query(ctx, "SELECT episode_key,deleted_at FROM history_tombstones WHERE user_id=$1 AND episode_key<>'' AND ($2='' OR catalog_id=$2 OR episode_key=ANY($3::text[]))", uid, catalogID, needed)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var id string
		var deletedAt time.Time
		if err = rows.Scan(&id, &deletedAt); err != nil {
			rows.Close()
			return out, err
		}
		state := out.States[id]
		if !state.Direct {
			newerLocalWatch := state.Derived
			for _, source := range state.Sources {
				record := out.Direct[source]
				newerLocalWatch = newerLocalWatch && record.LocalActionAt != nil && record.LocalActionAt.After(deletedAt)
			}
			if !newerLocalWatch {
				state.Derived = false
				state.ExplicitUnwatch = true
			}
			out.States[id] = state
		}
	}
	rows.Close()
	return out, rows.Err()
}
func linkedRemovalPlan(uid string, targets []*string, snapshot effectiveSnapshot) ([]recordedLink, string) {
	selected := map[string]bool{}
	for _, target := range targets {
		if target != nil {
			selected[*target] = true
		}
	}
	ids := []string{}
	states := map[string]mapping.State{}
	links := map[string]recordedLink{}
	for _, target := range targets {
		if target == nil {
			continue
		}
		ids = append(ids, *target)
		state := snapshot.States[*target]
		states[*target] = state
		if state.Blocked {
			continue
		}
		for _, source := range state.Sources {
			if record, ok := snapshot.Direct[source]; ok && !selected[source] {
				links[source] = record
			}
		}
	}
	sort.Strings(ids)
	sourceIDs := []string{}
	for id := range links {
		sourceIDs = append(sourceIDs, id)
	}
	sort.Strings(sourceIDs)
	out := []recordedLink{}
	for _, id := range sourceIDs {
		out = append(out, links[id])
	}
	versions := map[string]string{}
	for _, record := range out {
		versions[record.EpisodeID] = record.RecordID
	}
	raw, _ := json.Marshal(struct {
		User    string
		Targets []string
		States  map[string]mapping.State
		Sources map[string]string
	}{uid, ids, states, versions})
	return out, fmt.Sprintf("%x", sha256.Sum256(raw))
}
func (a *API) effectiveHistory(w http.ResponseWriter, r *http.Request) {
	uid := auth.Current(r.Context()).ID
	var owned bool
	if err := a.DB.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM library WHERE user_id=$1 AND catalog_id=$2)", uid, r.PathValue("id")).Scan(&owned); err != nil {
		http.Error(w, "effective history unavailable", http.StatusServiceUnavailable)
		return
	}
	if !owned {
		http.Error(w, "library title not found", http.StatusNotFound)
		return
	}
	snapshot, err := loadEffective(r.Context(), a.DB, uid, r.PathValue("id"))
	if err != nil {
		http.Error(w, "effective history unavailable", http.StatusServiceUnavailable)
		return
	}
	rows, err := a.DB.Query(r.Context(), "SELECT id FROM episodes WHERE catalog_id=$1", r.PathValue("id"))
	if err != nil {
		http.Error(w, "effective history unavailable", http.StatusServiceUnavailable)
		return
	}
	defer rows.Close()
	out := map[string]mapping.State{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			http.Error(w, "effective history unavailable", http.StatusServiceUnavailable)
			return
		}
		state := snapshot.States[id]
		if state.Sources == nil {
			state.Sources = []string{}
		}
		out[id] = state
	}
	if rows.Err() != nil {
		http.Error(w, "effective history unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, out)
}
