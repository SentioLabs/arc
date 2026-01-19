-- name: CreateWorkspace :exec
INSERT INTO workspaces (id, name, path, description, prefix, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetWorkspace :one
SELECT * FROM workspaces WHERE id = ?;

-- name: GetWorkspaceByName :one
SELECT * FROM workspaces WHERE name = ?;

-- name: GetWorkspaceByPath :one
SELECT * FROM workspaces WHERE path = ?;

-- name: ListWorkspaces :many
SELECT * FROM workspaces ORDER BY name;

-- name: UpdateWorkspace :exec
UPDATE workspaces
SET name = ?, path = ?, description = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteWorkspace :exec
DELETE FROM workspaces WHERE id = ?;

-- name: CountWorkspacesByID :one
SELECT COUNT(*) as count FROM workspaces WHERE id = ?;

-- name: CountWorkspacesByName :one
SELECT COUNT(*) as count FROM workspaces WHERE name = ?;
