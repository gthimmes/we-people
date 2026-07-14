package database

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Migration is a single versioned schema change loaded from an embedded .sql file.
type Migration struct {
	Version int
	Name    string
	Up      string
	Down    string
}

// LoadMigrations reads paired `NNNNNN_name.up.sql` / `.down.sql` files from fsys.
func LoadMigrations(fsys fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	byVersion := map[int]*Migration{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		// Expect: 000001_init.up.sql
		parts := strings.SplitN(e.Name(), "_", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("bad migration filename %q", e.Name())
		}
		var version int
		if _, err := fmt.Sscanf(parts[0], "%d", &version); err != nil {
			return nil, fmt.Errorf("bad migration version in %q: %w", e.Name(), err)
		}
		content, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return nil, err
		}
		m := byVersion[version]
		if m == nil {
			m = &Migration{Version: version, Name: strings.TrimSuffix(parts[1], ".up.sql")}
			byVersion[version] = m
		}
		switch {
		case strings.HasSuffix(e.Name(), ".up.sql"):
			m.Up = string(content)
		case strings.HasSuffix(e.Name(), ".down.sql"):
			m.Down = string(content)
		}
	}

	out := make([]Migration, 0, len(byVersion))
	for _, m := range byVersion {
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// MigrateUp applies all migrations that have not yet been recorded, each in its
// own transaction, in version order.
func MigrateUp(ctx context.Context, pool *pgxpool.Pool, migrations []Migration) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    integer PRIMARY KEY,
			name       text NOT NULL,
			applied_at timestamptz NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return err
	}

	for _, m := range migrations {
		if applied[m.Version] {
			continue
		}
		if err := applyOne(ctx, pool, m); err != nil {
			return fmt.Errorf("migration %d (%s): %w", m.Version, m.Name, err)
		}
	}
	return nil
}

func applyOne(ctx context.Context, pool *pgxpool.Pool, m Migration) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, m.Up); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`,
		m.Version, m.Name); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func appliedVersions(ctx context.Context, pool *pgxpool.Pool) (map[int]bool, error) {
	rows, err := pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

// ErrNoMigrations is returned when a rollback is requested but nothing is applied.
var ErrNoMigrations = errors.New("no migrations to roll back")

// MigrateDown rolls back the single most-recently applied migration.
func MigrateDown(ctx context.Context, pool *pgxpool.Pool, migrations []Migration) error {
	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return err
	}
	byVersion := map[int]Migration{}
	for _, m := range migrations {
		byVersion[m.Version] = m
	}
	// Find highest applied version.
	latest := -1
	for v := range applied {
		if v > latest {
			latest = v
		}
	}
	if latest < 0 {
		return ErrNoMigrations
	}
	m, ok := byVersion[latest]
	if !ok {
		return fmt.Errorf("no source for applied migration %d", latest)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if m.Down != "" {
		if _, err := tx.Exec(ctx, m.Down); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM schema_migrations WHERE version = $1`, latest); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
