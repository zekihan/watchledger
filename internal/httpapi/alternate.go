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
	rows, err := tx.Query(r.Context(), `SELECT m.id,m.revision,m.relationships,(m.reviewed AND NOT EXISTS(SELECT 1 FROM jsonb_array_elements(m.relationships) edge LEFT JOIN episodes src ON src.id=edge->>'from' LEFT JOIN episodes dst ON dst.id=edge->>'to' WHERE src.id IS NULL OR dst.id IS NULL OR src.retired OR dst.retired)),m.requires_consent,coalesce(c.revision,''),m.provenance FROM mappings m LEFT JOIN mapping_consents c ON c.mapping_id=m.id AND c.user_id=$1 ORDER BY m.id`, uid)
	if err != nil {
		http.Error(w, "mappings unavailable", http.StatusServiceUnavailable)
		return
	}
	definitions := []mapping.Definition{}
	attributionByID := map[string]string{}
	for rows.Next() {
		var d mapping.Definition
		var raw, provenance []byte
		if err = rows.Scan(&d.ID, &d.Revision, &raw, &d.Reviewed, &d.RequiresConsent, &d.Consented, &provenance); err != nil {
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
	rows, err = tx.Query(r.Context(), `SELECT episode_id FROM history WHERE user_id=$1 AND episode_id IS NOT NULL`, uid)
	if err != nil {
		http.Error(w, "history unavailable", http.StatusServiceUnavailable)
		return
	}
	watched := map[string]bool{}
	for rows.Next() {
		var eid string
		if err = rows.Scan(&eid); err != nil {
			rows.Close()
			http.Error(w, "history unavailable", http.StatusServiceUnavailable)
			return
		}
		watched[eid] = true
	}
	rows.Close()
	if rows.Err() != nil {
		http.Error(w, "history unavailable", http.StatusServiceUnavailable)
		return
	}
	states := mapping.Project(definitions, watched)
	var sourceEpisodes []string
	err = tx.QueryRow(r.Context(), `SELECT coalesce(array_agg(id),'{}'::text[]) FROM episodes WHERE catalog_id=$1`, id).Scan(&sourceEpisodes)
	if err != nil {
		http.Error(w, "episodes unavailable", http.StatusServiceUnavailable)
		return
	}
	sourceSet := map[string]bool{}
	for _, eid := range sourceEpisodes {
		sourceSet[eid] = true
	}
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
	rows, err = tx.Query(r.Context(), `SELECT c.id,c.title,c.kind,l.base,l.rating,e.id,e.season,e.number,e.kind,e.retired,coalesce(e.metadata->>'name','') FROM catalog c JOIN episodes e ON e.catalog_id=c.id LEFT JOIN library l ON l.catalog_id=c.id AND l.user_id=$1 WHERE c.id=$2 OR c.id IN (SELECT catalog_id FROM episodes WHERE id=ANY($3::text[])) ORDER BY c.id,e.season,e.number,e.id`, uid, id, ids)
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
		if !ep.Retired && ep.Kind == "regular" {
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
	tag, err := a.DB.Exec(r.Context(), "UPDATE library SET anime_view=$3,updated_at=now() WHERE user_id=$1 AND catalog_id=$2", auth.Current(r.Context()).ID, r.PathValue("id"), value)
	if err != nil {
		http.Error(w, "structure update unavailable", http.StatusServiceUnavailable)
		return
	}
	if tag.RowsAffected() != 1 {
		http.Error(w, "library title not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
