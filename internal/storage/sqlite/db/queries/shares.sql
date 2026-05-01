-- name: UpsertShare :exec
INSERT INTO shares (id, kind, url, key_b64url, edit_token, plan_file, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    kind       = excluded.kind,
    url        = excluded.url,
    key_b64url = excluded.key_b64url,
    edit_token = excluded.edit_token,
    plan_file  = excluded.plan_file,
    created_at = excluded.created_at;

-- name: GetShare :one
SELECT * FROM shares WHERE id = ?;

-- name: ListShares :many
SELECT * FROM shares ORDER BY created_at DESC;

-- name: DeleteShare :exec
DELETE FROM shares WHERE id = ?;
