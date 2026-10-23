// Package migrations contains the versioned database schema.
package migrations

import "embed"

// Files contains migration SQL, applied transactionally in filename order.
//
//go:embed *.sql
var Files embed.FS
