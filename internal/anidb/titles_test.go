package anidb

import (
	"bytes"
	"compress/gzip"
	"os"
	"testing"
)

func TestTitleArchiveValidationAndAliases(t *testing.T) {
	raw := []byte(`<animetitles><anime aid="42"><title xml:lang="en" type="official">Synthetic &amp; Alias</title><title xml:lang="ja" type="main">架空</title></anime></animetitles>`)
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	_, _ = gz.Write(raw)
	_ = gz.Close()
	for _, data := range [][]byte{raw, b.Bytes()} {
		entries, err := ParseTitles(data)
		if err != nil || len(entries) != 1 || entries[0].ID != "42" || entries[0].Titles[0].Text != "Synthetic & Alias" {
			t.Fatal(entries, err)
		}
	}
	for _, data := range []string{"<html>denied</html>", "<animetitles/>", `<animetitles><anime aid="x"><title>Bad</title></anime></animetitles>`, string(raw) + "<extra/>"} {
		if _, err := ParseTitles([]byte(data)); err == nil {
			t.Fatal("invalid index accepted")
		}
	}
}
func TestOperatorTitleArchive(t *testing.T) {
	path := os.Getenv("ANIDB_TITLE_FIXTURE")
	if path == "" {
		t.Skip("ANIDB_TITLE_FIXTURE not set")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := ParseTitles(raw)
	if err != nil || len(entries) == 0 {
		t.Fatal("operator title archive", err)
	}
}
