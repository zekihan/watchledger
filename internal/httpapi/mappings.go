package httpapi

import (
	"encoding/json"
	"fmt"
	"github.com/zekihan/watchledger/internal/auth"
	"github.com/zekihan/watchledger/internal/mapping"
	"net/http"
)

func (a *API) mappingRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/mappings", a.mappings)
	m.HandleFunc("GET /api/library/{id}/alternate", a.alternateView)
	m.HandleFunc("PUT /api/library/{id}/structure", a.setAnimeStructure)
	m.HandleFunc("POST /api/admin/mappings", a.saveMapping)
	m.HandleFunc("GET /api/admin/mappings/{id}/history", a.mappingHistory)
	m.HandleFunc("POST /api/mappings/{id}/confirm", a.confirmMapping)
	m.HandleFunc("GET /api/admin/accounts", a.accounts)
	m.HandleFunc("GET /api/notices", a.notices)
	m.HandleFunc("GET /api/admin/jobs", a.jobStatus)
}
func (a *API) mappings(w http.ResponseWriter, r *http.Request) {
	rows, e := a.DB.Query(r.Context(), `SELECT jsonb_build_object('id',m.id,'kind',m.kind,'revision',m.revision,'provenance',m.provenance,'relationships',m.relationships,'requires_consent',m.requires_consent,'reviewed',m.reviewed,'confirmed',coalesce(c.revision=m.revision,false),'episodes',coalesce((SELECT jsonb_object_agg(e.id,jsonb_build_object('catalog_id',cat.id,'title',catalog_display_title(cat.kind,cat.metadata,cat.title,u.settings),'name',coalesce(e.metadata->>'name',''),'season',e.season,'number',e.number,'kind',e.kind,'retired',e.retired)) FROM mapping_nodes e JOIN catalog cat ON cat.id=e.catalog_id JOIN users u ON u.id=$1 WHERE EXISTS(SELECT 1 FROM jsonb_array_elements(m.relationships) edge WHERE e.id=edge->>'from' OR e.id=edge->>'to')),'{}'::jsonb)) FROM mappings m LEFT JOIN mapping_consents c ON c.mapping_id=m.id AND c.user_id=$1 ORDER BY m.updated_at DESC`, auth.Current(r.Context()).ID)
	if e != nil {
		http.Error(w, "mappings unavailable", http.StatusServiceUnavailable)
		return
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var b json.RawMessage
		if e = rows.Scan(&b); e != nil {
			http.Error(w, "mappings unavailable", http.StatusServiceUnavailable)
			return
		}
		out = append(out, b)
	}
	if rows.Err() != nil {
		http.Error(w, "mappings unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, out)
}
func (a *API) saveMapping(w http.ResponseWriter, r *http.Request) {
	if !auth.Current(r.Context()).Admin {
		http.Error(w, "admin required", http.StatusForbidden)
		return
	}
	var in struct {
		Kind             string         `json:"kind,omitempty"`
		ID               string         `json:"id"`
		ExpectedRevision string         `json:"expected_revision"`
		ProposalID       string         `json:"proposal_id,omitempty"`
		Relationships    []mapping.Edge `json:"relationships"`
		Provenance       struct {
			Source          string          `json:"source"`
			Version         string          `json:"version"`
			Inferred        bool            `json:"inferred"`
			Note            string          `json:"note"`
			DatasetChecksum string          `json:"dataset_checksum,omitempty"`
			AnimeID         int             `json:"anime_id,omitempty"`
			Attribution     json.RawMessage `json:"attribution,omitempty"`
		} `json:"provenance"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Kind == "" {
		in.Kind = "episodes"
	}
	edges, revision, split, e := mapping.CanonicalForKind(in.Relationships, in.Kind)
	if e != nil || in.Provenance.Source == "" || in.Provenance.Version == "" {
		http.Error(w, "valid relationships and provenance required", http.StatusBadRequest)
		return
	}
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		http.Error(w, "mapping unavailable", http.StatusServiceUnavailable)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	if in.Provenance.Source == "animap" {
		if in.ProposalID == "" {
			http.Error(w, "dataset mapping requires a verified proposal", http.StatusBadRequest)
			return
		}
		var proposedRevision, proposedKind string
		var unresolved []byte
		e = tx.QueryRow(r.Context(), `SELECT p.revision,p.kind,p.unresolved,p.anidb_id,d.checksum,d.version,d.attribution FROM mapping_proposals p JOIN mapping_datasets d ON d.checksum=p.dataset_checksum WHERE p.id=$1 AND p.created_at>now()-interval '24 hours'`, in.ProposalID).Scan(&proposedRevision, &proposedKind, &unresolved, &in.Provenance.AnimeID, &in.Provenance.DatasetChecksum, &in.Provenance.Version, &in.Provenance.Attribution)
		if e != nil || proposedRevision != revision || proposedKind != in.Kind {
			http.Error(w, "proposal identities changed; resolve and review again", http.StatusConflict)
			return
		}
		var remaining []json.RawMessage
		_ = json.Unmarshal(unresolved, &remaining)
		in.Provenance.Inferred = false
		in.Provenance.Note = fmt.Sprintf("Explicit TVDB episode IDs resolved through TMDB Find. %d unresolved source placements retained in proposal.", len(remaining))
		if in.Kind == "movie-aliases" {
			in.Provenance.Note = fmt.Sprintf("Explicit TMDB aliases of the same movie, not required episode parts. %d unresolved placements retained in proposal.", len(remaining))
		}
	} else if in.ProposalID != "" {
		http.Error(w, "proposal source must be animap", http.StatusBadRequest)
		return
	}
	ids := []string{}
	for _, edge := range edges {
		for _, id := range []string{edge.From, edge.To} {
			var exists bool
			if e = tx.QueryRow(r.Context(), "SELECT count(*)=1 FROM mapping_nodes WHERE id=$1 AND NOT retired", id).Scan(&exists); e != nil || !exists {
				http.Error(w, "referenced source episode not cached", http.StatusConflict)
				return
			}
			ids = append(ids, id)
		}
	}
	for _, edge := range edges {
		var valid bool
		query := "SELECT EXISTS(SELECT 1 FROM episodes a JOIN episodes b ON b.id=$2 WHERE a.id=$1)"
		if in.Kind == "movie-aliases" {
			query = "SELECT EXISTS(SELECT 1 FROM episodes a JOIN catalog native ON native.id=a.catalog_id JOIN catalog movie ON movie.id=$2 WHERE a.id=$1 AND a.source='anidb' AND native.source='anidb' AND native.kind='anime' AND movie.source='tmdb' AND movie.kind='movie')"
		}
		if e = tx.QueryRow(r.Context(), query, edge.From, edge.To).Scan(&valid); e != nil || !valid {
			http.Error(w, "identities do not match the selected mapping kind", http.StatusConflict)
			return
		}
	}
	oldRevision := ""
	if in.ID != "" {
		var old []byte
		if e = tx.QueryRow(r.Context(), "SELECT revision,relationships FROM mappings WHERE id=$1 FOR UPDATE", in.ID).Scan(&oldRevision, &old); e != nil || oldRevision != in.ExpectedRevision {
			http.Error(w, "mapping revision changed; reload review", http.StatusConflict)
			return
		}
		var oldEdges []mapping.Edge
		_ = json.Unmarshal(old, &oldEdges)
		for _, edge := range oldEdges {
			ids = append(ids, edge.From, edge.To)
		}
	}
	b, e := json.Marshal(edges)
	if e != nil {
		http.Error(w, "invalid mapping", http.StatusBadRequest)
		return
	}
	provenance, e := json.Marshal(in.Provenance)
	if e != nil {
		http.Error(w, "invalid provenance", http.StatusBadRequest)
		return
	}
	if in.ID == "" {
		e = tx.QueryRow(r.Context(), "INSERT INTO mappings(revision,provenance,relationships,requires_consent,reviewed,kind) VALUES($1,$2,$3,$4,true,$5) RETURNING id", revision, provenance, b, split, in.Kind).Scan(&in.ID)
	} else {
		_, e = tx.Exec(r.Context(), "UPDATE mappings SET revision=$2,provenance=$3,relationships=$4,requires_consent=$5,reviewed=true,kind=$6,updated_at=now() WHERE id=$1", in.ID, revision, provenance, b, split, in.Kind)
	}
	if e != nil {
		http.Error(w, "mapping update failed", http.StatusConflict)
		return
	}
	_, e = tx.Exec(r.Context(), `INSERT INTO mapping_revision_history(mapping_id,revision,provenance,relationships,kind) SELECT $1,$2,$3,$4,$5 WHERE NOT EXISTS(SELECT 1 FROM mapping_revision_history WHERE mapping_id=$1 AND revision=$2 AND provenance=$3::jsonb)`, in.ID, revision, provenance, b, in.Kind)
	if e != nil {
		http.Error(w, "mapping history unavailable", http.StatusServiceUnavailable)
		return
	}
	if oldRevision != "" && oldRevision != revision {
		_, e = tx.Exec(r.Context(), `INSERT INTO notices(user_id,kind,data) SELECT affected.user_id,'mapping_changed',jsonb_build_object('mapping_id',$1::text,'revision',$2::text) FROM (SELECT h.user_id FROM history h WHERE coalesce(h.episode_id,h.catalog_id)=ANY($3::text[]) UNION SELECT l.user_id FROM library l JOIN mapping_nodes ep ON ep.catalog_id=l.catalog_id WHERE ep.id=ANY($3::text[])) affected`, in.ID, revision, ids)
		if e != nil {
			http.Error(w, "mapping notice failed", http.StatusServiceUnavailable)
			return
		}
	}
	if e = tx.Commit(r.Context()); e != nil {
		http.Error(w, "mapping update failed", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, map[string]string{"id": in.ID, "revision": revision})
}
func (a *API) confirmMapping(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Revision string `json:"revision"`
	}
	if !decode(w, r, &in) {
		return
	}
	tag, e := a.DB.Exec(r.Context(), `INSERT INTO mapping_consents(user_id,mapping_id,revision) SELECT $1,id,revision FROM mappings WHERE id=$2 AND revision=$3 AND reviewed ON CONFLICT(user_id,mapping_id) DO UPDATE SET revision=EXCLUDED.revision,confirmed_at=now()`, auth.Current(r.Context()).ID, r.PathValue("id"), in.Revision)
	if e != nil || tag.RowsAffected() == 0 {
		http.Error(w, "review current mapping before confirming", http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *API) accounts(w http.ResponseWriter, r *http.Request) {
	if !auth.Current(r.Context()).Admin {
		http.Error(w, "admin required", http.StatusForbidden)
		return
	}
	var result []byte
	e := a.DB.QueryRow(r.Context(), "SELECT coalesce(jsonb_agg(jsonb_build_object('id',id,'name',name,'admin',admin,'created_at',created_at)),'[]') FROM users").Scan(&result)
	if e != nil {
		http.Error(w, "accounts unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, json.RawMessage(result))
}
func (a *API) notices(w http.ResponseWriter, r *http.Request) {
	var result []byte
	e := a.DB.QueryRow(r.Context(), "SELECT coalesce(jsonb_agg(to_jsonb(n)-'user_id'),'[]') FROM notices n WHERE user_id=$1", auth.Current(r.Context()).ID).Scan(&result)
	if e != nil {
		http.Error(w, "notices unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, json.RawMessage(result))
}

func (a *API) jobStatus(w http.ResponseWriter, r *http.Request) {
	if !auth.Current(r.Context()).Admin {
		http.Error(w, "admin required", http.StatusForbidden)
		return
	}
	var result []byte
	e := a.DB.QueryRow(r.Context(), "SELECT coalesce(jsonb_agg(to_jsonb(j)),'[]') FROM jobs j WHERE last_error IS NOT NULL").Scan(&result)
	if e != nil {
		http.Error(w, "jobs unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, json.RawMessage(result))
}

func (a *API) mappingHistory(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var raw []byte
	err := a.DB.QueryRow(r.Context(), `SELECT coalesce(jsonb_agg(x ORDER BY recorded_at DESC),'[]') FROM (SELECT revision,kind,provenance,relationships,recorded_at FROM mapping_revision_history WHERE mapping_id=$1 ORDER BY recorded_at DESC LIMIT 100) x`, r.PathValue("id")).Scan(&raw)
	if err != nil {
		http.Error(w, "mapping history unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, json.RawMessage(raw))
}
