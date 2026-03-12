-- name: CreateWorkspacePath :exec
INSERT INTO workspace_paths (id, workspace_id, path, label, hostname, git_remote, last_accessed_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetWorkspacePath :one
SELECT * FROM workspace_paths WHERE id = ?;

-- name: ListWorkspacePaths :many
SELECT * FROM workspace_paths WHERE workspace_id = ? ORDER BY path;

-- name: UpdateWorkspacePath :exec
UPDATE workspace_paths SET label = ?, hostname = ?, git_remote = ?, updated_at = ? WHERE id = ?;

-- name: DeleteWorkspacePath :exec
DELETE FROM workspace_paths WHERE id = ?;

-- name: ResolveWorkspaceByPath :one
SELECT * FROM workspace_paths WHERE path = ?;

-- name: UpdatePathLastAccessed :exec
UPDATE workspace_paths SET last_accessed_at = ?, updated_at = ? WHERE id = ?;

-- name: ListAllWorkspacePaths :many
SELECT * FROM workspace_paths ORDER BY workspace_id, path;

-- name: DeleteWorkspacePathsByWorkspace :exec
DELETE FROM workspace_paths WHERE workspace_id = ?;
