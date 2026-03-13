-- name: CreateProject :exec
INSERT INTO projects (id, name, description, prefix, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetProject :one
SELECT * FROM projects WHERE id = ?;

-- name: GetProjectByName :one
SELECT * FROM projects WHERE name = ?;

-- name: ListProjects :many
SELECT * FROM projects ORDER BY name;

-- name: UpdateProject :exec
UPDATE projects
SET name = ?, description = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteProject :exec
DELETE FROM projects WHERE id = ?;

-- name: CountProjectsByID :one
SELECT COUNT(*) as count FROM projects WHERE id = ?;

-- name: CountProjectsByName :one
SELECT COUNT(*) as count FROM projects WHERE name = ?;

-- name: MoveIssuesToProject :execresult
UPDATE issues SET project_id = ? WHERE project_id = ?;

-- name: MovePlansToProject :execresult
UPDATE plans SET project_id = ? WHERE project_id = ?;

-- name: DeleteConfigByProject :exec
DELETE FROM config WHERE project_id = ?;
