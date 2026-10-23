package portable

import (
	"encoding/json"
	"github.com/zekihan/watchledger/internal/smart"
	"testing"
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
