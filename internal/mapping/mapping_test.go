package mapping

import "testing"

func TestConsentAndHistory(t *testing.T) {
	edges, rev, split, e := Canonical([]Edge{{"a", "t"}, {"b", "t"}})
	if e != nil || !split {
		t.Fatal(e)
	}
	watched := map[string]bool{"a": true}
	if len(Derived(edges, watched, true, rev, "")) != 0 {
		t.Fatal("unconfirmed propagation")
	}
	if Derived(edges, watched, true, rev, rev)["t"] {
		t.Fatal("partial combined episode marked watched")
	}
	watched["b"] = true
	if !Derived(edges, watched, true, rev, rev)["t"] {
		t.Fatal("confirmed progress missing")
	}
	changed, newRev, _, e := Canonical([]Edge{{"a", "t"}, {"c", "t"}})
	if e != nil {
		t.Fatal(e)
	}
	if len(Derived(changed, watched, true, newRev, rev)) != 0 {
		t.Fatal("stale consent reused")
	}
	if !watched["a"] || !watched["b"] {
		t.Fatal("history mutated")
	}
}
func TestCanonicalRevision(t *testing.T) {
	_, a, _, _ := Canonical([]Edge{{"b", "y"}, {"a", "x"}})
	_, b, _, _ := Canonical([]Edge{{"a", "x"}, {"b", "y"}})
	if a != b {
		t.Fatal("order changed consent")
	}
}
