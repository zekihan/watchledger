// Package store owns PostgreSQL persistence and transactional schema upgrades.
package store

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zekihan/watchledger/db/migrations"
	"io/fs"
)

func Open(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	p, e := pgxpool.New(ctx, dsn)
	if e != nil {
		return nil, e
	}
	if e = p.Ping(ctx); e != nil {
		p.Close()
		return nil, e
	}
	var encoding string
	if e = p.QueryRow(ctx, "SHOW server_encoding").Scan(&encoding); e != nil {
		p.Close()
		return nil, e
	}
	if encoding != "UTF8" {
		p.Close()
		return nil, fmt.Errorf("database encoding must be UTF8, got %s", encoding)
	}
	return p, nil
}
func Migrate(ctx context.Context, p *pgxpool.Pool) error {
	tx, e := p.Begin(ctx)
	if e != nil {
		return e
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(81270643); CREATE TABLE IF NOT EXISTS schema_migrations(name text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`); e != nil {
		return e
	}
	files, e := fs.Glob(migrations.Files, "*.sql")
	if e != nil {
		return e
	}
	for _, name := range files {
		data, err := migrations.Files.ReadFile(name)
		if err != nil {
			return err
		}
		sum := fmt.Sprintf("%x", sha256.Sum256(data))
		var old string
		var count int
		if err = tx.QueryRow(ctx, "SELECT count(*) FROM schema_migrations WHERE name=$1", name).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			if err = tx.QueryRow(ctx, "SELECT checksum FROM schema_migrations WHERE name=$1", name).Scan(&old); err != nil {
				return err
			}
			if old != sum {
				return fmt.Errorf("migration %s checksum changed", name)
			}
			continue
		}
		if _, err = tx.Exec(ctx, string(data)); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO schema_migrations(name,checksum) VALUES($1,$2)", name, sum); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
