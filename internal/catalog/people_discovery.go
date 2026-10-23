package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
)

// PeopleTV uses a paged person-credit candidate set; result counts describe only
// the inspected page, not exhaustive global TV discovery after local filters.
func (c *Cache) PeopleTV(ctx context.Context, person string, in url.Values) (json.RawMessage, error) {
	if _, e := Identity("person", person); e != nil {
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
	var data map[string][]struct {
		ID int `json:"id"`
	}
	if e = json.Unmarshal(raw, &data); e != nil {
		return nil, e
	}
	candidates := []int{}
	seen := map[int]bool{}
	for _, item := range data[credit] {
		if !seen[item.ID] {
			candidates = append(candidates, item.ID)
			seen[item.ID] = true
		}
	}
	page, _ := strconv.Atoi(in.Get("page"))
	if page < 1 {
		page = 1
	}
	start := min((page-1)*20, len(candidates))
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
		if v := in.Get("with_genres"); v != "" {
			found := false
			for _, g := range m.Genres {
				found = found || strconv.Itoa(g.ID) == v
			}
			if !found {
				continue
			}
		}
		low, _ := strconv.Atoi(in.Get("with_runtime.gte"))
		high, _ := strconv.Atoi(in.Get("with_runtime.lte"))
		if low > 0 || high > 0 {
			if len(m.Runtime) == 0 {
				continue
			}
			runtime := m.Runtime[0]
			if runtime < low || (high > 0 && runtime > high) {
				continue
			}
		}
		if provider := in.Get("with_watch_providers"); provider != "" {
			offers, err := c.Availability(ctx, "tv", strconv.Itoa(id), in.Get("watch_region"), false)
			if err != nil {
				failures++
				continue
			}
			var av struct {
				Flatrate []struct {
					ID int `json:"provider_id"`
				} `json:"flatrate"`
			}
			if json.Unmarshal(offers.Offers, &av) != nil {
				continue
			}
			found := false
			for _, p := range av.Flatrate {
				found = found || strconv.Itoa(p.ID) == provider
			}
			if !found {
				continue
			}
		}
		results = append(results, d.Metadata)
	}
	return json.Marshal(map[string]interface{}{"results": results, "page": page, "total_pages": max(1, (len(candidates)+19)/20), "candidate_count": len(candidates), "inspected_candidates": end - start, "enrichment_failures": failures, "exhaustive": false})
}
