-- name: CreateLabel :exec
INSERT INTO labels (name, color, description)
VALUES (?, ?, ?)
ON CONFLICT(name) DO UPDATE SET
    color = excluded.color,
    description = excluded.description;

-- name: GetLabel :one
SELECT * FROM labels WHERE name = ?;

-- name: ListLabels :many
SELECT * FROM labels ORDER BY name;

-- name: UpdateLabel :exec
UPDATE labels SET color = ?, description = ?
WHERE name = ?;

-- name: DeleteLabel :exec
DELETE FROM labels WHERE name = ?;

-- name: AddLabelToIssue :exec
INSERT INTO issue_labels (issue_id, label)
VALUES (?, ?)
ON CONFLICT(issue_id, label) DO NOTHING;

-- name: RemoveLabelFromIssue :exec
DELETE FROM issue_labels WHERE issue_id = ? AND label = ?;

-- name: GetIssueLabels :many
SELECT label FROM issue_labels WHERE issue_id = ? ORDER BY label;

-- name: GetIssuesByLabel :many
SELECT i.* FROM issues i
JOIN issue_labels il ON i.id = il.issue_id
WHERE il.label = ?
ORDER BY i.priority ASC, i.updated_at DESC;

-- name: GetLabelsForIssues :many
SELECT issue_id, label FROM issue_labels
WHERE issue_id IN (sqlc.slice('issue_ids'))
ORDER BY issue_id, label;

-- name: DeleteIssueLabels :exec
DELETE FROM issue_labels WHERE issue_id = ?;
