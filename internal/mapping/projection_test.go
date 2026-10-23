package mapping

import "testing"

func TestProjectionSplitBothDirectionsAndNoTransitiveFacts(t *testing.T) {
	edges, rev, split, e := Canonical([]Edge{{"a", "x"}, {"a", "y"}, {"b", "y"}})
	if e != nil {
		t.Fatal(e)
	}
	d := Definition{ID: "m", Revision: rev, Edges: edges, Reviewed: true, RequiresConsent: split}
	states := Project([]Definition{d}, map[string]bool{"a": true, "b": true})
	if states["x"].Derived || !states["x"].Blocked {
		t.Fatal("consent bypass", states)
	}
	d.Consented = rev
	states = Project([]Definition{d}, map[string]bool{"a": true})
	if !states["x"].Derived || states["y"].Derived || states["b"].Derived {
		t.Fatal("partial mapping invented history", states)
	}
	states = Project([]Definition{d}, map[string]bool{"x": true, "y": true})
	if !states["a"].Derived || !states["b"].Derived {
		t.Fatal("reverse projection missing", states)
	}
	states = Project([]Definition{d}, map[string]bool{"x": true})
	if states["a"].Derived {
		t.Fatal("partial reverse projection marked watched")
	}
	d.Revision = "changed"
	if !Project([]Definition{d}, map[string]bool{"a": true})["x"].Blocked {
		t.Fatal("invalid revision accepted")
	}
}
func TestProjectionAmbiguityAndUnreviewedMappings(t *testing.T) {
	edges, rev, _, _ := Canonical([]Edge{{"a", "x"}})
	d := Definition{ID: "one", Revision: rev, Edges: edges}
	if Project([]Definition{d}, map[string]bool{"a": true})["x"].Derived {
		t.Fatal("unreviewed propagation")
	}
	d.Reviewed = true
	other := d
	other.ID = "two"
	states := Project([]Definition{d, other}, map[string]bool{"a": true})
	if !states["x"].Blocked || states["x"].Derived || !states["a"].Direct {
		t.Fatal("overlap or recorded event lost", states)
	}
	if _, _, _, err := Canonical([]Edge{{"a", "b"}, {"b", "c"}}); err == nil {
		t.Fatal("mixed mapping sides accepted")
	}
}

func TestReviewedMappingWinsOverUnreviewedSuggestion(t *testing.T) {
	edges, rev, _, _ := Canonical([]Edge{{"a", "x"}})
	reviewed := Definition{ID: "reviewed", Revision: rev, Edges: edges, Reviewed: true}
	suggestion := reviewed
	suggestion.ID = "suggestion"
	suggestion.Reviewed = false
	for _, definitions := range [][]Definition{{reviewed, suggestion}, {suggestion, reviewed}} {
		state := Project(definitions, map[string]bool{"a": true})["x"]
		if !state.Derived || state.Blocked || state.MappingID != "reviewed" {
			t.Fatal("unreviewed suggestion suppressed approved mapping", state)
		}
	}
}

func TestMovieAliasesAreAlternativesNotRequiredParts(t *testing.T) {
	edges, revision, _, err := CanonicalForKind([]Edge{{"native", "movie-a"}, {"native", "movie-b"}}, "movie-aliases")
	if err != nil {
		t.Fatal(err)
	}
	d := Definition{ID: "aliases", Kind: "movie-aliases", Revision: revision, Edges: edges, Reviewed: true, RequiresConsent: true}
	if states := Project([]Definition{d}, map[string]bool{"movie-a": true}); states["native"].Derived || !states["native"].Blocked {
		t.Fatal("aliases bypassed confirmation", states)
	}
	d.Consented = revision
	states := Project([]Definition{d}, map[string]bool{"movie-a": true})
	if !states["native"].Derived || !states["movie-b"].Derived || !states["movie-a"].Direct {
		t.Fatal("one alias should suffice without copied facts", states)
	}
	d.Kind = "episodes"
	if !Project([]Definition{d}, map[string]bool{"movie-a": true})["native"].Blocked {
		t.Fatal("semantics changed without renewed revision")
	}
}

func TestNativeMoviePartsStillRequireEveryPart(t *testing.T) {
	edges, revision, _, err := CanonicalForKind([]Edge{{"part1", "movie-a"}, {"part2", "movie-a"}, {"part1", "movie-b"}, {"part2", "movie-b"}}, "movie-aliases")
	if err != nil {
		t.Fatal(err)
	}
	d := Definition{ID: "film", Kind: "movie-aliases", Revision: revision, Edges: edges, Reviewed: true, RequiresConsent: true, Consented: revision}
	states := Project([]Definition{d}, map[string]bool{"part1": true})
	if states["movie-a"].Derived || states["movie-b"].Derived || states["part2"].Derived {
		t.Fatal("one native part incorrectly completed the film", states)
	}
	states = Project([]Definition{d}, map[string]bool{"part1": true, "part2": true})
	if !states["movie-a"].Derived || !states["movie-b"].Derived {
		t.Fatal("all native film parts did not complete the work", states)
	}
	states = Project([]Definition{d}, map[string]bool{"movie-b": true})
	if !states["part1"].Derived || !states["part2"].Derived || !states["movie-a"].Derived {
		t.Fatal("one full movie alias did not cover all parts", states)
	}
}
