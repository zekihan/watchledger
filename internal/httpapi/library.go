package httpapi

import (
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/zekihan/watchledger/internal/auth"
	"github.com/zekihan/watchledger/internal/tracking"
	"net/http"
	"strings"
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
	m.HandleFunc("GET /api/library/{id}/effective", a.effectiveHistory)
	m.HandleFunc("POST /api/library/{id}/track", a.track)
	m.HandleFunc("GET /api/statuses", a.statuses)
	m.HandleFunc("POST /api/statuses", a.saveStatus)
	m.HandleFunc("DELETE /api/statuses/{id}", a.deleteStatus)
	m.HandleFunc("PUT /api/settings", a.settings)
}
func (a *API) library(w http.ResponseWriter, r *http.Request) {
	rows, e := a.DB.Query(r.Context(), `SELECT jsonb_build_object('id',l.catalog_id,'title',catalog_display_title(c.kind,c.metadata,c.title,u.settings),'kind',c.kind,'metadata',jsonb_build_object('selected',catalog_metadata_variant(c.metadata,u.settings)),'base',l.base,'status_id',l.status_id,'status_name',s.name,'rating',l.rating,'added_at',l.added_at,'position',l.position,'anime_view',l.anime_view) FROM library l JOIN users u ON u.id=l.user_id JOIN catalog c ON c.id=l.catalog_id LEFT JOIN statuses s ON s.user_id=l.user_id AND s.id=l.status_id WHERE l.user_id=$1 ORDER BY l.position,c.title`, auth.Current(r.Context()).ID)
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
	_, e = tx.Exec(r.Context(), `INSERT INTO library(user_id,catalog_id,base,status_id,rating,position,anime_view) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(user_id,catalog_id) DO UPDATE SET base=EXCLUDED.base,status_id=EXCLUDED.status_id,rating=EXCLUDED.rating,position=EXCLUDED.position,anime_view=coalesce(EXCLUDED.anime_view,library.anime_view),updated_at=now()`, uid, r.PathValue("id"), in.Base, in.StatusID, in.Rating, in.Position, in.AnimeView)
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
		ViewAnchor     string        `json:"view_anchor,omitempty"`
		EpisodeID      *string       `json:"episode_id"`
		Season         *int          `json:"season"`
		Whole          bool          `json:"whole"`
		Watched        *bool         `json:"watched"`
		Date           tracking.Date `json:"date"`
		Earlier        bool          `json:"earlier"`
		ReturnWatching bool          `json:"return_watching"`
		Preview        bool          `json:"preview"`
		IncludeLinked  bool          `json:"include_linked"`
		LinkedPlan     string        `json:"linked_plan"`
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
	// Lock mapping changes before the account row, matching correction-notice
	// ordering and keeping a confirmed cross-view plan stable during this write.
	if _, e = tx.Exec(r.Context(), "LOCK TABLE mappings IN SHARE MODE"); e != nil {
		http.Error(w, "mapping state unavailable", http.StatusServiceUnavailable)
		return
	}
	// Coordinate tracking with restore/import and other account-scoped mutations.
	if _, e = tx.Exec(r.Context(), "SELECT id FROM users WHERE id=$1 FOR UPDATE", uid); e != nil {
		http.Error(w, "tracking unavailable", http.StatusServiceUnavailable)
		return
	}
	var base, kind string
	var meta json.RawMessage
	e = tx.QueryRow(r.Context(), `SELECT l.base,c.kind,c.metadata FROM library l JOIN catalog c ON c.id=l.catalog_id WHERE l.user_id=$1 AND l.catalog_id=$2 FOR UPDATE OF l`, uid, id).Scan(&base, &kind, &meta)
	virtual := errors.Is(e, pgx.ErrNoRows) && in.ViewAnchor != ""
	if virtual {
		e = tx.QueryRow(r.Context(), `SELECT 'watching',c.kind,c.metadata FROM catalog c WHERE c.id=$2 AND c.kind IN('tv','movie') AND EXISTS(SELECT 1 FROM library l JOIN catalog native ON native.id=l.catalog_id WHERE l.user_id=$1 AND l.catalog_id=$3 AND native.kind='anime')`, uid, id, in.ViewAnchor).Scan(&base, &kind, &meta)
	}
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
	snapshot, e := loadEffective(r.Context(), tx, uid, id)
	if e != nil {
		http.Error(w, "linked history unavailable", http.StatusServiceUnavailable)
		return
	}
	for i := range episodes {
		state := snapshot.States[episodes[i].ID]
		episodes[i].Watched = state.Direct || state.Derived
	}
	// Resolve only this account's saved native entries. A guessed anchor or an
	// unconfirmed mapping must not authorize tracking a virtual show.
	peerIDs := []string{}
	for _, state := range snapshot.States {
		if !state.Blocked {
			peerIDs = append(peerIDs, state.Sources...)
		}
	}
	nativeByEpisode := map[string]string{}
	nativeRows, err := tx.Query(r.Context(), `SELECT e.id,e.catalog_id FROM episodes e JOIN catalog c ON c.id=e.catalog_id JOIN library l ON l.catalog_id=c.id AND l.user_id=$1 WHERE c.kind='anime' AND e.id=ANY($2::text[])`, uid, peerIDs)
	if err != nil {
		http.Error(w, "native entries unavailable", http.StatusServiceUnavailable)
		return
	}
	for nativeRows.Next() {
		var eid, cid string
		if err = nativeRows.Scan(&eid, &cid); err != nil {
			nativeRows.Close()
			http.Error(w, "native entries unavailable", http.StatusServiceUnavailable)
			return
		}
		nativeByEpisode[eid] = cid
	}
	nativeRows.Close()
	if nativeRows.Err() != nil {
		http.Error(w, "native entries unavailable", http.StatusServiceUnavailable)
		return
	}
	eligible := map[string]bool{}
	anchored := false
	nodes := append([]tracking.Episode{}, episodes...)
	if kind == "movie" {
		nodes = append(nodes, tracking.Episode{ID: id})
	}
	for _, ep := range nodes {
		state := snapshot.States[ep.ID]
		valid := !state.Blocked && len(state.Sources) > 0
		for _, peer := range state.Sources {
			valid = valid && (nativeByEpisode[peer] != "" || (state.AnySource && strings.HasPrefix(peer, "tmdb:movie:")))
			anchored = anchored || (!state.Blocked && nativeByEpisode[peer] == in.ViewAnchor)
		}
		eligible[ep.ID] = valid
	}
	retainedView := false
	if virtual && !watched {
		if e = tx.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM alternate_history_views WHERE user_id=$1 AND catalog_id=$2 AND anchor_id=$3)", uid, id, in.ViewAnchor).Scan(&retainedView); e != nil {
			http.Error(w, "alternate history unavailable", http.StatusServiceUnavailable)
			return
		}
	}
	if virtual && !anchored && !retainedView {
		http.Error(w, "reviewed mapping to the saved native anchor required", http.StatusConflict)
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
				if virtual {
					filtered := []tracking.Episode{}
					for _, prior := range earlier {
						if eligible[prior.ID] {
							filtered = append(filtered, prior)
						}
					}
					earlier = filtered
				}
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
	nativeSet := map[string]bool{}
	for _, target := range targets {
		nodeID := id
		if target != nil {
			nodeID = *target
		}
		if virtual && !eligible[nodeID] && (!retainedView || !snapshot.States[nodeID].Direct) {
			http.Error(w, "all selected episodes need confirmed mappings and saved native entries", http.StatusConflict)
			return
		}
		state := snapshot.States[nodeID]
		if !state.Blocked {
			for _, peer := range state.Sources {
				if cid := nativeByEpisode[peer]; cid != "" && cid != id {
					nativeSet[cid] = true
				}
			}
		}
	}
	nativeIDs := []string{}
	for cid := range nativeSet {
		nativeIDs = append(nativeIDs, cid)
	}
	linkedChanges, linkedPlan := linkedRemovalPlan(uid, targets, snapshot, id)
	if watched {
		linkedChanges = nil
	}
	derivedChanges := []string{}
	if !watched {
		derivedChanges = derivedUnwatchNodes(targets, id, linkedChanges, snapshot)
	}
	offerReturn := !watched && base == "completed"
	if !watched && len(linkedChanges) > 0 {
		catalogs := []string{}
		for _, change := range linkedChanges {
			catalogs = append(catalogs, change.CatalogID)
		}
		var otherCompleted bool
		if e = tx.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM library WHERE user_id=$1 AND catalog_id=ANY($2::text[]) AND base='completed')", uid, catalogs).Scan(&otherCompleted); e != nil {
			http.Error(w, "linked status unavailable", http.StatusServiceUnavailable)
			return
		}
		offerReturn = offerReturn || otherCompleted
	}
	if !watched && len(nativeIDs) > 0 {
		var completed bool
		if e = tx.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM library WHERE user_id=$1 AND catalog_id=ANY($2::text[]) AND base='completed')", uid, nativeIDs).Scan(&completed); e != nil {
			http.Error(w, "native status unavailable", http.StatusServiceUnavailable)
			return
		}
		offerReturn = offerReturn || completed
	}
	if !watched && len(derivedChanges) > 0 {
		var completed bool
		if e = tx.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM library l JOIN mapping_nodes n ON n.catalog_id=l.catalog_id WHERE l.user_id=$1 AND n.id=ANY($2::text[]) AND l.base='completed')", uid, derivedChanges).Scan(&completed); e != nil {
			http.Error(w, "derived status unavailable", http.StatusServiceUnavailable)
			return
		}
		offerReturn = offerReturn || completed
	}
	if in.Preview {
		writeJSON(w, map[string]interface{}{"earlier": earlier, "target_count": len(targets), "offer_return_watching": offerReturn, "linked_changes": linkedChanges, "linked_plan": linkedPlan})
		return
	}
	if !watched && (len(linkedChanges) > 0 || in.IncludeLinked) && (!in.IncludeLinked || in.LinkedPlan != linkedPlan) {
		http.Error(w, "linked history changed or was not confirmed; preview again", http.StatusConflict)
		return
	}
	if virtual {
		if _, e = tx.Exec(r.Context(), "INSERT INTO alternate_history_views(user_id,catalog_id,anchor_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING", uid, id, in.ViewAnchor); e != nil {
			http.Error(w, "alternate history unavailable", http.StatusServiceUnavailable)
			return
		}
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
		nodeID := id
		if ep != nil {
			nodeID = *ep
		}
		if watched && snapshot.RawStates[nodeID].Derived {
			if _, e = tx.Exec(r.Context(), "DELETE FROM history_tombstones WHERE user_id=$1 AND catalog_id=$2 AND episode_key=$3", uid, id, episodeKey); e != nil {
				http.Error(w, "tracking failed", http.StatusServiceUnavailable)
				return
			}
			for i := range episodes {
				if episodes[i].ID == nodeID {
					episodes[i].Watched = true
				}
			}
			continue
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
			_, e = tx.Exec(r.Context(), `INSERT INTO history(user_id,catalog_id,episode_id,watched_date,watched_time,provenance,local_action_at) VALUES($1,$2,$3,$4,$5,'{"source":"app"}',now()) ON CONFLICT(user_id,catalog_id,episode_id) DO NOTHING`, uid, id, ep, day, in.Date.Timestamp)
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
	if !watched {
		for _, change := range linkedChanges {
			episodeKey := change.EpisodeID
			var episodeID interface{} = change.EpisodeID
			if change.Movie {
				episodeKey = ""
				episodeID = nil
			}
			_, e = tx.Exec(r.Context(), "INSERT INTO history_tombstones(user_id,catalog_id,episode_key) VALUES($1,$2,$3) ON CONFLICT(user_id,catalog_id,episode_key) DO UPDATE SET deleted_at=now()", uid, change.CatalogID, episodeKey)
			if e != nil {
				http.Error(w, "linked unwatch failed", http.StatusServiceUnavailable)
				return
			}
			if _, e = tx.Exec(r.Context(), "DELETE FROM history WHERE user_id=$1 AND catalog_id=$2 AND episode_id IS NOT DISTINCT FROM $3::text", uid, change.CatalogID, episodeID); e != nil {
				http.Error(w, "linked unwatch failed", http.StatusServiceUnavailable)
				return
			}
			if in.ReturnWatching {
				if _, e = tx.Exec(r.Context(), "UPDATE library SET base='watching',status_id=NULL,updated_at=now() WHERE user_id=$1 AND catalog_id=$2 AND base='completed'", uid, change.CatalogID); e != nil {
					http.Error(w, "linked status failed", http.StatusServiceUnavailable)
					return
				}
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
	if !virtual && dest != base {
		_, e = tx.Exec(r.Context(), "UPDATE library SET base=$3,status_id=NULL,updated_at=now() WHERE user_id=$1 AND catalog_id=$2", uid, id, dest)
	}
	if e != nil {
		http.Error(w, "status update failed", http.StatusServiceUnavailable)
		return
	}
	if !watched {
		if e = persistDerivedUnwatch(r.Context(), tx, uid, derivedChanges, in.ReturnWatching); e != nil {
			http.Error(w, "derived unwatch failed", http.StatusServiceUnavailable)
			return
		}
	}
	if e = updateNativeProgress(r.Context(), tx, uid, nativeIDs, watched, in.ReturnWatching); e != nil {
		http.Error(w, "native status update failed", http.StatusServiceUnavailable)
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
	if in.Replacement == "" {
		http.Error(w, "replacement status required", http.StatusBadRequest)
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
	var oldID string
	if e = tx.QueryRow(r.Context(), "SELECT id FROM statuses WHERE user_id=$1 AND id=$2 FOR UPDATE", uid, r.PathValue("id")).Scan(&oldID); e != nil {
		http.Error(w, "status not found", http.StatusNotFound)
		return
	}
	base := in.Replacement
	var replacementID interface{}
	if !tracking.ValidBase(base) {
		var targetID string
		if e = tx.QueryRow(r.Context(), "SELECT id,base FROM statuses WHERE user_id=$1 AND id=$2 AND id<>$3 FOR UPDATE", uid, in.Replacement, oldID).Scan(&targetID, &base); e != nil {
			http.Error(w, "replacement status unavailable", http.StatusBadRequest)
			return
		}
		replacementID = targetID
	}
	moved, e := tx.Exec(r.Context(), "UPDATE library SET base=$3,status_id=$4,updated_at=now() WHERE user_id=$1 AND status_id=$2", uid, oldID, base, replacementID)
	if e != nil {
		http.Error(w, "replacement failed", http.StatusConflict)
		return
	}
	if replacementID != nil && moved.RowsAffected() > 0 {
		if _, e = tx.Exec(r.Context(), "UPDATE statuses SET used=true WHERE user_id=$1 AND id=$2", uid, replacementID); e != nil {
			http.Error(w, "replacement failed", http.StatusConflict)
			return
		}
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
	if _, e = a.DB.Exec(r.Context(), "UPDATE users SET settings=$2::jsonb || CASE WHEN settings?'up_next_order' THEN jsonb_build_object('up_next_order',settings->'up_next_order') ELSE '{}'::jsonb END WHERE id=$1", auth.Current(r.Context()).ID, b); e != nil {
		http.Error(w, "settings unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
