package httpapi

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCountryRulesDistinguishKnownAndUnknownRegions(t *testing.T) {
	e := refreshRegressionSetup(t, "tmdb:movie:987610")
	audit2Movie(t, e, "987610")
	audit2Exec(t, e, `INSERT INTO availability(catalog_id,country,offers) VALUES('tmdb:movie:987610','TR','{"flatrate":[{"provider_id":8}]}'),('tmdb:movie:987610','US','{"flatrate":[{"provider_id":8}]}'),('tmdb:movie:987610','GB','null')`)
	for _, tc := range []struct {
		region             string
		positive, negative int
	}{{"US", 1, 0}, {"GB", 0, 1}, {"DE", 0, 0}} {
		for _, negate := range []bool{false, true} {
			rule := map[string]any{"op": "eq", "field": "availability", "value": tc.region}
			want := tc.positive
			if negate {
				rule = map[string]any{"op": "not", "children": []any{rule}}
				want = tc.negative
			}
			raw := e.req(t, 0, "POST", "/api/lists/preview", map[string]any{"rule": rule}, 200)
			var result struct{ Count int }
			if err := json.Unmarshal(raw, &result); err != nil || result.Count != want {
				t.Fatalf("region=%s negative=%v result=%s err=%v", tc.region, negate, raw, err)
			}
		}
	}
	// Services still refer to the selected region, independently of country rules.
	raw := e.req(t, 0, "POST", "/api/lists/preview", map[string]any{"rule": map[string]any{"op": "eq", "field": "service", "value": "8"}}, 200)
	var result struct{ Count int }
	if err := json.Unmarshal(raw, &result); err != nil || result.Count != 1 {
		t.Fatal(string(raw), err)
	}
}

func TestRepeatImportPreservesResolvedAppDecisions(t *testing.T) {
	e := refreshRegressionSetup(t, "tmdb:movie:987640")
	audit2Movie(t, e, "987640")
	e.req(t, 0, "GET", "/api/export", nil, 200)
	backup := json.RawMessage(`{"movies":[{"movie":{"title":"Audit","ids":{"simkl":987640,"tmdb":987640}},"status":"completed"}]}`)
	id := audit2ID(e.req(t, 0, "POST", "/api/simkl/preview", backup, 200))
	e.req(t, 0, "POST", "/api/simkl/"+id+"/apply", nil, 200)
	raw := e.req(t, 0, "GET", "/api/import-reviews", nil, 200)
	var reviews []struct{ ID string }
	if err := json.Unmarshal(raw, &reviews); err != nil || len(reviews) != 1 {
		t.Fatal(string(raw), err)
	}
	e.req(t, 0, "POST", "/api/import-reviews/"+reviews[0].ID+"/resolve", map[string]any{"confirm": true, "accept_existing": true, "links": []any{}}, 200)
	for range 2 {
		id = audit2ID(e.req(t, 0, "POST", "/api/simkl/preview", backup, 200))
		e.req(t, 0, "POST", "/api/simkl/"+id+"/apply", nil, 200)
		raw = e.req(t, 0, "GET", "/api/import-reviews", nil, 200)
		if err := json.Unmarshal(raw, &reviews); err != nil || len(reviews) != 0 {
			t.Fatal("identical import reopened resolved review", string(raw), err)
		}
	}
	// New source content must still receive a new review rather than inherit acceptance.
	changed := json.RawMessage(strings.ReplaceAll(string(backup), `"completed"`, `"dropped"`))
	id = audit2ID(e.req(t, 0, "POST", "/api/simkl/preview", changed, 200))
	e.req(t, 0, "POST", "/api/simkl/"+id+"/apply", nil, 200)
	raw = e.req(t, 0, "GET", "/api/import-reviews", nil, 200)
	if err := json.Unmarshal(raw, &reviews); err != nil || len(reviews) != 1 {
		t.Fatal("changed source conflict disappeared", string(raw), err)
	}
	var history int
	if err := e.p.QueryRow(t.Context(), "SELECT count(*) FROM history WHERE user_id=$1", e.users[0]).Scan(&history); err != nil || history != 1 {
		t.Fatal("repeat import duplicated history", history, err)
	}
}

func TestStatusNameLimitCountsUnicodeCharacters(t *testing.T) {
	e := auditSetup(t)
	id := audit2ID(e.req(t, 0, "POST", "/api/statuses", map[string]any{"Name": strings.Repeat("İ", 100), "Base": "watching"}, 200))
	e.req(t, 0, "POST", "/api/statuses", map[string]any{"ID": id, "Name": strings.Repeat("ş", 100), "Base": "watching"}, 200)
	for _, name := range []string{strings.Repeat("İ", 101), strings.Repeat("I", 101)} {
		e.req(t, 0, "POST", "/api/statuses", map[string]any{"Name": name, "Base": "watching"}, 400)
	}
}
