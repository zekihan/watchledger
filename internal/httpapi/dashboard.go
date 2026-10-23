package httpapi

import (
	"encoding/json"
	"github.com/zekihan/watchledger/internal/auth"
	"net/http"
)

func (a *API) dashboard(w http.ResponseWriter, r *http.Request) {
	uid := auth.Current(r.Context()).ID
	var result json.RawMessage
	e := a.DB.QueryRow(r.Context(), `SELECT jsonb_build_object(
 'up_next',coalesce((SELECT jsonb_agg(x ORDER BY position,title) FROM (SELECT l.catalog_id,c.title,l.position,e.id AS episode_id,e.season,e.number,e.air_date,e.air_time,e.metadata FROM library l JOIN catalog c ON c.id=l.catalog_id JOIN LATERAL (SELECT ep.* FROM episodes ep WHERE ep.catalog_id=l.catalog_id AND ep.kind='regular' AND NOT ep.retired AND (ep.air_date<=current_date OR ep.air_time<=now()) AND NOT EXISTS(SELECT 1 FROM history h WHERE h.user_id=l.user_id AND h.episode_id=ep.id) ORDER BY ep.season,ep.number LIMIT 1) e ON true WHERE l.user_id=$1 AND l.base='watching') x),'[]'::jsonb),
 'upcoming',coalesce((SELECT jsonb_agg(x) FROM (SELECT e.id,e.catalog_id,c.title,e.season,e.number,e.air_date,e.air_time FROM episodes e JOIN catalog c ON c.id=e.catalog_id JOIN library l ON l.catalog_id=c.id AND l.user_id=$1 WHERE e.air_date>current_date OR e.air_time>now() ORDER BY e.air_date,e.air_time LIMIT 100) x),'[]'::jsonb),
 'activity',coalesce((SELECT jsonb_agg(x) FROM (SELECT h.catalog_id,c.title,h.episode_id,h.watched_date,h.watched_time,h.created_at FROM history h JOIN catalog c ON c.id=h.catalog_id WHERE h.user_id=$1 ORDER BY h.created_at DESC LIMIT 30) x),'[]'::jsonb),
 'statistics',jsonb_build_object('movies',(SELECT count(*) FROM history WHERE user_id=$1 AND episode_id IS NULL),'episodes',(SELECT count(*) FROM history WHERE user_id=$1 AND episode_id IS NOT NULL),'monthly',coalesce((SELECT jsonb_agg(x) FROM (SELECT to_char(coalesce(watched_date,watched_time::date),'YYYY-MM') AS month,count(*) AS count FROM history WHERE user_id=$1 AND (watched_date IS NOT NULL OR watched_time IS NOT NULL) GROUP BY 1 ORDER BY 1) x),'[]'::jsonb)))`, uid).Scan(&result)
	if e != nil {
		http.Error(w, "dashboard unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, result)
}
