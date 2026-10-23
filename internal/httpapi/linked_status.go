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
