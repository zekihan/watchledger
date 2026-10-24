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
	IntentScope   []string   `json:"-"`
	Movie         bool       `json:"movie,omitempty"`
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
 SELECT DISTINCT m.id FROM mappings m CROSS JOIN LATERAL jsonb_array_elements(m.relationships) edge JOIN mapping_nodes e ON e.id=edge->>'from' OR e.id=edge->>'to' WHERE e.catalog_id=$2 OR e.catalog_id=ANY($3::text[]) OR ($2='' AND EXISTS(SELECT 1 FROM library l WHERE l.user_id=$1 AND l.catalog_id=e.catalog_id))
 ), vertices AS (
 SELECT edge->>'from' AS id FROM mappings m JOIN related r ON r.id=m.id CROSS JOIN LATERAL jsonb_array_elements(m.relationships) edge
 UNION SELECT edge->>'to' FROM mappings m JOIN related r ON r.id=m.id CROSS JOIN LATERAL jsonb_array_elements(m.relationships) edge
 )
 SELECT m.id,m.revision,m.relationships,m.kind,(m.reviewed AND NOT EXISTS(SELECT 1 FROM jsonb_array_elements(m.relationships) edge LEFT JOIN mapping_nodes src ON src.id=edge->>'from' LEFT JOIN mapping_nodes dst ON dst.id=edge->>'to' WHERE src.id IS NULL OR dst.id IS NULL OR src.retired OR dst.retired)),m.requires_consent,coalesce(c.revision,'') FROM mappings m LEFT JOIN mapping_consents c ON c.mapping_id=m.id AND c.user_id=$1 WHERE m.id IN(SELECT id FROM related) OR EXISTS(SELECT 1 FROM jsonb_array_elements(m.relationships) edge WHERE edge->>'from' IN(SELECT id FROM vertices) OR edge->>'to' IN(SELECT id FROM vertices)) ORDER BY m.id`
	rows, err := db.Query(ctx, query, uid, catalogID, extraCatalogs)
	if err != nil {
		return out, err
	}
	definitions := []mapping.Definition{}
	for rows.Next() {
		var d mapping.Definition
		var raw []byte
		if err = rows.Scan(&d.ID, &d.Revision, &raw, &d.Kind, &d.Reviewed, &d.RequiresConsent, &d.Consented); err != nil {
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
	rows, err = db.Query(ctx, "SELECT h.id::text,coalesce(h.episode_id,h.catalog_id),h.catalog_id,c.title,h.local_action_at,h.intent_scope,(h.episode_id IS NULL) FROM history h JOIN catalog c ON c.id=h.catalog_id WHERE h.user_id=$1 AND (h.episode_id IS NOT NULL OR c.kind='movie') AND ($2='' OR h.catalog_id=$2 OR coalesce(h.episode_id,h.catalog_id)=ANY($3::text[]) OR h.catalog_id=ANY($4::text[]))", uid, catalogID, needed, extraCatalogs)
	if err != nil {
		return out, err
	}
	watched := map[string]bool{}
	for rows.Next() {
		var link recordedLink
		if err = rows.Scan(&link.RecordID, &link.EpisodeID, &link.CatalogID, &link.Title, &link.LocalActionAt, &link.IntentScope, &link.Movie); err != nil {
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
	rows, err = db.Query(ctx, "SELECT coalesce(nullif(episode_key,''),catalog_id),deleted_at,intent_scope FROM history_tombstones WHERE user_id=$1 AND ($2='' OR catalog_id=$2 OR coalesce(nullif(episode_key,''),catalog_id)=ANY($3::text[]) OR catalog_id=ANY($4::text[]))", uid, catalogID, needed, extraCatalogs)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var id string
		var deletedAt time.Time
		var deletedScope []string
		if err = rows.Scan(&id, &deletedAt, &deletedScope); err != nil {
			rows.Close()
			return out, err
		}
		state := out.States[id]
		if !state.Direct {
			groups := state.SourceGroups
			if len(groups) == 0 {
				groups = [][]string{state.Sources}
			}
			newerLocalWatch := false
			for _, group := range groups {
				newer := state.Derived && len(group) > 0
				for _, source := range group {
					record := out.Direct[source]
					newer = newer && record.LocalActionAt != nil && record.LocalActionAt.After(deletedAt) && intentCanOverride(record.IntentScope, deletedScope)
				}
				newerLocalWatch = newerLocalWatch || newer
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
func linkedRemovalPlan(uid string, targets []*string, snapshot effectiveSnapshot, movieID ...string) ([]recordedLink, string) {
	targets = append([]*string(nil), targets...)
	if len(movieID) > 0 {
		for i, target := range targets {
			if target == nil {
				value := movieID[0]
				targets[i] = &value
			}
		}
	}
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
	rows, err := a.DB.Query(r.Context(), "SELECT id FROM mapping_nodes WHERE catalog_id=$1", r.PathValue("id"))
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

// At the first differing restore generation, a destination-local action wins.
// Prefixing BOTH imported scopes with the new import ID preserves their prior
// relationship, including across repeated export/restore generations.
func intentCanOverride(watch, unwatch []string) bool {
	for i, part := range watch {
		if i >= len(unwatch) || part != unwatch[i] {
			return false
		}
	}
	return true
}
