package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

type Availability struct {
	Offers      json.RawMessage `json:"offers"`
	FetchedAt   time.Time       `json:"fetched_at"`
	Stale       bool            `json:"stale"`
	Warning     string          `json:"warning,omitempty"`
	Attribution string          `json:"attribution"`
}

func (c *Cache) Availability(ctx context.Context, kind, id, country string, force bool) (Availability, error) {
	out := Availability{Attribution: "JustWatch via TMDB"}
	key, e := Identity(kind, id)
	if e != nil || kind == "person" || len(country) != 2 {
		return out, errors.New("invalid title or country")
	}
	e = c.DB.QueryRow(ctx, "SELECT offers,fetched_at FROM availability WHERE catalog_id=$1 AND country=$2", key, country).Scan(&out.Offers, &out.FetchedAt)
	cached := e == nil
	if cached {
		if !force {
			_, e = c.DB.Exec(ctx, "UPDATE availability SET used_at=now() WHERE catalog_id=$1 AND country=$2", key, country)
			if e != nil {
				return out, e
			}

		}
		if !force && time.Since(out.FetchedAt) < 24*time.Hour {
			return out, nil
		}
	}
	raw, e := c.Provider.Get(ctx, "/"+kind+"/"+id+"/watch/providers", nil)
	if e != nil {
		if cached {
			out.Stale = true
			out.Warning = e.Error()
			return out, nil
		}
		return out, e
	}
	var data struct {
		Results map[string]json.RawMessage `json:"results"`
	}
	e = json.Unmarshal(raw, &data)
	if e == nil && data.Results != nil {
		e = validateOffers(data.Results[country])
	}
	if e != nil || data.Results == nil {
		if e == nil {
			e = errors.New("provider omitted availability results")
		}
		if cached {
			out.Stale = true
			out.Warning = e.Error()
			return out, nil
		}
		return out, e
	}
	out.Offers = data.Results[country]
	if len(out.Offers) == 0 {
		out.Offers = json.RawMessage(`null`)
	}
	var exists bool
	if e = c.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM catalog WHERE id=$1)", key).Scan(&exists); e != nil {
		return out, e
	}
	if !exists {
		if _, e = c.Detail(ctx, kind, id, "", false); e != nil {
			return out, e
		}
	}
	e = c.DB.QueryRow(ctx, `INSERT INTO availability(catalog_id,country,offers) VALUES($1,$2,$3) ON CONFLICT(catalog_id,country) DO UPDATE SET offers=EXCLUDED.offers,fetched_at=now(),used_at=CASE WHEN $4 THEN availability.used_at ELSE now() END RETURNING fetched_at`, key, country, out.Offers, force).Scan(&out.FetchedAt)
	return out, e
}

// Country omission is a valid negative result; malformed offer structures are not.
func validateOffers(raw json.RawMessage) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return errors.New("invalid availability country response")
	}
	for _, kind := range []string{"flatrate", "rent", "buy", "free", "ads"} {
		if value, ok := fields[kind]; ok {
			var offers []struct {
				ID   int    `json:"provider_id"`
				Name string `json:"provider_name"`
			}
			if err := json.Unmarshal(value, &offers); err != nil {
				return errors.New("invalid availability offers")
			}
			for _, offer := range offers {
				if offer.ID < 1 {
					return errors.New("invalid availability provider")
				}
			}
		}
	}
	if link, ok := fields["link"]; ok {
		var value string
		if err := json.Unmarshal(link, &value); err != nil {
			return errors.New("invalid availability link")
		}
	}
	return nil
}
