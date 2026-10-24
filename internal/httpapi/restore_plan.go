package httpapi

import (
	"context"
	"github.com/zekihan/watchledger/internal/portable"
)

// Fetch the destination set once, rather than one round trip per incoming title.
func restoreLibrarySummary(ctx context.Context, db queryReader, uid string, library []portable.Library) ([]string, int, error) {
	ids := make([]string, 0, len(library))
	for _, l := range library {
		ids = append(ids, l.CatalogID)
	}
	rows, err := db.Query(ctx, "SELECT catalog_id FROM library WHERE user_id=$1 AND catalog_id=ANY($2::text[])", uid, ids)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	existing := map[string]bool{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, 0, err
		}
		existing[id] = true
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}
	conflicts := []string{}
	additions := 0
	for _, id := range ids {
		if existing[id] {
			conflicts = append(conflicts, id)
		} else {
			additions++
		}
	}
	return conflicts, additions, nil
}

// Compatible names can be reused, but never reinterpret a different base status.
// Apply recomputes this plan while holding the account lock.
func restoreStatusPlan(ctx context.Context, db queryReader, uid string, incoming []portable.Status) (map[string]string, []string, error) {
	rows, err := db.Query(ctx, "SELECT id::text,name,base FROM statuses WHERE user_id=$1", uid)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	byID := map[string]portable.Status{}
	byName := map[string]portable.Status{}
	for rows.Next() {
		var s portable.Status
		if err = rows.Scan(&s.ID, &s.Name, &s.Base); err != nil {
			return nil, nil, err
		}
		byID[s.ID] = s
		byName[s.Name] = s
	}
	if err = rows.Err(); err != nil {
		return nil, nil, err
	}
	ids := map[string]string{}
	conflicts := []string{}
	for _, s := range incoming {
		id := scopedID(uid, "status", s.ID)
		old, ok := byID[s.ID]
		if !ok {
			old, ok = byID[id]
		}
		if !ok {
			old, ok = byName[s.Name]
		}
		if ok {
			if old.Base != s.Base {
				conflicts = append(conflicts, s.Name)
				continue
			}
			id = old.ID
		}
		ids[s.ID] = id
		if !ok {
			s.ID = id
			byID[id] = s
			byName[s.Name] = s
		}
	}
	return ids, conflicts, nil
}
