-- +goose Up
-- Drop notes and acceptance_criteria columns (consolidating into description)
-- SQLite doesn't support DROP COLUMN directly, so we recreate the table

CREATE TABLE issues_new (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'open',
    priority INTEGER NOT NULL DEFAULT 2,
    issue_type TEXT NOT NULL DEFAULT 'task',
    assignee TEXT,
    external_ref TEXT,
    rank INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    closed_at DATETIME,
    close_reason TEXT
);

INSERT INTO issues_new (id, workspace_id, title, description, status, priority, issue_type, assignee, external_ref, rank, created_at, updated_at, closed_at, close_reason)
SELECT id, workspace_id, title, description, status, priority, issue_type, assignee, external_ref, rank, created_at, updated_at, closed_at, close_reason
FROM issues;

DROP TABLE issues;
ALTER TABLE issues_new RENAME TO issues;

CREATE INDEX idx_issues_workspace ON issues(workspace_id);
CREATE INDEX idx_issues_status ON issues(status);
CREATE INDEX idx_issues_assignee ON issues(assignee);

-- +goose Down
-- Re-add notes and acceptance_criteria columns
ALTER TABLE issues ADD COLUMN acceptance_criteria TEXT;
ALTER TABLE issues ADD COLUMN notes TEXT;
