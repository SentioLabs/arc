-- +goose Up
CREATE TABLE shares (
    id         TEXT      PRIMARY KEY,
    kind       TEXT      NOT NULL CHECK (kind IN ('local', 'shared')),
    url        TEXT      NOT NULL,
    key_b64url TEXT      NOT NULL,
    edit_token TEXT      NOT NULL,
    plan_file  TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_shares_created_at ON shares(created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_shares_created_at;
DROP TABLE IF EXISTS shares;
