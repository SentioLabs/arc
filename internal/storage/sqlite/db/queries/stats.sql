-- name: GetWorkspaceStats :one
SELECT
    sqlc.arg(workspace_id) as workspace_id,
    (SELECT COUNT(*) FROM issues WHERE issues.workspace_id = sqlc.arg(workspace_id)) as total_issues,
    (SELECT COUNT(*) FROM issues WHERE issues.workspace_id = sqlc.arg(workspace_id) AND issues.status = 'open') as open_issues,
    (SELECT COUNT(*) FROM issues WHERE issues.workspace_id = sqlc.arg(workspace_id) AND issues.status = 'in_progress') as in_progress_issues,
    (SELECT COUNT(*) FROM issues WHERE issues.workspace_id = sqlc.arg(workspace_id) AND issues.status = 'closed') as closed_issues,
    (SELECT COUNT(*) FROM issues WHERE issues.workspace_id = sqlc.arg(workspace_id) AND issues.status = 'blocked') as blocked_issues,
    (SELECT COUNT(*) FROM issues WHERE issues.workspace_id = sqlc.arg(workspace_id) AND issues.status = 'deferred') as deferred_issues;

-- name: GetReadyIssueCount :one
SELECT COUNT(*) as count FROM issues i
WHERE i.workspace_id = ?
  AND i.status IN ('open', 'in_progress')
  AND NOT EXISTS (
    SELECT 1 FROM dependencies d
    JOIN issues blocker ON d.depends_on_id = blocker.id
    WHERE d.issue_id = i.id
      AND d.type IN ('blocks', 'parent-child')
      AND blocker.status != 'closed'
  );

-- name: GetAverageLeadTime :one
SELECT AVG(
    (julianday(closed_at) - julianday(created_at)) * 24
) as avg_lead_time_hours
FROM issues
WHERE workspace_id = ?
  AND status = 'closed'
  AND closed_at IS NOT NULL;
