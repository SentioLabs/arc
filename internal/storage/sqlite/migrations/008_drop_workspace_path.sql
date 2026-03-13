-- +goose Up
ALTER TABLE workspaces DROP COLUMN path;

-- +goose Down
ALTER TABLE workspaces ADD COLUMN path TEXT;
