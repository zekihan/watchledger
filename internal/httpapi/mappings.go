package httpapi

import (
	"encoding/json"
	"github.com/zekihan/watchledger/internal/auth"
	"github.com/zekihan/watchledger/internal/mapping"
	"net/http"
)

func (a *API) mappingRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/mappings", a.mappings)
	m.HandleFunc("GET /api/library/{id}/alternate", a.alternateView)
	m.HandleFunc("POST /api/admin/mappings", a.saveMapping)
	m.HandleFunc("POST /api/mappings/{id}/confirm", a.confirmMapping)
	m.HandleFunc("GET /api/admin/accounts", a.accounts)
	m.HandleFunc("GET /api/notices", a.notices)
	m.HandleFunc("GET /api/admin/jobs", a.jobStatus)
}
func (a *API) mappings(w http.ResponseWriter, r *http.Request) {
	rows, e := a.DB.Query(r.Context(), `SELECT jsonb_build_object('id',m.id,'revision',m.revision,'provenance',m.provenance,'relationships',m.relationships,'requires_consent',m.requires_consent,'reviewed',m.reviewed,'confirmed',coalesce(c.revision=m.revision,false)) FROM mappings m LEFT JOIN mapping_consents c ON c.mapping_id=m.id AND c.user_id=$1 ORDER BY m.updated_at DESC`, auth.Current(r.Context()).ID)
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
		ID               string         `json:"id"`
		ExpectedRevision string         `json:"expected_revision"`
		Relationships    []mapping.Edge `json:"relationships"`
		Provenance       struct {
			Source   string `json:"source"`
			Version  string `json:"version"`
			Inferred bool   `json:"inferred"`
			Note     string `json:"note"`
		} `json:"provenance"`
	}
	if !decode(w, r, &in) {
		return
	}
	edges, revision, split, e := mapping.Canonical(in.Relationships)
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
	ids := []string{}
	for _, edge := range edges {
		for _, id := range []string{edge.From, edge.To} {
			var exists bool
			if e = tx.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM episodes WHERE id=$1)", id).Scan(&exists); e != nil || !exists {
				http.Error(w, "referenced source episode not cached", http.StatusConflict)
				return
			}
			ids = append(ids, id)
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
		e = tx.QueryRow(r.Context(), "INSERT INTO mappings(revision,provenance,relationships,requires_consent,reviewed) VALUES($1,$2,$3,$4,true) RETURNING id", revision, provenance, b, split).Scan(&in.ID)
	} else {
		_, e = tx.Exec(r.Context(), "UPDATE mappings SET revision=$2,provenance=$3,relationships=$4,requires_consent=$5,reviewed=true,updated_at=now() WHERE id=$1", in.ID, revision, provenance, b, split)
	}
	if e != nil {
		http.Error(w, "mapping update failed", http.StatusConflict)
		return
	}
	if oldRevision != "" && oldRevision != revision {
		_, e = tx.Exec(r.Context(), `INSERT INTO notices(user_id,kind,data) SELECT DISTINCT h.user_id,'mapping_changed',jsonb_build_object('mapping_id',$1::text,'revision',$2::text) FROM history h WHERE h.episode_id=ANY($3::text[])`, in.ID, revision, ids)
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
