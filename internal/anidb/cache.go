package anidb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Cache struct {
	DB       *pgxpool.Pool
	Provider *Client
}
type Result struct {
	ID        string                 `json:"id"`
	Metadata  map[string]interface{} `json:"metadata"`
	FetchedAt time.Time              `json:"fetched_at"`
	Stale     bool                   `json:"stale"`
	Warning   string                 `json:"warning,omitempty"`
}

func Metadata(a Anime, language, style string) map[string]interface{} {
	status := "Unknown"
	if end := Day(a.EndDate); end != nil && *end <= time.Now().UTC().Format("2006-01-02") {
		status = "Ended"
	}
	regular, extras := 0, 0
	episodes := []map[string]interface{}{}
	for _, e := range a.Episodes {
		n, k, _ := e.Position()
		if k == "regular" {
			regular++
		} else {
			extras++
		}
		season := 1
		if k != "regular" {
			season = 0
		}
		numeric, _ := strconv.ParseInt(e.ID, 10, 64)
		episodes = append(episodes, map[string]interface{}{"id": numeric, "source_id": e.ID, "name": Choose(e.Titles, language, style), "season_number": season, "episode_number": n, "native_number": e.Number.Text, "native_type": e.Number.Type, "kind": k, "air_date": Day(e.AirDate), "runtime": e.Length})
	}
	numeric, _ := strconv.ParseInt(a.ID, 10, 64)
	metadata := map[string]interface{}{"id": numeric, "anime_type": a.Type, "name": Choose(a.Titles, language, style), "titles": a.Titles, "overview": a.Description, "poster_path": PosterURL(a.Picture), "first_air_date": Day(a.StartDate), "end_date": Day(a.EndDate), "status": status, "number_of_episodes": a.EpisodeCount, "episodes": episodes, "seasons": []map[string]interface{}{{"season_number": 1, "name": "Regular", "episode_count": regular}, {"season_number": 0, "name": "Specials / extras", "episode_count": extras}}, "source": "anidb", "source_url": "https://anidb.net/anime/" + a.ID, "license": "CC-BY-NC-SA-4.0"}
	metadata["credits"] = Credits(a)
	related := []map[string]interface{}{}
	for _, entry := range a.Related {
		if id, ok := positiveIdentity(entry.ID); ok {
			related = append(related, map[string]interface{}{"id": id, "name": entry.Name, "media_type": "anime", "relation": entry.Type})
		}
	}
	metadata["recommendations"] = map[string]interface{}{"results": related}
	tags := []map[string]interface{}{}
	for _, tag := range a.Tags {
		if _, ok := positiveIdentity(tag.ID); ok && tag.Verified == "true" && tag.Weight > 0 && tag.LocalSpoiler == "false" && tag.GlobalSpoiler == "false" {
			tags = append(tags, map[string]interface{}{"id": "anidb:tag:" + tag.ID, "name": tag.Name})
		}
	}
	metadata["native_tags"] = tags
	if a.Rating.Count > 0 && a.Rating.Value >= 0 && a.Rating.Value <= 10 {
		metadata["vote_average"] = a.Rating.Value
		metadata["vote_count"] = a.Rating.Count
	}
	return metadata
}

// reserve persists both the two-second provider interval and one-day per-dataset
// cooldown before I/O. Concurrent instances and process restarts cannot bypass it.
func (c *Cache) reserve(ctx context.Context, id string) error {
	return c.reserveDataset(ctx, "anidb:anime:"+id)
}
func (c *Cache) reserveDataset(ctx context.Context, dataset string) error {
	tx, err := c.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(74192301)")
	if err != nil {
		return err
	}
	var blocked bool
	err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM provider_request_limits WHERE key=ANY($1::text[]) AND next_allowed_at>now())", []string{"anidb:global", dataset}).Scan(&blocked)
	if err != nil {
		return err
	}
	if blocked {
		return errors.New("AniDB cooldown active; use cached data or retry later")
	}
	_, err = tx.Exec(ctx, `INSERT INTO provider_request_limits(key,next_allowed_at) VALUES('anidb:global',now()+interval '2 seconds'),($1,now()+interval '24 hours') ON CONFLICT(key) DO UPDATE SET next_allowed_at=EXCLUDED.next_allowed_at`, dataset)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (c *Cache) Detail(ctx context.Context, id, language, style string) (Result, error) {
	result := Result{ID: "anidb:anime:" + id}
	numeric, parseErr := strconv.ParseInt(id, 10, 64)
	if parseErr != nil || numeric < 1 || strconv.FormatInt(numeric, 10) != id {
		return result, errors.New("invalid AniDB identity")
	}
	var raw []byte
	err := c.DB.QueryRow(ctx, "SELECT document,fetched_at FROM anidb_documents WHERE aid=$1", id).Scan(&raw, &result.FetchedAt)
	cached := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return result, err
	}
	catalogCached := false
	if !cached {
		var readErr error
		result, catalogCached, readErr = c.catalogFallback(ctx, id, language, style)
		if readErr != nil {
			return result, readErr
		}
		if catalogCached && time.Since(result.FetchedAt) < 24*time.Hour {
			return result, nil
		}
	}
	var a Anime
	if cached {
		a, err = Parse(raw, id)
		if err != nil {
			return result, err
		}
		result.Metadata = Metadata(a, language, style)
		presentation, marshalErr := json.Marshal(map[string]interface{}{"en": Metadata(a, "en", "translated")})
		if marshalErr != nil {
			return result, marshalErr
		}
		if _, updateErr := c.DB.Exec(ctx, "UPDATE catalog SET metadata=$1,title=$2 WHERE id=$3 AND fetched_at<=$4 AND metadata IS DISTINCT FROM $1::jsonb", presentation, Choose(a.Titles, "en", "translated"), result.ID, result.FetchedAt); updateErr != nil {
			return result, updateErr
		}
		if time.Since(result.FetchedAt) < 24*time.Hour {
			return result, nil
		}
	}
	fallback := func(e error) (Result, error) {
		if cached || catalogCached {
			result.Stale = true
			result.Warning = e.Error()
			return result, nil
		}
		return result, e
	}
	if !c.Provider.Available() {
		return fallback(errors.New("AniDB requires a registered HTTP client name and version"))
	}
	if err = c.reserve(ctx, id); err != nil {
		return fallback(err)
	}
	a, raw, err = c.Provider.Fetch(ctx, id)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Banned() {
			_, _ = c.DB.Exec(ctx, `UPDATE provider_request_limits SET next_allowed_at=now()+interval '24 hours' WHERE key='anidb:global'`)
		}
		return fallback(err)
	}
	tx, err := c.DB.Begin(ctx)
	if err != nil {
		return fallback(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	key := result.ID
	metadata := Metadata(a, "en", "translated")
	payload, err := json.Marshal(map[string]interface{}{"en": metadata})
	if err != nil {
		return fallback(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO catalog(id,source,source_id,kind,title,metadata) VALUES($1,'anidb',$2,'anime',$3,$4) ON CONFLICT(id) DO UPDATE SET title=EXCLUDED.title,metadata=EXCLUDED.metadata,fetched_at=now()`, key, id, Choose(a.Titles, "en", "translated"), payload)
	if err != nil {
		return fallback(err)
	}
	// Serialize reconciliation for this catalog. Stable IDs never move between titles.
	_, err = tx.Exec(ctx, "SELECT id FROM catalog WHERE id=$1 FOR UPDATE", key)
	if err != nil {
		return fallback(err)
	}
	ids := []string{}
	for _, ep := range a.Episodes {
		eid := "anidb:episode:" + ep.ID
		ids = append(ids, eid)
		n, kind, err := ep.Position()
		if err != nil {
			return fallback(err)
		}
		season := 1
		if kind != "regular" {
			season = 0
		}
		epRaw, err := json.Marshal(map[string]interface{}{"name": Choose(ep.Titles, "en", "translated"), "titles": ep.Titles, "runtime": ep.Length, "native_number": ep.Number.Text, "native_type": ep.Number.Type})
		if err != nil {
			return fallback(err)
		}
		var oldCatalog string
		var oldSeason, oldNumber int
		err = tx.QueryRow(ctx, "SELECT catalog_id,season,number FROM episodes WHERE id=$1", eid).Scan(&oldCatalog, &oldSeason, &oldNumber)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fallback(err)
		}
		if oldCatalog != "" && oldCatalog != key {
			return fallback(errors.New("AniDB episode moved to another entry; review required"))
		}
		if oldCatalog != "" && (oldSeason != season || oldNumber != n) {
			_, err = tx.Exec(ctx, `INSERT INTO notices(user_id,kind,data) SELECT DISTINCT user_id,'episode_changed',jsonb_build_object('catalog_id',$1::text,'episode_id',$2::text) FROM history WHERE episode_id=$2`, key, eid)
			if err != nil {
				return fallback(err)
			}
		}
		_, err = tx.Exec(ctx, `INSERT INTO episodes(id,catalog_id,source,source_id,season,number,kind,metadata,air_date) VALUES($1,$2,'anidb',$3,$4,$5,$6,$7,$8) ON CONFLICT(id) DO UPDATE SET season=EXCLUDED.season,number=EXCLUDED.number,kind=EXCLUDED.kind,metadata=EXCLUDED.metadata,air_date=EXCLUDED.air_date,retired=false`, eid, key, ep.ID, season, n, kind, epRaw, Day(ep.AirDate))
		if err != nil {
			return fallback(err)
		}
	}
	if a.EpisodeCount > 0 && len(ids) == 0 {
		return fallback(errors.New("AniDB response is missing expected episode records"))
	}
	_, err = tx.Exec(ctx, `INSERT INTO notices(user_id,kind,data) SELECT DISTINCT h.user_id,'episode_changed',jsonb_build_object('catalog_id',$1::text,'episode_id',e.id) FROM episodes e JOIN history h ON h.episode_id=e.id WHERE e.catalog_id=$1 AND NOT e.retired AND NOT(e.id=ANY($2::text[]))`, key, ids)
	if err != nil {
		return fallback(err)
	}
	_, err = tx.Exec(ctx, "UPDATE episodes SET retired=true WHERE catalog_id=$1 AND NOT(id=ANY($2::text[]))", key, ids)
	if err != nil {
		return fallback(err)
	}
	err = tx.QueryRow(ctx, `INSERT INTO anidb_documents(aid,document) VALUES($1,$2) ON CONFLICT(aid) DO UPDATE SET document=EXCLUDED.document,fetched_at=now() RETURNING fetched_at`, id, raw).Scan(&result.FetchedAt)
	if err != nil {
		return fallback(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fallback(fmt.Errorf("AniDB cache update failed: %w", err))
	}
	result.Metadata = Metadata(a, language, style)
	return result, nil
}

// Portable exports preserve catalog/episode metadata, not provider XML documents.
// Restored native entries must remain usable without making a fresh source call.
func (c *Cache) catalogFallback(ctx context.Context, id, language, style string) (Result, bool, error) {
	result := Result{ID: "anidb:anime:" + id}
	settings, err := json.Marshal(map[string]string{"metadata_language": language, "anime_title": style})
	if err != nil {
		return result, false, err
	}
	var raw, episodes []byte
	var name string
	err = c.DB.QueryRow(ctx, `SELECT catalog_metadata_variant(c.metadata,$2::jsonb),catalog_display_title(c.kind,c.metadata,c.title,$2::jsonb),c.fetched_at,
 coalesce((SELECT jsonb_agg(e.metadata || jsonb_build_object('id',e.source_id::bigint,'source_id',e.source_id,'season_number',e.season,'episode_number',e.number,'kind',e.kind,'air_date',e.air_date) ORDER BY e.season,e.number,e.id) FROM episodes e WHERE e.catalog_id=c.id AND NOT e.retired),'[]'::jsonb)
 FROM catalog c WHERE c.id=$1 AND c.source='anidb' AND c.kind='anime'`, result.ID, settings).Scan(&raw, &name, &result.FetchedAt, &episodes)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, false, nil
	}
	if err != nil {
		return result, false, err
	}
	if err = json.Unmarshal(raw, &result.Metadata); err != nil {
		return result, false, err
	}
	if result.Metadata == nil {
		result.Metadata = map[string]interface{}{}
	}
	var restoredEpisodes []map[string]interface{}
	decoder := json.NewDecoder(bytes.NewReader(episodes))
	decoder.UseNumber()
	if err = decoder.Decode(&restoredEpisodes); err != nil {
		return result, false, err
	}
	result.Metadata["id"] = json.Number(id)
	result.Metadata["name"] = name
	result.Metadata["episodes"] = restoredEpisodes
	return result, true, nil
}
