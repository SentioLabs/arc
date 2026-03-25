-- +goose Up
-- Drop assignee column from issues table
-- SQLite doesn't support DROP COLUMN directly, so we recreate the table
--
-- Defer FK checks: the data copy triggers FK re-validation on the new table,
-- but the referenced projects already exist. PRAGMA defer_foreign_keys defers
-- the check to transaction commit (unlike foreign_keys, it works inside a tx).
PRAGMA defer_foreign_keys = ON;

CREATE TABLE issues_new (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'open',
    priority INTEGER NOT NULL DEFAULT 2,
    issue_type TEXT NOT NULL DEFAULT 'task',
    ai_session_id TEXT,
    external_ref TEXT,
    rank INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    closed_at TIMESTAMP,
    close_reason TEXT,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

INSERT INTO issues_new (id, project_id, title, description, status, priority, issue_type, ai_session_id, external_ref, rank, created_at, updated_at, closed_at, close_reason)
SELECT id, project_id, title, description, status, priority, issue_type, ai_session_id, external_ref, rank, created_at, updated_at, closed_at, close_reason
FROM issues;

DROP TABLE issues;
ALTER TABLE issues_new RENAME TO issues;

CREATE INDEX idx_issues_project ON issues(project_id);
CREATE INDEX idx_issues_status ON issues(project_id, status);
CREATE INDEX idx_issues_priority ON issues(project_id, priority);
CREATE INDEX idx_issues_type ON issues(project_id, issue_type);
CREATE INDEX idx_issues_updated ON issues(project_id, updated_at DESC);

-- Rebuild FTS5 index: the content-synced FTS table (content=issues) references
-- the old issues table which was dropped. Recreate it pointing at the new table.
DROP TABLE IF EXISTS issues_fts;
CREATE VIRTUAL TABLE issues_fts USING fts5(id, title, description, content=issues, content_rowid=rowid);
INSERT INTO issues_fts(id, title, description) SELECT id, title, COALESCE(description, '') FROM issues;

-- +goose Down
ALTER TABLE issues ADD COLUMN assignee TEXT;
CREATE INDEX idx_issues_assignee ON issues(project_id, assignee);
