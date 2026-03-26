-- name: CreateAISession :one
INSERT INTO ai_sessions (id, project_id, transcript_path, cwd, started_at)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: GetAISession :one
SELECT * FROM ai_sessions WHERE id = ?;

-- name: ListAISessionsByProject :many
SELECT * FROM ai_sessions WHERE project_id = ? ORDER BY started_at DESC LIMIT ? OFFSET ?;

-- name: CountAISessionsByProject :one
SELECT COUNT(*) FROM ai_sessions WHERE project_id = ?;

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

-- name: GetAgentSummariesForSessions :many
SELECT
  session_id,
  COUNT(*) AS agent_count,
  SUM(CASE WHEN status = 'running' THEN 1 ELSE 0 END) AS running_count,
  SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) AS completed_count,
  SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) AS error_count
FROM ai_agents
WHERE session_id IN (sqlc.slice('session_ids'))
GROUP BY session_id;
