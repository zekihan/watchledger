// Package animap ingests versioned mapping data, never copying private history.
package animap

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"time"

	"github.com/cplieger/animap/mapread"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Dataset struct {
	Checksum    string              `json:"checksum"`
	Version     string              `json:"version"`
	Revision    string              `json:"source_revision"`
	GeneratedAt string              `json:"generated_at"`
	Attribution mapread.Attribution `json:"attribution"`
	AnimeCount  int                 `json:"anime_count"`
}

func Decode(raw []byte, version, checksum string) (*mapread.Document, Dataset, error) {
	summary := Dataset{Version: version}
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,79}$`).MatchString(version) {
		return nil, summary, errors.New("invalid release version")
	}
	sum := sha256.Sum256(raw)
	summary.Checksum = hex.EncodeToString(sum[:])
	if summary.Checksum != checksum {
		return nil, summary, errors.New("mapping dataset checksum mismatch")
	}
	doc, err := mapread.Decode(bytes.NewReader(raw), mapread.DefaultLimits())
	if err != nil {
		return nil, summary, err
	}
	summary.Revision = doc.Sources.Mapping.Revision
	summary.GeneratedAt = doc.GeneratedAt
	summary.Attribution = doc.Attribution
	summary.AnimeCount = len(doc.Anime)
	return doc, summary, nil
}
func Import(ctx context.Context, db *pgxpool.Pool, raw []byte, version, checksum string) (Dataset, error) {
	doc, summary, err := Decode(raw, version, checksum)
	if err != nil {
		return summary, err
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return summary, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	attribution, err := json.Marshal(summary.Attribution)
	if err != nil {
		return summary, err
	}
	generated, err := time.Parse(time.RFC3339, summary.GeneratedAt)
	if err != nil {
		return summary, err
	}
	tag, err := tx.Exec(ctx, `INSERT INTO mapping_datasets(checksum,version,source_revision,generated_at,attribution,document) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, checksum, version, summary.Revision, generated, attribution, raw)
	if err != nil {
		return summary, err
	}
	if tag.RowsAffected() == 0 {
		if err = tx.QueryRow(ctx, "SELECT version FROM mapping_datasets WHERE checksum=$1", checksum).Scan(&summary.Version); err != nil {
			return summary, err
		}
		return summary, nil
	}
	rows := make([][]interface{}, 0, len(doc.Anime))
	for _, entry := range doc.Anime {
		data, e := json.Marshal(entry)
		if e != nil {
			return summary, e
		}
		rows = append(rows, []interface{}{checksum, entry.AniDBID, data})
	}
	if _, err = tx.CopyFrom(ctx, pgx.Identifier{"mapping_dataset_anime"}, []string{"dataset_checksum", "anidb_id", "data"}, pgx.CopyFromRows(rows)); err != nil {
		return summary, err
	}
	return summary, tx.Commit(ctx)
}

type Candidate struct {
	SourceEpisode string `json:"source_episode"`
	NativeNumber  int    `json:"native_number"`
	NativeKind    string `json:"native_kind"`
	TVDBEpisodeID int    `json:"tvdb_episode_id"`
	TVDBSeries    int    `json:"tvdb_series"`
	TVDBSeason    int    `json:"tvdb_season"`
	TVDBNumber    int    `json:"tvdb_number"`
	Inferred      bool   `json:"inferred"`
	Reason        string `json:"reason,omitempty"`
}

func Expand(a mapread.Anime) ([]Candidate, error) {
	total := 0
	for _, run := range a.Episodes {
		targets := 1
		if run.TVDB != nil {
			targets = max(1, len(*run.TVDB))
		}
		total += (run.End - run.Start + 1) * targets
		if total > 10000 {
			return nil, errors.New("mapping entry exceeds 10000 candidate limit")
		}
	}
	out := []Candidate{}
	for _, run := range a.Episodes {
		for offset := 0; offset <= run.End-run.Start; offset++ {
			base := Candidate{NativeNumber: run.Start + offset, NativeKind: string(run.Kind)}
			if len(run.AniDBEpisodeIDs) > offset {
				base.SourceEpisode = "anidb:episode:" + strconv.Itoa(run.AniDBEpisodeIDs[offset])
			}
			if run.TVDB == nil || len(*run.TVDB) == 0 {
				base.Reason = "no_explicit_tv_episode_target"
				if len(run.TMDBMovies) > 0 {
					base.Reason = "movie_relationship_requires_separate_review"
				}
				out = append(out, base)
				continue
			}
			for _, target := range *run.TVDB {
				c := base
				c.TVDBSeries = target.Series
				if c.TVDBSeries == 0 {
					c.TVDBSeries = a.TVDBID
				}
				c.TVDBSeason = target.Season
				c.TVDBNumber = target.Episode + offset
				c.Inferred = target.Default
				if len(target.EpisodeIDs) > offset {
					c.TVDBEpisodeID = target.EpisodeIDs[offset]
				}
				switch {
				case c.SourceEpisode == "":
					c.Reason = "native_episode_identity_missing"
				case c.Inferred:
					c.Reason = "inferred_default_requires_manual_review"
				case c.TVDBEpisodeID == 0:
					c.Reason = "tvdb_episode_identity_missing_no_number_guess"
				}
				out = append(out, c)
			}
		}
	}
	return out, nil
}
