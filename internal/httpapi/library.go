package httpapi

import (
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/zekihan/watchledger/internal/auth"
	"github.com/zekihan/watchledger/internal/tracking"
	"net/http"
	"time"
)

func decode(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return false
	}
	return true
}
func (a *API) libraryRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/library", a.library)
	m.HandleFunc("PUT /api/library/{id}", a.saveLibrary)
	m.HandleFunc("GET /api/library/{id}/history", a.history)
	m.HandleFunc("POST /api/library/{id}/track", a.track)
	m.HandleFunc("GET /api/statuses", a.statuses)
	m.HandleFunc("POST /api/statuses", a.saveStatus)
	m.HandleFunc("DELETE /api/statuses/{id}", a.deleteStatus)
	m.HandleFunc("PUT /api/settings", a.settings)
}
func (a *API) library(w http.ResponseWriter, r *http.Request) {
	rows, e := a.DB.Query(r.Context(), `SELECT jsonb_build_object('id',l.catalog_id,'title',coalesce(nullif(c.metadata->coalesce(u.settings->>'metadata_language','en')->>'title',''),nullif(c.metadata->coalesce(u.settings->>'metadata_language','en')->>'name',''),c.title),'kind',c.kind,'metadata',jsonb_build_object('selected',coalesce(c.metadata->coalesce(u.settings->>'metadata_language','en'),c.metadata->'',(SELECT value FROM jsonb_each(c.metadata) LIMIT 1))),'base',l.base,'status_id',l.status_id,'status_name',s.name,'rating',l.rating,'added_at',l.added_at,'position',l.position,'anime_view',l.anime_view) FROM library l JOIN users u ON u.id=l.user_id JOIN catalog c ON c.id=l.catalog_id LEFT JOIN statuses s ON s.user_id=l.user_id AND s.id=l.status_id WHERE l.user_id=$1 ORDER BY l.position,c.title`, auth.Current(r.Context()).ID)
	if e != nil {
		http.Error(w, "library unavailable", http.StatusServiceUnavailable)
		return
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var b json.RawMessage
		if e = rows.Scan(&b); e != nil {
			http.Error(w, "library unavailable", http.StatusServiceUnavailable)
			return
		}
		out = append(out, b)
	}
	if rows.Err() != nil {
		http.Error(w, "library unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, out)
}
func (a *API) saveLibrary(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Base      string  `json:"base"`
		StatusID  *string `json:"status_id"`
		Rating    *int    `json:"rating"`
		Position  int     `json:"position"`
		AnimeView *string `json:"anime_view"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !tracking.ValidBase(in.Base) || (in.Rating != nil && (*in.Rating < 1 || *in.Rating > 10)) || (in.AnimeView != nil && *in.AnimeView != "native" && *in.AnimeView != "show") {
		http.Error(w, "invalid library values", http.StatusBadRequest)
		return
	}
	uid := auth.Current(r.Context()).ID
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	if _, e = tx.Exec(r.Context(), "SELECT id FROM users WHERE id=$1 FOR UPDATE", uid); e != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	var kind string
	if e = tx.QueryRow(r.Context(), "SELECT kind FROM catalog WHERE id=$1", r.PathValue("id")).Scan(&kind); e != nil || kind == "person" {
		http.Error(w, "title not found", http.StatusNotFound)
		return
	}
	if in.StatusID != nil {
		if e = tx.QueryRow(r.Context(), "UPDATE statuses SET used=true WHERE user_id=$1 AND id=$2 RETURNING base", uid, *in.StatusID).Scan(&in.Base); e != nil {
			http.Error(w, "status not found", http.StatusNotFound)
			return
		}
	}
	_, e = tx.Exec(r.Context(), `INSERT INTO library(user_id,catalog_id,base,status_id,rating,position,anime_view) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(user_id,catalog_id) DO UPDATE SET base=EXCLUDED.base,status_id=EXCLUDED.status_id,rating=EXCLUDED.rating,position=EXCLUDED.position,anime_view=EXCLUDED.anime_view,updated_at=now()`, uid, r.PathValue("id"), in.Base, in.StatusID, in.Rating, in.Position, in.AnimeView)
	if e != nil {
		http.Error(w, "library update failed", http.StatusServiceUnavailable)
		return
	}
	if e = tx.Commit(r.Context()); e != nil {
		http.Error(w, "library update failed", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *API) history(w http.ResponseWriter, r *http.Request) {
	rows, e := a.DB.Query(r.Context(), `SELECT to_jsonb(h) - 'user_id' FROM history h WHERE user_id=$1 AND catalog_id=$2 ORDER BY created_at`, auth.Current(r.Context()).ID, r.PathValue("id"))
	if e != nil {
		http.Error(w, "history unavailable", http.StatusServiceUnavailable)
		return
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var b json.RawMessage
		if e = rows.Scan(&b); e != nil {
			http.Error(w, "history unavailable", http.StatusServiceUnavailable)
			return
		}
		out = append(out, b)
	}
	if rows.Err() != nil {
		http.Error(w, "history unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, out)
}
func (a *API) track(w http.ResponseWriter, r *http.Request) {
	var in struct {
		EpisodeID      *string       `json:"episode_id"`
		Season         *int          `json:"season"`
		Whole          bool          `json:"whole"`
		Watched        *bool         `json:"watched"`
		Date           tracking.Date `json:"date"`
		Earlier        bool          `json:"earlier"`
		ReturnWatching bool          `json:"return_watching"`
		Preview        bool          `json:"preview"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Watched == nil {
		http.Error(w, "watched flag is required", http.StatusBadRequest)
		return
	}
	watched := *in.Watched
	if e := in.Date.Validate(); e != nil {
		http.Error(w, "invalid date", http.StatusBadRequest)
		return
	}
	selectors := 0
	if in.EpisodeID != nil {
		selectors++
	}
	if in.Season != nil {
		selectors++
	}
	if in.Whole {
		selectors++
	}
	if selectors > 1 {
		http.Error(w, "choose one tracking scope", http.StatusBadRequest)
		return
	}
	uid, id := auth.Current(r.Context()).ID, r.PathValue("id")
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	// Coordinate tracking with restore/import and other account-scoped mutations.
	if _, e = tx.Exec(r.Context(), "SELECT id FROM users WHERE id=$1 FOR UPDATE", uid); e != nil {
		http.Error(w, "tracking unavailable", http.StatusServiceUnavailable)
		return
	}
	var base, kind string
	var meta json.RawMessage
	e = tx.QueryRow(r.Context(), `SELECT l.base,c.kind,c.metadata FROM library l JOIN catalog c ON c.id=l.catalog_id WHERE l.user_id=$1 AND l.catalog_id=$2 FOR UPDATE OF l`, uid, id).Scan(&base, &kind, &meta)
	if errors.Is(e, pgx.ErrNoRows) {
		http.Error(w, "library title not found", http.StatusNotFound)
		return
	}
	if e != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	rows, e := tx.Query(r.Context(), `SELECT e.id,e.season,e.number,e.kind<>'regular',coalesce(e.air_date::text,''),e.air_time,EXISTS(SELECT 1 FROM history h WHERE h.user_id=$1 AND h.episode_id=e.id) FROM episodes e WHERE e.catalog_id=$2 AND NOT e.retired ORDER BY e.season,e.number`, uid, id)
	if e != nil {
		http.Error(w, "episodes unavailable", http.StatusServiceUnavailable)
		return
	}
	episodes := []tracking.Episode{}
	for rows.Next() {
		var ep tracking.Episode
		if e = rows.Scan(&ep.ID, &ep.Season, &ep.Number, &ep.Special, &ep.AirDate, &ep.AirTime, &ep.Watched); e != nil {
			rows.Close()
			http.Error(w, "episodes unavailable", http.StatusServiceUnavailable)
			return
		}
		episodes = append(episodes, ep)
	}
	rows.Close()
	if rows.Err() != nil {
		http.Error(w, "episodes unavailable", http.StatusServiceUnavailable)
		return
	}
	location, _ := time.LoadLocation(timezone(r))
	now := time.Now().In(location)
	if in.Whole && kind != "movie" && watched {
		var variants map[string]struct {
			Count int `json:"number_of_episodes"`
		}
		_ = json.Unmarshal(meta, &variants)
		expected := 0
		for _, m := range variants {
			expected = max(expected, m.Count)
		}
		regular := 0
		for _, ep := range episodes {
			if !ep.Special {
				regular++
			}
		}
		if expected == 0 || regular < expected {
			http.Error(w, "load all seasons before marking the whole show; catalog is incomplete", http.StatusConflict)
			return
		}
	}

	targets := []*string{}
	earlier := []tracking.Episode{}
	if kind == "movie" {
		if selectors > 0 && !in.Whole {
			http.Error(w, "movies have no episodes", http.StatusBadRequest)
			return
		}
		targets = append(targets, nil)
	} else {
		if selectors == 0 {
			http.Error(w, "episode or bulk scope required", http.StatusBadRequest)
			return
		}
		for _, ep := range episodes {
			selected := in.EpisodeID != nil && *in.EpisodeID == ep.ID
			if selected {
				v := ep.ID
				targets = append(targets, &v)
				earlier = tracking.Earlier(episodes, ep, now)
			} else if (in.Whole || (in.Season != nil && *in.Season == ep.Season)) && (!watched || ep.Aired(now)) {
				v := ep.ID
				targets = append(targets, &v)
			}
		}
		if !watched {
			historyRows, err := tx.Query(r.Context(), "SELECT h.episode_id FROM history h LEFT JOIN episodes e ON e.id=h.episode_id WHERE h.user_id=$1 AND h.catalog_id=$2 AND ($3 OR ($4::text IS NOT NULL AND h.episode_id=$4) OR ($5::int IS NOT NULL AND e.season=$5))", uid, id, in.Whole, in.EpisodeID, in.Season)
			if err != nil {
				http.Error(w, "history unavailable", http.StatusServiceUnavailable)
				return
			}
			seen := map[string]bool{}
			for _, target := range targets {
				if target != nil {
					seen[*target] = true
				}
			}
			for historyRows.Next() {
				var ep *string
				if err = historyRows.Scan(&ep); err != nil {
					historyRows.Close()
					http.Error(w, "history unavailable", http.StatusServiceUnavailable)
					return
				}
				if ep != nil && !seen[*ep] {
					targets = append(targets, ep)
					seen[*ep] = true
				}
			}
			historyRows.Close()
			if historyRows.Err() != nil {
				http.Error(w, "history unavailable", http.StatusServiceUnavailable)
				return
			}
		}
		if len(targets) == 0 {
			http.Error(w, "no eligible cached episodes; load seasons first", http.StatusConflict)
			return
		}
		if in.Earlier && watched {
			for _, ep := range earlier {
				v := ep.ID
				targets = append(targets, &v)
			}
		}
	}
	if in.Preview {
		writeJSON(w, map[string]interface{}{"earlier": earlier, "target_count": len(targets), "offer_return_watching": !watched && base == "completed"})
		return
	}
	var day interface{}
	if in.Date.Day != "" {
		day = in.Date.Day
	}
	for _, ep := range targets {
		episodeKey := ""
		if ep != nil {
			episodeKey = *ep
		}
		if watched {
			_, e = tx.Exec(r.Context(), "DELETE FROM history_tombstones WHERE user_id=$1 AND catalog_id=$2 AND episode_key=$3", uid, id, episodeKey)
		} else {
			_, e = tx.Exec(r.Context(), "INSERT INTO history_tombstones(user_id,catalog_id,episode_key) VALUES($1,$2,$3) ON CONFLICT(user_id,catalog_id,episode_key) DO UPDATE SET deleted_at=now()", uid, id, episodeKey)
		}
		if e != nil {
			http.Error(w, "tracking failed", http.StatusServiceUnavailable)
			return
		}

		if watched {
			_, e = tx.Exec(r.Context(), `INSERT INTO history(user_id,catalog_id,episode_id,watched_date,watched_time,provenance) VALUES($1,$2,$3,$4,$5,'{"source":"app"}') ON CONFLICT(user_id,catalog_id,episode_id) DO NOTHING`, uid, id, ep, day, in.Date.Timestamp)
		} else {
			_, e = tx.Exec(r.Context(), "DELETE FROM history WHERE user_id=$1 AND catalog_id=$2 AND episode_id IS NOT DISTINCT FROM $3::text", uid, id, ep)
		}
		if e != nil {
			http.Error(w, "tracking failed", http.StatusServiceUnavailable)
			return
		}
		for i := range episodes {
			if ep != nil && episodes[i].ID == *ep {
				episodes[i].Watched = watched
			}
		}
	}
	dest := base
	if watched {
		dest = "completed"
		if kind != "movie" {
			var translations map[string]struct {
				Status string `json:"status"`
				Count  int    `json:"number_of_episodes"`
			}
			_ = json.Unmarshal(meta, &translations)
			ended := false
			expected := 0
			for _, m := range translations {
				ended = ended || m.Status == "Ended" || m.Status == "Canceled"
				if m.Count > expected {
					expected = m.Count
				}
			}
			regular := 0
			for _, ep := range episodes {
				if !ep.Special {
					regular++
				}
			}
			dest = tracking.Destination(ended, expected > 0 && regular == expected, episodes)
		}
	} else if in.ReturnWatching && base == "completed" {
		dest = "watching"
	}
	if dest != base {
		_, e = tx.Exec(r.Context(), "UPDATE library SET base=$3,status_id=NULL,updated_at=now() WHERE user_id=$1 AND catalog_id=$2", uid, id, dest)
	}
	if e != nil {
		http.Error(w, "status update failed", http.StatusServiceUnavailable)
		return
	}
	if e = tx.Commit(r.Context()); e != nil {
		http.Error(w, "tracking failed", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, map[string]interface{}{"base": dest, "earlier": earlier, "offer_return_watching": !watched && base == "completed" && !in.ReturnWatching})
}
func (a *API) statuses(w http.ResponseWriter, r *http.Request) {
	rows, e := a.DB.Query(r.Context(), "SELECT to_jsonb(s)-'user_id' FROM statuses s WHERE user_id=$1 ORDER BY name", auth.Current(r.Context()).ID)
	if e != nil {
		http.Error(w, "statuses unavailable", http.StatusServiceUnavailable)
		return
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var b json.RawMessage
		if e = rows.Scan(&b); e != nil {
			http.Error(w, "statuses unavailable", http.StatusServiceUnavailable)
			return
		}
		out = append(out, b)
	}
	if rows.Err() != nil {
		http.Error(w, "statuses unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, out)
}
func (a *API) saveStatus(w http.ResponseWriter, r *http.Request) {
	var in struct{ ID, Name, Base string }
	if !decode(w, r, &in) {
		return
	}
	if !tracking.ValidBase(in.Base) || len(in.Name) == 0 || len(in.Name) > 100 {
		http.Error(w, "invalid status", http.StatusBadRequest)
		return
	}
	uid := auth.Current(r.Context()).ID
	var id string
	var e error
	if in.ID == "" {
		e = a.DB.QueryRow(r.Context(), "INSERT INTO statuses(user_id,name,base) VALUES($1,$2,$3) RETURNING id", uid, in.Name, in.Base).Scan(&id)
	} else {
		e = a.DB.QueryRow(r.Context(), "UPDATE statuses SET name=$3,base=$4 WHERE user_id=$1 AND id=$2 AND (NOT used OR base=$4) RETURNING id", uid, in.ID, in.Name, in.Base).Scan(&id)
	}
	if e != nil {
		http.Error(w, "status unavailable, duplicate name, or used mapping is locked", http.StatusConflict)
		return
	}
	writeJSON(w, map[string]string{"id": id})
}
func (a *API) deleteStatus(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Replacement string `json:"replacement"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !tracking.ValidBase(in.Replacement) {
		http.Error(w, "replacement base required", http.StatusBadRequest)
		return
	}
	uid := auth.Current(r.Context()).ID
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	if _, e = tx.Exec(r.Context(), "SELECT id FROM users WHERE id=$1 FOR UPDATE", uid); e != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	if _, e = tx.Exec(r.Context(), "UPDATE library SET base=$3,status_id=NULL,updated_at=now() WHERE user_id=$1 AND status_id=$2", uid, r.PathValue("id"), in.Replacement); e != nil {
		http.Error(w, "replacement failed", http.StatusConflict)
		return
	}
	if _, e = tx.Exec(r.Context(), "DELETE FROM statuses WHERE user_id=$1 AND id=$2", uid, r.PathValue("id")); e != nil {
		http.Error(w, "deletion failed", http.StatusConflict)
		return
	}
	if e = tx.Commit(r.Context()); e != nil {
		http.Error(w, "deletion failed", http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *API) settings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Language         string `json:"language"`
		MetadataLanguage string `json:"metadata_language"`
		Country          string `json:"country"`
		Timezone         string `json:"timezone"`
		AnimeView        string `json:"anime_view"`
		AnimeTitle       string `json:"anime_title"`
		Spoilers         bool   `json:"spoilers"`
		Services         []int  `json:"services"`
	}
	if !decode(w, r, &in) {
		return
	}
	if (in.Language != "en" && in.Language != "tr") || len(in.Country) != 2 || (in.AnimeView != "native" && in.AnimeView != "show") {
		http.Error(w, "invalid settings", http.StatusBadRequest)
		return
	}
	if _, e := time.LoadLocation(in.Timezone); e != nil {
		http.Error(w, "invalid timezone", http.StatusBadRequest)
		return
	}
	b, e := json.Marshal(in)
	if e != nil {
		http.Error(w, "invalid settings", http.StatusBadRequest)
		return
	}
	if _, e = a.DB.Exec(r.Context(), "UPDATE users SET settings=$2 WHERE id=$1", auth.Current(r.Context()).ID, b); e != nil {
		http.Error(w, "settings unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
