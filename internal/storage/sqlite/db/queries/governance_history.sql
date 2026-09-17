-- name: ListExecutionEvidenceRecords :many
SELECT result FROM execution_evidence
WHERE project_id = ? AND issue_id = ? ORDER BY created_at, id LIMIT ? OFFSET ?;

-- name: ListPlanAdoptionRecords :many
SELECT result FROM plan_adoptions
WHERE project_id = ? AND container_id = ? ORDER BY created_at, id LIMIT ? OFFSET ?;
