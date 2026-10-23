package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"time"
)

type ExternalEpisode struct {
	ID     int `json:"id"`
	ShowID int `json:"show_id"`
	Season int `json:"season_number"`
	Number int `json:"episode_number"`
}

// ResolveTVDBEpisode uses the documented external-ID endpoint. TVDB season and
// episode numbers are deliberately not inputs, because their order may differ.
func (c *Cache) ResolveTVDBEpisode(ctx context.Context, id int) (ExternalEpisode, error) {
	out := ExternalEpisode{}
	if id < 1 {
		return out, errors.New("invalid TVDB episode identity")
	}
	var checked time.Time
	err := c.DB.QueryRow(ctx, "SELECT tmdb_episode_id,tmdb_show_id,season,number,checked_at FROM external_episode_links WHERE source='tvdb' AND source_id=$1", id).Scan(&out.ID, &out.ShowID, &out.Season, &out.Number, &checked)
	if err == nil && time.Since(checked) < 24*time.Hour {
		return out, nil
	}
	raw, err := c.Provider.Get(ctx, "/find/"+strconv.Itoa(id), url.Values{"external_source": {"tvdb_id"}})
	if err != nil {
		return out, err
	}
	var data struct {
		Episodes []ExternalEpisode `json:"tv_episode_results"`
	}
	if json.Unmarshal(raw, &data) != nil || len(data.Episodes) != 1 {
		return out, errors.New("TVDB episode has no unique TMDB episode match")
	}
	out = data.Episodes[0]
	if out.ID < 1 || out.ShowID < 1 || out.Season < 0 || out.Number < 1 {
		return out, errors.New("invalid external episode match")
	}
	_, err = c.DB.Exec(ctx, `INSERT INTO external_episode_links(source,source_id,tmdb_episode_id,tmdb_show_id,season,number) VALUES('tvdb',$1,$2,$3,$4,$5) ON CONFLICT(source,source_id) DO UPDATE SET tmdb_episode_id=EXCLUDED.tmdb_episode_id,tmdb_show_id=EXCLUDED.tmdb_show_id,season=EXCLUDED.season,number=EXCLUDED.number,checked_at=now()`, id, out.ID, out.ShowID, out.Season, out.Number)
	return out, err
}
func (c *Cache) CacheExternalEpisode(ctx context.Context, ep ExternalEpisode) error {
	key := "tmdb:episode:" + strconv.Itoa(ep.ID)
	owner := "tmdb:tv:" + strconv.Itoa(ep.ShowID)
	var matches bool
	err := c.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM episodes WHERE id=$1 AND catalog_id=$2 AND source='tmdb' AND NOT retired)", key, owner).Scan(&matches)
	if err != nil {
		return err
	}
	if matches {
		return nil
	}
	if _, err = c.Detail(ctx, "tv", strconv.Itoa(ep.ShowID), "en", false); err != nil {
		return err
	}
	if _, err = c.Season(ctx, strconv.Itoa(ep.ShowID), ep.Season, "en"); err != nil {
		return err
	}
	err = c.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM episodes WHERE id=$1 AND catalog_id=$2 AND source='tmdb' AND NOT retired)", key, owner).Scan(&matches)
	if err != nil {
		return err
	}
	if !matches {
		return errors.New("resolved episode is absent from provider season metadata")
	}
	return nil
}
