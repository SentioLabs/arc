-- name: AddDependency :exec
INSERT INTO dependencies (issue_id, depends_on_id, type, created_at, created_by)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(issue_id, depends_on_id) DO UPDATE SET
    type = excluded.type,
    created_at = excluded.created_at,
    created_by = excluded.created_by;

-- name: RemoveDependency :exec
DELETE FROM dependencies WHERE issue_id = ? AND depends_on_id = ?;

-- name: GetDependencies :many
SELECT i.* FROM issues i
JOIN dependencies d ON i.id = d.depends_on_id
WHERE d.issue_id = ?
ORDER BY i.priority ASC;

-- name: GetDependents :many
SELECT i.* FROM issues i
JOIN dependencies d ON i.id = d.issue_id
WHERE d.depends_on_id = ?
ORDER BY i.priority ASC;

-- name: GetDependencyRecords :many
SELECT * FROM dependencies WHERE issue_id = ?;

-- name: GetDependentRecords :many
SELECT * FROM dependencies WHERE depends_on_id = ?;

-- name: CountDependencies :one
SELECT COUNT(*) as count FROM dependencies
WHERE issue_id = ? AND depends_on_id = ?;

-- name: GetBlockingIssues :many
SELECT i.* FROM issues i
JOIN dependencies d ON i.id = d.depends_on_id
WHERE d.issue_id = ?
  AND d.type IN ('blocks', 'parent-child')
  AND i.status != 'closed'
ORDER BY i.priority ASC;

-- name: CountBlockingIssues :one
SELECT COUNT(*) as count FROM dependencies d
JOIN issues i ON d.depends_on_id = i.id
WHERE d.issue_id = ?
  AND d.type IN ('blocks', 'parent-child')
  AND i.status != 'closed';

-- name: GetBlockedIssuesInWorkspace :many
SELECT i.id, i.workspace_id, i.title, i.description, i.acceptance_criteria,
       i.notes, i.status, i.priority, i.issue_type, i.assignee, i.external_ref,
       i.created_at, i.updated_at, i.closed_at, i.close_reason,
       COUNT(blocker.id) as blocked_by_count
FROM issues i
JOIN dependencies d ON d.issue_id = i.id AND d.type IN ('blocks', 'parent-child')
JOIN issues blocker ON d.depends_on_id = blocker.id AND blocker.status != 'closed'
WHERE i.workspace_id = ?
  AND i.status != 'closed'
GROUP BY i.id
ORDER BY i.priority ASC
LIMIT ?;

-- name: DeleteDependenciesByIssue :exec
DELETE FROM dependencies WHERE issue_id = ? OR depends_on_id = ?;
