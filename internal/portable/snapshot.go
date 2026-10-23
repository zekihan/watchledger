// Package portable validates versioned per-user backups without importing authority.
package portable

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/zekihan/watchledger/internal/smart"
	"github.com/zekihan/watchledger/internal/tracking"
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
	ID      string      `json:"id"`
	Name    string      `json:"name"`
	Kind    string      `json:"kind"`
	Rule    *smart.Rule `json:"rule"`
	Sort    string      `json:"sort"`
	Members []string    `json:"members"`
}
type Snapshot struct {
	Version   int             `json:"version"`
	CreatedAt time.Time       `json:"created_at"`
	Settings  json.RawMessage `json:"settings"`
	Catalog   []Catalog       `json:"catalog"`
	Episodes  []Episode       `json:"episodes"`
	Statuses  []Status        `json:"statuses"`
	Library   []Library       `json:"library"`
	History   []History       `json:"history"`
	Lists     []List          `json:"lists"`
}

func (s Snapshot) Validate() error {
	if s.Version != 1 {
		return errors.New("unsupported backup version")
	}
	if len(s.Catalog) > 100000 || len(s.History) > 1000000 {
		return errors.New("backup exceeds record limit")
	}
	catalog, episodes, statuses, library := map[string]bool{}, map[string]string{}, map[string]string{}, map[string]bool{}
	for _, c := range s.Catalog {
		if c.ID == "" || c.Source == "" || c.SourceID == "" || c.Title == "" || catalog[c.ID] {
			return errors.New("invalid or duplicate catalog identity")
		}
		if c.Kind != "movie" && c.Kind != "tv" && c.Kind != "anime" {
			return errors.New("invalid catalog kind")
		}
		if !json.Valid(c.Metadata) {
			return errors.New("invalid catalog metadata")
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
	graph := map[string][]string{}
	for _, l := range s.Lists {
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
	return smart.Acyclic(graph)
}
