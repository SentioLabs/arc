// Package sqlite implements the storage interface using SQLite.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

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

	return store, nil
}

// initSchema runs all database migrations.
func (s *Store) initSchema(_ context.Context) error {
	return RunMigrations(s.db)
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
