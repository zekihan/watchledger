package httpapi

import (
	"encoding/json"
	"github.com/zekihan/watchledger/internal/auth"
	"github.com/zekihan/watchledger/internal/smart"
	"net/http"
	"sort"
	"strconv"
)

type savedList struct {
	ID      string      `json:"id"`
	Name    string      `json:"name"`
	Kind    string      `json:"kind"`
	Rule    *smart.Rule `json:"rule"`
	Sort    string      `json:"sort"`
	Members []string    `json:"members"`
}

func (a *API) listRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/lists", a.lists)
	m.HandleFunc("POST /api/lists", a.saveList)
	m.HandleFunc("DELETE /api/lists/{id}", a.deleteList)
	m.HandleFunc("POST /api/lists/preview", a.previewList)
}
func (a *API) lists(w http.ResponseWriter, r *http.Request) {
	rows, e := a.DB.Query(r.Context(), `SELECT jsonb_build_object('id',l.id,'name',l.name,'kind',l.kind,'rule',l.rule,'sort',l.sort,'members',coalesce((SELECT jsonb_agg(catalog_id ORDER BY position) FROM list_members m WHERE m.user_id=l.user_id AND m.list_id=l.id),'[]'::jsonb),'dependents',coalesce((SELECT jsonb_agg(list_id) FROM list_dependencies d WHERE d.user_id=l.user_id AND d.depends_on=l.id),'[]'::jsonb)) FROM lists l WHERE l.user_id=$1 ORDER BY name`, auth.Current(r.Context()).ID)
	if e != nil {
		http.Error(w, "lists unavailable", http.StatusServiceUnavailable)
		return
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var b json.RawMessage
		if e = rows.Scan(&b); e != nil {
			http.Error(w, "lists unavailable", http.StatusServiceUnavailable)
			return
		}
		out = append(out, b)
	}
	if rows.Err() != nil {
		http.Error(w, "lists unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, out)
}
func (a *API) saveList(w http.ResponseWriter, r *http.Request) {
	var in savedList
	if !decode(w, r, &in) {
		return
	}
	if in.Name == "" || len(in.Name) > 100 || (in.Kind != "manual" && in.Kind != "smart") {
		http.Error(w, "invalid list", http.StatusBadRequest)
		return
	}
	switch in.Sort {
	case "title", "release", "rating", "added":
	default:
		http.Error(w, "invalid sorting", http.StatusBadRequest)
		return
	}
	if in.Kind == "smart" {
		if in.Rule == nil {
			http.Error(w, "rule required", http.StatusBadRequest)
			return
		}
		if e := smart.Validate(*in.Rule); e != nil {
			http.Error(w, e.Error(), http.StatusBadRequest)
			return
		}
	} else {
		in.Rule = nil
	}
	uid := auth.Current(r.Context()).ID
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	if _, e = tx.Exec(r.Context(), "SELECT id FROM users WHERE id=$1 FOR UPDATE", uid); e != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	rule, e := json.Marshal(in.Rule)
	if e != nil {
		http.Error(w, "invalid rule", http.StatusBadRequest)
		return
	}
	if in.ID == "" {
		e = tx.QueryRow(r.Context(), "INSERT INTO lists(user_id,name,kind,rule,sort) VALUES($1,$2,$3,$4,$5) RETURNING id", uid, in.Name, in.Kind, rule, in.Sort).Scan(&in.ID)
	} else {
		e = tx.QueryRow(r.Context(), "UPDATE lists SET name=$3,kind=$4,rule=$5,sort=$6,updated_at=now() WHERE user_id=$1 AND id=$2 RETURNING id", uid, in.ID, in.Name, in.Kind, rule, in.Sort).Scan(&in.ID)
	}
	if e != nil {
		http.Error(w, "list unavailable", http.StatusNotFound)
		return
	}
	graph := map[string][]string{}
	rows, e := tx.Query(r.Context(), "SELECT id,rule FROM lists WHERE user_id=$1", uid)
	if e != nil {
		http.Error(w, "lists unavailable", http.StatusServiceUnavailable)
		return
	}
	for rows.Next() {
		var id string
		var raw []byte
		if e = rows.Scan(&id, &raw); e != nil {
			rows.Close()
			http.Error(w, "lists unavailable", http.StatusServiceUnavailable)
			return
		}
		var rule smart.Rule
		_ = json.Unmarshal(raw, &rule)
		graph[id] = smart.References(rule)
	}
	rows.Close()
	if rows.Err() != nil {
		http.Error(w, "lists unavailable", http.StatusServiceUnavailable)
		return
	}
	if e = smart.Acyclic(graph); e != nil {
		http.Error(w, e.Error(), http.StatusConflict)
		return
	}
	if _, e = tx.Exec(r.Context(), "DELETE FROM list_dependencies WHERE user_id=$1 AND list_id=$2", uid, in.ID); e != nil {
		http.Error(w, "list update failed", http.StatusConflict)
		return
	}
	for _, ref := range graph[in.ID] {
		if _, e = tx.Exec(r.Context(), "INSERT INTO list_dependencies(user_id,list_id,depends_on) VALUES($1,$2,$3) ON CONFLICT DO NOTHING", uid, in.ID, ref); e != nil {
			http.Error(w, "invalid reference", http.StatusConflict)
			return
		}
	}
	if _, e = tx.Exec(r.Context(), "DELETE FROM list_members WHERE user_id=$1 AND list_id=$2", uid, in.ID); e != nil {
		http.Error(w, "list update failed", http.StatusConflict)
		return
	}
	if in.Kind == "manual" {
		for n, id := range in.Members {
			if _, e = tx.Exec(r.Context(), "INSERT INTO list_members(user_id,list_id,catalog_id,position) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING", uid, in.ID, id, n); e != nil {
				http.Error(w, "member must belong to your library", http.StatusConflict)
				return
			}
		}
	}
	if e = tx.Commit(r.Context()); e != nil {
		http.Error(w, "list update failed", http.StatusConflict)
		return
	}
	writeJSON(w, map[string]string{"id": in.ID})
}
func (a *API) deleteList(w http.ResponseWriter, r *http.Request) {
	if _, e := a.DB.Exec(r.Context(), "DELETE FROM lists WHERE user_id=$1 AND id=$2", auth.Current(r.Context()).ID, r.PathValue("id")); e != nil {
		http.Error(w, "remove dependent list references before deletion", http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *API) previewList(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Rule smart.Rule `json:"rule"`
		Sort string     `json:"sort"`
	}
	if !decode(w, r, &in) {
		return
	}
	if e := smart.Validate(in.Rule); e != nil {
		http.Error(w, e.Error(), http.StatusBadRequest)
		return
	}
	uid := auth.Current(r.Context()).ID
	rows, e := a.DB.Query(r.Context(), `SELECT l.catalog_id,c.title,c.kind,l.base,coalesce(l.status_id::text,l.base),l.rating,c.metadata,l.added_at::text,coalesce(u.settings->>'country','TR') FROM library l JOIN catalog c ON c.id=l.catalog_id JOIN users u ON u.id=l.user_id WHERE l.user_id=$1`, uid)
	if e != nil {
		http.Error(w, "library unavailable", http.StatusServiceUnavailable)
		return
	}
	items := []smart.Item{}
	titles, release, added := map[string]string{}, map[string]string{}, map[string]string{}
	ratings := map[string]int{}
	country := "TR"
	for rows.Next() {
		var id, title, kind, base, status, add string
		var rating *int
		var raw []byte
		if e = rows.Scan(&id, &title, &kind, &base, &status, &rating, &raw, &add, &country); e != nil {
			rows.Close()
			http.Error(w, "library unavailable", http.StatusServiceUnavailable)
			return
		}
		values := map[string][]string{"kind": {kind}, "base": {base}, "status": {status}}
		if rating != nil {
			values["rating"] = []string{strconv.Itoa(*rating)}
			ratings[id] = *rating
		}
		var translations map[string]struct {
			Genres []struct {
				ID int `json:"id"`
			} `json:"genres"`
			Release string `json:"release_date"`
			First   string `json:"first_air_date"`
		}
		_ = json.Unmarshal(raw, &translations)
		for _, meta := range translations {
			for _, g := range meta.Genres {
				values["genre"] = append(values["genre"], strconv.Itoa(g.ID))
			}
			date := meta.Release
			if date == "" {
				date = meta.First
			}
			if len(date) >= 4 {
				values["year"] = []string{date[:4]}
				release[id] = date
			}
			break
		}
		items = append(items, smart.Item{ID: id, Values: values})
		titles[id] = title
		added[id] = add
	}
	rows.Close()
	if rows.Err() != nil {
		http.Error(w, "library unavailable", http.StatusServiceUnavailable)
		return
	}
	rules := map[string]smart.Rule{}
	members := map[string]map[string]bool{}
	graph := map[string][]string{}
	rows, e = a.DB.Query(r.Context(), "SELECT id,kind,rule FROM lists WHERE user_id=$1", uid)
	if e != nil {
		http.Error(w, "lists unavailable", http.StatusServiceUnavailable)
		return
	}
	for rows.Next() {
		var id, kind string
		var raw []byte
		if e = rows.Scan(&id, &kind, &raw); e != nil {
			rows.Close()
			http.Error(w, "lists unavailable", http.StatusServiceUnavailable)
			return
		}
		graph[id] = nil
		if kind == "smart" {
			var rule smart.Rule
			if json.Unmarshal(raw, &rule) != nil || smart.Validate(rule) != nil {
				rows.Close()
				http.Error(w, "invalid saved rule", http.StatusConflict)
				return
			}
			rules[id] = rule
			graph[id] = smart.References(rule)
		} else {
			members[id] = map[string]bool{}
		}
	}
	rows.Close()
	if rows.Err() != nil {
		http.Error(w, "lists unavailable", http.StatusServiceUnavailable)
		return
	}
	for _, ref := range smart.References(in.Rule) {
		if _, ok := graph[ref]; !ok {
			http.Error(w, "referenced list not found", http.StatusBadRequest)
			return
		}
	}
	if e = smart.Acyclic(graph); e != nil {
		http.Error(w, e.Error(), http.StatusConflict)
		return
	}
	rows, e = a.DB.Query(r.Context(), "SELECT list_id,catalog_id FROM list_members WHERE user_id=$1", uid)
	if e != nil {
		http.Error(w, "lists unavailable", http.StatusServiceUnavailable)
		return
	}
	for rows.Next() {
		var l, id string
		if e = rows.Scan(&l, &id); e != nil {
			rows.Close()
			http.Error(w, "lists unavailable", http.StatusServiceUnavailable)
			return
		}
		if members[l] != nil {
			members[l][id] = true
		}
	}
	rows.Close()
	if rows.Err() != nil {
		http.Error(w, "lists unavailable", http.StatusServiceUnavailable)
		return
	}
	out := []map[string]string{}
	for _, item := range items {
		var offers []byte
		if e = a.DB.QueryRow(r.Context(), "SELECT offers FROM availability WHERE catalog_id=$1 AND country=$2", item.ID, country).Scan(&offers); e == nil && string(offers) != "null" {
			var data map[string][]struct {
				ID int `json:"provider_id"`
			}
			var fields map[string]json.RawMessage
			if json.Unmarshal(offers, &fields) == nil {
				for _, key := range []string{"flatrate", "rent", "buy"} {
					var vals []struct {
						ID int `json:"provider_id"`
					}
					if raw, ok := fields[key]; ok && json.Unmarshal(raw, &vals) == nil {
						if data == nil {
							data = map[string][]struct {
								ID int `json:"provider_id"`
							}{}
						}
						data[key] = vals
					}
				}
			}
			item.Values["availability"] = []string{country}
			item.Values["service"] = []string{}
			for _, vals := range data {
				for _, v := range vals {
					item.Values["service"] = append(item.Values["service"], strconv.Itoa(v.ID))
				}
			}
		}
		var member smart.Membership
		member = func(list, id string) (smart.Truth, error) {
			if set, ok := members[list]; ok {
				if set[id] {
					return smart.True, nil
				}
				return smart.False, nil
			}
			return smart.Eval(rules[list], item, member)
		}
		v, err := smart.Eval(in.Rule, item, member)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if v == smart.True {
			out = append(out, map[string]string{"id": item.ID, "title": titles[item.ID]})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i]["id"], out[j]["id"]
		switch in.Sort {
		case "rating":
			if ratings[a] != ratings[b] {
				return ratings[a] > ratings[b]
			}
		case "release":
			if release[a] != release[b] {
				return release[a] > release[b]
			}
		case "added":
			if added[a] != added[b] {
				return added[a] > added[b]
			}
		}
		return titles[a] < titles[b]
	})
	writeJSON(w, map[string]interface{}{"count": len(out), "results": out})
}
