// Package db embeds SQL migration files so they ship inside the binary.
package db

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// MigrationsFS returns the embedded migrations as a filesystem rooted at the
// migrations directory (so entries are named like "000001_foundation.up.sql").
func MigrationsFS() fs.FS {
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		panic(err) // embedded path is a compile-time constant; this cannot fail
	}
	return sub
}
