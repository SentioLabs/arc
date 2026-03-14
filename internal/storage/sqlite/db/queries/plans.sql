-- name: CreatePlan :exec
INSERT INTO plans (id, file_path, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?);

-- name: GetPlan :one
SELECT id, file_path, status, created_at, updated_at
FROM plans WHERE id = ?;

-- name: UpdatePlanStatus :exec
UPDATE plans SET status = ?, updated_at = ? WHERE id = ?;

-- name: DeletePlan :exec
DELETE FROM plans WHERE id = ?;

-- name: CreatePlanComment :exec
INSERT INTO plan_comments (id, plan_id, line_number, content, created_at)
VALUES (?, ?, ?, ?, ?);

-- name: ListPlanComments :many
SELECT id, plan_id, line_number, content, created_at
FROM plan_comments WHERE plan_id = ? ORDER BY created_at ASC;
