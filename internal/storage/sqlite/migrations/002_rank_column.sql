-- +goose Up
ALTER TABLE issues ADD COLUMN rank INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_issues_rank ON issues(workspace_id, priority, rank, created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_issues_rank;
-- Note: SQLite doesn't support DROP COLUMN in older versions.
-- The column will remain but be unused after rollback.
