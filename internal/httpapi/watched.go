package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/zekihan/watchledger/internal/auth"
)

// knownWatchedTitles never treats a manually selected status as viewing history.
// Incomplete episode caches stay visible rather than guessing completion.
func knownWatchedTitles(ctx context.Context, db queryReader, uid string) ([]string, error) {
	snapshot, err := loadEffective(ctx, db, uid, "")
	if err != nil {
		return nil, err
	}
	nodes := make([]string, 0, len(snapshot.States))
	for id := range snapshot.States {
		nodes = append(nodes, id)
	}
	rows, err := db.Query(ctx, `SELECT c.id,c.kind,catalog_metadata_variant(c.metadata,u.settings),
 ARRAY(SELECT e.id FROM episodes e WHERE e.catalog_id=c.id AND e.kind='regular' AND NOT e.retired ORDER BY e.id)
 FROM catalog c JOIN users u ON u.id=$1 WHERE c.id IN (
 SELECT catalog_id FROM library WHERE user_id=$1
 UNION SELECT catalog_id FROM history WHERE user_id=$1
 UNION SELECT catalog_id FROM mapping_nodes WHERE id=ANY($2::text[])) ORDER BY c.id`, uid, nodes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id, kind string
		var raw []byte
		var episodes []string
		if err = rows.Scan(&id, &kind, &raw, &episodes); err != nil {
			return nil, err
		}
		if kind == "movie" {
			state := snapshot.States[id]
			if state.Direct || state.Derived {
				out = append(out, id)
			}
			continue
		}
		if kind != "tv" && kind != "anime" {
			continue
		}
		var metadata struct {
			Count  int    `json:"number_of_episodes"`
			Status string `json:"status"`
		}
		if json.Unmarshal(raw, &metadata) != nil || metadata.Count <= 0 || len(episodes) != metadata.Count || (metadata.Status != "Ended" && metadata.Status != "Canceled") {
			continue
		}
		complete := true
		for _, episode := range episodes {
			state := snapshot.States[episode]
			if !state.Direct && !state.Derived {
				complete = false
				break
			}
		}
		if complete {
			out = append(out, id)
		}
	}
	return out, rows.Err()
}

func (a *API) watchedTitles(w http.ResponseWriter, r *http.Request) {
	ids, err := knownWatchedTitles(r.Context(), a.DB, auth.Current(r.Context()).ID)
	if err != nil {
		http.Error(w, "watched history unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, ids)
}
