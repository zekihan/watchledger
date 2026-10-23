package simkl

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strconv"
	"unicode/utf8"
)

type ListMember struct {
	Key    string `json:"key"`
	Title  string `json:"title"`
	Target string `json:"target"`
}
type CustomList struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Members  []ListMember `json:"members"`
	Warnings []string     `json:"warnings"`
}
type listPage struct {
	ID         json.RawMessage `json:"id"`
	Name       string          `json:"name"`
	MediaType  string          `json:"media_type"`
	Type       string          `json:"type"`
	Sort       []string        `json:"sort"`
	UpdatedAt  string          `json:"updated_at"`
	Error      json.RawMessage `json:"error"`
	Pagination *struct {
		Page, Limit int
		TotalItems  int `json:"total_items"`
		TotalPages  int `json:"total_pages"`
	} `json:"pagination"`
	Counts struct {
		Items *int `json:"items"`
	} `json:"counts"`
	Items *[]struct {
		Title string                     `json:"title"`
		IDs   map[string]json.RawMessage `json:"ids"`
	} `json:"items"`
}

func stringID(raw json.RawMessage) (string, bool) {
	var value string
	if json.Unmarshal(raw, &value) != nil {
		var n json.Number
		if json.Unmarshal(raw, &n) != nil {
			return "", false
		}
		value = n.String()
	}
	return positiveID(value)
}

// ParseCustomList accepts the documented list-contents API response, or an array
// of its unmodified pages assembled by the upload UI. It is not a guessed field
// inside the downloadable watchlist backup. Error and incomplete responses fail.
func ParseCustomList(raw []byte) (CustomList, error) {
	result := CustomList{Members: []ListMember{}, Warnings: []string{}}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var document json.RawMessage
	if decoder.Decode(&document) != nil {
		return result, errors.New("invalid custom-list JSON")
	}
	var extra interface{}
	if decoder.Decode(&extra) != io.EOF {
		return result, errors.New("expected one JSON document")
	}
	docs := []json.RawMessage{document}
	if len(bytes.TrimSpace(document)) > 0 && bytes.TrimSpace(document)[0] == '[' {
		if json.Unmarshal(document, &docs) != nil {
			return result, errors.New("invalid page bundle")
		}
	}
	if len(docs) == 0 || len(docs) > 10000 {
		return result, errors.New("empty or oversized page bundle")
	}
	pages := make([]listPage, 0, len(docs))
	for _, doc := range docs {
		var page listPage
		if json.Unmarshal(doc, &page) != nil {
			return result, errors.New("invalid list contents response")
		}
		if len(page.Error) > 0 && string(page.Error) != "null" {
			return result, errors.New("SIMKL returned an error or premium-only response, not list contents")
		}
		id, ok := stringID(page.ID)
		if !ok || page.Name == "" || utf8.RuneCountInString(page.Name) > 100 || page.Items == nil || page.Pagination == nil {
			return result, errors.New("list ID, name, items, and pagination are required")
		}
		p := page.Pagination
		if p.Page < 1 || p.Limit < 1 || p.Limit > 500 || p.TotalItems < 0 || p.TotalItems > 10000 || p.Page*p.Limit > 10000 {
			return result, errors.New("invalid or inaccessible source pagination")
		}
		expectedPages := (p.TotalItems + p.Limit - 1) / p.Limit
		if p.TotalItems == 0 {
			expectedPages = 1
			if p.TotalPages == 0 {
				p.TotalPages = 1
			}
		}
		if p.TotalPages != expectedPages || p.Page > p.TotalPages || len(*page.Items) > p.Limit {
			return result, errors.New("inconsistent source pagination")
		}
		if page.Counts.Items != nil && *page.Counts.Items != p.TotalItems {
			return result, errors.New("source counts changed during export")
		}
		if result.ID == "" {
			result.ID = id
			result.Name = page.Name
		} else {
			first := pages[0]
			if id != result.ID || page.Name != result.Name || page.MediaType != first.MediaType || page.Type != first.Type || !slices.Equal(page.Sort, first.Sort) || p.Limit != first.Pagination.Limit || p.TotalItems != first.Pagination.TotalItems || p.TotalPages != first.Pagination.TotalPages || page.UpdatedAt == "" || page.UpdatedAt != first.UpdatedAt {
				return result, errors.New("pages do not describe one consistent list snapshot")
			}
		}
		pages = append(pages, page)
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].Pagination.Page < pages[j].Pagination.Page })
	if len(pages) != pages[0].Pagination.TotalPages {
		return result, errors.New("incomplete custom list: include every response page")
	}
	group := map[string]string{"movies": "movies", "tv": "shows", "anime": "anime"}[pages[0].MediaType]
	if group == "" {
		return result, errors.New("unsupported list media type")
	}
	seen := map[string]bool{}
	for index, page := range pages {
		if page.Pagination.Page != index+1 {
			return result, errors.New("duplicate or missing response page")
		}
		for _, item := range *page.Items {
			a, okA := stringID(item.IDs["simkl"])
			b, okB := stringID(item.IDs["simkl_id"])
			if okA && okB && a != b {
				return result, errors.New("conflicting source identities")
			}
			if !okA {
				a = b
			}
			if a == "" || item.Title == "" {
				return result, errors.New("member source ID and title are required")
			}
			key := group + ":" + a
			if seen[key] {
				return result, errors.New("duplicate source member identity")
			}
			seen[key] = true
			normalized := Item{Kind: group, IDs: map[string]string{}}
			for provider, id := range item.IDs {
				if value, ok := stringID(id); ok {
					normalized.IDs[provider] = value
				}
			}
			result.Members = append(result.Members, ListMember{Key: key, Title: item.Title, Target: normalized.Target()})
		}
	}
	if len(result.Members) != pages[0].Pagination.TotalItems {
		return result, fmt.Errorf("incomplete source membership: got %s of %s", strconv.Itoa(len(result.Members)), strconv.Itoa(pages[0].Pagination.TotalItems))
	}
	if pages[0].Type == "auto" {
		result.Warnings = append(result.Warnings, "auto_list_imported_as_manual_snapshot_rules_retained_in_archive")
	}
	return result, nil
}
