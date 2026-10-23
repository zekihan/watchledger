package simkl

import (
	"strings"
	"testing"
)

func TestExplicitHistoryAndPrecision(t *testing.T) {
	b, e := Parse([]byte(`{"shows":[{"show":{"title":"Synthetic gaps","ids":{"simkl":12,"tmdb":"34"}},"status":"watching","user_rating":8,"watched_episodes_count":3,"seasons":[{"number":1,"episodes":[{"number":1,"watched_at":"2025-01-02"},{"number":3,"watched_at":null},{"number":5,"watched_at":"2025-01-03T21:30:00+03:00"}]}]}],"movies":[{"movie":{"title":"Synthetic movie","ids":{"simkl":1,"tmdb":2}},"status":"completed"}],"anime":[{"show":{"title":"Native entry","ids":{"simkl":90,"tmdb":34}},"status":"completed","watched_episodes_count":12}]}`))
	if e != nil {
		t.Fatal(e)
	}
	if len(b.Items) != 3 {
		t.Fatal("items", len(b.Items))
	}
	movie, show, anime := b.Items[0], b.Items[1], b.Items[2]
	if !movie.MovieWatched || movie.MovieDate.Day != "" || movie.MovieDate.Timestamp != nil {
		t.Fatal("completion fabricated date")
	}
	if show.Target() != "tmdb:tv:34" || len(show.Episodes) != 3 || show.Episodes[1].Number != 3 {
		t.Fatal("gap changed", show)
	}
	if show.Episodes[0].Date.Day != "2025-01-02" || show.Episodes[0].Date.Timestamp != nil || show.Episodes[1].Date.Timestamp != nil || show.Episodes[2].Date.Timestamp.Format("15:04Z07:00") != "21:30+03:00" {
		t.Fatal("precision lost")
	}
	if anime.Target() != "" || len(anime.Episodes) != 0 {
		t.Fatal("unsafe anime merge or inferred history")
	}
	if b.CustomListsPresent {
		t.Fatal("invented lists")
	}
}
func TestInvalidDocuments(t *testing.T) {
	for _, raw := range []string{`null`, `[]`, `{}`, `{"shows":null}`, `{"shows":{}}`, `{"movies":[]} {}`, `{"movies":[]} trailing`} {
		t.Run(raw, func(t *testing.T) {
			if _, e := Parse([]byte(raw)); e == nil {
				t.Fatal("accepted invalid document")
			}
		})
	}
}
func TestInvalidItemsRemainReviewable(t *testing.T) {
	b, e := Parse([]byte(`{"movies":[null,{"movie":{"title":"A","ids":{"simkl":1}},"status":"completed","last_watched_at":"not-a-date","user_rating":11},{"movie":{"title":"B","ids":{"simkl":1}},"status":"new-status"}],"custom_lists":{"unsupported":true}}`))
	if e != nil || len(b.Items) != 3 {
		t.Fatal(e)
	}
	for _, i := range b.Items {
		if len(i.Issues) == 0 {
			t.Fatal("missing review issue", i.Key)
		}
	}
	if !b.CustomListsPresent || !strings.Contains(strings.Join(b.Warnings, ","), "unsupported_section:custom_lists") {
		t.Fatal("unsupported data not flagged")
	}
}
func TestEpisodeIdentityIsNeverInferred(t *testing.T) {
	b, e := Parse([]byte(`{"shows":[{"show":{"title":"A","ids":{"simkl":1,"tmdb":2}},"status":"completed","watched_episodes_count":10,"seasons":[{"number":0,"episodes":[{"number":1}]},{"number":1,"episodes":[{"number":2},{"number":2},{"number":-1}]}]}]}`))
	if e != nil {
		t.Fatal(e)
	}
	i := b.Items[0]
	if len(i.Episodes) != 2 || len(i.Issues) != 2 || !strings.Contains(strings.Join(i.Warnings, ","), "episode_identity_review_required") {
		t.Fatal(i)
	}
}
