package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/zekihan/watchledger/internal/auth"
)

func (a *API) searchAnime(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("query"))
	if q == "" {
		http.Error(w, "anime search query is required", http.StatusBadRequest)
		return
	}
	var settings struct {
		Language string `json:"metadata_language"`
		Style    string `json:"anime_title"`
	}
	_ = json.Unmarshal(auth.Current(r.Context()).Settings, &settings)
	if n, err := strconv.ParseInt(q, 10, 64); err == nil && n > 0 && strconv.FormatInt(n, 10) == q {
		if a.Anime == nil {
			http.Error(w, "AniDB unavailable", http.StatusServiceUnavailable)
			return
		}
		d, e := a.Anime.Detail(r.Context(), q, settings.Language, settings.Style)
		if e != nil {
			http.Error(w, e.Error(), http.StatusBadGateway)
			return
		}
		d.Metadata["media_type"] = "anime"
		writeJSON(w, map[string]interface{}{"results": []interface{}{d.Metadata}, "page": 1, "total_pages": 1, "total_results": 1, "exhaustive": false, "source": "anidb", "search_scope": "exact_source_id"})
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	if page > 10000 {
		http.Error(w, "invalid page", http.StatusBadRequest)
		return
	}
	pattern := "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(q) + "%"
	rows, err := a.DB.Query(r.Context(), `SELECT metadata->'en',count(*) OVER() FROM catalog c WHERE source='anidb' AND kind='anime' AND (title ILIKE $1 OR EXISTS(SELECT 1 FROM jsonb_array_elements(coalesce(c.metadata->'en'->'titles','[]'::jsonb)) t WHERE t->>'text' ILIKE $1)) ORDER BY title,id LIMIT 20 OFFSET $2`, pattern, (page-1)*20)
	if err != nil {
		http.Error(w, "cached anime search unavailable", http.StatusServiceUnavailable)
		return
	}
	defer rows.Close()
	results := []map[string]interface{}{}
	total := 0
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw, &total); err != nil {
			http.Error(w, "cached anime search unavailable", http.StatusServiceUnavailable)
			return
		}
		var meta map[string]interface{}
		if json.Unmarshal(raw, &meta) != nil {
			http.Error(w, "invalid cached metadata", http.StatusServiceUnavailable)
			return
		}
		meta["media_type"] = "anime"
		results = append(results, meta)
	}
	if rows.Err() != nil {
		http.Error(w, "cached anime search unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, map[string]interface{}{"results": results, "page": page, "total_pages": (total + 19) / 20, "total_results": total, "exhaustive": false, "source": "anidb", "search_scope": "cached_titles_only"})
}
