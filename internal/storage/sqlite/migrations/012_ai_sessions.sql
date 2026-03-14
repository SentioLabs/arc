-- +goose Up
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

-- +goose Down
DROP INDEX IF EXISTS idx_ai_agents_session;
DROP TABLE IF EXISTS ai_agents;
DROP TABLE IF EXISTS ai_sessions;
