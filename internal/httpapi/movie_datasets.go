package httpapi

import (
	"context"
	"encoding/json"
	"github.com/cplieger/animap/mapread"
	"github.com/zekihan/watchledger/internal/mapping"
	"net/http"
	"strconv"
	"time"
)

// TMDB movie IDs in an animap run are interchangeable records of one work,
// never separate required parts. Resolve them separately from episode mappings.
func (a *API) resolveMovieDataset(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	aid, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || aid < 1 {
		http.Error(w, "invalid AniDB identity", http.StatusBadRequest)
		return
	}
	nativeID := "anidb:anime:" + strconv.Itoa(aid)
	var blocked bool
	err = a.DB.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM mappings m CROSS JOIN LATERAL jsonb_array_elements(m.relationships) edge JOIN episodes ep ON ep.id=edge->>'from' WHERE m.kind='movie-aliases' AND m.reviewed AND m.provenance->>'source'='local-admin-override' AND ep.catalog_id=$1)`, nativeID).Scan(&blocked)
	if err != nil {
		http.Error(w, "mapping lookup unavailable", http.StatusServiceUnavailable)
		return
	}
	if blocked {
		http.Error(w, "local movie override requires review", http.StatusConflict)
		return
	}
	var raw, attribution []byte
	var version string
	err = a.DB.QueryRow(r.Context(), `SELECT a.data,d.attribution,d.version FROM mapping_dataset_anime a JOIN mapping_datasets d ON d.checksum=a.dataset_checksum WHERE a.dataset_checksum=$1 AND a.anidb_id=$2`, r.PathValue("checksum"), aid).Scan(&raw, &attribution, &version)
	if err != nil {
		http.Error(w, "dataset entry not found", http.StatusNotFound)
		return
	}
	var anime mapread.Anime
	if json.Unmarshal(raw, &anime) != nil {
		http.Error(w, "invalid stored dataset", http.StatusServiceUnavailable)
		return
	}
	pending := []map[string]interface{}{}
	edges := []mapping.Edge{}
	seen := map[mapping.Edge]bool{}
	count := 0
	for _, run := range anime.Episodes {
		if len(run.TMDBMovies) == 0 {
			continue
		}
		for offset := 0; offset <= run.End-run.Start; offset++ {
			for _, movie := range run.TMDBMovies {
				count++
				if count > 10000 {
					http.Error(w, "too many movie aliases", http.StatusBadRequest)
					return
				}
				source := ""
				if len(run.AniDBEpisodeIDs) > offset {
					source = "anidb:episode:" + strconv.Itoa(run.AniDBEpisodeIDs[offset])
				}
				target := "tmdb:movie:" + strconv.Itoa(movie)
				var available bool
				err = a.DB.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM episodes e JOIN catalog c ON c.id=$3 WHERE e.id=$1 AND e.catalog_id=$2 AND e.source='anidb' AND NOT e.retired AND c.source='tmdb' AND c.kind='movie')`, source, nativeID, target).Scan(&available)
				if err != nil {
					http.Error(w, "movie cache unavailable", http.StatusServiceUnavailable)
					return
				}
				if !available {
					pending = append(pending, map[string]interface{}{"candidate": map[string]interface{}{"source_episode": source, "native_number": run.Start + offset, "tmdb_movie_id": movie}, "reason": "load_exact_native_episode_and_movie_metadata_first"})
					continue
				}
				edge := mapping.Edge{From: source, To: target}
				if !seen[edge] {
					seen[edge] = true
					edges = append(edges, edge)
				}
			}
		}
	}
	revision := ""
	if len(edges) > 0 {
		edges, revision, _, err = mapping.CanonicalForKind(edges, "movie-aliases")
		if err != nil {
			http.Error(w, "invalid movie aliases", http.StatusConflict)
			return
		}
	}
	relationships, _ := json.Marshal(edges)
	unresolved, _ := json.Marshal(pending)
	var proposalID string
	err = a.DB.QueryRow(r.Context(), `INSERT INTO mapping_proposals(dataset_checksum,anidb_id,relationships,unresolved,revision,kind) VALUES($1,$2,$3,$4,$5,'movie-aliases') RETURNING id`, r.PathValue("checksum"), aid, relationships, unresolved, revision).Scan(&proposalID)
	if err != nil {
		http.Error(w, "movie proposal unavailable", http.StatusServiceUnavailable)
		return
	}
	var existingID, existingRevision string
	err = a.DB.QueryRow(r.Context(), `SELECT coalesce((SELECT id::text FROM mappings WHERE kind='movie-aliases' AND provenance->>'source'='animap' AND provenance->>'anime_id'=$1),''),coalesce((SELECT revision FROM mappings WHERE kind='movie-aliases' AND provenance->>'source'='animap' AND provenance->>'anime_id'=$1),'')`, strconv.Itoa(aid)).Scan(&existingID, &existingRevision)
	if err != nil {
		http.Error(w, "movie mapping lookup unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, map[string]interface{}{"kind": "movie-aliases", "existing_id": existingID, "existing_revision": existingRevision, "proposal_id": proposalID, "relationships": edges, "revision": revision, "unresolved": pending, "requires_consent": true, "provenance": map[string]interface{}{"source": "animap", "version": version, "inferred": false, "note": "Explicit TMDB movie aliases of the same work; any confirmed alias suffices, not every movie ID."}, "attribution": json.RawMessage(attribution)})
}
