package httpapi

import (
	"encoding/json"
	"github.com/jackc/pgx/v5"
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

	tx, err := a.DB.BeginTx(r.Context(), pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		http.Error(w, "statistics unavailable", http.StatusServiceUnavailable)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	data, err := buildStatistics(r.Context(), tx, auth.Current(r.Context()).ID, timezone(r))
	if err != nil {
		http.Error(w, "statistics unavailable", http.StatusServiceUnavailable)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
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
	rows, e := a.DB.Query(r.Context(), `SELECT jsonb_build_object('id',e.id,'catalog_id',e.catalog_id,'title',c.title,'season',e.season,'number',e.number,'air_date',e.air_date,'air_time',e.air_time) FROM episodes e JOIN catalog c ON c.id=e.catalog_id JOIN library l ON l.catalog_id=e.catalog_id AND l.user_id=$1 WHERE NOT e.retired AND coalesce(e.air_date,(e.air_time AT TIME ZONE $4)::date)>=$2::date AND coalesce(e.air_date,(e.air_time AT TIME ZONE $4)::date)<$3::date ORDER BY coalesce(e.air_date,(e.air_time AT TIME ZONE $4)::date),e.air_time NULLS FIRST,e.catalog_id,e.season,e.number,e.id`, auth.Current(r.Context()).ID, start.Format("2006-01-02"), start.AddDate(0, 1, 0).Format("2006-01-02"), timezone(r))
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
