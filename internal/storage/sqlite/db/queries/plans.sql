-- name: CreatePlan :one
INSERT INTO plans (id, project_id, issue_id, title, content, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetPlan :one
SELECT * FROM plans WHERE id = ?;

-- name: GetPlanByIssueID :one
SELECT * FROM plans WHERE issue_id = ?;

-- name: ListPlans :many
SELECT * FROM plans
WHERE project_id = ? AND (? = '' OR status = ?)
ORDER BY updated_at DESC;

-- name: UpdatePlanStatus :exec
UPDATE plans SET status = ?, updated_at = ? WHERE id = ?;

-- name: UpdatePlanContent :exec
UPDATE plans SET title = ?, content = ?, updated_at = ? WHERE id = ?;

-- name: DeletePlan :exec
DELETE FROM plans WHERE id = ?;

-- name: CountPlansByStatus :one
SELECT COUNT(*) as count FROM plans WHERE project_id = ? AND status = ?;

-- name: UpsertPlan :one
INSERT INTO plans (id, project_id, issue_id, title, content, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (issue_id) DO UPDATE SET
    title = excluded.title,
    content = excluded.content,
    updated_at = excluded.updated_at
RETURNING *;
