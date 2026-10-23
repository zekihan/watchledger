package mapping

import "sort"

// Projection derives flags only from recorded events, never recursively from
// other projections. This prevents partial split/combined chains becoming facts.
type Definition struct {
	ID              string
	Revision        string
	Edges           []Edge
	Reviewed        bool
	RequiresConsent bool
	Consented       string
}
type State struct {
	Direct    bool     `json:"direct"`
	Derived   bool     `json:"derived"`
	Blocked   bool     `json:"blocked"`
	Sources   []string `json:"sources"`
	MappingID string   `json:"mapping_id,omitempty"`
	Revision  string   `json:"revision,omitempty"`
}

func Project(definitions []Definition, watched map[string]bool) map[string]State {
	out := map[string]State{}
	owners := map[string]int{}
	for _, d := range definitions {
		seen := map[string]bool{}
		for _, e := range d.Edges {
			seen[e.From] = true
			seen[e.To] = true
		}
		for id := range seen {
			if d.Reviewed {
				owners[id]++
			}
		}
	}
	for _, d := range definitions {
		peers := map[string][]string{}
		_, rev, split, err := Canonical(d.Edges)
		blocked := err != nil || rev != d.Revision || !d.Reviewed || ((split || d.RequiresConsent) && d.Consented != d.Revision)
		for _, e := range d.Edges {
			peers[e.From] = append(peers[e.From], e.To)
			peers[e.To] = append(peers[e.To], e.From)
		}
		for id, parts := range peers {
			if !d.Reviewed && owners[id] > 0 {
				continue
			}
			sort.Strings(parts)
			state := State{Direct: watched[id], Blocked: blocked || owners[id] > 1, Sources: parts, MappingID: d.ID, Revision: d.Revision}
			all := true
			for _, part := range parts {
				all = all && watched[part]
				state.Blocked = state.Blocked || owners[part] > 1
			}
			state.Derived = !state.Blocked && all && !state.Direct
			out[id] = state
		}
	}
	for id, seen := range watched {
		if _, ok := out[id]; !ok {
			out[id] = State{Direct: seen, Sources: []string{}}
		}
	}
	return out
}
