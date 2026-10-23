package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/zekihan/watchledger/internal/anidb"
	"github.com/zekihan/watchledger/internal/auth"
)

func (a *API) discoverAnime(w http.ResponseWriter, r *http.Request) {
	if a.Anime == nil {
		http.Error(w, "AniDB unavailable", http.StatusServiceUnavailable)
		return
	}
	page := 1
	if value := r.URL.Query().Get("page"); value != "" {
		var err error
		page, err = strconv.Atoi(value)
		if err != nil || page < 1 || page > 50 {
			http.Error(w, "invalid discovery page", http.StatusBadRequest)
			return
		}
	}
	for key := range r.URL.Query() {
		if key != "page" && key != "language" {
			http.Error(w, "native hot-list discovery does not support this filter", http.StatusBadRequest)
			return
		}
	}
	feed, err := a.Anime.Hot(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	var settings struct {
		Style string `json:"anime_title"`
	}
	_ = json.Unmarshal(auth.Current(r.Context()).Settings, &settings)
	start := min((page-1)*20, len(feed.Items))
	end := min(start+20, len(feed.Items))
	results := []map[string]interface{}{}
	for _, item := range feed.Items[start:end] {
		id, _ := strconv.ParseInt(item.ID, 10, 64)
		entry := map[string]interface{}{"id": id, "name": anidb.Choose(item.Titles, r.URL.Query().Get("language"), settings.Style), "media_type": "anime", "poster_path": anidb.PosterURL(item.Picture), "first_air_date": anidb.Day(item.StartDate)}
		if item.Rating.Count > 0 && item.Rating.Value >= 0 && item.Rating.Value <= 10 {
			entry["vote_average"] = item.Rating.Value
			entry["vote_count"] = item.Rating.Count
		}
		results = append(results, entry)
	}
	writeJSON(w, map[string]interface{}{"results": results, "total_pages": max(1, (len(feed.Items)+19)/20), "exhaustive": false, "search_scope": "native_hot", "fetched_at": feed.FetchedAt, "stale": feed.Stale})
}
