-- +goose Up
ALTER TABLE plans RENAME TO legacy_plans;
ALTER TABLE plan_comments RENAME TO legacy_plan_comments;

CREATE TABLE plans (
 id TEXT PRIMARY KEY,
 project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
 title TEXT NOT NULL,
 lifecycle TEXT NOT NULL DEFAULT 'active' CHECK(lifecycle IN ('active','archived')),
 head_revision INTEGER NOT NULL CHECK(head_revision > 0),
 version INTEGER NOT NULL DEFAULT 1,
 feedback_version INTEGER NOT NULL DEFAULT 0,
 legacy_status_unverified TEXT,
 created_at TIMESTAMP NOT NULL,
 updated_at TIMESTAMP NOT NULL,
 UNIQUE(project_id,id)
);
CREATE INDEX idx_durable_plans_project ON plans(project_id);
CREATE TABLE plan_revisions (
 plan_id TEXT NOT NULL REFERENCES plans(id) ON DELETE RESTRICT,
 revision INTEGER NOT NULL CHECK(revision > 0),
 content_path TEXT NOT NULL UNIQUE,
 content_sha256 TEXT NOT NULL,
 content_bytes INTEGER NOT NULL,
 source_name TEXT NOT NULL DEFAULT '',
 review_status TEXT NOT NULL DEFAULT 'draft' CHECK(review_status IN ('draft','in_review','changes_requested','approved','rejected')),
 review_version INTEGER NOT NULL DEFAULT 0,
 created_at TIMESTAMP NOT NULL,
 PRIMARY KEY(plan_id,revision)
);
CREATE TABLE plan_comments (
 id TEXT PRIMARY KEY,
 plan_id TEXT NOT NULL REFERENCES plans(id) ON DELETE RESTRICT,
 revision INTEGER,
 version INTEGER NOT NULL,
 body TEXT NOT NULL,
 FOREIGN KEY(plan_id,revision) REFERENCES plan_revisions(plan_id,revision) ON DELETE RESTRICT
);
CREATE INDEX idx_durable_comments_plan ON plan_comments(plan_id,revision);
CREATE TABLE plan_comment_events (
 actor TEXT NOT NULL DEFAULT '',
 session_id TEXT NOT NULL DEFAULT '',
 comment_id TEXT NOT NULL REFERENCES plan_comments(id) ON DELETE RESTRICT,
 version INTEGER NOT NULL,
 body TEXT NOT NULL,
 created_at TIMESTAMP NOT NULL,
 PRIMARY KEY(comment_id,version)
);
CREATE TABLE plan_dispositions (
 actor TEXT NOT NULL DEFAULT '',
 session_id TEXT NOT NULL DEFAULT '',
 id TEXT PRIMARY KEY,
 plan_id TEXT NOT NULL,
 target_revision INTEGER NOT NULL,
 comment_id TEXT NOT NULL REFERENCES plan_comments(id) ON DELETE RESTRICT,
 comment_version INTEGER NOT NULL,
 disposition TEXT NOT NULL CHECK(disposition IN ('addressed','deferred')),
 reason TEXT NOT NULL CHECK(length(trim(reason)) > 0),
 created_at TIMESTAMP NOT NULL,
 FOREIGN KEY(plan_id,target_revision) REFERENCES plan_revisions(plan_id,revision) ON DELETE RESTRICT,
 FOREIGN KEY(comment_id,comment_version) REFERENCES plan_comment_events(comment_id,version) ON DELETE RESTRICT
);
CREATE TABLE plan_review_events (
 actor TEXT NOT NULL DEFAULT '',
 session_id TEXT NOT NULL DEFAULT '',
 id TEXT PRIMARY KEY,
 plan_id TEXT NOT NULL,
 revision INTEGER NOT NULL,
 status TEXT NOT NULL,
 review_version INTEGER NOT NULL,
 feedback_version INTEGER NOT NULL,
 disposition_ids TEXT NOT NULL,
 created_at TIMESTAMP NOT NULL,
 FOREIGN KEY(plan_id,revision) REFERENCES plan_revisions(plan_id,revision) ON DELETE RESTRICT,
 UNIQUE(plan_id,revision,review_version)
);
CREATE TABLE plan_idempotency (
 project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
 scope TEXT NOT NULL,
 key TEXT NOT NULL,
 fingerprint TEXT NOT NULL,
 result TEXT NOT NULL,
 created_at TIMESTAMP NOT NULL,
 PRIMARY KEY(project_id,scope,key)
);

-- +goose Down
-- Durable history must be restored from a coordinated pre-upgrade backup.
SELECT 1;
