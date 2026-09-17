-- name: CreatePlan :exec
INSERT INTO legacy_plans (id, file_path, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?);

-- name: GetPlan :one
SELECT id, file_path, status, created_at, updated_at
FROM legacy_plans WHERE id = ?;

-- name: UpdatePlanStatus :exec
UPDATE legacy_plans SET status = ?, updated_at = ? WHERE id = ?;

-- name: DeletePlan :exec
DELETE FROM legacy_plans WHERE id = ?;

-- name: CreatePlanComment :exec
INSERT INTO legacy_plan_comments (
  id, plan_id, line_number, content, created_at,
  line_start, line_end, quoted_text, occurrence,
  heading_slug, context_before, context_after,
  updated_at, resolved_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListPlanComments :many
SELECT * FROM legacy_plan_comments WHERE plan_id = ? ORDER BY created_at ASC;

-- name: GetPlanComment :one
SELECT * FROM legacy_plan_comments WHERE id = ?;

-- name: UpdatePlanComment :exec
UPDATE legacy_plan_comments
SET content = ?, line_number = ?,
    line_start = ?, line_end = ?, quoted_text = ?, occurrence = ?,
    heading_slug = ?, context_before = ?, context_after = ?,
    updated_at = ?, resolved_at = ?
WHERE id = ?;

-- name: DeletePlanComment :exec
DELETE FROM legacy_plan_comments WHERE id = ?;

-- name: GetDurablePlan :one
SELECT * FROM plans WHERE project_id = ? AND id = ?;

-- name: ListDurablePlans :many
SELECT * FROM plans WHERE project_id = ? AND lifecycle = ? ORDER BY created_at DESC, id LIMIT ? OFFSET ?;

-- name: GetPlanRevision :one
SELECT * FROM plan_revisions WHERE plan_id = ? AND revision = ?;

-- name: ListPlanRevisions :many
SELECT * FROM plan_revisions WHERE plan_id = ? ORDER BY revision DESC LIMIT ? OFFSET ?;

-- name: ListOwnedPlanIDs :many
SELECT id FROM plans WHERE project_id = ? ORDER BY id;
