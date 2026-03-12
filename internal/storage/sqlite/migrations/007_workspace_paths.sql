-- +goose Up
CREATE TABLE workspace_paths (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  path TEXT NOT NULL,
  label TEXT,
  hostname TEXT,
  git_remote TEXT,
  last_accessed_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(workspace_id, path)
);

CREATE INDEX idx_workspace_paths_workspace ON workspace_paths(workspace_id);

CREATE INDEX idx_workspace_paths_path ON workspace_paths(path);

-- Migrate existing workspace.path data
INSERT INTO workspace_paths (id, workspace_id, path, created_at, updated_at)
  SELECT 'wp-' || hex(randomblob(8)), id, path,
    COALESCE(created_at, CURRENT_TIMESTAMP),
    COALESCE(updated_at, CURRENT_TIMESTAMP)
  FROM workspaces WHERE path IS NOT NULL AND path != '';

-- +goose Down
DROP TABLE IF EXISTS workspace_paths;
