-- name: GetWorkspaceStats :one
SELECT
    workspace_id,
    COUNT(*) as total_issues,
    SUM(CASE WHEN status = 'open' THEN 1 ELSE 0 END) as open_issues,
    SUM(CASE WHEN status = 'in_progress' THEN 1 ELSE 0 END) as in_progress_issues,
    SUM(CASE WHEN status = 'closed' THEN 1 ELSE 0 END) as closed_issues,
    SUM(CASE WHEN status = 'blocked' THEN 1 ELSE 0 END) as blocked_issues,
    SUM(CASE WHEN status = 'deferred' THEN 1 ELSE 0 END) as deferred_issues
FROM issues
WHERE workspace_id = ?
GROUP BY workspace_id;

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
