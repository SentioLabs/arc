-- name: CreateAISession :one
INSERT INTO ai_sessions (id, transcript_path, cwd, started_at)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: GetAISession :one
SELECT * FROM ai_sessions WHERE id = ?;

-- name: ListAISessions :many
SELECT * FROM ai_sessions ORDER BY started_at DESC LIMIT ? OFFSET ?;

-- name: DeleteAISession :exec
DELETE FROM ai_sessions WHERE id = ?;

-- name: CreateAIAgent :one
INSERT INTO ai_agents (id, session_id, description, prompt, agent_type, model, status, duration_ms, total_tokens, tool_use_count, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetAIAgent :one
SELECT * FROM ai_agents WHERE id = ?;

-- name: ListAIAgents :many
SELECT * FROM ai_agents WHERE session_id = ? ORDER BY created_at ASC;

-- name: CountAIAgentsBySession :one
SELECT COUNT(*) FROM ai_agents WHERE session_id = ?;
