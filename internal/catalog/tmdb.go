// Package catalog implements real provider access with explicit source identity.
package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var ErrUnconfigured = errors.New("TMDB token is not configured")

type StatusError struct {
	Code       int
	RetryAfter time.Duration
}

func (e *StatusError) Error() string { return fmt.Sprintf("TMDB returned status %d", e.Code) }

type TMDB struct {
	Token, BaseURL string
	Client         *http.Client
}

func New(token string) *TMDB {
	return &TMDB{Token: token, BaseURL: "https://api.themoviedb.org/3", Client: &http.Client{Timeout: 20 * time.Second}}
}
func (p *TMDB) Get(ctx context.Context, path string, params url.Values) (json.RawMessage, error) {
	if p.Token == "" {
		return nil, ErrUnconfigured
	}
	if params == nil {
		params = url.Values{}
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, p.BaseURL+path+"?"+params.Encode(), nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("Authorization", "Bearer "+p.Token)
	req.Header.Set("Accept", "application/json")
	res, e := p.Client.Do(req)
	if e != nil {
		return nil, errors.New("TMDB is unavailable; retry later")
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		retry := time.Minute
		if seconds, err := strconv.Atoi(res.Header.Get("Retry-After")); err == nil && seconds > 0 {
			retry = time.Duration(seconds) * time.Second
		} else if until, err := http.ParseTime(res.Header.Get("Retry-After")); err == nil && until.After(time.Now()) {
			retry = time.Until(until)
		}
		return nil, &StatusError{Code: res.StatusCode, RetryAfter: retry}
	}
	b, e := io.ReadAll(io.LimitReader(res.Body, (8<<20)+1))
	if e != nil {
		return nil, e
	}
	if len(b) > 8<<20 {
		return nil, errors.New("TMDB response exceeds 8 MiB")
	}
	if !json.Valid(b) {
		return nil, errors.New("invalid TMDB response")
	}
	return b, nil
}
func ValidKind(kind string) bool { return kind == "movie" || kind == "tv" || kind == "person" }
func Identity(kind, id string) (string, error) {
	if !ValidKind(kind) {
		return "", errors.New("unsupported catalog kind")
	}
	n, e := strconv.ParseInt(id, 10, 64)
	if e != nil || n < 1 {
		return "", errors.New("invalid provider identifier")
	}
	return "tmdb:" + kind + ":" + strconv.FormatInt(n, 10), nil
}
func SearchPath(kind string) string {
	if ValidKind(kind) {
		return "/search/" + kind
	}
	return "/search/multi"
}

// Discovery forwards only documented filters. TV people filtering is a separate
// credits-candidate workflow, never silently forwarded as unsupported filters.
func Discovery(kind string, in url.Values) (url.Values, error) {
	if kind != "movie" && kind != "tv" {
		return nil, errors.New("unsupported discovery kind")
	}
	out := url.Values{}
	allowed := strings.Fields("language page sort_by with_genres without_genres with_original_language vote_average.gte vote_count.gte with_runtime.gte with_runtime.lte watch_region with_watch_providers with_watch_monetization_types primary_release_date.gte primary_release_date.lte first_air_date.gte first_air_date.lte year first_air_date_year")
	for _, k := range allowed {
		if v := in.Get(k); v != "" {
			out.Set(k, v)
		}
	}
	for _, k := range []string{"with_cast", "with_crew", "with_people"} {
		if v := in.Get(k); v != "" {
			if kind == "tv" {
				return nil, errors.New("TV people discovery requires person credits candidates")
			}
			out.Set(k, v)
		}
	}
	return out, nil
}
