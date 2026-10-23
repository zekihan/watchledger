package httpapi

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
)

type statisticEvent struct {
	EpisodeID                             *string
	CatalogID, Source, Kind, Day, Runtime string
	Genres                                []string
}
type statisticCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}
type statisticMonth struct {
	Month string `json:"month"`
	Count int    `json:"count"`
}
type statisticsResult struct {
	Movies                  int              `json:"movies"`
	Episodes                int              `json:"episodes"`
	EstimatedMinutes        int              `json:"estimated_minutes"`
	UnknownRuntime          int              `json:"unknown_runtime"`
	PartialNativeEpisodes   int              `json:"partial_native_episodes"`
	PartialSourceRecords    int              `json:"partial_source_records"`
	UnresolvedLinkedRecords int              `json:"unresolved_linked_records"`
	DerivedDateUnits        int              `json:"derived_date_units"`
	Monthly                 []statisticMonth `json:"monthly"`
	Genres                  []statisticCount `json:"genres"`
}

func buildStatistics(ctx context.Context, db queryReader, uid, zone string) (statisticsResult, error) {
	out := statisticsResult{Monthly: []statisticMonth{}, Genres: []statisticCount{}}
	snapshot, err := loadEffective(ctx, db, uid, "")
	if err != nil {
		return out, err
	}
	rows, err := db.Query(ctx, `SELECT h.episode_id,h.catalog_id,c.source,c.kind,coalesce(h.watched_date::text,to_char(h.watched_time AT TIME ZONE $2,'YYYY-MM-DD'),''),coalesce(CASE WHEN h.episode_id IS NULL THEN catalog_metadata_variant(c.metadata,u.settings)->>'runtime' ELSE e.metadata->>'runtime' END,''),coalesce(catalog_metadata_variant(c.metadata,u.settings)->'genres','[]') FROM history h JOIN catalog c ON c.id=h.catalog_id LEFT JOIN episodes e ON e.id=h.episode_id JOIN users u ON u.id=h.user_id WHERE h.user_id=$1 ORDER BY h.id`, uid, zone)
	if err != nil {
		return out, err
	}
	records := []statisticEvent{}
	byEpisode := map[string]statisticEvent{}
	for rows.Next() {
		var event statisticEvent
		var genres []byte
		if err = rows.Scan(&event.EpisodeID, &event.CatalogID, &event.Source, &event.Kind, &event.Day, &event.Runtime, &genres); err != nil {
			rows.Close()
			return out, err
		}
		event.Genres = statisticGenres(genres)
		records = append(records, event)
		if event.EpisodeID != nil {
			byEpisode[*event.EpisodeID] = event
		}
	}
	rows.Close()
	if rows.Err() != nil {
		return out, rows.Err()
	}
	ids := []string{}
	for id := range snapshot.States {
		ids = append(ids, id)
	}
	native := map[string]statisticEvent{}
	rows, err = db.Query(ctx, `SELECT e.id,e.catalog_id,coalesce(e.metadata->>'runtime',''),coalesce(catalog_metadata_variant(c.metadata,u.settings)->'genres','[]') FROM episodes e JOIN catalog c ON c.id=e.catalog_id JOIN users u ON u.id=$1 WHERE e.id=ANY($2::text[]) AND c.source='anidb' AND c.kind='anime'`, uid, ids)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var event statisticEvent
		var genres []byte
		if err = rows.Scan(&event.EpisodeID, &event.CatalogID, &event.Runtime, &genres); err != nil {
			rows.Close()
			return out, err
		}
		event.Source = "anidb"
		event.Kind = "anime"
		event.Genres = statisticGenres(genres)
		native[*event.EpisodeID] = event
	}
	rows.Close()
	if rows.Err() != nil {
		return out, rows.Err()
	}
	canonical := map[string]bool{}
	partial := map[string]bool{}
	events := []statisticEvent{}
	for _, record := range records {
		if record.EpisodeID == nil {
			events = append(events, record)
			continue
		}
		id := *record.EpisodeID
		if _, ok := native[id]; ok {
			events = append(events, record)
			canonical[id] = true
			continue
		}
		state := snapshot.States[id]
		safe := !state.Blocked && len(state.Sources) > 0
		for _, peer := range state.Sources {
			_, ok := native[peer]
			safe = safe && ok && !snapshot.States[peer].Blocked
		}
		if !safe {
			events = append(events, record)
			if len(state.Sources) > 0 {
				out.UnresolvedLinkedRecords++
			}
			continue
		}
		hasPartial := false
		for _, peer := range state.Sources {
			ns := snapshot.States[peer]
			if ns.Direct || ns.Derived {
				canonical[peer] = true
			} else if !ns.ExplicitUnwatch {
				partial[peer] = true
				hasPartial = true
			}
		}
		if hasPartial {
			out.PartialSourceRecords++
		}
	}
	// Direct native facts always win, including their deliberately unknown dates.
	// Derived native completion dates exist only when every required recorded
	// source part has a known local day. They are aggregate estimates, not history.
	for id := range canonical {
		if _, direct := byEpisode[id]; direct {
			continue
		}
		event := native[id]
		known := true
		for _, peer := range snapshot.States[id].Sources {
			source, ok := byEpisode[peer]
			if !ok || source.Day == "" {
				known = false
				break
			}
			if source.Day > event.Day {
				event.Day = source.Day
			}
		}
		if !known {
			event.Day = ""
		} else {
			out.DerivedDateUnits++
		}
		events = append(events, event)
	}
	out.PartialNativeEpisodes = len(partial)
	months, genres := map[string]int{}, map[string]int{}
	for _, event := range events {
		if event.EpisodeID == nil {
			out.Movies++
		} else {
			out.Episodes++
		}
		minutes, err := strconv.Atoi(event.Runtime)
		if err != nil || minutes <= 0 || minutes > 99999 {
			out.UnknownRuntime++
		} else {
			out.EstimatedMinutes += minutes
		}
		if len(event.Day) >= 7 {
			months[event.Day[:7]]++
		}
		for _, genre := range event.Genres {
			genres[genre]++
		}
	}
	for month, count := range months {
		out.Monthly = append(out.Monthly, statisticMonth{Month: month, Count: count})
	}
	sort.Slice(out.Monthly, func(i, j int) bool { return out.Monthly[i].Month < out.Monthly[j].Month })
	for name, count := range genres {
		out.Genres = append(out.Genres, statisticCount{Name: name, Count: count})
	}
	sort.Slice(out.Genres, func(i, j int) bool {
		if out.Genres[i].Count != out.Genres[j].Count {
			return out.Genres[i].Count > out.Genres[j].Count
		}
		return out.Genres[i].Name < out.Genres[j].Name
	})
	return out, nil
}
func statisticGenres(raw []byte) []string {
	var values []struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(raw, &values) != nil {
		return nil
	}
	out := []string{}
	seen := map[string]bool{}
	for _, v := range values {
		if v.Name != "" && !seen[v.Name] {
			out = append(out, v.Name)
			seen[v.Name] = true
		}
	}
	return out
}
