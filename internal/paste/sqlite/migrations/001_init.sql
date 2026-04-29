CREATE TABLE paste_shares (
    id           TEXT PRIMARY KEY,
    edit_token   TEXT NOT NULL,
    plan_blob    BLOB NOT NULL,
    plan_iv      BLOB NOT NULL,
    schema_ver   INTEGER NOT NULL DEFAULT 1,
    created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at   TIMESTAMP
);

CREATE TABLE paste_events (
    id           TEXT PRIMARY KEY,
    share_id     TEXT NOT NULL REFERENCES paste_shares(id) ON DELETE CASCADE,
    blob         BLOB NOT NULL,
    iv           BLOB NOT NULL,
    created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_paste_events_share ON paste_events(share_id, created_at);
