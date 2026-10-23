package portable

import (
	"encoding/json"
	"github.com/zekihan/watchledger/internal/smart"
	"testing"
	"time"
)

func snapshot() Snapshot {
	return Snapshot{Version: 1, Settings: json.RawMessage(`{}`), Catalog: []Catalog{{ID: "tmdb:tv:1", Source: "tmdb", SourceID: "1", Kind: "tv", Title: "Synthetic", Metadata: json.RawMessage(`{}`)}}, Episodes: []Episode{{ID: "e1", Source: "tmdb", SourceID: "1", CatalogID: "tmdb:tv:1"}}, Library: []Library{{CatalogID: "tmdb:tv:1", Base: "watching"}}, History: []History{{CatalogID: "tmdb:tv:1", EpisodeID: ptr("e1")}}}
}
func ptr(s string) *string { return &s }
func TestValidationPreservesUnknownDates(t *testing.T) {
	s := snapshot()
	if e := s.Validate(); e != nil {
		t.Fatal(e)
	}
	b, e := json.Marshal(s)
	if e != nil {
		t.Fatal(e)
	}
	var restored Snapshot
	if e = json.Unmarshal(b, &restored); e != nil {
		t.Fatal(e)
	}
	if restored.History[0].WatchedDate != nil || restored.History[0].WatchedTime != nil {
		t.Fatal("invented date")
	}
	restored.History[0].EpisodeID = ptr("foreign")
	if restored.Validate() == nil {
		t.Fatal("foreign episode accepted")
	}
}
func TestRejectCyclicRestore(t *testing.T) {
	s := snapshot()
	s.Lists = []List{{ID: "a", Name: "a", Kind: "smart", Rule: &smart.Rule{Op: "eq", Field: "list", Value: "a"}}}
	if s.Validate() == nil {
		t.Fatal("cycle accepted")
	}
}

func TestValidateSIMKLIdentityLinks(t *testing.T) {
	s := snapshot()
	s.SIMKLLinks = []SIMKLLink{{SourceKey: "shows:12:s1:e3", CatalogID: "tmdb:tv:1", EpisodeID: "e1"}}
	if e := s.Validate(); e != nil {
		t.Fatal(e)
	}
	s.SIMKLLinks = append(s.SIMKLLinks, s.SIMKLLinks[0])
	if s.Validate() == nil {
		t.Fatal("duplicate source identity accepted")
	}
	s.SIMKLLinks = s.SIMKLLinks[:1]
	s.SIMKLLinks[0].EpisodeID = "foreign"
	if s.Validate() == nil {
		t.Fatal("foreign episode link accepted")
	}
	s.SIMKLLinks[0].EpisodeID = "e1"
	s.SIMKLLinks[0].SourceKey = "arbitrary"
	if s.Validate() == nil {
		t.Fatal("invalid source identity accepted")
	}
}

func TestValidateCustomListSourceIdentity(t *testing.T) {
	s := snapshot()
	s.Lists = []List{{ID: "a", Name: "Synthetic", Kind: "manual", Source: ptr("simkl-custom-list"), SourceID: ptr("42")}}
	if e := s.Validate(); e != nil {
		t.Fatal(e)
	}
	s.Lists = append(s.Lists, List{ID: "b", Name: "Other", Kind: "manual", Source: ptr("simkl-custom-list"), SourceID: ptr("42")})
	if s.Validate() == nil {
		t.Fatal("duplicate imported list identity accepted")
	}
	s.Lists = s.Lists[:1]
	s.Lists[0].SourceID = nil
	if s.Validate() == nil {
		t.Fatal("partial provenance accepted")
	}
}

func TestValidateCustomListDeletionMarkers(t *testing.T) {
	s := snapshot()
	marker := ListSourceTombstone{Source: "simkl-custom-list", SourceID: "42", DeletedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)}
	s.ListSourceTombstones = []ListSourceTombstone{marker}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*ListSourceTombstone){
		"unknown source":  func(m *ListSourceTombstone) { m.Source = "other" },
		"noncanonical ID": func(m *ListSourceTombstone) { m.SourceID = "042" },
		"missing date":    func(m *ListSourceTombstone) { m.DeletedAt = time.Time{} },
	} {
		t.Run(name, func(t *testing.T) {
			bad := marker
			change(&bad)
			s.ListSourceTombstones = []ListSourceTombstone{bad}
			if s.Validate() == nil {
				t.Fatal("invalid deletion marker accepted")
			}
		})
	}
	s.ListSourceTombstones = []ListSourceTombstone{marker, marker}
	if s.Validate() == nil {
		t.Fatal("duplicate deletion marker accepted")
	}
	s.ListSourceTombstones = []ListSourceTombstone{marker}
	s.Lists = []List{{ID: "a", Name: "Synthetic", Kind: "manual", Source: ptr("simkl-custom-list"), SourceID: ptr("42")}}
	if s.Validate() == nil {
		t.Fatal("live list and deletion accepted together")
	}
}
