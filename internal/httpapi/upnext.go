package httpapi

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/zekihan/watchledger/internal/tracking"
)

type queryReader interface {
	queryRows
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}
type nextCard struct {
	CatalogID  string `json:"catalog_id"`
	Title      string `json:"title"`
	PosterPath string `json:"poster_path,omitempty"`
	EpisodeID  string `json:"episode_id"`
	Season     int    `json:"season"`
	Number     int    `json:"number"`
	Position   int    `json:"position"`
	ViewAnchor string `json:"view_anchor,omitempty"`
	Incomplete bool   `json:"incomplete,omitempty"`
}

func buildUpNext(ctx context.Context, db queryReader, uid, zone string) ([]nextCard, error) {
	snapshot, err := loadEffective(ctx, db, uid, "")
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(ctx, `SELECT l.catalog_id,catalog_display_title(c.kind,c.metadata,c.title,u.settings),coalesce(catalog_metadata_variant(c.metadata,u.settings)->>'poster_path',''),c.kind,l.position,coalesce(l.anime_view,u.settings->>'anime_view','native'),e.id,e.season,e.number FROM library l JOIN users u ON u.id=l.user_id JOIN catalog c ON c.id=l.catalog_id JOIN episodes e ON e.catalog_id=l.catalog_id WHERE l.user_id=$1 AND l.base='watching' AND NOT(c.source='anidb' AND coalesce(catalog_metadata_variant(c.metadata,u.settings)->>'anime_type','')='Movie') AND e.kind='regular' AND NOT e.retired AND (e.air_date<=(now() AT TIME ZONE $2)::date OR e.air_time<=now()) ORDER BY l.position,c.id,e.season,e.number,e.id`, uid, zone)
	if err != nil {
		return nil, err
	}
	type candidate struct {
		Card            nextCard
		Kind, Structure string
	}
	candidates := []candidate{}
	picked := map[string]bool{}
	targetIDs := []string{}
	for rows.Next() {
		var c candidate
		if err = rows.Scan(&c.Card.CatalogID, &c.Card.Title, &c.Card.PosterPath, &c.Kind, &c.Card.Position, &c.Structure, &c.Card.EpisodeID, &c.Card.Season, &c.Card.Number); err != nil {
			rows.Close()
			return nil, err
		}
		state := snapshot.States[c.Card.EpisodeID]
		project := c.Kind == "anime" && c.Structure == "show"
		if (!project && picked[c.Card.CatalogID]) || state.Direct || state.Derived {
			continue
		}
		picked[c.Card.CatalogID] = true
		candidates = append(candidates, c)
		if project && !state.Blocked {
			targetIDs = append(targetIDs, state.Sources...)
		}
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	type target struct {
		Card                      nextCard
		Kind, Source, EpisodeKind string
		Retired, Owned            bool
		AirDate                   string
		AirTime                   *time.Time
	}
	targets := map[string]target{}
	rows, err = db.Query(ctx, `SELECT e.id,e.catalog_id,catalog_display_title(c.kind,c.metadata,c.title,u.settings),coalesce(catalog_metadata_variant(c.metadata,u.settings)->>'poster_path',''),c.kind,e.source,e.kind,e.season,e.number,e.retired,coalesce(e.air_date::text,''),e.air_time,EXISTS(SELECT 1 FROM library l WHERE l.user_id=$1 AND l.catalog_id=c.id) FROM episodes e JOIN catalog c ON c.id=e.catalog_id JOIN users u ON u.id=$1 WHERE e.id=ANY($2::text[])`, uid, targetIDs)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var t target
		if err = rows.Scan(&t.Card.EpisodeID, &t.Card.CatalogID, &t.Card.Title, &t.Card.PosterPath, &t.Kind, &t.Source, &t.EpisodeKind, &t.Card.Season, &t.Card.Number, &t.Retired, &t.AirDate, &t.AirTime, &t.Owned); err != nil {
			rows.Close()
			return nil, err
		}
		targets[t.Card.EpisodeID] = t
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	location, _ := time.LoadLocation(zone)
	if location == nil {
		location = time.UTC
	}
	now := time.Now().In(location)
	cards := map[string]nextCard{}
	add := func(card nextCard) {
		old, exists := cards[card.CatalogID]
		if !exists {
			cards[card.CatalogID] = card
			return
		}
		position := min(old.Position, card.Position)
		incomplete := old.Incomplete || card.Incomplete
		owned := old.ViewAnchor == "" || card.ViewAnchor == ""
		// Prefer the earliest actual provider episode, with stable tie handling.
		if card.Season < old.Season || (card.Season == old.Season && card.Number < old.Number) {
			old = card
		}
		old.Position = position
		if owned {
			old.ViewAnchor = ""
		}
		old.Incomplete = incomplete
		cards[card.CatalogID] = old
	}
	for _, c := range candidates {
		if c.Kind != "anime" || c.Structure != "show" {
			add(c.Card)
			continue
		}
		state := snapshot.States[c.Card.EpisodeID]
		projected := []nextCard{}
		incomplete := state.Blocked
		if !state.Blocked {
			for _, peer := range state.Sources {
				t, ok := targets[peer]
				seen := snapshot.States[peer]
				aired := tracking.Episode{AirDate: t.AirDate, AirTime: t.AirTime}.Aired(now)
				if !ok || t.Kind != "tv" || t.EpisodeKind != "regular" || t.Source != "tmdb" || t.Retired || !aired {
					incomplete = true
					continue
				}
				if seen.Direct || seen.Derived {
					continue
				}
				card := t.Card
				card.Position = c.Card.Position
				if !t.Owned {
					card.ViewAnchor = c.Card.CatalogID
				}
				projected = append(projected, card)
			}
		}
		if incomplete || len(projected) == 0 {
			c.Card.Incomplete = true
			add(c.Card)
		} else {
			for _, card := range projected {
				add(card)
			}
		}
	}
	var raw []byte
	if err = db.QueryRow(ctx, "SELECT coalesce(settings->'up_next_order','[]'::jsonb) FROM users WHERE id=$1", uid).Scan(&raw); err != nil {
		return nil, err
	}
	order := []string{}
	_ = json.Unmarshal(raw, &order)
	rank := map[string]int{}
	for i, id := range order {
		rank[id] = i
	}
	out := make([]nextCard, 0, len(cards))
	for _, card := range cards {
		out = append(out, card)
	}
	sort.Slice(out, func(i, j int) bool {
		ri, oki := rank[out[i].CatalogID]
		rj, okj := rank[out[j].CatalogID]
		if oki != okj {
			return oki
		}
		if oki && ri != rj {
			return ri < rj
		}
		if out[i].Position != out[j].Position {
			return out[i].Position < out[j].Position
		}
		if out[i].Title != out[j].Title {
			return out[i].Title < out[j].Title
		}
		return out[i].CatalogID < out[j].CatalogID
	})
	return out, nil
}
