// Package sqlite implements the storage interface using SQLite.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	pastesqlite "github.com/sentiolabs/arc/internal/paste/sqlite"
	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/storage/sqlite/db"

	_ "modernc.org/sqlite"
)

// dirPermissions is the file mode for directories created by the store.
const dirPermissions = 0o755

// Store implements the storage.Storage interface using SQLite.
type Store struct {
	db      *sql.DB
	queries *db.Queries
	path    string
}

// New creates a new SQLite store at the given path.
// If the path is empty, uses ~/.arc/data.db
func New(path string) (*Store, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}
		path = filepath.Join(home, ".arc", "data.db")
	}

	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirPermissions); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	// Open database
	sqlDB, err := sql.Open("sqlite", path+"?_busy_timeout=5000&_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Ensure foreign keys are enforced (DSN param alone is unreliable with some drivers)
	if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxOpenConns(1) // SQLite doesn't handle concurrent writes well
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(time.Hour)

	store := &Store{
		db:      sqlDB,
		queries: db.New(sqlDB),
		path:    path,
	}

	// Initialize schema
	if err := store.initSchema(context.Background()); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	// Clean up orphaned workspaces from before foreign keys were enforced
	_, _ = sqlDB.Exec("DELETE FROM workspaces WHERE project_id NOT IN (SELECT id FROM projects)")

	// Populate FTS5 search index
	store.populateFTS(context.Background())

	return store, nil
}

// initSchema backs up the database, then runs all pending migrations.
// If a migration fails and a backup exists, the database is restored
// to its pre-migration state.
func (s *Store) initSchema(ctx context.Context) error {
	backupPath, err := backupForMigration(s.db, s.path)
	if err != nil {
		// Non-fatal: migrating without a backup is better than not migrating
		log.Printf("warning: could not back up database before migration: %v", err)
	}

	if err := RunMigrations(s.db); err != nil {
		if backupPath != "" {
			log.Printf("migration failed, restoring backup: %v", err)
			// Close the connection before overwriting the file
			_ = s.db.Close()
			if restoreErr := restoreBackup(s.path, backupPath); restoreErr != nil {
				return fmt.Errorf(
					"migration failed and restore also failed: migration: %w, restore: %w",
					err, restoreErr,
				)
			}
			return fmt.Errorf("migration failed (database restored to pre-migration state): %w", err)
		}
		return err
	}

	// Migration succeeded — clean up backup
	if backupPath != "" {
		_ = os.Remove(backupPath)
	}

	// Apply paste subsystem migrations on the same database.
	if err := pastesqlite.Apply(ctx, s.db); err != nil {
		return fmt.Errorf("apply paste migrations: %w", err)
	}

	return nil
}

// DB returns the underlying *sql.DB connection.
// It is used by callers that need direct database access (e.g. to register
// additional migration-based subsystems such as the paste package).
func (s *Store) DB() *sql.DB {
	return s.db
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// Path returns the database file path.
func (s *Store) Path() string {
	return s.path
}

// Ensure Store implements storage.Storage
var _ storage.Storage = (*Store)(nil)
