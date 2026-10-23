package smart

import "testing"

func TestUnknownDoesNotMatchNegative(t *testing.T) {
	r := Rule{Op: "not", Children: []Rule{{Op: "eq", Field: "service", Value: "netflix"}}}
	v, e := Eval(r, Item{Values: map[string][]string{}}, nil)
	if e != nil || v != Unknown {
		t.Fatal(v, e)
	}
	v, e = Eval(r, Item{Values: map[string][]string{"service": {}}}, nil)
	if e != nil || v != True {
		t.Fatal(v, e)
	}
	include := Rule{Op: "or", Children: []Rule{r, {Op: "unknown", Field: "service"}}}
	v, e = Eval(include, Item{Values: map[string][]string{}}, nil)
	if e != nil || v != True {
		t.Fatal(v, e)
	}
}
func TestDependencies(t *testing.T) {
	if Acyclic(map[string][]string{"a": {"b"}, "b": {"a"}}) == nil {
		t.Fatal("cycle accepted")
	}
	if Acyclic(map[string][]string{"a": {"missing"}}) == nil {
		t.Fatal("missing reference accepted")
	}
	if e := Acyclic(map[string][]string{"a": {"b"}, "b": {}}); e != nil {
		t.Fatal(e)
	}
}
func TestNested(t *testing.T) {
	r := Rule{Op: "and", Children: []Rule{{Op: "eq", Field: "base", Value: "watching"}, {Op: "or", Children: []Rule{{Op: "gte", Field: "rating", Value: "8"}, {Op: "unknown", Field: "rating"}}}}}
	if e := Validate(r); e != nil {
		t.Fatal(e)
	}
	v, e := Eval(r, Item{Values: map[string][]string{"base": {"watching"}}}, nil)
	if e != nil || v != True {
		t.Fatal(v, e)
	}
}
