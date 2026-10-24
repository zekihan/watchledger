package jobs

import (
	"context"
	"fmt"
	"github.com/zekihan/watchledger/internal/catalog"
	"testing"
)

func TestTMDBRefreshCadenceAndRecentlyUsedCountries(t *testing.T) {
	p := jobDatabase(t)
	ctx := context.Background()
	var uid string
	if err := p.QueryRow(ctx, "INSERT INTO users(issuer,subject) VALUES('fixture','tmdb-cadence') RETURNING id").Scan(&uid); err != nil {
		t.Fatal(err)
	}
	for i, fixture := range []struct {
		base, age string
		saved     bool
	}{
		{"watching", "25 hours", true}, {"watching", "23 hours", true},
		{"completed", "8 days", true}, {"planned", "6 days", true},
		{"watching", "8 days", false},
	} {
		id := fmt.Sprintf("tmdb:tv:%d", i+1)
		if _, err := p.Exec(ctx, "INSERT INTO catalog(id,source,source_id,kind,title,fetched_at) VALUES($1,'tmdb',$2,'tv','Synthetic',now()-$3::interval)", id, fmt.Sprint(i+1), fixture.age); err != nil {
			t.Fatal(err)
		}
		if fixture.saved {
			if _, err := p.Exec(ctx, "INSERT INTO library(user_id,catalog_id,base) VALUES($1,$2,$3)", uid, id, fixture.base); err != nil {
				t.Fatal(err)
			}
		}
	}
	for i, age := range []string{"2 days", "8 days"} {
		id := fmt.Sprintf("tmdb:movie:%d", 100+i)
		if _, err := p.Exec(ctx, "INSERT INTO catalog(id,source,source_id,kind,title,fetched_at) VALUES($1,'tmdb',$2,'movie','Synthetic movie',now()-$3::interval)", id, fmt.Sprint(100+i), age); err != nil {
			t.Fatal(err)
		}
		if _, err := p.Exec(ctx, "INSERT INTO library(user_id,catalog_id,base) VALUES($1,$2,'watching')", uid, id); err != nil {
			t.Fatal(err)
		}
	}
	for _, fixture := range []struct{ country, age, used string }{
		{"TR", "25 hours", "1 day"}, {"US", "23 hours", "1 day"}, {"DE", "25 hours", "31 days"},
	} {
		if _, err := p.Exec(ctx, "INSERT INTO availability(catalog_id,country,offers,fetched_at,used_at) VALUES('tmdb:tv:1',$1,'{}',now()-$2::interval,now()-$3::interval)", fixture.country, fixture.age, fixture.used); err != nil {
			t.Fatal(err)
		}
	}
	r := Runner{Cache: &catalog.Cache{DB: p, Provider: catalog.New("synthetic")}}
	for range 2 {
		if err := r.enqueue(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var ids []string
	if err := p.QueryRow(ctx, "SELECT array_agg(id ORDER BY id) FROM jobs").Scan(&ids); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(ids) != "[availability:tmdb:tv:1:TR metadata:tmdb:movie:101 metadata:tmdb:tv:1 metadata:tmdb:tv:3]" {
		t.Fatalf("unexpected queued work: %v", ids)
	}
}
