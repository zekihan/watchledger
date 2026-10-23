package simkl

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ReadOnlyClient only retrieves an explicitly selected custom list. It never
// writes to SIMKL, refreshes tokens, follows redirects, or stores credentials.
type ReadOnlyClient struct {
	BaseURL  string
	HTTP     *http.Client
	gate     chan struct{}
	last     time.Time
	interval time.Duration
}
type FetchedList struct {
	Raw  []byte
	Type string
}

func NewReadOnlyClient() *ReadOnlyClient {
	return &ReadOnlyClient{BaseURL: "https://api.simkl.com", HTTP: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, gate: make(chan struct{}, 1), interval: 250 * time.Millisecond}
}
func (c *ReadOnlyClient) page(ctx context.Context, clientID, token, listID string, page, limit int) ([]byte, error) {
	select {
	case c.gate <- struct{}{}:
	case <-ctx.Done():
		return nil, errors.New("SIMKL import cancelled")
	}
	defer func() { <-c.gate }()
	if wait := time.Until(c.last.Add(c.interval)); wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return nil, errors.New("SIMKL import cancelled")
		}
	}
	params := url.Values{"client_id": {clientID}, "app-name": {"watchledger"}, "app-version": {"1"}, "page": {strconv.Itoa(page)}, "limit": {strconv.Itoa(limit)}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/lists/"+listID+"?"+params.Encode(), nil)
	if err != nil {
		return nil, errors.New("SIMKL request unavailable")
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("User-Agent", "watchledger/1")
	request.Header.Set("Accept", "application/json")
	c.last = time.Now()
	response, err := c.HTTP.Do(request)
	if err != nil {
		return nil, errors.New("SIMKL request failed; retry later")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode == 401 {
		return nil, errors.New("SIMKL AUTH V2 access token required")
	}
	if response.StatusCode == 403 {
		return nil, errors.New("SIMKL rejected access; verify the AUTH V2 client and token")
	}
	if response.StatusCode == 429 {
		return nil, errors.New("SIMKL rate limit reached; retry later")
	}
	if response.StatusCode != 200 {
		return nil, errors.New("SIMKL list unavailable; no data imported")
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, (8<<20)+1))
	if err != nil || len(raw) > 8<<20 {
		return nil, errors.New("SIMKL page exceeds limit or could not be read")
	}
	var envelope map[string]json.RawMessage
	if json.Unmarshal(raw, &envelope) != nil {
		return nil, errors.New("invalid SIMKL response")
	}
	if errorBody, ok := envelope["error"]; ok && string(errorBody) != "null" {
		var code string
		_ = json.Unmarshal(errorBody, &code)
		switch code {
		case "premium_only":
			return nil, errors.New("SIMKL custom lists require PRO or VIP access")
		case "oauth2_token_required":
			return nil, errors.New("SIMKL AUTH V2 token required; AUTH V1 cannot access custom lists")
		default:
			return nil, errors.New("SIMKL returned an API error; no data imported")
		}
	}
	for _, key := range []string{"access_token", "refresh_token", "client_secret", "authorization"} {
		if _, ok := envelope[key]; ok {
			return nil, errors.New("unexpected authentication fields in SIMKL list response")
		}
	}
	return raw, nil
}
func (c *ReadOnlyClient) FetchList(ctx context.Context, clientID, token, listID string) (FetchedList, error) {
	out := FetchedList{}
	if _, ok := positiveID(listID); !ok {
		return out, errors.New("valid numeric SIMKL list ID required")
	}
	if len(clientID) == 0 || len(clientID) > 256 || len(token) == 0 || len(token) > 8192 || strings.ContainsAny(clientID+token, "\r\n\t ") {
		return out, errors.New("valid AUTH V2 client ID and access token required")
	}
	pages := []json.RawMessage{}
	limit := 500
	total := 1
	size := 0
	for page := 1; page <= total; page++ {
		raw, err := c.page(ctx, clientID, token, listID, page, limit)
		if err != nil {
			return out, err
		}
		var item listPage
		if json.Unmarshal(raw, &item) != nil || item.Pagination == nil || item.Items == nil {
			return out, errors.New("SIMKL returned no complete list page")
		}
		id, ok := stringID(item.ID)
		if !ok || id != listID || item.Pagination.Page != page {
			return out, errors.New("SIMKL list identity or page changed")
		}
		if item.Pagination.TotalItems == 0 && item.Pagination.TotalPages == 0 {
			item.Pagination.TotalPages = 1
		}
		if item.Pagination.TotalPages < 1 || item.Pagination.TotalPages > 20 || item.Pagination.TotalItems > 10000 || item.Pagination.Limit < 1 || item.Pagination.Limit > 500 {
			return out, errors.New("SIMKL pagination exceeds supported complete-import limits; use response files instead")
		}
		if page == 1 {
			total = item.Pagination.TotalPages
			limit = item.Pagination.Limit
			out.Type = item.Type
		} else if item.Pagination.TotalPages != total || item.Pagination.Limit != limit {
			return out, errors.New("SIMKL list changed during pagination; retry later")
		}
		size += len(raw)
		if size > 32<<20 {
			return out, errors.New("SIMKL list exceeds 32 MiB")
		}
		pages = append(pages, raw)
	}
	raw, err := json.Marshal(pages)
	if err != nil {
		return out, errors.New("SIMKL list could not be assembled")
	}
	if len(raw) > 32<<20 {
		return out, errors.New("SIMKL list exceeds 32 MiB")
	}
	if _, err = ParseCustomList(raw); err != nil {
		return out, errors.New("SIMKL list changed or is incomplete; no data imported")
	}
	out.Raw = raw
	return out, nil
}
