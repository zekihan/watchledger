package anidb

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"time"

	"github.com/jackc/pgx/v5"
)

type HotItem struct {
	ID         string  `xml:"id,attr"`
	Restricted bool    `xml:"restricted,attr"`
	Titles     []Title `xml:"title"`
	StartDate  string  `xml:"startdate"`
	Picture    string  `xml:"picture"`
	Rating     struct {
		Count int     `xml:"count,attr"`
		Value float64 `xml:",chardata"`
	} `xml:"ratings>permanent"`
}
type HotResult struct {
	Items     []HotItem
	FetchedAt time.Time
	Stale     bool
}

func parseHot(raw []byte) ([]HotItem, error) {
	var root struct {
		XMLName xml.Name
		Text    string    `xml:",chardata"`
		Items   []HotItem `xml:"anime"`
	}
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&root); err != nil {
		return nil, errors.New("invalid AniDB feed")
	}
	if root.XMLName.Local == "error" {
		return nil, &APIError{Message: root.Text}
	}
	if root.XMLName.Local != "hotanime" || len(root.Items) > 1000 {
		return nil, errors.New("invalid AniDB hot anime feed")
	}
	var extra interface{}
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("unexpected trailing feed XML")
	}
	seen := map[string]bool{}
	items := []HotItem{}
	for _, item := range root.Items {
		if _, ok := positiveIdentity(item.ID); !ok || len(item.Titles) == 0 || seen[item.ID] {
			return nil, errors.New("invalid feed anime identity")
		}
		seen[item.ID] = true
		if !item.Restricted {
			items = append(items, item)
		}
	}
	return items, nil
}
func (c *Cache) Hot(ctx context.Context) (HotResult, error) {
	result := HotResult{}
	var raw []byte
	err := c.DB.QueryRow(ctx, "SELECT document,fetched_at FROM anidb_feeds WHERE name='hotanime'").Scan(&raw, &result.FetchedAt)
	cached := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return result, err
	}
	if cached {
		result.Items, err = parseHot(raw)
		if err != nil {
			return result, err
		}
		if time.Since(result.FetchedAt) < 24*time.Hour {
			return result, nil
		}
	}
	fallback := func(err error) (HotResult, error) {
		if cached {
			result.Stale = true
			return result, nil
		}
		return result, err
	}
	if !c.Provider.Available() {
		return fallback(errors.New("AniDB requires a registered HTTP client name and version"))
	}
	if err = c.reserveDataset(ctx, "anidb:feed:hotanime"); err != nil {
		return fallback(err)
	}
	raw, err = c.Provider.request(ctx, "hotanime", nil)
	var items []HotItem
	if err == nil {
		items, err = parseHot(raw)
	}
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Banned() {
			_, _ = c.DB.Exec(ctx, "UPDATE provider_request_limits SET next_allowed_at=now()+interval '24 hours' WHERE key='anidb:global'")
		}
		return fallback(err)
	}
	if err = c.DB.QueryRow(ctx, "INSERT INTO anidb_feeds(name,document) VALUES('hotanime',$1) ON CONFLICT(name) DO UPDATE SET document=EXCLUDED.document,fetched_at=now() RETURNING fetched_at", raw).Scan(&result.FetchedAt); err != nil {
		return fallback(err)
	}
	result.Items = items
	return result, nil
}
