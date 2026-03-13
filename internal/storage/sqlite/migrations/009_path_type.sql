-- +goose Up
ALTER TABLE workspace_paths ADD COLUMN path_type TEXT NOT NULL DEFAULT 'canonical';

-- +goose Down
ALTER TABLE workspace_paths DROP COLUMN path_type;
