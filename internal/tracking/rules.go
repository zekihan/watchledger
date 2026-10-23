// Package tracking defines source-independent viewing-history rules.
package tracking

import (
	"errors"
	"sort"
	"time"
)

type Episode struct {
	ID             string
	Season, Number int
	Special        bool
	AirDate        string
	AirTime        *time.Time
	Watched        bool
}
type Date struct {
	Day       string     `json:"day,omitempty"`
	Timestamp *time.Time `json:"timestamp,omitempty"`
}

func (d Date) Validate() error {
	if d.Day != "" && d.Timestamp != nil {
		return errors.New("choose date or timestamp, not both")
	}
	if d.Day != "" {
		_, e := time.Parse("2006-01-02", d.Day)
		return e
	}
	return nil
}
func (e Episode) Aired(now time.Time) bool {
	if e.AirTime != nil {
		return !e.AirTime.After(now)
	}
	return e.AirDate != "" && e.AirDate <= now.Format("2006-01-02")
}
func Earlier(episodes []Episode, selected Episode, now time.Time) []Episode {
	out := []Episode{}
	for _, ep := range episodes {
		if !ep.Special && !ep.Watched && ep.Aired(now) && (ep.Season < selected.Season || (ep.Season == selected.Season && ep.Number < selected.Number)) {
			out = append(out, ep)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Season != out[j].Season {
			return out[i].Season < out[j].Season
		}
		return out[i].Number < out[j].Number
	})
	return out
}
func Next(episodes []Episode, now time.Time) *Episode {
	out := []Episode{}
	for _, ep := range episodes {
		if !ep.Special && !ep.Watched && ep.Aired(now) {
			out = append(out, ep)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Season != out[j].Season {
			return out[i].Season < out[j].Season
		}
		return out[i].Number < out[j].Number
	})
	if len(out) == 0 {
		return nil
	}
	return &out[0]
}
func Destination(ended, completeCatalog bool, episodes []Episode) string {
	regular := 0
	all := true
	for _, e := range episodes {
		if !e.Special {
			regular++
			all = all && e.Watched
		}
	}
	if ended && completeCatalog && regular > 0 && all {
		return "completed"
	}
	return "watching"
}
func ValidBase(s string) bool {
	return s == "planned" || s == "watching" || s == "completed" || s == "hold" || s == "dropped"
}
