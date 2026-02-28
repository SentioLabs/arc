-- +goose Up
CREATE TABLE labels_new (
    name TEXT PRIMARY KEY,
    color TEXT,
    description TEXT
);

INSERT OR IGNORE INTO labels_new (name, color, description)
    SELECT name, color, description FROM labels;

DROP TABLE labels;

ALTER TABLE labels_new RENAME TO labels;

-- +goose Down
CREATE TABLE labels_new (
    workspace_id TEXT NOT NULL,
    name TEXT NOT NULL,
    color TEXT,
    description TEXT,
    PRIMARY KEY (workspace_id, name),
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
);

INSERT INTO labels_new (workspace_id, name, color, description)
    SELECT '', name, color, description FROM labels;

DROP TABLE labels;

ALTER TABLE labels_new RENAME TO labels;
