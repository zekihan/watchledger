package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// ReconcileSeasons retires vanished seasons only after a complete successful
// refresh. Historical facts remain intact, and concurrent metadata changes retry.
func (c *Cache) ReconcileSeasons(ctx context.Context, id string, seasons []int, metadata json.RawMessage) error {
	key, err := Identity("tv", id)
	if err != nil {
		return err
	}
	tx, err := c.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var current bool
	if err = tx.QueryRow(ctx, `SELECT catalog_progress_metadata(id)=$2::jsonb FROM catalog WHERE id=$1 FOR NO KEY UPDATE`, key, metadata).Scan(&current); err != nil {
		return err
	}
	if !current {
		return errors.New("season inventory changed during refresh")
	}
	if _, err = tx.Exec(ctx, `INSERT INTO notices(user_id,kind,data) SELECT DISTINCT h.user_id,'episode_reconciliation',jsonb_build_object('catalog_id',$1::text,'episode_id',e.id) FROM episodes e JOIN history h ON h.episode_id=e.id WHERE e.catalog_id=$1 AND e.source='tmdb' AND NOT e.retired AND NOT(e.season=ANY($2::int[]))`, key, seasons); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE episodes SET retired=true WHERE catalog_id=$1 AND source='tmdb' AND NOT(season=ANY($2::int[]))`, key, seasons); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM season_cache WHERE catalog_id=$1 AND NOT(season=ANY($2::int[]))`, key, seasons); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (c *Cache) RefreshSeasons(ctx context.Context, id, language string, metadata json.RawMessage) error {
	var inventory struct {
		Seasons []struct {
			Number int `json:"season_number"`
		} `json:"seasons"`
	}
	if err := json.Unmarshal(metadata, &inventory); err != nil {
		return err
	}
	if inventory.Seasons == nil {
		return errors.New("provider omitted season inventory")
	}
	seasons := make([]int, 0, len(inventory.Seasons))
	for _, season := range inventory.Seasons {
		raw, err := c.Season(ctx, id, season.Number, language, true)
		if err != nil {
			return fmt.Errorf("season %d refresh failed: %w", season.Number, err)
		}
		var freshness struct {
			Stale bool `json:"stale"`
		}
		if err = json.Unmarshal(raw, &freshness); err != nil {
			return err
		}
		if freshness.Stale {
			return fmt.Errorf("season %d refresh failed; cached episodes retained", season.Number)
		}
		seasons = append(seasons, season.Number)
	}
	return c.ReconcileSeasons(ctx, id, seasons, metadata)
}
