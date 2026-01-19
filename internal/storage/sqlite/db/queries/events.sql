-- name: CreateEvent :exec
INSERT INTO events (issue_id, event_type, actor, old_value, new_value, comment, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetEvents :many
SELECT * FROM events
WHERE issue_id = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: GetRecentEvents :many
SELECT * FROM events
ORDER BY created_at DESC
LIMIT ?;

-- name: CountEvents :one
SELECT COUNT(*) as count FROM events WHERE issue_id = ?;

-- name: DeleteEventsByIssue :exec
DELETE FROM events WHERE issue_id = ?;
