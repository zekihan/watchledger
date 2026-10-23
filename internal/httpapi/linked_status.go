package httpapi

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"github.com/zekihan/watchledger/internal/tracking"
)

// Native entries own status and ratings. Alternate tracking never creates a
// show library row or overwrites a rating merely to display derived progress.
func updateNativeProgress(ctx context.Context, tx pgx.Tx, uid string, ids []string, watched, returnWatching bool) error {
	for _, id := range ids {
		if !watched {
			if returnWatching {
				if _, err := tx.Exec(ctx, "UPDATE library SET base='watching',status_id=NULL,updated_at=now() WHERE user_id=$1 AND catalog_id=$2 AND base='completed'", uid, id); err != nil {
					return err
				}
			}
			continue
		}
		var base string
		var raw []byte
		if err := tx.QueryRow(ctx, "SELECT l.base,c.metadata FROM library l JOIN catalog c ON c.id=l.catalog_id WHERE l.user_id=$1 AND l.catalog_id=$2", uid, id).Scan(&base, &raw); err != nil {
			return err
		}
		var variants map[string]struct {
			Status string `json:"status"`
			Count  int    `json:"number_of_episodes"`
		}
		if err := json.Unmarshal(raw, &variants); err != nil {
			return err
		}
		ended := false
		expected := 0
		for _, v := range variants {
			ended = ended || v.Status == "Ended" || v.Status == "Canceled"
			expected = max(expected, v.Count)
		}
		snapshot, err := loadEffective(ctx, tx, uid, id)
		if err != nil {
			return err
		}
		rows, err := tx.Query(ctx, "SELECT id,kind<>'regular' FROM episodes WHERE catalog_id=$1 AND NOT retired", id)
		if err != nil {
			return err
		}
		episodes := []tracking.Episode{}
		regular := 0
		for rows.Next() {
			var ep tracking.Episode
			if err = rows.Scan(&ep.ID, &ep.Special); err != nil {
				rows.Close()
				return err
			}
			state := snapshot.States[ep.ID]
			ep.Watched = state.Direct || state.Derived
			episodes = append(episodes, ep)
			if !ep.Special {
				regular++
			}
		}
		rows.Close()
		if rows.Err() != nil {
			return rows.Err()
		}
		dest := tracking.Destination(ended, expected > 0 && regular == expected, episodes)
		if dest != base {
			if _, err = tx.Exec(ctx, "UPDATE library SET base=$3,status_id=NULL,updated_at=now() WHERE user_id=$1 AND catalog_id=$2", uid, id, dest); err != nil {
				return err
			}
		}
	}
	return nil
}

func derivedUnwatchNodes(targets []*string, catalogID string, changes []recordedLink, before effectiveSnapshot) []string {
	removed := map[string]bool{}
	for _, target := range targets {
		if target == nil {
			removed[catalogID] = true
		} else {
			removed[*target] = true
		}
	}
	for _, change := range changes {
		removed[change.EpisodeID] = true
	}
	out := []string{}
	for id, state := range before.States {
		if !state.Derived || state.Direct {
			continue
		}
		groups := state.SourceGroups
		if len(groups) == 0 {
			groups = [][]string{state.Sources}
		}
		remains := false
		for _, group := range groups {
			complete := len(group) > 0
			for _, source := range group {
				_, recorded := before.Direct[source]
				complete = complete && recorded && !removed[source]
			}
			remains = remains || complete
		}
		if !remains {
			out = append(out, id)
		}
	}
	return out
}

// Preserve the explicit edit in already-private derived views as well, so a
// later import through another identity cannot resurrect an unwatched work.
func persistDerivedUnwatch(ctx context.Context, tx pgx.Tx, uid string, nodes []string, returnWatching bool) error {
	rows, err := tx.Query(ctx, `SELECT n.id,n.catalog_id,n.kind='movie' FROM mapping_nodes n WHERE n.id=ANY($2::text[]) AND (EXISTS(SELECT 1 FROM library l WHERE l.user_id=$1 AND l.catalog_id=n.catalog_id) OR EXISTS(SELECT 1 FROM alternate_history_views v WHERE v.user_id=$1 AND v.catalog_id=n.catalog_id))`, uid, nodes)
	if err != nil {
		return err
	}
	type node struct {
		ID, Catalog string
		Movie       bool
	}
	items := []node{}
	for rows.Next() {
		var n node
		if err = rows.Scan(&n.ID, &n.Catalog, &n.Movie); err != nil {
			rows.Close()
			return err
		}
		items = append(items, n)
	}
	rows.Close()
	if rows.Err() != nil {
		return rows.Err()
	}
	for _, n := range items {
		key := n.ID
		if n.Movie {
			key = ""
		}
		if _, err = tx.Exec(ctx, `INSERT INTO history_tombstones(user_id,catalog_id,episode_key) SELECT $1,$2,$3 WHERE NOT EXISTS(SELECT 1 FROM history WHERE user_id=$1 AND catalog_id=$2 AND coalesce(episode_id,'')=$3) ON CONFLICT(user_id,catalog_id,episode_key) DO UPDATE SET deleted_at=now()`, uid, n.Catalog, key); err != nil {
			return err
		}
		if returnWatching {
			if _, err = tx.Exec(ctx, "UPDATE library SET base='watching',status_id=NULL,updated_at=now() WHERE user_id=$1 AND catalog_id=$2 AND base='completed'", uid, n.Catalog); err != nil {
				return err
			}
		}
	}
	return nil
}
