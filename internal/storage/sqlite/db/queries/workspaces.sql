-- name: CreateWorkspace :exec
INSERT INTO workspaces (id, project_id, path, label, hostname, git_remote, path_type, last_accessed_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetWorkspace :one
SELECT * FROM workspaces WHERE id = ?;

-- name: ListWorkspaces :many
SELECT * FROM workspaces WHERE project_id = ? ORDER BY path;

-- name: UpdateWorkspace :exec
UPDATE workspaces SET label = ?, hostname = ?, git_remote = ?, path_type = ?, updated_at = ? WHERE id = ?;

-- name: DeleteWorkspace :exec
DELETE FROM workspaces WHERE id = ?;

-- name: ResolveProjectByPath :one
SELECT * FROM workspaces WHERE path = ?;

-- name: UpdateWorkspaceLastAccessed :exec
UPDATE workspaces SET last_accessed_at = ?, updated_at = ? WHERE id = ?;

-- name: ListAllWorkspaces :many
SELECT * FROM workspaces ORDER BY project_id, path;

-- name: DeleteWorkspacesByProject :exec
DELETE FROM workspaces WHERE project_id = ?;
