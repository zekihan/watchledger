package anidb

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type TitleEntry struct {
	ID     string  `xml:"aid,attr"`
	Titles []Title `xml:"title"`
}

func ParseTitles(raw []byte) ([]TitleEntry, error) {
	if len(raw) > 16<<20 {
		return nil, errors.New("title archive exceeds 16 MiB")
	}
	reader := io.Reader(bytes.NewReader(raw))
	if len(raw) > 2 && raw[0] == 0x1f && raw[1] == 0x8b {
		gz, err := gzip.NewReader(reader)
		if err != nil {
			return nil, errors.New("invalid title compression")
		}
		defer func() { _ = gz.Close() }()
		reader = gz
	}
	data, err := io.ReadAll(io.LimitReader(reader, (64<<20)+1))
	if err != nil || len(data) > 64<<20 {
		return nil, errors.New("title index exceeds 64 MiB")
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var doc struct {
		XMLName xml.Name     `xml:"animetitles"`
		Entries []TitleEntry `xml:"anime"`
	}
	if err = decoder.Decode(&doc); err != nil {
		return nil, errors.New("invalid AniDB title index XML")
	}
	var extra interface{}
	if err = decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("unexpected title index content")
	}
	if len(doc.Entries) == 0 || len(doc.Entries) > 200000 {
		return nil, errors.New("empty or oversized title index")
	}
	seen := map[string]bool{}
	for i, entry := range doc.Entries {
		id, e := strconv.ParseInt(entry.ID, 10, 64)
		if e != nil || id < 1 || strconv.FormatInt(id, 10) != entry.ID || seen[entry.ID] || len(entry.Titles) == 0 || len(entry.Titles) > 1000 {
			return nil, errors.New("invalid title identity")
		}
		seen[entry.ID] = true
		titles := []Title{}
		unique := map[string]bool{}
		for _, title := range entry.Titles {
			if strings.TrimSpace(title.Text) == "" || len(title.Text) > 8000 || len(title.Language) > 40 || len(title.Type) > 40 {
				return nil, errors.New("invalid title alias")
			}
			key := title.Language + "\x00" + title.Type + "\x00" + title.Text
			if !unique[key] {
				titles = append(titles, title)
				unique[key] = true
			}
		}
		doc.Entries[i].Titles = titles
	}
	return doc.Entries, nil
}

type IndexState struct {
	Checksum   string    `json:"checksum"`
	Entries    int       `json:"entries"`
	ImportedAt time.Time `json:"imported_at"`
}

func (c *Cache) ImportTitles(ctx context.Context, raw []byte) (IndexState, error) {
	entries, err := ParseTitles(raw)
	if err != nil {
		return IndexState{}, err
	}
	sum := sha256.Sum256(raw)
	state := IndexState{Checksum: hex.EncodeToString(sum[:]), Entries: len(entries)}
	tx, err := c.DB.Begin(ctx)
	if err != nil {
		return state, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(74192302)"); err != nil {
		return state, err
	}
	var existing IndexState
	if err = tx.QueryRow(ctx, "SELECT checksum,entry_count,imported_at FROM anidb_title_index_state WHERE singleton").Scan(&existing.Checksum, &existing.Entries, &existing.ImportedAt); err == nil && existing.Checksum == state.Checksum {
		return existing, nil
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return state, err
	}
	values := make([][]interface{}, 0, len(entries))
	for _, entry := range entries {
		raw, e := json.Marshal(entry.Titles)
		if e != nil {
			return state, e
		}
		values = append(values, []interface{}{entry.ID, raw})
	}
	if _, err = tx.Exec(ctx, "DELETE FROM anidb_title_index"); err != nil {
		return state, err
	}
	if _, err = tx.CopyFrom(ctx, pgx.Identifier{"anidb_title_index"}, []string{"aid", "titles"}, pgx.CopyFromRows(values)); err != nil {
		return state, err
	}
	err = tx.QueryRow(ctx, `INSERT INTO anidb_title_index_state(singleton,checksum,entry_count) VALUES(true,$1,$2) ON CONFLICT(singleton) DO UPDATE SET checksum=EXCLUDED.checksum,entry_count=EXCLUDED.entry_count,imported_at=now() RETURNING imported_at`, state.Checksum, state.Entries).Scan(&state.ImportedAt)
	if err != nil {
		return state, err
	}
	return state, tx.Commit(ctx)
}
