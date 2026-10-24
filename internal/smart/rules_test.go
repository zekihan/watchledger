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

func TestSupportedFieldTruthAndNumericBoundaries(t *testing.T) {
	for _, field := range []string{"kind", "status", "base", "genre", "year", "rating", "availability", "service"} {
		t.Run(field, func(t *testing.T) {
			rule := Rule{Op: "eq", Field: field, Value: "8"}
			for _, fixture := range []struct {
				values map[string][]string
				want   Truth
			}{
				{map[string][]string{}, Unknown}, {map[string][]string{field: {"8"}}, True}, {map[string][]string{field: {"9"}}, False}, {map[string][]string{field: {}}, False},
			} {
				got, err := Eval(rule, Item{Values: fixture.values}, nil)
				if err != nil || got != fixture.want {
					t.Fatal(got, fixture.want, err)
				}
				unknown, err := Eval(Rule{Op: "unknown", Field: field}, Item{Values: fixture.values}, nil)
				if err != nil || (unknown == True) != (fixture.want == Unknown) {
					t.Fatal("unknown semantics", unknown, err)
				}
			}
		})
	}
	for _, op := range []string{"gte", "lte"} {
		rule := Rule{Op: op, Field: "rating", Value: "8"}
		if err := Validate(rule); err != nil {
			t.Fatal(err)
		}
		got, err := Eval(rule, Item{Values: map[string][]string{"rating": {"8"}}}, nil)
		if err != nil || got != True {
			t.Fatal("inclusive boundary", got, err)
		}
	}
	member := func(list, item string) (Truth, error) {
		if list == "stable-smart-id" && item == "owned-title" {
			return True, nil
		}
		return False, nil
	}
	got, err := Eval(Rule{Op: "eq", Field: "list", Value: "stable-smart-id"}, Item{ID: "owned-title"}, member)
	if err != nil || got != True {
		t.Fatal("stable membership", got, err)
	}
}

func TestUnknownGroupTruthTables(t *testing.T) {
	values := map[string][]string{"kind": {"movie"}, "base": {"planned"}}
	for _, fixture := range []struct {
		op, left, right string
		want            Truth
	}{
		{"and", "kind", "genre", Unknown}, {"and", "base", "genre", False},
		{"or", "kind", "genre", True}, {"or", "base", "genre", Unknown},
	} {
		rule := Rule{Op: fixture.op, Children: []Rule{{Op: "eq", Field: fixture.left, Value: "movie"}, {Op: "eq", Field: fixture.right, Value: "movie"}}}
		got, err := Eval(rule, Item{Values: values}, nil)
		if err != nil || got != fixture.want {
			t.Fatal(fixture, got, err)
		}
	}
}
