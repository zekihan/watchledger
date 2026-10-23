package store

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestMigrationsAndIsolationConstraints(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not configured")
	}
	ctx := context.Background()
	p, e := Open(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	if e = Migrate(ctx, p); e != nil {
		t.Fatal(e)
	}
	if e = Migrate(ctx, p); e != nil {
		t.Fatal("idempotent migration", e)
	}
	tx, e := p.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var a, b, status string
	if e = tx.QueryRow(ctx, "INSERT INTO users(issuer,subject) VALUES('test','a') RETURNING id").Scan(&a); e != nil {
		t.Fatal(e)
	}
	if e = tx.QueryRow(ctx, "INSERT INTO users(issuer,subject) VALUES('test','b') RETURNING id").Scan(&b); e != nil {
		t.Fatal(e)
	}
	if e = tx.QueryRow(ctx, "INSERT INTO statuses(user_id,name,base) VALUES($1,'Custom','watching') RETURNING id", a).Scan(&status); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, "INSERT INTO catalog(id,source,source_id,kind,title) VALUES('fixture','fixture','1','tv','Synthetic')"); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, "INSERT INTO library(user_id,catalog_id,status_id) VALUES($1,'fixture',$2)", b, status); e == nil {
		t.Fatal("cross-account status accepted")
	}
}

func TestRejectNonUTF8Database(t *testing.T) {
	dsn := os.Getenv("TEST_NON_UTF8_DATABASE_URL")
	if dsn == "" {
		t.Skip("optional non-UTF8 fixture not configured")
	}
	p, e := Open(context.Background(), dsn)
	if p != nil {
		p.Close()
	}
	if e == nil || !strings.Contains(e.Error(), "database encoding must be UTF8") {
		t.Fatal("non-UTF8 database accepted", e)
	}
}
