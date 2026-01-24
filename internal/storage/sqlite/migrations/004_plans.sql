-- +goose Up

-- Add comment_type to existing comments for plan vs regular comment distinction
ALTER TABLE comments ADD COLUMN comment_type TEXT NOT NULL DEFAULT 'comment';
CREATE INDEX idx_comments_type ON comments(issue_id, comment_type);

-- Shared plans table for cross-issue planning
CREATE TABLE plans (
    id TEXT PRIMARY KEY,              -- plan.xxxxx format
    workspace_id TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
);

CREATE INDEX idx_plans_workspace ON plans(workspace_id);

-- Issue-plan links (many-to-many)
CREATE TABLE issue_plans (
    issue_id TEXT NOT NULL,
    plan_id TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (issue_id, plan_id),
    FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE,
    FOREIGN KEY (plan_id) REFERENCES plans(id) ON DELETE CASCADE
);

CREATE INDEX idx_issue_plans_plan ON issue_plans(plan_id);

-- +goose Down
DROP INDEX IF EXISTS idx_issue_plans_plan;
DROP TABLE IF EXISTS issue_plans;
DROP INDEX IF EXISTS idx_plans_workspace;
DROP TABLE IF EXISTS plans;
DROP INDEX IF EXISTS idx_comments_type;
-- SQLite doesn't support DROP COLUMN, so we'd need to recreate the table
-- For simplicity in the down migration, we leave the column (it will be ignored)
