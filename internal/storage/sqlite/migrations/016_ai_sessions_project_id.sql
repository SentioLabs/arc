-- +goose Up
-- Drop existing tables and recreate with project_id (existing data is test data, safe to drop)
DROP TABLE IF EXISTS ai_agents;
DROP TABLE IF EXISTS ai_sessions;

CREATE TABLE ai_sessions (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    transcript_path TEXT NOT NULL DEFAULT '',
    cwd TEXT DEFAULT '',
    started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_ai_sessions_project_id ON ai_sessions(project_id);
CREATE INDEX idx_ai_sessions_started_at ON ai_sessions(started_at);

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

-- +goose Down
DROP INDEX IF EXISTS idx_ai_agents_session;
DROP TABLE IF EXISTS ai_agents;
DROP INDEX IF EXISTS idx_ai_sessions_started_at;
DROP INDEX IF EXISTS idx_ai_sessions_project_id;
DROP TABLE IF EXISTS ai_sessions;

CREATE TABLE ai_sessions (
    id TEXT PRIMARY KEY,
    transcript_path TEXT NOT NULL,
    cwd TEXT,
    started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

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
