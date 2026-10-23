// Package portable validates versioned per-user backups without importing authority.
package portable

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/zekihan/watchledger/internal/smart"
	"github.com/zekihan/watchledger/internal/tracking"
	"regexp"
	"strconv"
	"time"
)

type Catalog struct {
	ID        string          `json:"id"`
	Source    string          `json:"source"`
	SourceID  string          `json:"source_id"`
	Kind      string          `json:"kind"`
	Title     string          `json:"title"`
	Metadata  json.RawMessage `json:"metadata"`
	FetchedAt time.Time       `json:"fetched_at"`
}
type Episode struct {
	ID        string          `json:"id"`
	CatalogID string          `json:"catalog_id"`
	Source    string          `json:"source"`
	SourceID  string          `json:"source_id"`
	Season    int             `json:"season"`
	Number    int             `json:"number"`
	Kind      string          `json:"kind"`
	Metadata  json.RawMessage `json:"metadata"`
	AirDate   *string         `json:"air_date"`
	AirTime   *time.Time      `json:"air_time"`
	Retired   bool            `json:"retired"`
}
type Status struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Base string `json:"base"`
	Used bool   `json:"used"`
}
type Library struct {
	CatalogID string    `json:"catalog_id"`
	Base      string    `json:"base"`
	StatusID  *string   `json:"status_id"`
	Rating    *int      `json:"rating"`
	Position  int       `json:"position"`
	AnimeView *string   `json:"anime_view"`
	AddedAt   time.Time `json:"added_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type History struct {
	ID          string          `json:"id"`
	CatalogID   string          `json:"catalog_id"`
	EpisodeID   *string         `json:"episode_id"`
	WatchedDate *string         `json:"watched_date"`
	WatchedTime *time.Time      `json:"watched_time"`
	Provenance  json.RawMessage `json:"provenance"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}
type List struct {
	Source   *string     `json:"source,omitempty"`
	SourceID *string     `json:"source_id,omitempty"`
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Kind     string      `json:"kind"`
	Rule     *smart.Rule `json:"rule"`
	Sort     string      `json:"sort"`
	Members  []string    `json:"members"`
}
type Tombstone struct {
	CatalogID  string    `json:"catalog_id"`
	EpisodeKey string    `json:"episode_key"`
	DeletedAt  time.Time `json:"deleted_at"`
}
type SIMKLLink struct {
	SourceKey string `json:"source_key"`
	CatalogID string `json:"catalog_id"`
	EpisodeID string `json:"episode_id"`
}

var sourceEpisodeKey = regexp.MustCompile(`^(shows|anime):[1-9][0-9]*:s[0-9]+:e[1-9][0-9]*$`)

type ListSourceTombstone struct {
	Source    string    `json:"source"`
	SourceID  string    `json:"source_id"`
	DeletedAt time.Time `json:"deleted_at"`
}
type Snapshot struct {
	ListSourceTombstones []ListSourceTombstone `json:"list_source_tombstones,omitempty"`
	SIMKLLinks           []SIMKLLink           `json:"simkl_links,omitempty"`
	Tombstones           []Tombstone           `json:"tombstones"`
	Version              int                   `json:"version"`
	CreatedAt            time.Time             `json:"created_at"`
	Settings             json.RawMessage       `json:"settings"`
	Catalog              []Catalog             `json:"catalog"`
	Episodes             []Episode             `json:"episodes"`
	Statuses             []Status              `json:"statuses"`
	Library              []Library             `json:"library"`
	History              []History             `json:"history"`
	Lists                []List                `json:"lists"`
}

func (s Snapshot) Validate() error {
	if s.Version != 1 {
		return errors.New("unsupported backup version")
	}
	if len(s.Catalog) > 100000 || len(s.History) > 1000000 {
		return errors.New("backup exceeds record limit")
	}
	if len(s.Settings) > 0 {
		var settings map[string]json.RawMessage
		if json.Unmarshal(s.Settings, &settings) != nil || settings == nil {
			return errors.New("invalid settings object")
		}
		for k, v := range settings {
			switch k {
			case "language", "metadata_language", "country", "timezone", "anime_view", "anime_title":
				var value string
				if json.Unmarshal(v, &value) != nil || len(value) > 100 {
					return errors.New("invalid setting")
				}
				if k == "timezone" {
					if _, e := time.LoadLocation(value); e != nil {
						return e
					}
				}
				if k == "language" && value != "en" && value != "tr" {
					return errors.New("unsupported interface language")
				}
				if k == "anime_view" && value != "native" && value != "show" {
					return errors.New("invalid anime view")
				}
			case "spoilers":
				var value bool
				if json.Unmarshal(v, &value) != nil {
					return errors.New("invalid spoiler preference")
				}
			case "services":
				var value []int
				if json.Unmarshal(v, &value) != nil {
					return errors.New("invalid services")
				}
			default:
				return errors.New("unsupported settings field")
			}
		}
	}
	catalog, episodes, statuses, library := map[string]bool{}, map[string]string{}, map[string]string{}, map[string]bool{}
	for _, c := range s.Catalog {
		if c.ID == "" || c.Source == "" || c.SourceID == "" || c.Title == "" || catalog[c.ID] {
			return errors.New("invalid or duplicate catalog identity")
		}
		if c.Kind != "movie" && c.Kind != "tv" && c.Kind != "anime" {
			return errors.New("invalid catalog kind")
		}
		var variants map[string]json.RawMessage
		if json.Unmarshal(c.Metadata, &variants) != nil || variants == nil {
			return errors.New("invalid catalog metadata")
		}
		for _, v := range variants {
			var object map[string]json.RawMessage
			if json.Unmarshal(v, &object) != nil || object == nil {
				return errors.New("invalid metadata variant")
			}
		}
		catalog[c.ID] = true
	}
	for _, e := range s.Episodes {
		if e.ID == "" || e.Source == "" || e.SourceID == "" || !catalog[e.CatalogID] || episodes[e.ID] != "" {
			return errors.New("invalid episode reference")
		}
		d := tracking.Date{Timestamp: e.AirTime}
		if e.AirDate != nil {
			d.Day = *e.AirDate
		}
		if err := d.Validate(); err != nil {
			return err
		}
		var metadata map[string]json.RawMessage
		if len(e.Metadata) > 0 && (json.Unmarshal(e.Metadata, &metadata) != nil || metadata == nil) {
			return errors.New("invalid episode metadata")
		}
		episodes[e.ID] = e.CatalogID
	}
	for _, st := range s.Statuses {
		if st.ID == "" || st.Name == "" || !tracking.ValidBase(st.Base) || statuses[st.ID] != "" {
			return errors.New("invalid custom status")
		}
		statuses[st.ID] = st.Base
	}
	for _, l := range s.Library {
		if !catalog[l.CatalogID] || library[l.CatalogID] || !tracking.ValidBase(l.Base) {
			return errors.New("invalid library reference")
		}
		if l.Rating != nil && (*l.Rating < 1 || *l.Rating > 10) {
			return errors.New("invalid rating")
		}
		if l.StatusID != nil && statuses[*l.StatusID] != l.Base {
			return errors.New("status base mismatch")
		}
		library[l.CatalogID] = true
	}
	history := map[string]bool{}

	seenLinks := map[string]bool{}
	for _, link := range s.SIMKLLinks {
		if len(link.SourceKey) > 256 || !sourceEpisodeKey.MatchString(link.SourceKey) || seenLinks[link.SourceKey] || !library[link.CatalogID] || episodes[link.EpisodeID] != link.CatalogID {
			return errors.New("invalid or conflicting SIMKL episode link")
		}
		seenLinks[link.SourceKey] = true
	}
	for _, h := range s.History {
		if !library[h.CatalogID] {
			return errors.New("history without library title")
		}
		key := h.CatalogID
		if h.EpisodeID != nil {
			if episodes[*h.EpisodeID] != h.CatalogID {
				return errors.New("episode belongs to different title")
			}
			key += "/" + *h.EpisodeID
		}
		if history[key] {
			return errors.New("duplicate history")
		}
		history[key] = true
		d := tracking.Date{Timestamp: h.WatchedTime}
		if h.WatchedDate != nil {
			d.Day = *h.WatchedDate
		}
		if err := d.Validate(); err != nil {
			return err
		}
	}
	for _, t := range s.Tombstones {
		if !library[t.CatalogID] || (t.EpisodeKey != "" && episodes[t.EpisodeKey] != t.CatalogID) {
			return errors.New("invalid tombstone reference")
		}
		key := t.CatalogID
		if t.EpisodeKey != "" {
			key += "/" + t.EpisodeKey
		}
		if history[key] {
			return errors.New("history conflicts with explicit unwatch")
		}
	}
	graph := map[string][]string{}
	seenSources := map[string]bool{}
	for _, l := range s.Lists {
		if (l.Source == nil) != (l.SourceID == nil) {
			return errors.New("incomplete list source identity")
		}
		if l.Source != nil {
			number, err := strconv.ParseInt(*l.SourceID, 10, 64)
			if *l.Source != "simkl-custom-list" || err != nil || number < 1 || strconv.FormatInt(number, 10) != *l.SourceID || seenSources[*l.SourceID] {
				return errors.New("invalid or duplicate list source identity")
			}
			seenSources[*l.SourceID] = true
		}

		if l.ID == "" || l.Name == "" {
			return errors.New("invalid list")
		}
		if _, ok := graph[l.ID]; ok {
			return errors.New("duplicate list")
		}
		graph[l.ID] = nil
		if l.Kind == "smart" {
			if l.Rule == nil {
				return errors.New("missing smart rule")
			}
			if err := smart.Validate(*l.Rule); err != nil {
				return err
			}
			graph[l.ID] = smart.References(*l.Rule)
		} else if l.Kind != "manual" {
			return errors.New("invalid list kind")
		}
		for _, id := range l.Members {
			if !library[id] {
				return fmt.Errorf("list member %s is outside library", id)
			}
		}
	}

	deletedSources := map[string]bool{}
	for _, deleted := range s.ListSourceTombstones {
		number, err := strconv.ParseInt(deleted.SourceID, 10, 64)
		if deleted.Source != "simkl-custom-list" || err != nil || number < 1 || strconv.FormatInt(number, 10) != deleted.SourceID || deleted.DeletedAt.IsZero() || deletedSources[deleted.SourceID] || seenSources[deleted.SourceID] {
			return errors.New("invalid or contradictory list source deletion")
		}
		deletedSources[deleted.SourceID] = true
	}
	return smart.Acyclic(graph)
}
