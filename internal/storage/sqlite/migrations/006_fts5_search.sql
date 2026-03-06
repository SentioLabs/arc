-- +goose Up
CREATE VIRTUAL TABLE IF NOT EXISTS issues_fts USING fts5(
    issue_id UNINDEXED,
    workspace_id UNINDEXED,
    title,
    description,
    comments_text,
    labels_text,
    tokenize='porter unicode61 remove_diacritics 2'
);

-- +goose Down
DROP TABLE IF EXISTS issues_fts;
