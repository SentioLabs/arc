-- +goose Up
ALTER TABLE issues ADD COLUMN ai_session_id TEXT;

-- +goose Down
ALTER TABLE issues DROP COLUMN ai_session_id;
