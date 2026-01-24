-- name: CreatePlan :one
INSERT INTO plans (id, workspace_id, title, content, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetPlan :one
SELECT * FROM plans WHERE id = ?;

-- name: ListPlans :many
SELECT * FROM plans
WHERE workspace_id = ?
ORDER BY updated_at DESC;

-- name: UpdatePlan :exec
UPDATE plans SET title = ?, content = ?, updated_at = ?
WHERE id = ?;

-- name: DeletePlan :exec
DELETE FROM plans WHERE id = ?;

-- name: LinkIssueToPlan :exec
INSERT INTO issue_plans (issue_id, plan_id, created_at)
VALUES (?, ?, ?)
ON CONFLICT (issue_id, plan_id) DO NOTHING;

-- name: UnlinkIssueFromPlan :exec
DELETE FROM issue_plans WHERE issue_id = ? AND plan_id = ?;

-- name: GetLinkedIssues :many
SELECT issue_id FROM issue_plans WHERE plan_id = ?;

-- name: GetLinkedPlans :many
SELECT p.* FROM plans p
INNER JOIN issue_plans ip ON ip.plan_id = p.id
WHERE ip.issue_id = ?
ORDER BY p.updated_at DESC;

-- name: CountPlanLinks :one
SELECT COUNT(*) as count FROM issue_plans WHERE plan_id = ?;
