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
