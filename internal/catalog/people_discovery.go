package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// PeopleTV uses a paged person-credit candidate set; result counts describe only
// the inspected page, not exhaustive global TV discovery after local filters.
func (c *Cache) PeopleTV(ctx context.Context, person string, in url.Values) (json.RawMessage, error) {
	if _, e := Identity("person", person); e != nil {
		return nil, e
	}
	genres, e := parseDiscoverySet(in.Get("with_genres"), positiveID)
	if e != nil {
		return nil, e
	}
	excluded, e := parseDiscoverySet(in.Get("without_genres"), positiveID)
	if e != nil {
		return nil, e
	}
	order := in.Get("sort_by")
	switch order {
	case "", "popularity.asc", "popularity.desc", "vote_average.asc", "vote_average.desc", "vote_count.asc", "vote_count.desc", "first_air_date.asc", "first_air_date.desc", "name.asc", "name.desc", "original_name.asc", "original_name.desc":
	default:
		return nil, errors.New("unsupported TV people sorting")
	}
	providers, e := parseDiscoverySet(in.Get("with_watch_providers"), positiveID)
	if e != nil {
		return nil, e
	}
	monetization, e := parseDiscoverySet(in.Get("with_watch_monetization_types"), func(value string) bool {
		return value == "flatrate" || value == "rent" || value == "buy" || value == "free" || value == "ads"
	})
	if e != nil {
		return nil, e
	}
	credit := in.Get("credit")
	if credit == "" {
		credit = "cast"
	}
	if credit != "cast" && credit != "crew" {
		return nil, errors.New("invalid credit type")
	}
	raw, e := c.Provider.Get(ctx, "/person/"+person+"/tv_credits", url.Values{"language": {in.Get("language")}})
	if e != nil {
		return nil, e
	}
	type creditEntry struct {
		ID           int     `json:"id"`
		Popularity   float64 `json:"popularity"`
		Score        float64 `json:"vote_average"`
		Votes        int     `json:"vote_count"`
		Date         string  `json:"first_air_date"`
		Name         string  `json:"name"`
		OriginalName string  `json:"original_name"`
	}
	var data struct {
		Cast []creditEntry `json:"cast"`
		Crew []creditEntry `json:"crew"`
	}
	if e = json.Unmarshal(raw, &data); e != nil {
		return nil, e
	}
	candidates := []int{}
	seen := map[int]bool{}
	entries := data.Cast
	if credit == "crew" {
		entries = data.Crew
	}
	if order != "" {
		sort.SliceStable(entries, func(i, j int) bool {
			a, b := entries[i], entries[j]
			if strings.HasSuffix(order, ".desc") {
				a, b = b, a
			}
			switch strings.Split(order, ".")[0] {
			case "popularity":
				if a.Popularity != b.Popularity {
					return a.Popularity < b.Popularity
				}
			case "vote_average":
				if a.Score != b.Score {
					return a.Score < b.Score
				}
			case "vote_count":
				if a.Votes != b.Votes {
					return a.Votes < b.Votes
				}
			case "first_air_date":
				if a.Date != b.Date {
					return a.Date < b.Date
				}
			case "name":
				if a.Name != b.Name {
					return a.Name < b.Name
				}
			case "original_name":
				if a.OriginalName != b.OriginalName {
					return a.OriginalName < b.OriginalName
				}
			}
			return entries[i].ID < entries[j].ID
		})
	}
	for _, item := range entries {
		if !seen[item.ID] {
			candidates = append(candidates, item.ID)
			seen[item.ID] = true
		}
	}
	page, _ := strconv.Atoi(in.Get("page"))
	if page < 1 {
		page = 1
	}
	start := len(candidates)
	if page <= max(1, (len(candidates)+19)/20) {
		start = min((page-1)*20, len(candidates))
	}
	end := min(start+20, len(candidates))
	results := []json.RawMessage{}
	failures := 0
	for _, id := range candidates[start:end] {
		d, err := c.Detail(ctx, "tv", strconv.Itoa(id), in.Get("language"), false)
		if err != nil {
			failures++
			continue
		}
		var m struct {
			Genres []struct {
				ID int `json:"id"`
			} `json:"genres"`
			Date     string  `json:"first_air_date"`
			Language string  `json:"original_language"`
			Votes    int     `json:"vote_count"`
			Score    float64 `json:"vote_average"`
			Runtime  []int   `json:"episode_run_time"`
		}
		if json.Unmarshal(d.Metadata, &m) != nil {
			failures++
			continue
		}
		if v := in.Get("with_original_language"); v != "" && v != m.Language {
			continue
		}
		if v := in.Get("first_air_date_year"); v != "" && (len(m.Date) < 4 || v != m.Date[:4]) {
			continue
		}
		if v := in.Get("first_air_date.gte"); v != "" && (m.Date == "" || m.Date < v) {
			continue
		}
		if v := in.Get("first_air_date.lte"); v != "" && (m.Date == "" || m.Date > v) {
			continue
		}
		minVotes, _ := strconv.Atoi(in.Get("vote_count.gte"))
		minScore, _ := strconv.ParseFloat(in.Get("vote_average.gte"), 64)
		if m.Votes < minVotes || m.Score < minScore {
			continue
		}
		availableGenres := map[string]bool{}
		for _, g := range m.Genres {
			availableGenres[strconv.Itoa(g.ID)] = true
		}
		excludedGenre := false
		for _, genre := range excluded.values {
			excludedGenre = excludedGenre || availableGenres[genre]
		}
		if excludedGenre {
			continue
		}
		if !genres.matches(availableGenres) {
			continue
		}
		low, _ := strconv.Atoi(in.Get("with_runtime.gte"))
		high, _ := strconv.Atoi(in.Get("with_runtime.lte"))
		if low > 0 || high > 0 {
			if len(m.Runtime) == 0 {
				continue
			}
			runtime := m.Runtime[0]
			if runtime <= 0 || runtime < low || (high > 0 && runtime > high) {
				continue
			}
		}
		if len(providers.values) > 0 || len(monetization.values) > 0 {
			offers, err := c.Availability(ctx, "tv", strconv.Itoa(id), in.Get("watch_region"), false)
			if err != nil {
				failures++
				continue
			}
			var av map[string][]struct {
				ID int `json:"provider_id"`
			}
			// Ignore the availability link string; decode only offer arrays.
			var fields map[string]json.RawMessage
			if json.Unmarshal(offers.Offers, &fields) != nil {
				failures++
				continue
			}
			av = make(map[string][]struct {
				ID int `json:"provider_id"`
			})
			for _, category := range []string{"flatrate", "rent", "buy", "free", "ads"} {
				if raw := fields[category]; len(raw) > 0 {
					var items []struct {
						ID int `json:"provider_id"`
					}
					if json.Unmarshal(raw, &items) != nil {
						continue
					}
					av[category] = items
				}
			}
			availableProviders := map[string]bool{}
			categories := map[string]bool{}
			for category, items := range av {
				if len(items) > 0 {
					categories[category] = true
				}
				if len(monetization.values) > 0 && !monetization.contains(category) {
					continue
				}
				for _, item := range items {
					availableProviders[strconv.Itoa(item.ID)] = true
				}
			}
			if !monetization.matches(categories) || !providers.matches(availableProviders) {
				continue
			}
		}
		results = append(results, d.Metadata)
	}
	return json.Marshal(map[string]interface{}{"results": results, "page": page, "total_pages": max(1, (len(candidates)+19)/20), "candidate_count": len(candidates), "inspected_candidates": end - start, "enrichment_failures": failures, "exhaustive": false})
}

// Mirror documented simple AND (comma) / OR (pipe) filters. Mixed expressions
// are rejected rather than inventing undocumented precedence.
type discoverySet struct {
	values []string
	any    bool
}

func positiveID(value string) bool {
	id, err := strconv.Atoi(value)
	return err == nil && id > 0 && strconv.Itoa(id) == value
}
func parseDiscoverySet(raw string, valid func(string) bool) (discoverySet, error) {
	out := discoverySet{}
	if raw == "" {
		return out, nil
	}
	if len(raw) > 1000 || strings.Contains(raw, ",") && strings.Contains(raw, "|") {
		return out, errors.New("invalid discovery filter")
	}
	out.any = strings.Contains(raw, "|")
	separator := ","
	if out.any {
		separator = "|"
	}
	out.values = strings.Split(raw, separator)
	if len(out.values) > 100 {
		return out, errors.New("too many discovery filter values")
	}
	for _, value := range out.values {
		if !valid(value) {
			return out, errors.New("invalid discovery filter value")
		}
	}
	return out, nil
}
func (s discoverySet) contains(value string) bool {
	for _, item := range s.values {
		if value == item {
			return true
		}
	}
	return false
}
func (s discoverySet) matches(available map[string]bool) bool {
	if len(s.values) == 0 {
		return true
	}
	for _, item := range s.values {
		if s.any && available[item] {
			return true
		}
		if !s.any && !available[item] {
			return false
		}
	}
	return !s.any
}
