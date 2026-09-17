-- Reference schema for sqlc code generation.
-- Applied at runtime via golang-migrate (see migrations/*.sql).
-- Keep this in sync with the migration files.

-- Projects table (issue containers, previously named workspaces)
CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    prefix TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Workspaces table (directory paths, previously named workspace_paths)
CREATE TABLE workspaces (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    label TEXT,
    hostname TEXT,
    git_remote TEXT,
    path_type TEXT NOT NULL DEFAULT 'canonical',
    last_accessed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(project_id, path)
);

CREATE INDEX idx_workspaces_project_id ON workspaces(project_id);
CREATE INDEX idx_workspaces_path ON workspaces(path);

-- Issues table
CREATE TABLE issues (
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

-- Index for common queries
CREATE INDEX idx_issues_project ON issues(project_id);
CREATE INDEX idx_issues_status ON issues(project_id, status);
CREATE INDEX idx_issues_priority ON issues(project_id, priority);
CREATE INDEX idx_issues_type ON issues(project_id, issue_type);
CREATE INDEX idx_issues_updated ON issues(project_id, updated_at DESC);
CREATE UNIQUE INDEX idx_issues_external_ref ON issues(external_ref) WHERE external_ref IS NOT NULL;
CREATE INDEX idx_issues_rank ON issues(project_id, priority, rank, created_at);

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
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP,
    FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE
);

CREATE INDEX idx_comments_issue ON comments(issue_id);

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

-- Config table (for project settings)
CREATE TABLE config (
    project_id TEXT NOT NULL,
    key TEXT NOT NULL,
    value TEXT,
    PRIMARY KEY (project_id, key),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
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

-- Plans table (ephemeral review artifacts, content on filesystem)
CREATE TABLE legacy_plans (
    id TEXT PRIMARY KEY,
    file_path TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Plan review comments (overall, legacy line-level, or quoted-range anchored)
CREATE TABLE legacy_plan_comments (
    id TEXT PRIMARY KEY,
    plan_id TEXT NOT NULL REFERENCES legacy_plans(id) ON DELETE CASCADE,
    line_number INTEGER,
    content TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    line_start INTEGER,
    line_end INTEGER,
    quoted_text TEXT,
    occurrence INTEGER,
    heading_slug TEXT,
    context_before TEXT,
    context_after TEXT,
    updated_at TIMESTAMP,
    resolved_at TIMESTAMP
);

CREATE INDEX idx_plan_comments_plan ON legacy_plan_comments(plan_id);

-- AI sessions table (AI coding session tracking)
CREATE TABLE ai_sessions (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    transcript_path TEXT NOT NULL DEFAULT '',
    cwd TEXT DEFAULT '',
    started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_ai_sessions_project_id ON ai_sessions(project_id);
CREATE INDEX idx_ai_sessions_started_at ON ai_sessions(started_at);

-- AI agents table (sub-agents spawned within sessions)
CREATE TABLE ai_agents (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES ai_sessions(id) ON DELETE CASCADE,
    description TEXT,
    prompt TEXT,
    agent_type TEXT,
    model TEXT,
    status TEXT NOT NULL DEFAULT 'running',
    duration_ms INTEGER,
    total_tokens INTEGER,
    tool_use_count INTEGER,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_ai_agents_session ON ai_agents(session_id);

CREATE TABLE plans (
 id TEXT PRIMARY KEY,
 project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
 title TEXT NOT NULL,
 lifecycle TEXT NOT NULL DEFAULT 'active' CHECK(lifecycle IN ('active','archived')),
 head_revision INTEGER NOT NULL CHECK(head_revision > 0),
 version INTEGER NOT NULL DEFAULT 1,
 feedback_version INTEGER NOT NULL DEFAULT 0,
 legacy_status_unverified TEXT,
 created_at TIMESTAMP NOT NULL,
 updated_at TIMESTAMP NOT NULL,
 UNIQUE(project_id,id)
);
CREATE INDEX idx_durable_plans_project ON plans(project_id);
CREATE TABLE plan_revisions (
 plan_id TEXT NOT NULL REFERENCES plans(id) ON DELETE RESTRICT,
 revision INTEGER NOT NULL CHECK(revision > 0),
 content_path TEXT NOT NULL UNIQUE,
 content_sha256 TEXT NOT NULL,
 content_bytes INTEGER NOT NULL,
 source_name TEXT NOT NULL DEFAULT '',
 review_status TEXT NOT NULL DEFAULT 'draft' CHECK(review_status IN ('draft','in_review','changes_requested','approved','rejected')),
 review_version INTEGER NOT NULL DEFAULT 0,
 created_at TIMESTAMP NOT NULL,
 PRIMARY KEY(plan_id,revision)
);
CREATE TABLE plan_comments (
 id TEXT PRIMARY KEY,
 plan_id TEXT NOT NULL REFERENCES plans(id) ON DELETE RESTRICT,
 revision INTEGER,
 version INTEGER NOT NULL,
 body TEXT NOT NULL,
 FOREIGN KEY(plan_id,revision) REFERENCES plan_revisions(plan_id,revision) ON DELETE RESTRICT
);
CREATE INDEX idx_durable_comments_plan ON plan_comments(plan_id,revision);
CREATE TABLE plan_comment_events (
 actor TEXT NOT NULL DEFAULT '',
 session_id TEXT NOT NULL DEFAULT '',
 comment_id TEXT NOT NULL REFERENCES plan_comments(id) ON DELETE RESTRICT,
 version INTEGER NOT NULL,
 body TEXT NOT NULL,
 created_at TIMESTAMP NOT NULL,
 PRIMARY KEY(comment_id,version)
);
CREATE TABLE plan_dispositions (
 actor TEXT NOT NULL DEFAULT '',
 session_id TEXT NOT NULL DEFAULT '',
 id TEXT PRIMARY KEY,
 plan_id TEXT NOT NULL,
 target_revision INTEGER NOT NULL,
 comment_id TEXT NOT NULL REFERENCES plan_comments(id) ON DELETE RESTRICT,
 comment_version INTEGER NOT NULL,
 disposition TEXT NOT NULL CHECK(disposition IN ('addressed','deferred')),
 reason TEXT NOT NULL CHECK(length(trim(reason)) > 0),
 created_at TIMESTAMP NOT NULL,
 FOREIGN KEY(plan_id,target_revision) REFERENCES plan_revisions(plan_id,revision) ON DELETE RESTRICT,
 FOREIGN KEY(comment_id,comment_version) REFERENCES plan_comment_events(comment_id,version) ON DELETE RESTRICT
);
CREATE TABLE plan_review_events (
 actor TEXT NOT NULL DEFAULT '',
 session_id TEXT NOT NULL DEFAULT '',
 id TEXT PRIMARY KEY,
 plan_id TEXT NOT NULL,
 revision INTEGER NOT NULL,
 status TEXT NOT NULL,
 review_version INTEGER NOT NULL,
 feedback_version INTEGER NOT NULL,
 disposition_ids TEXT NOT NULL,
 created_at TIMESTAMP NOT NULL,
 FOREIGN KEY(plan_id,revision) REFERENCES plan_revisions(plan_id,revision) ON DELETE RESTRICT,
 UNIQUE(plan_id,revision,review_version)
);
CREATE TABLE plan_idempotency (
 project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
 scope TEXT NOT NULL,
 key TEXT NOT NULL,
 fingerprint TEXT NOT NULL,
 result TEXT NOT NULL,
 created_at TIMESTAMP NOT NULL,
 PRIMARY KEY(project_id,scope,key)
);
