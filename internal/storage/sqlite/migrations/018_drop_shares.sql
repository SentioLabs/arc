-- +goose Up
-- Remove the arc-paste / share stack. The shares table (017) and the paste
-- engine's own tables are dropped here. Forward-only: 017 is left intact as a
-- released migration; this migration supersedes it.
DROP TABLE IF EXISTS paste_events;
DROP TABLE IF EXISTS paste_shares;
DROP TABLE IF EXISTS paste_migrations;
DROP TABLE IF EXISTS shares;

-- +goose Down
-- Rollback not supported for this cleanup migration
