package httpapi

import (
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"github.com/zekihan/watchledger/internal/auth"
	"net/http"
)

func (a *API) dashboard(w http.ResponseWriter, r *http.Request) {
	uid := auth.Current(r.Context()).ID
	tx, err := a.DB.BeginTx(r.Context(), pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		http.Error(w, "dashboard unavailable", http.StatusServiceUnavailable)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	var result json.RawMessage
	e := tx.QueryRow(r.Context(), `SELECT jsonb_build_object(
 'upcoming',coalesce((SELECT jsonb_agg(x) FROM (SELECT e.id,e.catalog_id,catalog_display_title(c.kind,c.metadata,c.title,u.settings) AS title,e.season,e.number,e.air_date,e.air_time FROM episodes e JOIN catalog c ON c.id=e.catalog_id JOIN library l ON l.catalog_id=c.id AND l.user_id=$1 JOIN users u ON u.id=l.user_id WHERE NOT e.retired AND (e.air_date>(now() AT TIME ZONE $2)::date OR e.air_time>now()) ORDER BY coalesce(e.air_date,(e.air_time AT TIME ZONE $2)::date),e.air_time NULLS FIRST,e.catalog_id,e.season,e.number,e.id LIMIT 100) x),'[]'::jsonb),
 'activity',coalesce((SELECT jsonb_agg(x) FROM (SELECT h.catalog_id,catalog_display_title(c.kind,c.metadata,c.title,u.settings) AS title,h.episode_id,h.watched_date,h.watched_time,h.created_at,v.anchor_id AS view_anchor FROM history h JOIN catalog c ON c.id=h.catalog_id JOIN users u ON u.id=h.user_id LEFT JOIN alternate_history_views v ON v.user_id=h.user_id AND v.catalog_id=h.catalog_id WHERE h.user_id=$1 ORDER BY h.created_at DESC LIMIT 30) x),'[]'::jsonb))
`, uid, timezone(r)).Scan(&result)
	if e != nil {
		http.Error(w, "dashboard unavailable", http.StatusServiceUnavailable)
		return
	}
	cards, err := buildUpNext(r.Context(), tx, uid, timezone(r))
	if err != nil {
		http.Error(w, "up next unavailable", http.StatusServiceUnavailable)
		return
	}
	var dashboard map[string]interface{}
	if json.Unmarshal(result, &dashboard) != nil {
		http.Error(w, "dashboard unavailable", http.StatusServiceUnavailable)
		return
	}
	stats, err := buildStatistics(r.Context(), tx, uid, timezone(r))
	if err != nil {
		http.Error(w, "statistics unavailable", http.StatusServiceUnavailable)
		return
	}
	dashboard["statistics"] = stats
	dashboard["up_next"] = cards
	if err = tx.Commit(r.Context()); err != nil {
		http.Error(w, "dashboard unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, dashboard)
}

func (a *API) upNextOrder(w http.ResponseWriter, r *http.Request) {
	var in struct {
		IDs []string `json:"ids"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.IDs) > 10000 {
		http.Error(w, "too many titles", http.StatusBadRequest)
		return
	}
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		http.Error(w, "order unavailable", http.StatusServiceUnavailable)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	if _, e = tx.Exec(r.Context(), "SELECT id FROM users WHERE id=$1 FOR UPDATE", auth.Current(r.Context()).ID); e != nil {
		http.Error(w, "order unavailable", http.StatusServiceUnavailable)
		return
	}
	cards, e := buildUpNext(r.Context(), tx, auth.Current(r.Context()).ID, timezone(r))
	if e != nil {
		http.Error(w, "up next unavailable", http.StatusServiceUnavailable)
		return
	}
	visible := map[string]bool{}
	for _, card := range cards {
		visible[card.CatalogID] = true
	}
	seen := map[string]bool{}
	for n, id := range in.IDs {
		if seen[id] || !visible[id] {
			http.Error(w, "duplicate or non-visible title", http.StatusBadRequest)
			return
		}
		seen[id] = true
		_, err := tx.Exec(r.Context(), "UPDATE library SET position=$3,updated_at=now() WHERE user_id=$1 AND catalog_id=$2", auth.Current(r.Context()).ID, id, n)
		if err != nil {
			http.Error(w, "library title not found", http.StatusNotFound)
			return
		}
	}
	var oldRaw []byte
	if e = tx.QueryRow(r.Context(), "SELECT coalesce(settings->'up_next_order','[]'::jsonb) FROM users WHERE id=$1", auth.Current(r.Context()).ID).Scan(&oldRaw); e != nil {
		http.Error(w, "order unavailable", http.StatusServiceUnavailable)
		return
	}
	var old []string
	_ = json.Unmarshal(oldRaw, &old)
	for _, id := range old {
		if !seen[id] {
			in.IDs = append(in.IDs, id)
			seen[id] = true
		}
	}
	if len(in.IDs) > 10000 {
		in.IDs = in.IDs[:10000]
	}
	orderJSON, _ := json.Marshal(in.IDs)
	if _, e = tx.Exec(r.Context(), "UPDATE users SET settings=jsonb_set(settings,'{up_next_order}',$2::jsonb,true) WHERE id=$1", auth.Current(r.Context()).ID, orderJSON); e != nil {
		http.Error(w, "order unavailable", http.StatusServiceUnavailable)
		return
	}
	if e = tx.Commit(r.Context()); e != nil {
		http.Error(w, "order unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
