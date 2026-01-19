// Package sqlite implements the storage interface using SQLite.
package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/storage/sqlite/db"
	"github.com/sentiolabs/arc/internal/workspace"

	_ "modernc.org/sqlite"
)

// Store implements the storage.Storage interface using SQLite.
type Store struct {
	db      *sql.DB
	queries *db.Queries
	path    string
}

// New creates a new SQLite store at the given path.
// If the path is empty, uses ~/.arc-server/data.db
func New(path string) (*Store, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}
		path = filepath.Join(home, ".arc-server", "data.db")
	}

	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
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
		sqlDB.Close()
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

// generateID creates a short base36 hash ID with the given prefix.
// Format: prefix-{6-char-base36-hash}
func generateID(prefix string, content string) string {
	h := sha256.Sum256([]byte(content + time.Now().String()))
	// Use first 3 bytes for ~5-6 base36 characters
	encoded := workspace.Base36Encode(h[:3])

	// Pad to exactly 6 chars
	for len(encoded) < 6 {
		encoded = "0" + encoded
	}
	// Trim if longer
	if len(encoded) > 6 {
		encoded = encoded[:6]
	}

	return prefix + "-" + encoded
}

// Ensure Store implements storage.Storage
var _ storage.Storage = (*Store)(nil)
