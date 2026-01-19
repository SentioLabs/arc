// Package sqlite implements repository interfaces using SQLite.
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

	"github.com/sentiolabs/arc/internal/repository"
	"github.com/sentiolabs/arc/internal/storage/sqlite/db"

	_ "modernc.org/sqlite"
)

// Repository provides access to all SQLite repositories.
// It manages the database connection and transaction support.
type Repository struct {
	db      *sql.DB
	queries *db.Queries
	path    string

	workspaces   *WorkspaceRepository
	issues       *IssueRepository
	dependencies *DependencyRepository
	labels       *LabelRepository
	comments     *CommentRepository
	events       *EventRepository
}

// New creates a new SQLite repository at the given path.
// If the path is empty, uses ~/.arc-server/data.db
func New(path string) (*Repository, error) {
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

	queries := db.New(sqlDB)

	repo := &Repository{
		db:      sqlDB,
		queries: queries,
		path:    path,
	}

	// Initialize repositories
	repo.workspaces = &WorkspaceRepository{repo: repo}
	repo.issues = &IssueRepository{repo: repo}
	repo.dependencies = &DependencyRepository{repo: repo}
	repo.labels = &LabelRepository{repo: repo}
	repo.comments = &CommentRepository{repo: repo}
	repo.events = &EventRepository{repo: repo}

	// Initialize schema
	if err := repo.initSchema(context.Background()); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return repo, nil
}

// Repositories returns the container with all repository interfaces.
func (r *Repository) Repositories() *repository.Repositories {
	return &repository.Repositories{
		Workspaces:   r.workspaces,
		Issues:       r.issues,
		Dependencies: r.dependencies,
		Labels:       r.labels,
		Comments:     r.comments,
		Events:       r.events,
	}
}

// Close closes the database connection.
func (r *Repository) Close() error {
	return r.db.Close()
}

// Path returns the database file path.
func (r *Repository) Path() string {
	return r.path
}

// initSchema creates the database tables if they don't exist.
func (r *Repository) initSchema(ctx context.Context) error {
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
	_, err := r.db.ExecContext(ctx, schema)
	return err
}

// generateID creates a short hash ID with the given prefix.
func generateID(prefix string, content string) string {
	h := sha256.New()
	h.Write([]byte(content))
	h.Write([]byte(time.Now().String()))
	hash := hex.EncodeToString(h.Sum(nil))
	return prefix + "-" + hash[:8]
}

// Helper functions for nullable fields
func toNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func fromNullString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func toNullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

func fromNullTime(nt sql.NullTime) *time.Time {
	if nt.Valid {
		return &nt.Time
	}
	return nil
}

func nullStringToPtr(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
}

func ptrToString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// Compile-time interface compliance checks
var (
	_ repository.WorkspaceRepository  = (*WorkspaceRepository)(nil)
	_ repository.IssueRepository      = (*IssueRepository)(nil)
	_ repository.DependencyRepository = (*DependencyRepository)(nil)
	_ repository.LabelRepository      = (*LabelRepository)(nil)
	_ repository.CommentRepository    = (*CommentRepository)(nil)
	_ repository.EventRepository      = (*EventRepository)(nil)
)
