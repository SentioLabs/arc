-- name: CreateComment :one
INSERT INTO comments (issue_id, author, text, created_at)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: GetComment :one
SELECT * FROM comments WHERE id = ?;

-- name: ListComments :many
SELECT * FROM comments
WHERE issue_id = ?
ORDER BY created_at ASC;

-- name: UpdateComment :exec
UPDATE comments SET text = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteComment :exec
DELETE FROM comments WHERE id = ?;

-- name: GetCommentsForIssues :many
SELECT * FROM comments
WHERE issue_id IN (sqlc.slice('issue_ids'))
ORDER BY issue_id, created_at ASC;

-- name: CountComments :one
SELECT COUNT(*) as count FROM comments WHERE issue_id = ?;
