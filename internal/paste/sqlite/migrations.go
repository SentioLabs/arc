package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// migrationsFS holds the embedded *.sql files. Each file is one migration;
// they are applied in filename-sorted order, and the names are recorded in
// the paste_migrations table so we never re-run one.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

// Apply runs every embedded migration that hasn't already been recorded in
// paste_migrations, in lexicographic filename order. Idempotent — safe to call
// on every server boot.
func Apply(ctx context.Context, db *sql.DB) error {
	// The bookkeeping table itself uses CREATE IF NOT EXISTS rather than a
	// migration file so we have somewhere to write the first migration's
	// completion record.
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS paste_migrations (
        name TEXT PRIMARY KEY, applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		return fmt.Errorf("create paste_migrations: %w", err)
	}
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return err
	}
	// Filter to .sql, sort lexicographically — naming convention:
	// 0001_*.sql, 0002_*.sql, ... ensures the order is also chronological.
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		// Skip migrations we've already applied — `name` is the primary key
		// in paste_migrations, so a successful Scan means we're done.
		var existing string
		err := db.QueryRowContext(ctx, `SELECT name FROM paste_migrations WHERE name = ?`, name).Scan(&existing)
		if err == nil {
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		body, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, string(body)); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
		// Record the completion so the next boot skips this file.
		if _, err := db.ExecContext(ctx, `INSERT INTO paste_migrations(name) VALUES (?)`, name); err != nil {
			return err
		}
	}
	return nil
}
