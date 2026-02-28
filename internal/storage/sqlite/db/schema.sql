-- Reference schema for sqlc code generation.
-- Applied at runtime via golang-migrate (see migrations/*.sql).
-- Keep this in sync with the migration files.

-- Workspaces table
CREATE TABLE workspaces (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    path TEXT,
    description TEXT,
    prefix TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Issues table
CREATE TABLE issues (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'open',
    priority INTEGER NOT NULL DEFAULT 2,
    issue_type TEXT NOT NULL DEFAULT 'task',
    assignee TEXT,
    external_ref TEXT,
    rank INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    closed_at TIMESTAMP,
    close_reason TEXT,
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
);

-- Index for common queries
CREATE INDEX idx_issues_workspace ON issues(workspace_id);
CREATE INDEX idx_issues_status ON issues(workspace_id, status);
CREATE INDEX idx_issues_priority ON issues(workspace_id, priority);
CREATE INDEX idx_issues_assignee ON issues(workspace_id, assignee);
CREATE INDEX idx_issues_type ON issues(workspace_id, issue_type);
CREATE INDEX idx_issues_updated ON issues(workspace_id, updated_at DESC);
CREATE UNIQUE INDEX idx_issues_external_ref ON issues(external_ref) WHERE external_ref IS NOT NULL;
CREATE INDEX idx_issues_rank ON issues(workspace_id, priority, rank, created_at);

-- Dependencies table
CREATE TABLE dependencies (
    issue_id TEXT NOT NULL,
    depends_on_id TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'blocks',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by TEXT,
    PRIMARY KEY (issue_id, depends_on_id),
    FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE,
    FOREIGN KEY (depends_on_id) REFERENCES issues(id) ON DELETE CASCADE
);

CREATE INDEX idx_dependencies_issue ON dependencies(issue_id);
CREATE INDEX idx_dependencies_depends_on ON dependencies(depends_on_id);

-- Labels definition table (global)
CREATE TABLE labels (
    name TEXT PRIMARY KEY,
    color TEXT,
    description TEXT
);

-- Issue-label associations
CREATE TABLE issue_labels (
    issue_id TEXT NOT NULL,
    label TEXT NOT NULL,
    PRIMARY KEY (issue_id, label),
    FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE
);

CREATE INDEX idx_issue_labels_label ON issue_labels(label);

-- Comments table
CREATE TABLE comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    issue_id TEXT NOT NULL,
    author TEXT NOT NULL,
    text TEXT NOT NULL,
    comment_type TEXT NOT NULL DEFAULT 'comment',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP,
    FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE
);

CREATE INDEX idx_comments_issue ON comments(issue_id);
CREATE INDEX idx_comments_type ON comments(issue_id, comment_type);

-- Events table (audit trail)
CREATE TABLE events (
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

CREATE INDEX idx_events_issue ON events(issue_id);

-- Blocked issues cache (for efficient ready work queries)
CREATE TABLE blocked_issues_cache (
    issue_id TEXT PRIMARY KEY,
    blocked_by_count INTEGER NOT NULL DEFAULT 0,
    blocked_by_ids TEXT, -- JSON array of blocking issue IDs
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE
);

-- Config table (for workspace settings)
CREATE TABLE config (
    workspace_id TEXT NOT NULL,
    key TEXT NOT NULL,
    value TEXT,
    PRIMARY KEY (workspace_id, key),
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
);

-- Global config table (server-wide settings)
CREATE TABLE global_config (
    key TEXT PRIMARY KEY,
    value TEXT
);

-- Child counters for hierarchical issue IDs
CREATE TABLE child_counters (
    parent_id TEXT PRIMARY KEY,
    last_child INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (parent_id) REFERENCES issues(id) ON DELETE CASCADE
);

-- Shared plans table for cross-issue planning
CREATE TABLE plans (
    id TEXT PRIMARY KEY,              -- plan.xxxxx format
    workspace_id TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
);

CREATE INDEX idx_plans_workspace ON plans(workspace_id);

-- Issue-plan links (many-to-many)
CREATE TABLE issue_plans (
    issue_id TEXT NOT NULL,
    plan_id TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (issue_id, plan_id),
    FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE,
    FOREIGN KEY (plan_id) REFERENCES plans(id) ON DELETE CASCADE
);

CREATE INDEX idx_issue_plans_plan ON issue_plans(plan_id);
