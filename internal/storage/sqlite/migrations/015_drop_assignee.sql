-- +goose Up
-- Drop assignee column from issues table
-- SQLite doesn't support DROP COLUMN directly, so we recreate the table

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

-- +goose Down
ALTER TABLE issues ADD COLUMN assignee TEXT;
CREATE INDEX idx_issues_assignee ON issues(project_id, assignee);
