package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/zekihan/watchledger/internal/auth"
	"github.com/zekihan/watchledger/internal/mapping"
)

func (a *API) alternateView(w http.ResponseWriter, r *http.Request) {
	uid, id := auth.Current(r.Context()).ID, r.PathValue("id")
	tx, err := a.DB.BeginTx(r.Context(), pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		http.Error(w, "alternate view unavailable", http.StatusServiceUnavailable)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	var structure string
	var override *string
	err = tx.QueryRow(r.Context(), `SELECT coalesce(l.anime_view,u.settings->>'anime_view','native'),l.anime_view FROM library l JOIN users u ON u.id=l.user_id WHERE l.user_id=$1 AND l.catalog_id=$2`, uid, id).Scan(&structure, &override)
	if err != nil {
		http.Error(w, "library title not found", http.StatusNotFound)
		return
	}
	// Include overlapping definitions as well, so ambiguity never depends on
	// which title happened to request the projection.
	rows, err := tx.Query(r.Context(), `SELECT m.id,m.revision,m.relationships,m.kind,(m.reviewed AND NOT EXISTS(SELECT 1 FROM jsonb_array_elements(m.relationships) edge LEFT JOIN mapping_nodes src ON src.id=edge->>'from' LEFT JOIN mapping_nodes dst ON dst.id=edge->>'to' WHERE src.id IS NULL OR dst.id IS NULL OR src.retired OR dst.retired)),m.requires_consent,coalesce(c.revision,''),m.provenance FROM mappings m LEFT JOIN mapping_consents c ON c.mapping_id=m.id AND c.user_id=$1 ORDER BY m.id`, uid)
	if err != nil {
		http.Error(w, "mappings unavailable", http.StatusServiceUnavailable)
		return
	}
	definitions := []mapping.Definition{}
	attributionByID := map[string]string{}
	for rows.Next() {
		var d mapping.Definition
		var raw, provenance []byte
		if err = rows.Scan(&d.ID, &d.Revision, &raw, &d.Kind, &d.Reviewed, &d.RequiresConsent, &d.Consented, &provenance); err != nil {
			rows.Close()
			http.Error(w, "mappings unavailable", http.StatusServiceUnavailable)
			return
		}
		if json.Unmarshal(raw, &d.Edges) != nil {
			rows.Close()
			http.Error(w, "invalid mapping data", http.StatusServiceUnavailable)
			return
		}
		var source struct {
			Source      string `json:"source"`
			Version     string `json:"version"`
			Attribution struct {
				Notice string `json:"notice"`
			} `json:"attribution"`
		}
		if json.Unmarshal(provenance, &source) == nil && source.Attribution.Notice != "" {
			attributionByID[d.ID] = source.Source + " " + source.Version + ": " + source.Attribution.Notice
		}
		definitions = append(definitions, d)
	}
	rows.Close()
	if rows.Err() != nil {
		http.Error(w, "mappings unavailable", http.StatusServiceUnavailable)
		return
	}
	var sourceEpisodes, relatedCatalogs []string
	err = tx.QueryRow(r.Context(), `WITH neighbors AS (
 SELECT DISTINCT e.catalog_id FROM mappings m CROSS JOIN LATERAL jsonb_array_elements(m.relationships) edge JOIN mapping_nodes e ON e.id=edge->>'from' OR e.id=edge->>'to'
 WHERE EXISTS(SELECT 1 FROM jsonb_array_elements(m.relationships) seed JOIN mapping_nodes root ON root.id=seed->>'from' OR root.id=seed->>'to' WHERE root.catalog_id=$1)
 ) SELECT coalesce(array_agg(id),'{}'::text[]),coalesce(array_agg(DISTINCT catalog_id),'{}'::text[]) FROM mapping_nodes WHERE catalog_id=$1 OR catalog_id IN(SELECT catalog_id FROM neighbors) OR catalog_id IN(SELECT catalog_id FROM alternate_history_views WHERE user_id=$2 AND anchor_id=$1)`, id, uid).Scan(&sourceEpisodes, &relatedCatalogs)
	if err != nil {
		http.Error(w, "episodes unavailable", http.StatusServiceUnavailable)
		return
	}
	sourceSet := map[string]bool{}
	for _, eid := range sourceEpisodes {
		sourceSet[eid] = true
	}
	snapshot, err := loadEffective(r.Context(), tx, uid, id, relatedCatalogs...)
	if err != nil {
		http.Error(w, "effective history unavailable", http.StatusServiceUnavailable)
		return
	}
	states := snapshot.States
	linked := map[string]bool{}
	relevant := []mapping.Definition{}
	attributionNotices := []string{}
	attributionSeen := map[string]bool{}
	for _, d := range definitions {
		touches := false
		for _, edge := range d.Edges {
			touches = touches || sourceSet[edge.From] || sourceSet[edge.To]
		}
		if !touches {
			continue
		}
		relevant = append(relevant, d)
		if notice := attributionByID[d.ID]; notice != "" && !attributionSeen[notice] {
			attributionNotices = append(attributionNotices, notice)
			attributionSeen[notice] = true
		}
		for _, edge := range d.Edges {
			linked[edge.From] = true
			linked[edge.To] = true
		}
	}
	ids := []string{}
	for eid := range linked {
		ids = append(ids, eid)
	}
	rows, err = tx.Query(r.Context(), `SELECT c.id,catalog_display_title(c.kind,c.metadata,c.title,u.settings),c.kind,l.base,l.rating,e.id,e.season,e.number,e.kind,e.retired,coalesce(e.metadata->>'name','') FROM catalog c JOIN mapping_nodes e ON e.catalog_id=c.id JOIN users u ON u.id=$1 LEFT JOIN library l ON l.catalog_id=c.id AND l.user_id=$1 WHERE c.id=$2 OR c.id IN (SELECT catalog_id FROM mapping_nodes WHERE id=ANY($3::text[])) OR c.id IN(SELECT catalog_id FROM alternate_history_views WHERE user_id=$1 AND anchor_id=$2) ORDER BY c.id,e.season,e.number,e.id`, uid, id, ids)
	if err != nil {
		http.Error(w, "alternate episodes unavailable", http.StatusServiceUnavailable)
		return
	}
	type episode struct {
		ID      string        `json:"id"`
		Season  int           `json:"season"`
		Number  int           `json:"number"`
		Kind    string        `json:"kind"`
		Retired bool          `json:"retired"`
		Name    string        `json:"name"`
		State   mapping.State `json:"state"`
	}
	type title struct {
		ID         string    `json:"id"`
		Title      string    `json:"title"`
		Kind       string    `json:"kind"`
		Base       *string   `json:"base"`
		Rating     *int      `json:"rating"`
		Episodes   []episode `json:"episodes"`
		Incomplete bool      `json:"incomplete"`
		Watched    int       `json:"watched"`
		Total      int       `json:"total"`
	}
	titles := []*title{}
	byID := map[string]*title{}
	incomplete := len(relevant) == 0
	for rows.Next() {
		var cid, name, kind string
		var base *string
		var rating *int
		var ep episode
		if err = rows.Scan(&cid, &name, &kind, &base, &rating, &ep.ID, &ep.Season, &ep.Number, &ep.Kind, &ep.Retired, &ep.Name); err != nil {
			rows.Close()
			http.Error(w, "alternate episodes unavailable", http.StatusServiceUnavailable)
			return
		}
		t := byID[cid]
		if t == nil {
			t = &title{ID: cid, Title: name, Kind: kind, Base: base, Rating: rating, Episodes: []episode{}}
			byID[cid] = t
			titles = append(titles, t)
		}
		ep.State = states[ep.ID]
		if ep.State.Sources == nil {
			ep.State.Sources = []string{}
		}
		if ep.State.Blocked || !linked[ep.ID] {
			t.Incomplete = true
			incomplete = true
		}
		if ep.Retired {
			ep.State.Derived = false
			ep.State.Blocked = true
			t.Incomplete = true
			incomplete = true
		}
		if !ep.Retired && (ep.Kind == "regular" || ep.Kind == "movie") {
			t.Total++
			if ep.State.Direct || ep.State.Derived {
				t.Watched++
			}
		}
		t.Episodes = append(t.Episodes, ep)
	}
	rows.Close()
	if rows.Err() != nil {
		http.Error(w, "alternate episodes unavailable", http.StatusServiceUnavailable)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		http.Error(w, "alternate view unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, map[string]interface{}{"structure": structure, "override": override, "incomplete": incomplete, "titles": titles, "recorded_history_unchanged": true, "attribution_notices": attributionNotices})
}

func (a *API) setAnimeStructure(w http.ResponseWriter, r *http.Request) {
	var in struct {
		View string `json:"view"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.View != "default" && in.View != "native" && in.View != "show" {
		http.Error(w, "invalid anime structure", http.StatusBadRequest)
		return
	}
	var value *string
	if in.View != "default" {
		value = &in.View
	}
	// Use an explicit transaction: an autocommit statement could finish after
	// its caller times out but before PostgreSQL receives the cancel request.
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		http.Error(w, "structure update unavailable", http.StatusServiceUnavailable)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	uid := auth.Current(r.Context()).ID
	// Serialize with library PATCH and restore before reading or changing rows.
	if _, err = tx.Exec(r.Context(), "SELECT id FROM users WHERE id=$1 FOR UPDATE", uid); err != nil {
		http.Error(w, "structure update unavailable", http.StatusServiceUnavailable)
		return
	}
	tag, err := tx.Exec(r.Context(), "UPDATE library SET anime_view=$3,updated_at=now() WHERE user_id=$1 AND catalog_id=$2", uid, r.PathValue("id"), value)
	if err != nil {
		http.Error(w, "structure update unavailable", http.StatusServiceUnavailable)
		return
	}
	if tag.RowsAffected() != 1 {
		http.Error(w, "library title not found", http.StatusNotFound)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		http.Error(w, "structure update unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
