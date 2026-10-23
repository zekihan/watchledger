package animap

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"testing"
)

const fixture = `{"version":2,"generated_at":"2026-10-21T00:00:00Z","sources":{"mapping":{"revision":"synthetic"},"anidb":{"titles_dump":"2026-10-21"}},"attribution":{"license":"ODbL-1.0","license_url":"https://opendatacommons.org/licenses/odbl/1-0/","contents_license":"DbCL-1.0","contents_license_url":"https://opendatacommons.org/licenses/dbcl/1-0/","notice":"Synthetic test mapping"},"anime":[{"anidb_id":42,"type":"TV","tvdb_id":90,"last":{"regular":2},"episodes_listed":true,"episodes":[{"kind":"regular","start":1,"end":2,"anidb_episode_ids":[501,502],"tvdb":[{"season":9,"episode":20,"episode_ids":[701,702]}]}]}],"entries":[]}`

func TestDatasetIdentityAndDefaults(t *testing.T) {
	checksum := fmt.Sprintf("%x", sha256.Sum256([]byte(fixture)))
	doc, summary, err := Decode([]byte(fixture), "vsynthetic", checksum)
	if err != nil || summary.Revision != "synthetic" {
		t.Fatal(summary, err)
	}
	candidates, err := Expand(doc.Anime[0])
	if err != nil || len(candidates) != 2 || candidates[1].SourceEpisode != "anidb:episode:502" || candidates[1].TVDBEpisodeID != 702 || candidates[1].TVDBNumber != 21 {
		t.Fatal(candidates, err)
	}
	(*doc.Anime[0].Episodes[0].TVDB)[0].Default = true
	candidates, err = Expand(doc.Anime[0])
	if err != nil || candidates[0].Reason != "inferred_default_requires_manual_review" {
		t.Fatal(candidates, err)
	}
	if _, _, err = Decode([]byte(fixture), "vsynthetic", strings.Repeat("0", 64)); err == nil {
		t.Fatal("checksum mismatch accepted")
	}
	bad := strings.Replace(fixture, `"version":2`, `"version":3`, 1)
	if _, _, err = Decode([]byte(bad), "vsynthetic", fmt.Sprintf("%x", sha256.Sum256([]byte(bad)))); err == nil {
		t.Fatal("unknown format accepted")
	}
}
func TestOperatorMappingDataset(t *testing.T) {
	path := os.Getenv("ANIMAP_FIXTURE")
	if path == "" {
		t.Skip("ANIMAP_FIXTURE not set")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc, summary, err := Decode(raw, "v2026.10.21", "57d291192bd647657d093617d3164f07db87392f46d9bff3e5bb1d0b7fc4ca47")
	if err != nil || summary.AnimeCount == 0 {
		t.Fatal(summary, err)
	}
	for _, anime := range doc.Anime {
		if _, err = Expand(anime); err != nil {
			t.Fatal(anime.AniDBID, err)
		}
	}
}
