package httpapi

import (
	"encoding/json"
	"github.com/zekihan/watchledger/internal/auth"
	"net/http"
	"time"
)

func timezone(r *http.Request) string {
	var s struct {
		Timezone string `json:"timezone"`
	}
	_ = json.Unmarshal(auth.Current(r.Context()).Settings, &s)
	if s.Timezone == "" {
		return "UTC"
	}
	if _, e := time.LoadLocation(s.Timezone); e != nil {
		return "UTC"
	}
	return s.Timezone
}
func (a *API) statistics(w http.ResponseWriter, r *http.Request) {
	var data json.RawMessage
	e := a.DB.QueryRow(r.Context(), `WITH events AS (SELECT h.*,e.metadata AS episode_meta,(SELECT value FROM jsonb_each(c.metadata) LIMIT 1) AS meta FROM history h JOIN catalog c ON c.id=h.catalog_id LEFT JOIN episodes e ON e.id=h.episode_id WHERE h.user_id=$1), durations AS (SELECT CASE WHEN coalesce(episode_meta->>'runtime',meta->>'runtime','')~'^[0-9]{1,5}$' THEN coalesce(episode_meta->>'runtime',meta->>'runtime')::int ELSE NULL END AS minutes FROM events) SELECT jsonb_build_object('movies',(SELECT count(*) FROM events WHERE episode_id IS NULL),'episodes',(SELECT count(*) FROM events WHERE episode_id IS NOT NULL),'estimated_minutes',(SELECT coalesce(sum(minutes),0) FROM durations),'unknown_runtime',(SELECT count(*) FROM durations WHERE minutes IS NULL),'monthly',coalesce((SELECT jsonb_agg(x) FROM (SELECT to_char(coalesce(watched_date,(watched_time AT TIME ZONE $2)::date),'YYYY-MM') AS month,count(*) AS count FROM events WHERE watched_date IS NOT NULL OR watched_time IS NOT NULL GROUP BY 1 ORDER BY 1) x),'[]'),'genres',coalesce((SELECT jsonb_agg(x) FROM (SELECT g->>'name' AS name,count(*) AS count FROM events CROSS JOIN LATERAL jsonb_array_elements(CASE WHEN jsonb_typeof(meta->'genres')='array' THEN meta->'genres' ELSE '[]' END) g GROUP BY 1 ORDER BY 2 DESC) x),'[]'))`, auth.Current(r.Context()).ID, timezone(r)).Scan(&data)
	if e != nil {
		http.Error(w, "statistics unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, data)
}
func (a *API) calendar(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")
	start, e := time.Parse("2006-01", month)
	if e != nil {
		http.Error(w, "month must be YYYY-MM", http.StatusBadRequest)
		return
	}
	rows, e := a.DB.Query(r.Context(), `SELECT jsonb_build_object('id',e.id,'catalog_id',e.catalog_id,'title',c.title,'season',e.season,'number',e.number,'air_date',e.air_date,'air_time',e.air_time) FROM episodes e JOIN catalog c ON c.id=e.catalog_id JOIN library l ON l.catalog_id=e.catalog_id AND l.user_id=$1 WHERE NOT e.retired AND coalesce(e.air_date,(e.air_time AT TIME ZONE $4)::date)>=$2::date AND coalesce(e.air_date,(e.air_time AT TIME ZONE $4)::date)<$3::date ORDER BY coalesce(e.air_date,(e.air_time AT TIME ZONE $4)::date),e.season,e.number`, auth.Current(r.Context()).ID, start.Format("2006-01-02"), start.AddDate(0, 1, 0).Format("2006-01-02"), timezone(r))
	if e != nil {
		http.Error(w, "calendar unavailable", http.StatusServiceUnavailable)
		return
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var b json.RawMessage
		if e = rows.Scan(&b); e != nil {
			http.Error(w, "calendar unavailable", http.StatusServiceUnavailable)
			return
		}
		out = append(out, b)
	}
	if rows.Err() != nil {
		http.Error(w, "calendar unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, out)
}
