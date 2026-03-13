-- +goose Up
ALTER TABLE workspaces RENAME TO projects;
ALTER TABLE workspace_paths RENAME TO workspaces;
ALTER TABLE issues RENAME COLUMN workspace_id TO project_id;
ALTER TABLE config RENAME COLUMN workspace_id TO project_id;
ALTER TABLE plans RENAME COLUMN workspace_id TO project_id;
ALTER TABLE workspaces RENAME COLUMN workspace_id TO project_id;
DROP INDEX IF EXISTS idx_plans_workspace;
CREATE INDEX idx_plans_project ON plans(project_id);
DROP INDEX IF EXISTS idx_workspace_paths_workspace;
DROP INDEX IF EXISTS idx_workspace_paths_path;
CREATE INDEX idx_workspaces_project_id ON workspaces(project_id);
CREATE INDEX idx_workspaces_path ON workspaces(path);
DROP TABLE IF EXISTS issues_fts;
CREATE VIRTUAL TABLE issues_fts USING fts5(id, title, description, content=issues, content_rowid=rowid);
INSERT INTO issues_fts(id, title, description) SELECT id, title, COALESCE(description, '') FROM issues;

-- +goose Down
DROP TABLE IF EXISTS issues_fts;
DROP INDEX IF EXISTS idx_workspaces_path;
DROP INDEX IF EXISTS idx_workspaces_project_id;
ALTER TABLE workspaces RENAME COLUMN project_id TO workspace_id;
DROP INDEX IF EXISTS idx_plans_project;
ALTER TABLE plans RENAME COLUMN project_id TO workspace_id;
CREATE INDEX idx_plans_workspace ON plans(workspace_id);
ALTER TABLE config RENAME COLUMN project_id TO workspace_id;
ALTER TABLE issues RENAME COLUMN project_id TO workspace_id;
ALTER TABLE workspaces RENAME TO workspace_paths;
ALTER TABLE projects RENAME TO workspaces;
CREATE INDEX idx_workspace_paths_workspace ON workspace_paths(workspace_id);
CREATE INDEX idx_workspace_paths_path ON workspace_paths(path);
CREATE VIRTUAL TABLE issues_fts USING fts5(id, title, description, content=issues, content_rowid=rowid);
INSERT INTO issues_fts(id, title, description) SELECT id, title, COALESCE(description, '') FROM issues;
