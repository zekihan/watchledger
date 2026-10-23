// Package mapping describes versioned episode relationships without mutating history.
package mapping

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
)

type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func Canonical(edges []Edge) ([]Edge, string, bool, error) {
	if len(edges) == 0 || len(edges) > 10000 {
		return nil, "", false, errors.New("mapping requires 1–10000 relationships")
	}
	out := append([]Edge(nil), edges...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		return out[i].To < out[j].To
	})
	from, to := map[string]int{}, map[string]int{}
	split := false
	for i, e := range out {
		if e.From == "" || e.To == "" || e.From == e.To {
			return nil, "", false, errors.New("invalid episode relationship")
		}
		if i > 0 && e == out[i-1] {
			return nil, "", false, errors.New("duplicate relationship")
		}
		from[e.From]++
		to[e.To]++
		split = split || from[e.From] > 1 || to[e.To] > 1
	}
	for id := range from {
		if to[id] > 0 {
			return nil, "", false, errors.New("episode cannot occur on both sides of a mapping")
		}
	}
	b, e := json.Marshal(out)
	if e != nil {
		return nil, "", false, e
	}
	sum := sha256.Sum256(b)
	return out, hex.EncodeToString(sum[:]), split, nil
}

// Derived requires every source part for a combined target. Consent is bound to
// the material relationship revision; corrections never rewrite source events.
func Derived(edges []Edge, watched map[string]bool, requiresConsent bool, revision, consented string) map[string]bool {
	out := map[string]bool{}
	if requiresConsent && revision != consented {
		return out
	}
	sources := map[string][]string{}
	for _, e := range edges {
		sources[e.To] = append(sources[e.To], e.From)
	}
	for target, parts := range sources {
		all := true
		for _, p := range parts {
			all = all && watched[p]
		}
		out[target] = all
	}
	return out
}
