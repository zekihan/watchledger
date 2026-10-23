package tracking

import (
	"testing"
	"time"
)

func TestEarlierAndNext(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	eps := []Episode{{ID: "special", Season: 0, Number: 1, Special: true, AirDate: "2020-01-01"}, {ID: "old", Season: 1, Number: 3, AirDate: "2020-01-01"}, {ID: "future", Season: 2, Number: 1, AirDate: "2030-01-01"}, {ID: "unknown", Season: 1, Number: 1}, {ID: "watched", Season: 1, Number: 2, AirDate: "2020-01-01", Watched: true}}
	earlier := Earlier(eps, Episode{Season: 2, Number: 2}, now)
	if len(earlier) != 1 || earlier[0].ID != "old" {
		t.Fatal(earlier)
	}
	if Next(eps, now).ID != "old" {
		t.Fatal("wrong next")
	}
}
func TestCompletionAndPrecision(t *testing.T) {
	eps := []Episode{{Watched: true}, {Special: true}}
	if Destination(false, true, eps) != "watching" || Destination(true, false, eps) != "watching" || Destination(true, true, eps) != "completed" {
		t.Fatal("completion rule")
	}
	if (Date{Day: "2026-02-30"}).Validate() == nil {
		t.Fatal("invalid date")
	}
	if (Date{}).Validate() != nil {
		t.Fatal("unknown date must remain valid")
	}
}
