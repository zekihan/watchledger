// Package jobs runs bounded, retryable catalog refreshes in the application process.
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/zekihan/watchledger/internal/catalog"
	"time"
)

type Runner struct{ Cache *catalog.Cache }

func (r *Runner) Run(ctx context.Context) {
	timer := time.NewTicker(time.Minute)
	defer timer.Stop()
	for {
		r.pass(ctx)
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}
}
func (r *Runner) pass(ctx context.Context) {
	if ctx.Err() != nil || r.Cache.Provider.Token == "" {
		return
	}
	_, e := r.Cache.DB.Exec(ctx, `INSERT INTO jobs(id,kind,payload) SELECT 'metadata:'||c.id,'metadata',jsonb_build_object('kind',c.kind,'source_id',c.source_id) FROM catalog c JOIN library l ON l.catalog_id=c.id WHERE c.source='tmdb' AND c.fetched_at<now()-CASE WHEN l.base='watching' THEN interval '1 day' ELSE interval '7 days' END GROUP BY c.id ON CONFLICT(id) DO NOTHING`)
	if e != nil {
		return
	}
	_, e = r.Cache.DB.Exec(ctx, `INSERT INTO jobs(id,kind,payload) SELECT 'availability:'||a.catalog_id||':'||a.country,'availability',jsonb_build_object('kind',c.kind,'source_id',c.source_id,'country',a.country) FROM availability a JOIN catalog c ON c.id=a.catalog_id WHERE a.fetched_at<now()-interval '1 day' AND a.used_at>now()-interval '30 days' ON CONFLICT(id) DO NOTHING`)
	if e != nil {
		return
	}
	// Process a bounded number each pass; leases permit safe recovery after a crash.
	for range 10 {
		if e = r.one(ctx); e != nil {
			return
		}
	}
}
func (r *Runner) one(ctx context.Context) error {
	var id string
	var payload []byte
	var attempts int
	e := r.Cache.DB.QueryRow(ctx, `UPDATE jobs SET locked_until=now()+interval '5 minutes' WHERE id=(SELECT id FROM jobs WHERE run_at<=now() AND (locked_until IS NULL OR locked_until<now()) ORDER BY run_at FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING id,payload,attempts`).Scan(&id, &payload, &attempts)
	if errors.Is(e, pgx.ErrNoRows) {
		return e
	}
	if e != nil {
		return e
	}
	jobctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	var in struct {
		Kind    string `json:"kind"`
		ID      string `json:"source_id"`
		Country string `json:"country"`
	}
	e = json.Unmarshal(payload, &in)
	if e == nil {
		if in.Country != "" {
			var availability catalog.Availability
			availability, e = r.Cache.Availability(jobctx, in.Kind, in.ID, in.Country, true)
			if e == nil && availability.Stale {
				e = errors.New("availability refresh failed; cached data retained")
			}
		} else {
			var d catalog.Detail
			d, e = r.Cache.Detail(jobctx, in.Kind, in.ID, "", true)
			if e == nil && d.Stale {
				e = errors.New("provider refresh failed; cached metadata retained")
			}
			if e == nil && in.Kind == "tv" {
				var meta struct {
					Seasons []struct {
						Number int `json:"season_number"`
					} `json:"seasons"`
				}
				e = json.Unmarshal(d.Metadata, &meta)
				if e == nil {
					for _, season := range meta.Seasons {
						if _, e = r.Cache.Season(jobctx, in.ID, season.Number, ""); e != nil {
							break
						}
					}
				}
			}
		}
	}
	if e != nil {
		attempts++
		delay := time.Minute * time.Duration(1<<min(attempts, 8))
		_, saveErr := r.Cache.DB.Exec(ctx, "UPDATE jobs SET attempts=$2,last_error=$3,run_at=$4,locked_until=NULL WHERE id=$1", id, attempts, fmt.Sprint(e), time.Now().Add(delay))
		return saveErr
	}
	_, e = r.Cache.DB.Exec(ctx, "DELETE FROM jobs WHERE id=$1", id)
	return e
}
