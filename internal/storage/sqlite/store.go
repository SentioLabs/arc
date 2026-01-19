// Package sqlite implements the storage interface using SQLite.
package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/storage/sqlite/db"

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

// initSchema creates the database tables if they don't exist.
func (s *Store) initSchema(ctx context.Context) error {
	schema := `
-- Workspaces table
CREATE TABLE IF NOT EXISTS workspaces (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    path TEXT,
    description TEXT,
    prefix TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Issues table
CREATE TABLE IF NOT EXISTS issues (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    acceptance_criteria TEXT,
    notes TEXT,
    status TEXT NOT NULL DEFAULT 'open',
    priority INTEGER NOT NULL DEFAULT 2,
    issue_type TEXT NOT NULL DEFAULT 'task',
    assignee TEXT,
    external_ref TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    closed_at TIMESTAMP,
    close_reason TEXT,
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_issues_workspace ON issues(workspace_id);
CREATE INDEX IF NOT EXISTS idx_issues_status ON issues(workspace_id, status);
CREATE INDEX IF NOT EXISTS idx_issues_priority ON issues(workspace_id, priority);
CREATE INDEX IF NOT EXISTS idx_issues_assignee ON issues(workspace_id, assignee);
CREATE INDEX IF NOT EXISTS idx_issues_type ON issues(workspace_id, issue_type);
CREATE INDEX IF NOT EXISTS idx_issues_updated ON issues(workspace_id, updated_at DESC);

-- Dependencies table
CREATE TABLE IF NOT EXISTS dependencies (
    issue_id TEXT NOT NULL,
    depends_on_id TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'blocks',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by TEXT,
    PRIMARY KEY (issue_id, depends_on_id),
    FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE,
    FOREIGN KEY (depends_on_id) REFERENCES issues(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_dependencies_issue ON dependencies(issue_id);
CREATE INDEX IF NOT EXISTS idx_dependencies_depends_on ON dependencies(depends_on_id);

-- Labels definition table
CREATE TABLE IF NOT EXISTS labels (
    workspace_id TEXT NOT NULL,
    name TEXT NOT NULL,
    color TEXT,
    description TEXT,
    PRIMARY KEY (workspace_id, name),
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
);

-- Issue-label associations
CREATE TABLE IF NOT EXISTS issue_labels (
    issue_id TEXT NOT NULL,
    label TEXT NOT NULL,
    PRIMARY KEY (issue_id, label),
    FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_issue_labels_label ON issue_labels(label);

-- Comments table
CREATE TABLE IF NOT EXISTS comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    issue_id TEXT NOT NULL,
    author TEXT NOT NULL,
    text TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP,
    FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_comments_issue ON comments(issue_id);

-- Events table
CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    issue_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    actor TEXT NOT NULL,
    old_value TEXT,
    new_value TEXT,
    comment TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_events_issue ON events(issue_id);

-- Config table
CREATE TABLE IF NOT EXISTS config (
    workspace_id TEXT NOT NULL,
    key TEXT NOT NULL,
    value TEXT,
    PRIMARY KEY (workspace_id, key),
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
);

-- Global config table
CREATE TABLE IF NOT EXISTS global_config (
    key TEXT PRIMARY KEY,
    value TEXT
);
`
	_, err := s.db.ExecContext(ctx, schema)
	return err
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// Path returns the database file path.
func (s *Store) Path() string {
	return s.path
}

// generateID creates a short hash ID with the given prefix.
func generateID(prefix string, content string) string {
	h := sha256.New()
	h.Write([]byte(content))
	h.Write([]byte(time.Now().String()))
	hash := hex.EncodeToString(h.Sum(nil))
	return prefix + "-" + hash[:8]
}

// Ensure Store implements storage.Storage
var _ storage.Storage = (*Store)(nil)
