package httpapi

import (
	"encoding/json"
	"testing"

	"github.com/zekihan/watchledger/internal/portable"
)

func TestRestoreRejectsConflictingEpisodeSourceIdentity(t *testing.T) {
	for _, field := range []string{"source", "source_id"} {
		t.Run(field, func(t *testing.T) {
			e := auditSetup(t)
			id := "fixture:restore-identity:" + e.users[0]
			epID := id + ":episode"
			audit2Exec(t, e, `INSERT INTO catalog(id,source,source_id,kind,title) VALUES($1,'fixture',$1,'tv','Synthetic restore show')`, id)
			audit2Exec(t, e, `INSERT INTO episodes(id,catalog_id,source,source_id,season,number) VALUES($1,$2,'fixture',$1,1,1)`, epID, id)
			t.Cleanup(func() {
				audit2Exec(t, e, "DELETE FROM history WHERE catalog_id=$1", id)
				audit2Exec(t, e, "DELETE FROM library WHERE catalog_id=$1", id)
				audit2Exec(t, e, "DELETE FROM episodes WHERE catalog_id=$1", id)
				audit2Exec(t, e, "DELETE FROM catalog WHERE id=$1", id)
			})
			e.req(t, 0, "PUT", "/api/library/"+id, map[string]any{"base": "watching"}, 204)
			var backup portable.Snapshot
			if err := json.Unmarshal(e.req(t, 0, "GET", "/api/export", nil, 200), &backup); err != nil {
				t.Fatal(err)
			}
			// The local row and backup claim different source episodes for the same ID.
			if field == "source" {
				backup.Episodes[0].Source = "different-provider"
			} else {
				backup.Episodes[0].SourceID = "different-episode"
			}
			backup.History = []portable.History{{CatalogID: id, EpisodeID: &epID, Provenance: json.RawMessage(`{"source":"synthetic-backup"}`), CreatedAt: backup.CreatedAt, UpdatedAt: backup.CreatedAt}}
			preview := audit2ID(e.req(t, 0, "POST", "/api/restore/preview", backup, 200))
			e.req(t, 0, "POST", "/api/restore/"+preview+"/apply", nil, 409)
			var count int
			if err := e.p.QueryRow(t.Context(), "SELECT count(*) FROM history WHERE user_id=$1", e.users[0]).Scan(&count); err != nil || count != 0 {
				t.Fatalf("history count=%d err=%v", count, err)
			}
			var state string
			if err := e.p.QueryRow(t.Context(), "SELECT state FROM imports WHERE id=$1", preview).Scan(&state); err != nil || state == "applied" {
				t.Fatalf("state=%s err=%v", state, err)
			}
			// Stable source identity still restores if numbering/metadata changed upstream.
			backup.Episodes[0].Source = "fixture"
			backup.Episodes[0].SourceID = epID
			backup.Episodes[0].Number = 7
			preview = audit2ID(e.req(t, 0, "POST", "/api/restore/preview", backup, 200))
			e.req(t, 0, "POST", "/api/restore/"+preview+"/apply", nil, 200)
			// Reapplying is idempotent and keeps the current cached episode numbering.
			e.req(t, 0, "POST", "/api/restore/"+preview+"/apply", nil, 200)
			if err := e.p.QueryRow(t.Context(), "SELECT count(*) FROM history WHERE user_id=$1 AND episode_id=$2", e.users[0], epID).Scan(&count); err != nil || count != 1 {
				t.Fatalf("restored history count=%d err=%v", count, err)
			}
			var number int
			if err := e.p.QueryRow(t.Context(), "SELECT number FROM episodes WHERE id=$1", epID).Scan(&number); err != nil || number != 1 {
				t.Fatalf("cached number=%d err=%v", number, err)
			}
		})
	}
}
