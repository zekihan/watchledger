package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/zekihan/watchledger/internal/anidb"
	"github.com/zekihan/watchledger/internal/auth"
)

// Only already cached source credits are searched. This endpoint neither
// mirrors a person catalog nor links unrelated TMDB identities by name/number.
func (a *API) animeCreator(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 || strconv.FormatInt(id, 10) != r.PathValue("id") {
		http.Error(w, "invalid creator identity", http.StatusBadRequest)
		return
	}
	rows, err := a.DB.Query(r.Context(), `SELECT c.source_id,c.metadata->'en',catalog_display_title(c.kind,c.metadata,c.title,$2::jsonb)
 FROM catalog c WHERE c.source='anidb' AND c.kind='anime' AND (
 c.metadata->'en'->'credits'->'cast' @> jsonb_build_array(jsonb_build_object('id',$1::bigint)) OR
 c.metadata->'en'->'credits'->'crew' @> jsonb_build_array(jsonb_build_object('id',$1::bigint))) ORDER BY c.fetched_at DESC,c.id LIMIT 101`, id, auth.Current(r.Context()).Settings)
	if err != nil {
		http.Error(w, "cached creator unavailable", http.StatusServiceUnavailable)
		return
	}
	defer rows.Close()
	cast := []map[string]interface{}{}
	crew := []map[string]interface{}{}
	name, picture := "", ""
	count := 0
	truncated := false
	for rows.Next() {
		count++
		if count > 100 {
			truncated = true
			break
		}
		var aid, title string
		var raw []byte
		if err = rows.Scan(&aid, &raw, &title); err != nil {
			http.Error(w, "cached creator unavailable", http.StatusServiceUnavailable)
			return
		}
		var metadata struct {
			Credits anidb.CreditSet `json:"credits"`
			Poster  string          `json:"poster_path"`
			Date    *string         `json:"first_air_date"`
			Score   *float64        `json:"vote_average"`
			Votes   *int            `json:"vote_count"`
		}
		if err = json.Unmarshal(raw, &metadata); err != nil {
			http.Error(w, "cached creator unavailable", http.StatusServiceUnavailable)
			return
		}
		numeric, parseErr := strconv.ParseInt(aid, 10, 64)
		if parseErr != nil {
			continue
		}
		item := map[string]interface{}{"id": numeric, "name": title, "media_type": "anime", "poster_path": metadata.Poster, "first_air_date": metadata.Date}
		if metadata.Score != nil {
			item["vote_average"] = *metadata.Score
		}
		if metadata.Votes != nil {
			item["vote_count"] = *metadata.Votes
		}
		for _, credit := range metadata.Credits.Cast {
			if credit.ID == id {
				if name == "" {
					name = credit.Name
				}
				if picture == "" {
					picture = credit.ProfilePath
				}
				cast = append(cast, item)
				break
			}
		}
		for _, credit := range metadata.Credits.Crew {
			if credit.ID == id {
				if name == "" {
					name = credit.Name
				}
				crew = append(crew, item)
				break
			}
		}
	}
	if rows.Err() != nil {
		http.Error(w, "cached creator unavailable", http.StatusServiceUnavailable)
		return
	}
	if name == "" {
		http.Error(w, "creator not found in cached AniDB credits", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]interface{}{"id": id, "name": name, "profile_path": picture, "cached_only": true, "truncated": truncated, "source_url": "https://anidb.net/cr" + strconv.FormatInt(id, 10), "combined_credits": map[string]interface{}{"cast": cast, "crew": crew}})
}
