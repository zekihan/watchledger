package simkl

import (
	"fmt"
	"strings"
	"testing"
)

func listFixture(page, total, limit int, items string) string {
	return fmt.Sprintf(`{"id":42,"name":"Synthetic picks","media_type":"movies","updated_at":"2026-01-01T00:00:00Z","pagination":{"page":%d,"limit":%d,"total_items":%d,"total_pages":%d},"items":%s}`, page, limit, total, (total+limit-1)/limit, items)
}
func TestCustomListPreservesResponseOrder(t *testing.T) {
	b, e := ParseCustomList([]byte(listFixture(1, 2, 50, `[{"title":"B","position":9,"ids":{"simkl_id":2,"tmdb":"20"}},{"title":"A","position":1,"ids":{"simkl":1}}]`)))
	if e != nil || len(b.Members) != 2 || b.Members[0].Key != "movies:2" || b.Members[0].Target != "tmdb:movie:20" {
		t.Fatal(b, e)
	}
}
func TestCustomListRejectsIncompleteAndErrorResponses(t *testing.T) {
	for _, raw := range []string{`{"error":"premium_only","item":{"title":"Upgrade"}}`, `{"lists":[]}`, `[]`, listFixture(1, 2, 1, `[{"title":"A","ids":{"simkl_id":1}}]`), listFixture(1, 2, 50, `[]`), listFixture(1, 2, 50, `[{"title":"A","ids":{"simkl_id":1}},{"title":"B","ids":{"simkl_id":1}}]`)} {
		if _, e := ParseCustomList([]byte(raw)); e == nil {
			t.Fatal("accepted incomplete/error input", raw)
		}
	}
}
func TestCustomListPageBundleAndAnimeIdentity(t *testing.T) {
	one := listFixture(1, 2, 1, `[{"title":"A","ids":{"simkl_id":1}}]`)
	two := listFixture(2, 2, 1, `[{"title":"B","ids":{"simkl_id":2}}]`)
	b, e := ParseCustomList([]byte("[" + two + "," + one + "]"))
	if e != nil || b.Members[0].Key != "movies:1" {
		t.Fatal(b, e)
	}
	if _, e = ParseCustomList([]byte("[" + one + "," + one + "]")); e == nil {
		t.Fatal("duplicate page accepted")
	}
	if _, e = ParseCustomList([]byte("[" + one + "," + strings.Replace(two, "2026-01-01", "2026-01-02", 1) + "]")); e == nil {
		t.Fatal("changed snapshot accepted")
	}
	raw := strings.Replace(listFixture(1, 1, 50, `[{"title":"Native","ids":{"simkl_id":1,"tmdb":"20"}}]`), `"movies"`, `"anime"`, 1)
	b, e = ParseCustomList([]byte(raw))
	if e != nil || b.Members[0].Target != "" {
		t.Fatal("anime merged through shared TMDB ID", e)
	}
}
