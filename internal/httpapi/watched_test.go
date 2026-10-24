package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/zekihan/watchledger/internal/mapping"
	"github.com/zekihan/watchledger/internal/store"
)

func TestWatchedDiscoveryUsesPrivateFactsNotStatusOrIncompleteMetadata(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	ctx := context.Background()
	p, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if err = store.Migrate(ctx, p); err != nil {
		t.Fatal(err)
	}
	suffix := fmt.Sprint(time.Now().UnixNano())
	var uid, other string
	for i, target := range []*string{&uid, &other} {
		if err = p.QueryRow(ctx, "INSERT INTO users(issuer,subject) VALUES('watched-fixture',$1) RETURNING id", suffix+fmt.Sprint(i)).Scan(target); err != nil {
			t.Fatal(err)
		}
	}
	defer func() { _, _ = p.Exec(ctx, "DELETE FROM users WHERE id=ANY($1::uuid[])", []string{uid, other}) }()
	ids := []string{}
	for i, kind := range []string{"movie", "movie", "tv", "tv", "anime", "movie"} {
		id := "watched-fixture:" + suffix + fmt.Sprint(i)
		ids = append(ids, id)
		if _, err = p.Exec(ctx, `INSERT INTO catalog(id,source,source_id,kind,title,metadata) VALUES($1,'watched-fixture',$1,$2,'Synthetic','{"en":{"number_of_episodes":2,"status":"Ended"}}')`, id, kind); err != nil {
			t.Fatal(err)
		}
		if _, err = p.Exec(ctx, "INSERT INTO library(user_id,catalog_id,base) VALUES($1,$2,$3)", uid, id, []string{"watching", "completed", "watching", "completed", "watching", "planned"}[i]); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = p.Exec(ctx, "INSERT INTO history(user_id,catalog_id) VALUES($1,$2),($3,$4)", uid, ids[0], other, ids[1]); err != nil {
		t.Fatal(err)
	}
	for _, index := range []int{2, 3, 4} {
		for n := 1; n <= 2; n++ {
			if index == 3 && n == 2 {
				continue
			} // Declared complete but missing part of its cached metadata.
			eid := ids[index] + fmt.Sprint(n)
			if _, err = p.Exec(ctx, "INSERT INTO episodes(id,catalog_id,source,source_id,season,number) VALUES($1,$2,'watched-fixture',$1,1,$3)", eid, ids[index], n); err != nil {
				t.Fatal(err)
			}
			if _, err = p.Exec(ctx, "INSERT INTO history(user_id,catalog_id,episode_id) VALUES($1,$2,$3)", uid, ids[index], eid); err != nil {
				t.Fatal(err)
			}
		}
	}
	edges, revision, _, err := mapping.CanonicalForKind([]mapping.Edge{{From: ids[4] + "1", To: ids[5]}, {From: ids[4] + "2", To: ids[5]}}, "movie-aliases")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(edges)
	var mid string
	if err = p.QueryRow(ctx, "INSERT INTO mappings(kind,revision,provenance,relationships,requires_consent,reviewed) VALUES('movie-aliases',$1,'{}',$2,true,true) RETURNING id", revision, raw).Scan(&mid); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = p.Exec(ctx, "DELETE FROM mapping_consents WHERE mapping_id=$1", mid)
		_, _ = p.Exec(ctx, "DELETE FROM mappings WHERE id=$1", mid)
	}()
	check := func(want []string) {
		t.Helper()
		got, e := knownWatchedTitles(ctx, p, uid)
		if e != nil {
			t.Fatal(e)
		}
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	check([]string{ids[0], ids[2], ids[4]})
	if _, err = p.Exec(ctx, `UPDATE catalog SET metadata=jsonb_set(metadata,'{en,status}','"Returning Series"') WHERE id=$1`, ids[2]); err != nil {
		t.Fatal(err)
	}
	check([]string{ids[0], ids[4]})
	if _, err = p.Exec(ctx, `UPDATE catalog SET metadata=jsonb_set(metadata,'{en,status}','"Ended"') WHERE id=$1`, ids[2]); err != nil {
		t.Fatal(err)
	}
	if _, err = p.Exec(ctx, "INSERT INTO mapping_consents(user_id,mapping_id,revision) VALUES($1,$2,$3)", uid, mid, revision); err != nil {
		t.Fatal(err)
	}
	check([]string{ids[0], ids[2], ids[4], ids[5]})
	if _, err = p.Exec(ctx, "INSERT INTO history_tombstones(user_id,catalog_id) VALUES($1,$2)", uid, ids[5]); err != nil {
		t.Fatal(err)
	}
	check([]string{ids[0], ids[2], ids[4]})
	got, err := knownWatchedTitles(ctx, p, other)
	if err != nil || !slices.Equal(got, []string{ids[1]}) {
		t.Fatalf("cross-account results %v: %v", got, err)
	}
}
