-- +goose Up
ALTER TABLE plans ADD COLUMN status TEXT NOT NULL DEFAULT 'draft';
ALTER TABLE plans ADD COLUMN issue_id TEXT REFERENCES issues(id) ON DELETE CASCADE;
CREATE INDEX idx_plans_status ON plans(project_id, status);
CREATE UNIQUE INDEX idx_plans_issue ON plans(issue_id);

-- Migrate latest inline plan comments into the plans table
INSERT INTO plans (id, project_id, title, content, status, issue_id, created_at, updated_at)
SELECT
    'plan.' || c.id, i.project_id, 'Plan for ' || i.title, c.text,
    'approved', c.issue_id, c.created_at, c.created_at
FROM comments c
JOIN issues i ON c.issue_id = i.id
WHERE c.comment_type = 'plan'
AND c.id IN (
    SELECT c2.id FROM comments c2
    WHERE c2.issue_id = c.issue_id AND c2.comment_type = 'plan'
    ORDER BY c2.created_at DESC LIMIT 1
);

-- Clean up old plan comments and issue_plans link table
DELETE FROM comments WHERE comment_type = 'plan';
DROP TABLE IF EXISTS issue_plans;

-- +goose Down
DROP INDEX IF EXISTS idx_plans_issue;
DROP INDEX IF EXISTS idx_plans_status;
