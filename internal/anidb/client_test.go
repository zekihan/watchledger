package anidb

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

const fixture = `<anime id="42"><type>TV Series</type><episodecount>2</episodecount><titles><title xml:lang="x-jat" type="main">Synthetic Romaji</title><title xml:lang="en" type="official">Synthetic English</title><title xml:lang="ja" type="official">架空</title></titles><episodes><episode id="501"><epno type="1">1</epno><length>24</length><airdate>2026-01-02</airdate><title xml:lang="en">First</title></episode><episode id="502"><epno type="2">S1</epno><length>0</length><title xml:lang="en">Special</title></episode></episodes></anime>`

func TestRegisteredClientAndCompressedXML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("client") != "synthetic" || q.Get("clientver") != "1" || q.Get("protover") != "1" || q.Get("aid") != "42" {
			t.Error("invalid API parameters")
		}
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		_, _ = gz.Write([]byte(fixture))
		_ = gz.Close()
	}))
	defer server.Close()
	c := New(Config{Client: "synthetic", Version: 1})
	c.URL = server.URL
	a, raw, err := c.Fetch(context.Background(), "42")
	if err != nil || len(raw) == 0 || len(a.Episodes) != 2 {
		t.Fatal(a, err)
	}
	if Choose(a.Titles, "en", "romaji") != "Synthetic Romaji" || Choose(a.Titles, "en", "native") != "架空" || Choose(a.Titles, "en", "translated") != "Synthetic English" || Choose(a.Titles, "ja", "english") != "Synthetic English" {
		t.Fatal("title style lost")
	}
	n, k, err := a.Episodes[1].Position()
	if err != nil || n != 1 || k != "special" {
		t.Fatal(n, k, err)
	}
	if _, _, err = New(Config{}).Fetch(context.Background(), "42"); err == nil {
		t.Fatal("unregistered request")
	}
}
func TestErrorsAndPrecision(t *testing.T) {
	if _, err := Parse([]byte(fixture), "43"); err == nil {
		t.Fatal("identity mismatch accepted")
	}
	_, err := Parse([]byte("<error>Banned</error>"), "42")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || !apiErr.Banned() {
		t.Fatal("HTTP200 error treated as metadata", err)
	}
	if Day("2026-00-00") != nil || Day("") != nil || Day("2026-01-02") == nil {
		t.Fatal("invented date precision")
	}
}

// Optional operator-supplied response validates compatibility without another
// network request or committing source content.
func TestOperatorProvidedXML(t *testing.T) {
	path := os.Getenv("ANIDB_XML_FIXTURE")
	if path == "" {
		t.Skip("ANIDB_XML_FIXTURE not set")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	a, err := Parse(raw, "1")
	if err != nil || len(a.Episodes) == 0 {
		t.Fatal("live response parse", err)
	}
}

func TestSourcePosterValidation(t *testing.T) {
	if PosterURL("123.jpg") != "https://cdn-eu.anidb.net/images/main/123.jpg" {
		t.Fatal("native picture path missing")
	}
	for _, value := range []string{"../123.jpg", "https://example.invalid/x.jpg", "123.jpg?token=x", "x) url(evil)"} {
		if PosterURL(value) != "" {
			t.Fatal("unsafe picture path accepted")
		}
	}
}

func TestMetadataPreservesNativeMovieType(t *testing.T) {
	metadata := Metadata(Anime{ID: "1", Type: "Movie", EpisodeCount: 1}, "en", "romaji")
	if metadata["anime_type"] != "Movie" {
		t.Fatal("native movie type omitted", metadata)
	}
}

func TestNativeCreditsRatingsRelationsAndNonSpoilerTags(t *testing.T) {
	raw := []byte(`<anime id="42"><titles><title xml:lang="en" type="official">Synthetic work</title></titles><creators><name id="7" type="Direction">Synthetic director</name><name id="7" type="Direction">Synthetic director</name><name id="../8" type="Music">Invalid</name></creators><characters><character><name>First role</name><seiyuu id="9" picture="10.jpeg">Synthetic voice</seiyuu></character><character><name>Second role</name><seiyuu id="9" picture="10.jpeg">Synthetic voice</seiyuu></character></characters><relatedanime><anime id="43" type="Sequel">Synthetic follow-up</anime><anime id="bad">Invalid</anime></relatedanime><ratings><permanent count="25">8.5</permanent></ratings><tags><tag id="1" weight="200" verified="true" localspoiler="false" globalspoiler="false"><name>Safe source tag</name></tag><tag id="2" weight="200" verified="true" localspoiler="true" globalspoiler="false"><name>Hidden spoiler</name></tag><tag id="3" weight="200" verified="true"><name>Unknown spoiler status</name></tag></tags></anime>`)
	a, err := Parse(raw, "42")
	if err != nil {
		t.Fatal(err)
	}
	credits := Credits(a)
	if len(credits.Crew) != 1 || len(credits.Cast) != 1 || credits.Cast[0].Character != "First role, Second role" || credits.Cast[0].Source != "anidb" || credits.Cast[0].ProfilePath != "https://cdn-eu.anidb.net/images/main/10.jpeg" {
		t.Fatal(credits)
	}
	metadata := Metadata(a, "en", "english")
	if metadata["vote_average"] != 8.5 || metadata["vote_count"] != 25 {
		t.Fatal(metadata)
	}
	rawMeta, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	text := string(rawMeta)
	if !strings.Contains(text, "Synthetic follow-up") || !strings.Contains(text, "anidb:tag:1") || strings.Contains(text, "Hidden spoiler") || strings.Contains(text, "Unknown spoiler status") || strings.Contains(text, "\"genres\"") {
		t.Fatal(text)
	}
}
