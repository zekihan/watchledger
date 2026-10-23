package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/cplieger/animap/mapread"
	"github.com/zekihan/watchledger/internal/animap"
	"github.com/zekihan/watchledger/internal/auth"
	"github.com/zekihan/watchledger/internal/mapping"
)

func (a *API) datasetRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/admin/mapping-datasets", a.datasets)
	m.HandleFunc("POST /api/admin/mapping-datasets", a.importDataset)
	m.HandleFunc("POST /api/admin/mapping-datasets/{checksum}/anime/{id}/resolve", a.resolveDataset)
}
func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if !auth.Current(r.Context()).Admin {
		http.Error(w, "admin required", http.StatusForbidden)
		return false
	}
	return true
}
func (a *API) datasets(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var raw []byte
	err := a.DB.QueryRow(r.Context(), `SELECT coalesce(jsonb_agg(x ORDER BY imported_at DESC),'[]') FROM (SELECT checksum,version,source_revision,generated_at,attribution,imported_at FROM mapping_datasets) x`).Scan(&raw)
	if err != nil {
		http.Error(w, "mapping datasets unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, json.RawMessage(raw))
}
func (a *API) importDataset(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 32<<20))
	if err != nil {
		http.Error(w, "mapping dataset exceeds 32 MiB", http.StatusBadRequest)
		return
	}
	summary, err := animap.Import(r.Context(), a.DB, raw, r.URL.Query().Get("version"), r.URL.Query().Get("checksum"))
	if err != nil {
		http.Error(w, "mapping dataset rejected: "+err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, summary)
}
func (a *API) resolveDataset(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	aid, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || aid < 1 {
		http.Error(w, "invalid AniDB identity", http.StatusBadRequest)
		return
	}
	var localOverride bool
	err = a.DB.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM mappings m CROSS JOIN LATERAL jsonb_array_elements(m.relationships) edge JOIN episodes ep ON ep.id=edge->>'from' OR ep.id=edge->>'to' WHERE m.reviewed AND m.provenance->>'source'='local-admin-override' AND ep.catalog_id=$1)`, "anidb:anime:"+strconv.Itoa(aid)).Scan(&localOverride)
	if err != nil {
		http.Error(w, "mapping lookup unavailable", http.StatusServiceUnavailable)
		return
	}
	if localOverride {
		http.Error(w, "a local override exists; review it instead of replacing it with a dataset proposal", http.StatusConflict)
		return
	}
	var raw, attribution []byte
	var version string
	err = a.DB.QueryRow(r.Context(), `SELECT a.data,d.attribution,d.version FROM mapping_dataset_anime a JOIN mapping_datasets d ON d.checksum=a.dataset_checksum WHERE a.dataset_checksum=$1 AND a.anidb_id=$2`, r.PathValue("checksum"), aid).Scan(&raw, &attribution, &version)
	if err != nil {
		http.Error(w, "dataset entry not found", http.StatusNotFound)
		return
	}
	var row mapread.Anime
	if json.Unmarshal(raw, &row) != nil {
		http.Error(w, "invalid stored dataset", http.StatusServiceUnavailable)
		return
	}
	candidates, err := animap.Expand(row)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	type unresolved struct {
		Candidate animap.Candidate `json:"candidate"`
		Reason    string           `json:"reason"`
	}
	pending := []unresolved{}
	edges := []mapping.Edge{}
	seen := map[mapping.Edge]bool{}
	for _, candidate := range candidates {
		reason := candidate.Reason
		if reason == "" {
			var native bool
			err = a.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM episodes WHERE id=$1 AND catalog_id=$2 AND source='anidb' AND NOT retired)", candidate.SourceEpisode, "anidb:anime:"+strconv.Itoa(aid)).Scan(&native)
			if err != nil {
				http.Error(w, "mapping resolution interrupted; retry", http.StatusServiceUnavailable)
				return
			}
			if !native {
				reason = "load_native_source_metadata_first"
			} else {
				target, e := a.Catalog.ResolveTVDBEpisode(ctx, candidate.TVDBEpisodeID)
				if e != nil {
					reason = e.Error()
				} else if e = a.Catalog.CacheExternalEpisode(ctx, target); e != nil {
					reason = e.Error()
				} else {
					edge := mapping.Edge{From: candidate.SourceEpisode, To: "tmdb:episode:" + strconv.Itoa(target.ID)}
					if !seen[edge] {
						edges = append(edges, edge)
						seen[edge] = true
					}
				}
			}
		}
		if reason != "" {
			pending = append(pending, unresolved{Candidate: candidate, Reason: reason})
		}
	}
	revision := ""
	split := false
	if len(edges) > 0 {
		edges, revision, split, err = mapping.Canonical(edges)
		if err != nil {
			http.Error(w, "resolved mapping is inconsistent", http.StatusConflict)
			return
		}
	}
	relationships, err := json.Marshal(edges)
	if err != nil {
		http.Error(w, "invalid proposal", http.StatusConflict)
		return
	}
	unresolvedJSON, err := json.Marshal(pending)
	if err != nil {
		http.Error(w, "invalid proposal", http.StatusConflict)
		return
	}
	var id string
	err = a.DB.QueryRow(r.Context(), `INSERT INTO mapping_proposals(dataset_checksum,anidb_id,relationships,unresolved,revision) VALUES($1,$2,$3,$4,$5) RETURNING id`, r.PathValue("checksum"), aid, relationships, unresolvedJSON, revision).Scan(&id)
	if err != nil {
		http.Error(w, "proposal unavailable", http.StatusServiceUnavailable)
		return
	}
	var existingID, existingRevision string
	err = a.DB.QueryRow(r.Context(), `SELECT coalesce((SELECT id::text FROM mappings WHERE provenance->>'source'='animap' AND provenance->>'anime_id'=$1 ORDER BY updated_at DESC LIMIT 1),''),coalesce((SELECT revision FROM mappings WHERE provenance->>'source'='animap' AND provenance->>'anime_id'=$1 ORDER BY updated_at DESC LIMIT 1),'')`, strconv.Itoa(aid)).Scan(&existingID, &existingRevision)
	if err != nil {
		http.Error(w, "existing mapping lookup unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, map[string]interface{}{"existing_id": existingID, "existing_revision": existingRevision, "proposal_id": id, "relationships": edges, "unresolved": pending, "requires_consent": split, "revision": revision, "provenance": map[string]interface{}{"source": "animap", "version": version, "inferred": false, "note": fmt.Sprintf("Verified TVDB episode identities resolved through TMDB Find; %d unresolved source placements remain.", len(pending))}, "attribution": json.RawMessage(attribution)})
}
