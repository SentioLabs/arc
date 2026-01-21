-- name: CreateIssue :exec
INSERT INTO issues (
    id, workspace_id, title, description,
    status, priority, issue_type, assignee, external_ref,
    created_at, updated_at, closed_at, close_reason
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetIssue :one
SELECT * FROM issues WHERE id = ?;

-- name: GetIssueByExternalRef :one
SELECT * FROM issues WHERE external_ref = ?;

-- name: ListIssuesByWorkspace :many
SELECT * FROM issues
WHERE workspace_id = ?
ORDER BY priority ASC, updated_at DESC
LIMIT ? OFFSET ?;

-- name: ListIssuesByStatus :many
SELECT * FROM issues
WHERE workspace_id = ? AND status = ?
ORDER BY priority ASC, updated_at DESC
LIMIT ? OFFSET ?;

-- name: ListIssuesByAssignee :many
SELECT * FROM issues
WHERE workspace_id = ? AND assignee = ?
ORDER BY priority ASC, updated_at DESC
LIMIT ? OFFSET ?;

-- name: ListIssuesByType :many
SELECT * FROM issues
WHERE workspace_id = ? AND issue_type = ?
ORDER BY priority ASC, updated_at DESC
LIMIT ? OFFSET ?;

-- name: SearchIssues :many
SELECT * FROM issues
WHERE workspace_id = ?
  AND (title LIKE ? OR description LIKE ?)
ORDER BY priority ASC, updated_at DESC
LIMIT ? OFFSET ?;

-- name: UpdateIssueTitle :exec
UPDATE issues SET title = ?, updated_at = ? WHERE id = ?;

-- name: UpdateIssueDescription :exec
UPDATE issues SET description = ?, updated_at = ? WHERE id = ?;

-- name: UpdateIssueStatus :exec
UPDATE issues SET status = ?, updated_at = ? WHERE id = ?;

-- name: UpdateIssuePriority :exec
UPDATE issues SET priority = ?, updated_at = ? WHERE id = ?;

-- name: UpdateIssueType :exec
UPDATE issues SET issue_type = ?, updated_at = ? WHERE id = ?;

-- name: UpdateIssueAssignee :exec
UPDATE issues SET assignee = ?, updated_at = ? WHERE id = ?;

-- name: UpdateIssueExternalRef :exec
UPDATE issues SET external_ref = ?, updated_at = ? WHERE id = ?;

-- name: CloseIssue :exec
UPDATE issues SET
    status = 'closed',
    closed_at = ?,
    close_reason = ?,
    updated_at = ?
WHERE id = ?;

-- name: ReopenIssue :exec
UPDATE issues SET
    status = 'open',
    closed_at = NULL,
    close_reason = NULL,
    updated_at = ?
WHERE id = ?;

-- name: DeleteIssue :exec
DELETE FROM issues WHERE id = ?;

-- name: CountIssuesByWorkspace :one
SELECT COUNT(*) as count FROM issues WHERE workspace_id = ?;

-- name: CountIssuesByStatus :one
SELECT COUNT(*) as count FROM issues WHERE workspace_id = ? AND status = ?;

-- name: CountIssuesByID :one
SELECT COUNT(*) as count FROM issues WHERE id = ?;

-- name: GetOpenNonBlockedIssues :many
SELECT i.* FROM issues i
LEFT JOIN dependencies d ON d.issue_id = i.id AND d.type = 'blocks'
LEFT JOIN issues blocker ON d.depends_on_id = blocker.id AND blocker.status != 'closed'
WHERE i.workspace_id = ?
  AND i.status IN ('open', 'in_progress')
GROUP BY i.id
HAVING COUNT(blocker.id) = 0
ORDER BY i.priority ASC, i.updated_at DESC
LIMIT ?;

-- name: GetReadyIssuesHybrid :many
-- Hybrid sort: recent issues (<48h) by priority/rank, older issues by age.
-- Uses CASE to create two sorting groups, then appropriate sub-ordering within each.
-- Note: Only 'blocks' dependencies are blocking; parent-child is organizational only.
SELECT i.* FROM issues i
LEFT JOIN dependencies d ON d.issue_id = i.id AND d.type = 'blocks'
LEFT JOIN issues blocker ON d.depends_on_id = blocker.id AND blocker.status != 'closed'
WHERE i.workspace_id = ?
  AND i.status IN ('open', 'in_progress')
GROUP BY i.id
HAVING COUNT(blocker.id) = 0
ORDER BY
  CASE WHEN i.updated_at >= datetime('now', '-48 hours') THEN 0 ELSE 1 END ASC,
  CASE WHEN i.updated_at >= datetime('now', '-48 hours')
       THEN i.priority
       ELSE 999 END ASC,
  CASE WHEN i.updated_at >= datetime('now', '-48 hours')
       THEN CASE WHEN i.rank = 0 THEN 999999 ELSE i.rank END
       ELSE 999999 END ASC,
  CASE WHEN i.updated_at >= datetime('now', '-48 hours')
       THEN datetime('9999-12-31')
       ELSE i.created_at END ASC
LIMIT ?;

-- name: GetReadyIssuesPriority :many
-- Priority-first sort: priority -> rank -> created_at.
-- Note: Only 'blocks' dependencies are blocking; parent-child is organizational only.
SELECT i.* FROM issues i
LEFT JOIN dependencies d ON d.issue_id = i.id AND d.type = 'blocks'
LEFT JOIN issues blocker ON d.depends_on_id = blocker.id AND blocker.status != 'closed'
WHERE i.workspace_id = ?
  AND i.status IN ('open', 'in_progress')
GROUP BY i.id
HAVING COUNT(blocker.id) = 0
ORDER BY
  i.priority ASC,
  CASE WHEN i.rank = 0 THEN 999999 ELSE i.rank END ASC,
  i.created_at ASC
LIMIT ?;

-- name: GetReadyIssuesOldest :many
-- Oldest-first sort: for backlog clearing.
-- Note: Only 'blocks' dependencies are blocking; parent-child is organizational only.
SELECT i.* FROM issues i
LEFT JOIN dependencies d ON d.issue_id = i.id AND d.type = 'blocks'
LEFT JOIN issues blocker ON d.depends_on_id = blocker.id AND blocker.status != 'closed'
WHERE i.workspace_id = ?
  AND i.status IN ('open', 'in_progress')
GROUP BY i.id
HAVING COUNT(blocker.id) = 0
ORDER BY i.created_at ASC
LIMIT ?;

-- name: UpdateIssueRank :exec
UPDATE issues SET rank = ?, updated_at = ? WHERE id = ?;
