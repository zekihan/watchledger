package mapping

import "sort"

// Projection derives flags only from recorded events, never recursively from
// other projections. This prevents partial split/combined chains becoming facts.
type Definition struct {
	Kind            string
	ID              string
	Revision        string
	Edges           []Edge
	Reviewed        bool
	RequiresConsent bool
	Consented       string
}
type State struct {
	NativeParts     []string   `json:"native_parts,omitempty"`
	MovieAliases    []string   `json:"movie_aliases,omitempty"`
	SourceGroups    [][]string `json:"source_groups,omitempty"`
	AnySource       bool       `json:"any_source,omitempty"`
	ExplicitUnwatch bool       `json:"explicit_unwatch,omitempty"`
	Direct          bool       `json:"direct"`
	Derived         bool       `json:"derived"`
	Blocked         bool       `json:"blocked"`
	Sources         []string   `json:"sources"`
	MappingID       string     `json:"mapping_id,omitempty"`
	Revision        string     `json:"revision,omitempty"`
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
		_, rev, split, err := CanonicalForKind(d.Edges, d.Kind)
		blocked := err != nil || rev != d.Revision || !d.Reviewed || ((split || d.RequiresConsent) && d.Consented != d.Revision)
		for _, e := range d.Edges {
			peers[e.From] = append(peers[e.From], e.To)
			peers[e.To] = append(peers[e.To], e.From)
		}
		groups := map[string][][]string{}
		nativeParts := map[string][]string{}
		movieAliases := map[string][]string{}
		if d.Kind == "movie-aliases" && err == nil {
			native := map[string]bool{}
			for _, edge := range d.Edges {
				native[edge.From] = true
			}
			expanded := map[string][]string{}
			for id := range peers {
				seen := map[string]bool{id: true}
				queue := []string{id}
				for len(queue) > 0 {
					current := queue[0]
					queue = queue[1:]
					for _, next := range peers[current] {
						if !seen[next] {
							seen[next] = true
							queue = append(queue, next)
						}
					}
				}
				parts, aliases := []string{}, []string{}
				for member := range seen {
					if native[member] {
						parts = append(parts, member)
					} else {
						aliases = append(aliases, member)
					}
				}
				sort.Strings(parts)
				sort.Strings(aliases)
				nativeParts[id] = parts
				movieAliases[id] = aliases
				if !native[id] {
					groups[id] = append(groups[id], parts)
					expanded[id] = append(expanded[id], parts...)
				}
				for _, alias := range aliases {
					if alias != id {
						groups[id] = append(groups[id], []string{alias})
						expanded[id] = append(expanded[id], alias)
					}
				}
			}
			peers = expanded
		}
		for id, parts := range peers {
			if !d.Reviewed && owners[id] > 0 {
				continue
			}
			sort.Strings(parts)
			state := State{AnySource: d.Kind == "movie-aliases", Direct: watched[id], Blocked: blocked || owners[id] > 1, Sources: parts, MappingID: d.ID, Revision: d.Revision}
			all := true
			for _, part := range parts {
				all = all && watched[part]
				state.Blocked = state.Blocked || owners[part] > 1
			}
			if state.AnySource {
				state.NativeParts = nativeParts[id]
				state.MovieAliases = movieAliases[id]
				state.SourceGroups = groups[id]
				all = false
				for _, group := range groups[id] {
					complete := len(group) > 0
					for _, source := range group {
						complete = complete && watched[source]
					}
					all = all || complete
				}
				for _, member := range append(append([]string{}, state.NativeParts...), state.MovieAliases...) {
					state.Blocked = state.Blocked || owners[member] > 1
				}
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
