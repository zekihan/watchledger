// Package smart evaluates private-library rules using three-valued logic.
package smart

import (
	"errors"
	"fmt"
	"strconv"
)

type Truth int

const (
	Unknown Truth = iota
	False
	True
)

type Rule struct {
	Op       string `json:"op"`
	Children []Rule `json:"children,omitempty"`
	Field    string `json:"field,omitempty"`
	Value    string `json:"value,omitempty"`
}
type Item struct {
	ID           string
	Values       map[string][]string
	Availability map[string]Truth
}
type Membership func(list, item string) (Truth, error)

func Validate(r Rule) error { return validate(r, 0, new(int)) }
func validate(r Rule, depth int, count *int) error {
	*count++
	if depth > 12 || *count > 200 {
		return errors.New("rule exceeds complexity limit")
	}
	switch r.Op {
	case "and", "or":
		if len(r.Children) == 0 {
			return errors.New("group requires conditions")
		}
		for _, c := range r.Children {
			if e := validate(c, depth+1, count); e != nil {
				return e
			}
		}
	case "not":
		if len(r.Children) != 1 {
			return errors.New("not requires one condition")
		}
		return validate(r.Children[0], depth+1, count)
	case "eq", "gte", "lte", "unknown":
		switch r.Field {
		case "kind", "status", "base", "genre", "year", "rating", "availability", "service", "list":
		default:
			return errors.New("unknown field")
		}
		if r.Op != "unknown" && r.Value == "" {
			return errors.New("missing condition value")
		}
		if r.Op == "gte" || r.Op == "lte" {
			if r.Field != "year" && r.Field != "rating" {
				return errors.New("comparison requires numeric field")
			}
			if _, e := strconv.Atoi(r.Value); e != nil {
				return errors.New("invalid number")
			}
		}
	default:
		return errors.New("unknown operator")
	}
	return nil
}
func Eval(r Rule, item Item, member Membership) (Truth, error) {
	switch r.Op {
	case "not":
		v, e := Eval(r.Children[0], item, member)
		if v == True {
			return False, e
		}
		if v == False {
			return True, e
		}
		return Unknown, e
	case "and", "or":
		unknown := false
		for _, c := range r.Children {
			v, e := Eval(c, item, member)
			if e != nil {
				return Unknown, e
			}
			if r.Op == "and" && v == False {
				return False, nil
			}
			if r.Op == "or" && v == True {
				return True, nil
			}
			unknown = unknown || v == Unknown
		}
		if unknown {
			return Unknown, nil
		}
		if r.Op == "and" {
			return True, nil
		}
		return False, nil
	}
	if r.Field == "list" && r.Op != "unknown" {
		return member(r.Value, item.ID)
	}
	if r.Field == "availability" && r.Op == "eq" && item.Availability != nil {
		return item.Availability[r.Value], nil
	}
	values, known := item.Values[r.Field]
	if r.Op == "unknown" {
		if !known {
			return True, nil
		}
		return False, nil
	}
	if !known {
		return Unknown, nil
	}
	for _, v := range values {
		if r.Op == "eq" && v == r.Value {
			return True, nil
		}
		if r.Op == "gte" || r.Op == "lte" {
			n, e := strconv.Atoi(v)
			if e != nil {
				return Unknown, nil
			}
			want, e := strconv.Atoi(r.Value)
			if e != nil {
				return Unknown, e
			}
			if (r.Op == "gte" && n >= want) || (r.Op == "lte" && n <= want) {
				return True, nil
			}
		}
	}
	return False, nil
}
func References(r Rule) []string {
	out := []string{}
	if r.Field == "list" && r.Value != "" {
		out = append(out, r.Value)
	}
	for _, c := range r.Children {
		out = append(out, References(c)...)
	}
	return out
}
func Acyclic(graph map[string][]string) error {
	visiting, done := map[string]bool{}, map[string]bool{}
	var walk func(string) error
	walk = func(id string) error {
		if visiting[id] {
			return fmt.Errorf("list dependency cycle at %s", id)
		}
		if done[id] {
			return nil
		}
		visiting[id] = true
		for _, ref := range graph[id] {
			if _, ok := graph[ref]; !ok {
				return fmt.Errorf("referenced list %s not found", ref)
			}
			if e := walk(ref); e != nil {
				return e
			}
		}
		visiting[id] = false
		done[id] = true
		return nil
	}
	for id := range graph {
		if e := walk(id); e != nil {
			return e
		}
	}
	return nil
}

// UsesStatus finds custom-status predicates at any supported nesting depth.
func UsesStatus(r Rule, id string) bool {
	if r.Field == "status" && r.Op != "unknown" && r.Value == id {
		return true
	}
	for _, child := range r.Children {
		if UsesStatus(child, id) {
			return true
		}
	}
	return false
}

// StatusReferences returns exact custom/base status predicates at any depth.
func StatusReferences(r Rule) []string {
	out := []string{}
	if r.Field == "status" && r.Op != "unknown" {
		out = append(out, r.Value)
	}
	for _, child := range r.Children {
		out = append(out, StatusReferences(child)...)
	}
	return out
}
