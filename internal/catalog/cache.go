package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/url"
	"sort"
	"strconv"
	"time"
)

type Cache struct {
	DB       *pgxpool.Pool
	Provider *TMDB
}
type Detail struct {
	Language  string          `json:"language"`
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
	d := Detail{ID: key, Language: language}
	var translated map[string]json.RawMessage
	e = c.DB.QueryRow(ctx, "SELECT metadata,fetched_at FROM catalog WHERE id=$1", key).Scan(&d.Metadata, &d.FetchedAt)
	cached := e == nil
	if cached {
		var localized json.RawMessage
		var fetched time.Time
		if err := c.DB.QueryRow(ctx, "SELECT metadata,fetched_at FROM catalog_localizations WHERE catalog_id=$1 AND language=$2", key, language).Scan(&localized, &fetched); err == nil {
			d.FetchedAt = fetched
		}
	}
	if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return d, e
	}

	requestedCached := false
	if cached {
		_ = json.Unmarshal(d.Metadata, &translated)
		if raw, ok := translated[language]; ok {
			d.Metadata = raw
			requestedCached = true
		} else {
			keys := make([]string, 0, len(translated))
			for key := range translated {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			if len(keys) > 0 {
				d.Language = keys[0]
				d.Metadata = translated[keys[0]]
			} else {
				cached = false
			}
		}
	}
	if requestedCached && !force && time.Since(d.FetchedAt) < 24*time.Hour {
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
			if d.Language != language {
				d.Warning += "; requested translation unavailable, showing cached language " + d.Language
			}
			return d, nil
		}
		return d, e
	}
	var meta struct {
		ID       int64  `json:"id"`
		Title    string `json:"title"`
		Name     string `json:"name"`
		Overview string `json:"overview"`
	}
	if e = json.Unmarshal(raw, &meta); e != nil {
		return d, e
	}
	expectedID, _ := strconv.ParseInt(id, 10, 64)
	if meta.ID != expectedID {
		return d, errors.New("provider returned a different source identity")
	}
	if meta.Title == "" {
		meta.Title = meta.Name
	}
	if translated == nil {
		translated = map[string]json.RawMessage{}
	}
	if language != "" && meta.Overview == "" && kind != "person" {
		defaults, err := c.Provider.Get(ctx, "/"+kind+"/"+id, url.Values{"append_to_response": {"credits,recommendations,videos,external_ids"}})
		if err == nil {
			translated[""] = defaults
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
	_, e = c.DB.Exec(ctx, `INSERT INTO catalog(id,source,source_id,kind,title,metadata,fetched_at) VALUES($1,'tmdb',$2,$3,$4,$5,$6) ON CONFLICT(id) DO UPDATE SET title=EXCLUDED.title,metadata=catalog.metadata || EXCLUDED.metadata,fetched_at=EXCLUDED.fetched_at`, key, id, kind, meta.Title, stored, d.FetchedAt)
	if e == nil {
		_, e = c.DB.Exec(ctx, "INSERT INTO catalog_localizations(catalog_id,language,metadata,fetched_at) VALUES($1,$2,$3,$4) ON CONFLICT(catalog_id,language) DO UPDATE SET metadata=EXCLUDED.metadata,fetched_at=EXCLUDED.fetched_at", key, language, raw, d.FetchedAt)
	}
	return d, e
}
func (c *Cache) Season(ctx context.Context, id string, season int, language string) (json.RawMessage, error) {
	key, e := Identity("tv", id)
	if e != nil || season < 0 {
		return nil, errors.New("invalid season")
	}
	var stored json.RawMessage
	var storedAt time.Time
	if err := c.DB.QueryRow(ctx, "SELECT metadata,fetched_at FROM season_cache WHERE catalog_id=$1 AND season=$2 AND language=$3", key, season, language).Scan(&stored, &storedAt); err == nil && time.Since(storedAt) < 24*time.Hour {
		return stored, nil
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
	if data.Episodes == nil {
		return nil, errors.New("provider omitted episode list")
	}
	ids := []string{}
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
		ids = append(ids, eid)
		_, e = tx.Exec(ctx, `INSERT INTO episodes(id,catalog_id,source,source_id,season,number,kind,metadata,air_date) VALUES($1,$2,'tmdb',$3,$4,$5,$6,$7,$8) ON CONFLICT(id) DO UPDATE SET metadata=EXCLUDED.metadata,air_date=EXCLUDED.air_date,season=EXCLUDED.season,number=EXCLUDED.number,retired=false WHERE episodes.catalog_id=EXCLUDED.catalog_id`, eid, key, fmt.Sprint(ep.ID), season, ep.Number, kind, b, date)
		if e != nil {
			return nil, e
		}
	}
	if _, e = tx.Exec(ctx, `INSERT INTO notices(user_id,kind,data) SELECT DISTINCT h.user_id,'episode_reconciliation',jsonb_build_object('catalog_id',$1::text,'episode_id',e.id) FROM episodes e JOIN history h ON h.episode_id=e.id WHERE e.catalog_id=$1 AND e.season=$2 AND e.source='tmdb' AND NOT e.retired AND NOT(e.id=ANY($3::text[]))`, key, season, ids); e != nil {
		return nil, e
	}
	if _, e = tx.Exec(ctx, "UPDATE episodes SET retired=true WHERE catalog_id=$1 AND season=$2 AND source='tmdb' AND NOT(id=ANY($3::text[]))", key, season, ids); e != nil {
		return nil, e
	}
	if _, e = tx.Exec(ctx, "INSERT INTO season_cache(catalog_id,season,language,metadata) VALUES($1,$2,$3,$4) ON CONFLICT(catalog_id,season,language) DO UPDATE SET metadata=EXCLUDED.metadata,fetched_at=now()", key, season, language, raw); e != nil {
		return nil, e
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, e
	}
	return raw, nil
}
