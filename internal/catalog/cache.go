package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/url"
	"time"
)

type Cache struct {
	DB       *pgxpool.Pool
	Provider *TMDB
}
type Detail struct {
	ID        string          `json:"id"`
	Metadata  json.RawMessage `json:"metadata"`
	FetchedAt time.Time       `json:"fetched_at"`
	Stale     bool            `json:"stale"`
	Warning   string          `json:"warning,omitempty"`
}

func (c *Cache) Detail(ctx context.Context, kind, id, language string, force bool) (Detail, error) {
	key, e := Identity(kind, id)
	if e != nil {
		return Detail{}, e
	}
	d := Detail{ID: key}
	var translated map[string]json.RawMessage
	e = c.DB.QueryRow(ctx, "SELECT metadata,fetched_at FROM catalog WHERE id=$1", key).Scan(&d.Metadata, &d.FetchedAt)
	cached := e == nil
	if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return d, e
	}
	// Per-language representations preserve the source default for fallback.
	if cached {
		_ = json.Unmarshal(d.Metadata, &translated)
		if raw, ok := translated[language]; ok {
			d.Metadata = raw
		} else {
			cached = false
		}
	}
	if cached && !force && time.Since(d.FetchedAt) < 24*time.Hour {
		return d, nil
	}
	params := url.Values{"append_to_response": {"credits,recommendations,videos,external_ids"}}
	if kind == "person" {
		params.Set("append_to_response", "combined_credits")
	}
	if language != "" {
		params.Set("language", language)
	}
	raw, e := c.Provider.Get(ctx, "/"+kind+"/"+id, params)
	if e != nil {
		if cached {
			d.Stale = true
			d.Warning = e.Error()
			return d, nil
		}
		return d, e
	}
	var meta struct {
		Title    string `json:"title"`
		Name     string `json:"name"`
		Overview string `json:"overview"`
	}
	if e = json.Unmarshal(raw, &meta); e != nil {
		return d, e
	}
	if meta.Title == "" {
		meta.Title = meta.Name
	}
	if language != "" && meta.Overview == "" && kind != "person" {
		defaults, err := c.Provider.Get(ctx, "/"+kind+"/"+id, url.Values{"append_to_response": {"credits,recommendations,videos,external_ids"}})
		if err == nil {
			var local, base map[string]interface{}
			if json.Unmarshal(raw, &local) == nil && json.Unmarshal(defaults, &base) == nil {
				for _, k := range []string{"overview", "title", "name"} {
					if local[k] == "" || local[k] == nil {
						local[k] = base[k]
					}
				}
				if merged, err := json.Marshal(local); err == nil {
					raw = merged
				}
			}
		}
	}
	if translated == nil {
		translated = map[string]json.RawMessage{}
	}
	translated[language] = raw
	stored, e := json.Marshal(translated)
	if e != nil {
		return d, e
	}
	d.Metadata = raw
	d.FetchedAt = time.Now().UTC()
	_, e = c.DB.Exec(ctx, `INSERT INTO catalog(id,source,source_id,kind,title,metadata,fetched_at) VALUES($1,'tmdb',$2,$3,$4,$5,$6) ON CONFLICT(id) DO UPDATE SET title=EXCLUDED.title,metadata=EXCLUDED.metadata,fetched_at=EXCLUDED.fetched_at`, key, id, kind, meta.Title, stored, d.FetchedAt)
	return d, e
}
func (c *Cache) Season(ctx context.Context, id string, season int, language string) (json.RawMessage, error) {
	key, e := Identity("tv", id)
	if e != nil || season < 0 {
		return nil, errors.New("invalid season")
	}
	if _, e = c.Detail(ctx, "tv", id, language, false); e != nil {
		return nil, e
	}
	raw, e := c.Provider.Get(ctx, fmt.Sprintf("/tv/%s/season/%d", id, season), url.Values{"language": {language}})
	if e != nil {
		rows, err := c.DB.Query(ctx, "SELECT metadata FROM episodes WHERE catalog_id=$1 AND season=$2 ORDER BY number", key, season)
		if err != nil {
			return nil, e
		}
		defer rows.Close()
		eps := []json.RawMessage{}
		for rows.Next() {
			var b json.RawMessage
			if err = rows.Scan(&b); err != nil {
				return nil, err
			}
			eps = append(eps, b)
		}
		if err = rows.Err(); err != nil {
			return nil, err
		}
		if len(eps) == 0 {
			return nil, e
		}
		return json.Marshal(map[string]interface{}{"episodes": eps, "stale": true, "warning": e.Error()})
	}
	var data struct {
		Episodes []json.RawMessage `json:"episodes"`
	}
	if e = json.Unmarshal(raw, &data); e != nil {
		return nil, e
	}
	tx, e := c.DB.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, b := range data.Episodes {
		var ep struct {
			ID      int64  `json:"id"`
			Number  int    `json:"episode_number"`
			AirDate string `json:"air_date"`
		}
		if e = json.Unmarshal(b, &ep); e != nil {
			return nil, e
		}
		if ep.ID < 1 {
			return nil, errors.New("missing episode identity")
		}
		kind := "regular"
		if season == 0 {
			kind = "special"
		}
		var date interface{}
		if ep.AirDate != "" {
			date = ep.AirDate
		}
		eid := fmt.Sprintf("tmdb:episode:%d", ep.ID)
		_, e = tx.Exec(ctx, `INSERT INTO episodes(id,catalog_id,source,source_id,season,number,kind,metadata,air_date) VALUES($1,$2,'tmdb',$3,$4,$5,$6,$7,$8) ON CONFLICT(id) DO UPDATE SET metadata=EXCLUDED.metadata,air_date=EXCLUDED.air_date,season=EXCLUDED.season,number=EXCLUDED.number WHERE episodes.catalog_id=EXCLUDED.catalog_id`, eid, key, fmt.Sprint(ep.ID), season, ep.Number, kind, b, date)
		if e != nil {
			return nil, e
		}
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, e
	}
	return raw, nil
}
