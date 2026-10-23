// Package simkl reads the observed downloadable JSON backup without inferring
// episode identities or watched dates from summary counters.
package simkl

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/zekihan/watchledger/internal/tracking"
)

type Episode struct {
	Key    string        `json:"key"`
	Season int           `json:"season"`
	Number int           `json:"number"`
	Date   tracking.Date `json:"date"`
}
type Item struct {
	Key          string            `json:"key"`
	Kind         string            `json:"kind"`
	Title        string            `json:"title"`
	IDs          map[string]string `json:"ids"`
	Base         string            `json:"base"`
	Rating       *int              `json:"rating"`
	AddedAt      *time.Time        `json:"added_at,omitempty"`
	MovieWatched bool              `json:"movie_watched"`
	MovieDate    tracking.Date     `json:"movie_date"`
	Episodes     []Episode         `json:"episodes"`
	Issues       []string          `json:"issues"`
	Warnings     []string          `json:"warnings"`
}
type Backup struct {
	Items              []Item   `json:"items"`
	Warnings           []string `json:"warnings"`
	CustomListsPresent bool     `json:"custom_lists_present"`
}
type sourceTitle struct {
	Title string                     `json:"title"`
	IDs   map[string]json.RawMessage `json:"ids"`
}
type sourceItem struct {
	Show    sourceTitle `json:"show"`
	Movie   sourceTitle `json:"movie"`
	Status  string      `json:"status"`
	Rating  *int        `json:"user_rating"`
	Added   string      `json:"added_to_watchlist_at"`
	Last    string      `json:"last_watched_at"`
	Watched int         `json:"watched_episodes_count"`
	Rewatch bool        `json:"is_rewatch"`
	Seasons []struct {
		Number   int `json:"number"`
		Episodes []struct {
			Number  int    `json:"number"`
			Watched string `json:"watched_at"`
		} `json:"episodes"`
	} `json:"seasons"`
}

// Parse retains all original material at the transport layer. Its normalized
// items contain only explicitly recorded history; completed counts are not dates.
func Parse(raw []byte) (Backup, error) {
	out := Backup{Items: []Item{}, Warnings: []string{}}
	dec := json.NewDecoder(bytes.NewReader(raw))
	var root map[string]json.RawMessage
	if err := dec.Decode(&root); err != nil || root == nil {
		return out, errors.New("expected a SIMKL JSON object")
	}
	var extra interface{}
	if err := dec.Decode(&extra); err != io.EOF {
		return out, errors.New("expected one JSON document")
	}
	recognized := false
	seen := map[string]int{}
	for _, group := range []string{"movies", "shows", "anime"} {
		section, ok := root[group]
		if !ok {
			continue
		}
		recognized = true
		if bytes.Equal(bytes.TrimSpace(section), []byte("null")) {
			return out, fmt.Errorf("%s must be an array", group)
		}
		var entries []json.RawMessage
		if err := json.Unmarshal(section, &entries); err != nil {
			return out, fmt.Errorf("%s must be an array", group)
		}
		for index, rawItem := range entries {
			item := Item{Key: fmt.Sprintf("%s:%d", group, index), Kind: group, IDs: map[string]string{}, Episodes: []Episode{}, Issues: []string{}, Warnings: []string{}}
			var src sourceItem
			if err := json.Unmarshal(rawItem, &src); err != nil || bytes.Equal(bytes.TrimSpace(rawItem), []byte("null")) {
				item.Issues = append(item.Issues, "invalid_item")
				out.Items = append(out.Items, item)
				continue
			}
			title := src.Show
			if group == "movies" {
				title = src.Movie
			}
			item.Title = title.Title
			for provider, value := range title.IDs {
				var s string
				if json.Unmarshal(value, &s) != nil {
					var n json.Number
					if json.Unmarshal(value, &n) != nil {
						continue
					}
					s = n.String()
				}
				if s != "" {
					item.IDs[provider] = s
				}
			}
			if id, ok := positiveID(item.IDs["simkl"]); ok {
				item.Key = group + ":" + id
				if previous, exists := seen[item.Key]; exists {
					item.Issues = append(item.Issues, "duplicate_source_identity")
					out.Items[previous].Issues = append(out.Items[previous].Issues, "duplicate_source_identity")
				} else {
					seen[item.Key] = len(out.Items)
				}
			} else {
				item.Issues = append(item.Issues, "missing_simkl_identity")
			}
			if item.Title == "" {
				item.Issues = append(item.Issues, "missing_title")
			}
			bases := map[string]string{"plantowatch": "planned", "watching": "watching", "completed": "completed", "hold": "hold", "dropped": "dropped"}
			item.Base = bases[src.Status]
			if item.Base == "" {
				item.Issues = append(item.Issues, "unknown_status")
			}
			item.Rating = src.Rating
			if src.Rating != nil && (*src.Rating < 1 || *src.Rating > 10) {
				item.Issues = append(item.Issues, "invalid_rating")
			}
			if src.Added != "" {
				ts, err := time.Parse(time.RFC3339Nano, src.Added)
				if err == nil {
					item.AddedAt = &ts
				} else {
					item.Warnings = append(item.Warnings, "source_added_date_retained_in_archive")
				}
			}
			if src.Rewatch {
				item.Warnings = append(item.Warnings, "rewatch_retained_in_archive")
			}
			if group == "movies" {
				item.MovieWatched = src.Status == "completed" || src.Watched > 0 || src.Last != ""
				d, err := parseDate(src.Last)
				item.MovieDate = d
				if err != nil {
					item.Issues = append(item.Issues, "invalid_movie_watch_date")
				}
			} else {
				episodeSeen := map[string]bool{}
				for _, season := range src.Seasons {
					for _, ep := range season.Episodes {
						key := fmt.Sprintf("%s:s%d:e%d", item.Key, season.Number, ep.Number)
						if season.Number < 0 || ep.Number < 1 {
							item.Issues = append(item.Issues, "invalid_episode_number")
							continue
						}
						if episodeSeen[key] {
							item.Issues = append(item.Issues, "duplicate_episode_record")
							continue
						}
						episodeSeen[key] = true
						d, err := parseDate(ep.Watched)
						if err != nil {
							item.Issues = append(item.Issues, "invalid_episode_watch_date")
						}
						item.Episodes = append(item.Episodes, Episode{Key: key, Season: season.Number, Number: ep.Number, Date: d})
					}
				}
				if src.Watched != len(item.Episodes) {
					item.Warnings = append(item.Warnings, "summary_count_differs_from_explicit_history")
				}
				if src.Status == "completed" && len(item.Episodes) == 0 {
					item.Warnings = append(item.Warnings, "completed_without_episode_records")
				}
				// SIMKL season/number pairs do not establish TMDB or AniDB episode identity.
				if len(item.Episodes) > 0 {
					item.Warnings = append(item.Warnings, "episode_identity_review_required")
				}
			}
			out.Items = append(out.Items, item)
		}
	}
	if !recognized {
		return out, errors.New("no movies, shows, or anime backup sections found")
	}
	keys := make([]string, 0, len(root))
	for key := range root {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if key != "movies" && key != "shows" && key != "anime" {
			out.Warnings = append(out.Warnings, "unsupported_section:"+key)
		}
		if strings.Contains(strings.ToLower(key), "list") {
			out.CustomListsPresent = true
		}
	}
	if !out.CustomListsPresent {
		out.Warnings = append(out.Warnings, "custom_lists_absent_from_backup")
	}
	return out, nil
}
func parseDate(value string) (tracking.Date, error) {
	if value == "" {
		return tracking.Date{}, nil
	}
	if len(value) == 10 {
		d := tracking.Date{Day: value}
		return d, d.Validate()
	}
	ts, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return tracking.Date{}, err
	}
	return tracking.Date{Timestamp: &ts}, nil
}
func positiveID(value string) (string, bool) {
	n, e := strconv.ParseInt(value, 10, 64)
	return strconv.FormatInt(n, 10), e == nil && n > 0
}

// Target returns a title identity only, never an inferred episode mapping.
func (i Item) Target() string {
	if i.Kind == "movies" || i.Kind == "shows" {
		if id, ok := positiveID(i.IDs["tmdb"]); ok {
			kind := "movie"
			if i.Kind == "shows" {
				kind = "tv"
			}
			return "tmdb:" + kind + ":" + id
		}
	}
	if i.Kind == "anime" {
		if id, ok := positiveID(i.IDs["anidb"]); ok {
			return "anidb:anime:" + id
		}
	}
	return ""
}
