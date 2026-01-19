-- name: SetConfig :exec
INSERT INTO config (workspace_id, key, value)
VALUES (?, ?, ?)
ON CONFLICT(workspace_id, key) DO UPDATE SET value = excluded.value;

-- name: GetConfig :one
SELECT value FROM config WHERE workspace_id = ? AND key = ?;

-- name: GetAllConfig :many
SELECT key, value FROM config WHERE workspace_id = ?;

-- name: DeleteConfig :exec
DELETE FROM config WHERE workspace_id = ? AND key = ?;

-- name: SetGlobalConfig :exec
INSERT INTO global_config (key, value)
VALUES (?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value;

-- name: GetGlobalConfig :one
SELECT value FROM global_config WHERE key = ?;

-- name: GetAllGlobalConfig :many
SELECT key, value FROM global_config;

-- name: DeleteGlobalConfig :exec
DELETE FROM global_config WHERE key = ?;
