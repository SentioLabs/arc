-- +goose Up
CREATE TABLE plan_adoptions (
 id TEXT PRIMARY KEY,
 project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
 container_id TEXT NOT NULL REFERENCES issues(id) ON DELETE RESTRICT,
 request TEXT NOT NULL,
 result TEXT NOT NULL,
 actor TEXT NOT NULL,
 session_id TEXT NOT NULL,
 created_at DATETIME NOT NULL
);
CREATE TABLE adoption_idempotency (
 project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
 container_id TEXT NOT NULL REFERENCES issues(id) ON DELETE RESTRICT,
 key TEXT NOT NULL,
 fingerprint TEXT NOT NULL,
 adoption_id TEXT NOT NULL REFERENCES plan_adoptions(id) ON DELETE RESTRICT,
 PRIMARY KEY(project_id,container_id,key)
);
CREATE TABLE execution_evidence (
 id TEXT PRIMARY KEY,
 project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
 issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE RESTRICT,
 result TEXT NOT NULL,
 created_at DATETIME NOT NULL
);
CREATE INDEX execution_evidence_issue ON execution_evidence(project_id,issue_id,created_at);
CREATE INDEX plan_adoptions_container ON plan_adoptions(project_id,container_id,created_at);
-- +goose StatementBegin
CREATE TRIGGER plan_adoptions_immutable_update BEFORE UPDATE ON plan_adoptions BEGIN
 SELECT RAISE(ABORT,'retained governance history is immutable');
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER plan_adoptions_immutable_delete BEFORE DELETE ON plan_adoptions BEGIN
 SELECT RAISE(ABORT,'retained governance history is immutable');
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER adoption_idempotency_immutable_update BEFORE UPDATE ON adoption_idempotency BEGIN
 SELECT RAISE(ABORT,'retained governance history is immutable');
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER adoption_idempotency_immutable_delete BEFORE DELETE ON adoption_idempotency BEGIN
 SELECT RAISE(ABORT,'retained governance history is immutable');
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER execution_evidence_immutable_update BEFORE UPDATE ON execution_evidence
WHEN OLD.id IS NOT NEW.id OR OLD.issue_id IS NOT NEW.issue_id OR OLD.result IS NOT NEW.result OR OLD.created_at IS NOT NEW.created_at OR json_extract(OLD.result,'$.expected.governing') IS NOT NULL BEGIN
 SELECT RAISE(ABORT,'retained governance history is immutable');
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER execution_evidence_immutable_delete BEFORE DELETE ON execution_evidence BEGIN
 SELECT RAISE(ABORT,'retained governance history is immutable');
END;
-- +goose StatementEnd
-- +goose Down
-- Retained provenance requires coordinated backup restoration.
SELECT 1;
