package jobs

import (
	"context"
	"fmt"
	"github.com/zekihan/watchledger/internal/catalog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRemovedSeasonRetiresGhostEpisodes(t *testing.T) {
	p := jobDatabase(t)
	ctx := context.Background()
	exec := func(q string, args ...any) {
		if _, err := p.Exec(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO catalog(id,source,source_id,kind,title,metadata,fetched_at) VALUES('tmdb:tv:992001','tmdb','992001','tv','Fixture','{"":{"number_of_episodes":2,"status":"Ended","seasons":[{"season_number":1},{"season_number":2}]}}',now()-interval '8 days')`)
	for n := 1; n <= 2; n++ {
		exec(`INSERT INTO episodes(id,catalog_id,source,source_id,season,number,air_date) VALUES($1,'tmdb:tv:992001','tmdb',$2,$3,1,'2020-01-01')`, fmt.Sprintf("tmdb:episode:%d", 992010+n), fmt.Sprint(992010+n), n)
	}
	exec(`INSERT INTO jobs(id,kind,payload) VALUES('metadata:tmdb:tv:992001','metadata','{"source":"tmdb","kind":"tv","source_id":"992001"}')`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tv/992001/season/1" {
			_, _ = fmt.Fprint(w, `{"episodes":[{"id":992011,"episode_number":1,"air_date":"2020-01-01"}]}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"id":992001,"name":"Fixture","overview":"OK","number_of_episodes":1,"status":"Ended","seasons":[{"season_number":1}]}`)
	}))
	defer server.Close()
	provider := catalog.New("fixture")
	provider.BaseURL = server.URL
	r := Runner{Cache: &catalog.Cache{DB: p, Provider: provider}}
	if err := r.one(ctx); err != nil {
		t.Fatal(err)
	}
	var active, jobs int
	_ = p.QueryRow(ctx, `SELECT count(*) FROM episodes WHERE catalog_id='tmdb:tv:992001' AND NOT retired`).Scan(&active)
	_ = p.QueryRow(ctx, `SELECT count(*) FROM jobs`).Scan(&jobs)
	if active != 1 || jobs != 0 {
		t.Fatalf("probe changed: active %d jobs %d", active, jobs)
	}
	t.Log("Successful refresh retires the removed season")
}
