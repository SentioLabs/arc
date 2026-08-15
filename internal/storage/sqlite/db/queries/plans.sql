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
INSERT INTO plan_comments (
  id, plan_id, line_number, content, created_at,
  line_start, line_end, quoted_text, occurrence,
  heading_slug, context_before, context_after,
  updated_at, resolved_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListPlanComments :many
SELECT * FROM plan_comments WHERE plan_id = ? ORDER BY created_at ASC;

-- name: GetPlanComment :one
SELECT * FROM plan_comments WHERE id = ?;

-- name: UpdatePlanComment :exec
UPDATE plan_comments
SET content = ?, line_number = ?,
    line_start = ?, line_end = ?, quoted_text = ?, occurrence = ?,
    heading_slug = ?, context_before = ?, context_after = ?,
    updated_at = ?, resolved_at = ?
WHERE id = ?;

-- name: DeletePlanComment :exec
DELETE FROM plan_comments WHERE id = ?;
