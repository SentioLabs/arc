-- +goose Up
CREATE TABLE shares (
    id         TEXT    PRIMARY KEY,
    kind       TEXT    NOT NULL,
    url        TEXT    NOT NULL,
    key_b64url TEXT    NOT NULL,
    edit_token TEXT    NOT NULL,
    plan_file  TEXT,
    created_at TEXT    NOT NULL
) STRICT;

CREATE INDEX idx_shares_created_at ON shares(created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_shares_created_at;
DROP TABLE IF EXISTS shares;
