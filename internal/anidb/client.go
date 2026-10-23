// Package anidb implements the documented, on-demand HTTP XML API.
// AniDB content is CC-BY-NC-SA 4.0, not covered by Watchledger's MIT license.
package anidb

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const Endpoint = "http://api.anidb.net:9001/httpapi"

type Config struct {
	Client  string
	Version int
}
type Client struct {
	Config Config
	HTTP   *http.Client
	URL    string
}

func New(c Config) *Client {
	return &Client{Config: c, HTTP: &http.Client{Timeout: 25 * time.Second}, URL: Endpoint}
}
func (c *Client) Available() bool {
	return c != nil && c.Config.Client != "" && c.Config.Client == strings.ToLower(c.Config.Client) && c.Config.Version > 0
}

type APIError struct{ Message string }

func (e *APIError) Error() string { return "AniDB returned an API error" }
func (e *APIError) Banned() bool  { return strings.Contains(strings.ToLower(e.Message), "banned") }
func (c *Client) Fetch(ctx context.Context, id string) (Anime, []byte, error) {
	if _, ok := positiveIdentity(id); !ok {
		return Anime{}, nil, errors.New("invalid AniDB identity")
	}
	raw, err := c.request(ctx, "anime", url.Values{"aid": {id}})
	if err != nil {
		return Anime{}, nil, err
	}
	anime, err := Parse(raw, id)
	return anime, raw, err
}
func (c *Client) request(ctx context.Context, request string, parameters url.Values) ([]byte, error) {
	if !c.Available() {
		return nil, errors.New("AniDB requires a registered HTTP client name and version")
	}
	u, err := url.Parse(c.URL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("client", c.Config.Client)
	q.Set("clientver", strconv.Itoa(c.Config.Version))
	q.Set("protover", "1")
	q.Set("request", request)
	for key, values := range parameters {
		q[key] = values
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept-Encoding", "gzip")
	response, err := c.HTTP.Do(req)
	if err != nil {
		return nil, errors.New("AniDB request failed")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AniDB HTTP status %d", response.StatusCode)
	}
	var reader io.Reader = response.Body
	if strings.EqualFold(response.Header.Get("Content-Encoding"), "gzip") {
		gz, e := gzip.NewReader(reader)
		if e != nil {
			return nil, errors.New("invalid AniDB compression")
		}
		defer func() { _ = gz.Close() }()
		reader = gz
	}
	raw, err := io.ReadAll(io.LimitReader(reader, (8<<20)+1))
	if err != nil || len(raw) > 8<<20 {
		return nil, errors.New("invalid or oversized AniDB response")
	}
	return raw, nil
}

type Title struct {
	Language string `xml:"http://www.w3.org/XML/1998/namespace lang,attr" json:"language"`
	Type     string `xml:"type,attr" json:"type,omitempty"`
	Text     string `xml:",chardata" json:"text"`
}
type Episode struct {
	ID     string `xml:"id,attr"`
	Number struct {
		Type int    `xml:"type,attr"`
		Text string `xml:",chardata"`
	} `xml:"epno"`
	Length  int     `xml:"length"`
	AirDate string  `xml:"airdate"`
	Titles  []Title `xml:"title"`
}
type Creator struct {
	ID      string `xml:"id,attr"`
	Type    string `xml:"type,attr"`
	Name    string `xml:",chardata"`
	Picture string `xml:"picture,attr"`
}
type Related struct {
	ID   string `xml:"id,attr"`
	Type string `xml:"type,attr"`
	Name string `xml:",chardata"`
}
type Character struct {
	Name   string    `xml:"name"`
	Voices []Creator `xml:"seiyuu"`
}
type Tag struct {
	ID            string `xml:"id,attr"`
	Name          string `xml:"name"`
	Weight        int    `xml:"weight,attr"`
	LocalSpoiler  string `xml:"localspoiler,attr"`
	GlobalSpoiler string `xml:"globalspoiler,attr"`
	Verified      string `xml:"verified,attr"`
}
type Anime struct {
	Creators   []Creator   `xml:"creators>name"`
	Characters []Character `xml:"characters>character"`
	Related    []Related   `xml:"relatedanime>anime"`
	Tags       []Tag       `xml:"tags>tag"`
	Rating     struct {
		Count int     `xml:"count,attr"`
		Value float64 `xml:",chardata"`
	} `xml:"ratings>permanent"`
	XMLName      xml.Name  `xml:"anime"`
	ID           string    `xml:"id,attr"`
	Type         string    `xml:"type"`
	EpisodeCount int       `xml:"episodecount"`
	StartDate    string    `xml:"startdate"`
	EndDate      string    `xml:"enddate"`
	Description  string    `xml:"description"`
	Picture      string    `xml:"picture"`
	Titles       []Title   `xml:"titles>title"`
	Episodes     []Episode `xml:"episodes>episode"`
}

func Parse(raw []byte, expected string) (Anime, error) {
	var root struct {
		XMLName xml.Name
		Text    string `xml:",chardata"`
	}
	if err := xml.Unmarshal(raw, &root); err != nil {
		return Anime{}, errors.New("invalid AniDB XML")
	}
	if root.XMLName.Local == "error" {
		return Anime{}, &APIError{Message: root.Text}
	}
	var a Anime
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&a); err != nil || a.ID != expected || len(a.Titles) == 0 {
		return Anime{}, errors.New("AniDB response identity or title is invalid")
	}
	var extra interface{}
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Anime{}, errors.New("unexpected trailing AniDB XML")
	}
	seen := map[string]bool{}
	for _, e := range a.Episodes {
		n, err := strconv.ParseInt(e.ID, 10, 64)
		if err != nil || n < 1 || strconv.FormatInt(n, 10) != e.ID || seen[e.ID] {
			return Anime{}, errors.New("invalid or duplicate AniDB episode identity")
		}
		seen[e.ID] = true
		if _, _, err = e.Position(); err != nil {
			return Anime{}, err
		}
	}
	return a, nil
}
func Choose(titles []Title, language, style string) string {
	wanted := language
	switch style {
	case "english":
		wanted = "en"
	case "", "romaji":
		wanted = "x-jat"
	case "native":
		wanted = "ja"
	}
	for _, t := range titles {
		if t.Language == wanted && (t.Type == "official" || t.Type == "main" || t.Type == "") {
			return t.Text
		}
	}
	for _, t := range titles {
		if t.Type == "main" {
			return t.Text
		}
	}
	for _, t := range titles {
		if t.Language == "en" {
			return t.Text
		}
	}
	if len(titles) > 0 {
		return titles[0].Text
	}
	return ""
}
func (e Episode) Position() (int, string, error) {
	kinds := map[int]string{1: "regular", 2: "special", 3: "credit", 4: "trailer", 5: "parody", 6: "other"}
	kind, ok := kinds[e.Number.Type]
	if !ok {
		return 0, "", errors.New("unknown AniDB episode kind")
	}
	raw := strings.TrimSpace(e.Number.Text)
	if e.Number.Type != 1 {
		raw = strings.TrimLeft(raw, "SCTPO")
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, "", errors.New("unsupported AniDB episode number")
	}
	return n, kind, nil
}
func Day(value string) *string {
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return nil
	}
	return &value
}

var pictureName = regexp.MustCompile(`^[1-9][0-9]*\.(jpg|jpeg|png|webp)$`)

func PosterURL(filename string) string {
	if !pictureName.MatchString(filename) {
		return ""
	}
	return "https://cdn-eu.anidb.net/images/main/" + filename
}
