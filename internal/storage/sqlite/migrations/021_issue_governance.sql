-- +goose Up
ALTER TABLE issues ADD COLUMN contract_version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE issues ADD COLUMN governing_plan_id TEXT REFERENCES plans(id) ON DELETE RESTRICT;
ALTER TABLE issues ADD COLUMN governing_plan_revision INTEGER;
ALTER TABLE projects ADD COLUMN governance_generation INTEGER NOT NULL DEFAULT 0;

-- Retain container provenance even after an explicit pin is detached. Reconciliation
-- and phase evidence are owned by their own append-only records in the adoption layer.
CREATE TABLE issue_governance_history (
 project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
 issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE RESTRICT,
 plan_id TEXT NOT NULL,
 revision INTEGER NOT NULL,
 PRIMARY KEY(issue_id,plan_id,revision),
 FOREIGN KEY(project_id,plan_id) REFERENCES plans(project_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(plan_id,revision) REFERENCES plan_revisions(plan_id,revision) ON DELETE RESTRICT
);
-- +goose StatementBegin
CREATE TRIGGER issue_contract_updated AFTER UPDATE ON issues
WHEN OLD.title IS NOT NEW.title OR OLD.description IS NOT NEW.description
 OR OLD.issue_type IS NOT NEW.issue_type OR OLD.status IS NOT NEW.status
 OR OLD.ai_session_id IS NOT NEW.ai_session_id OR OLD.project_id IS NOT NEW.project_id
BEGIN
 UPDATE issues SET contract_version=contract_version+1 WHERE id=NEW.id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER issue_created_generation AFTER INSERT ON issues BEGIN
 UPDATE projects SET governance_generation=governance_generation+1 WHERE id=NEW.project_id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER issue_deleted_generation AFTER DELETE ON issues BEGIN
 UPDATE projects SET governance_generation=governance_generation+1 WHERE id=OLD.project_id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER issue_type_generation AFTER UPDATE OF issue_type ON issues
WHEN OLD.issue_type IS NOT NEW.issue_type BEGIN
 UPDATE projects SET governance_generation=governance_generation+1 WHERE id=NEW.project_id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER issue_pin_valid_insert BEFORE INSERT ON issues
WHEN NEW.governing_plan_id IS NOT NULL OR NEW.governing_plan_revision IS NOT NULL BEGIN
 SELECT RAISE(ABORT,'pins require an existing epic or milestone');
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER issue_pin_valid_update BEFORE UPDATE OF governing_plan_id,governing_plan_revision,project_id,issue_type ON issues
WHEN NEW.governing_plan_id IS NOT NULL OR NEW.governing_plan_revision IS NOT NULL BEGIN
 SELECT RAISE(ABORT,'invalid governing pin') WHERE NEW.issue_type NOT IN ('epic','milestone')
 OR NEW.governing_plan_id IS NULL OR NEW.governing_plan_revision IS NULL
 OR NOT EXISTS (SELECT 1 FROM plans p JOIN plan_revisions r ON r.plan_id=p.id
 WHERE p.id=NEW.governing_plan_id AND p.project_id=NEW.project_id
 AND r.revision=NEW.governing_plan_revision AND r.review_status='approved');
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER issue_pin_changed AFTER UPDATE OF governing_plan_id,governing_plan_revision ON issues
WHEN OLD.governing_plan_id IS NOT NEW.governing_plan_id OR OLD.governing_plan_revision IS NOT NEW.governing_plan_revision BEGIN
 INSERT OR IGNORE INTO issue_governance_history(project_id,issue_id,plan_id,revision)
 SELECT NEW.project_id,NEW.id,NEW.governing_plan_id,NEW.governing_plan_revision WHERE NEW.governing_plan_id IS NOT NULL;
 UPDATE projects SET governance_generation=governance_generation+1 WHERE id=NEW.project_id;
 UPDATE issues SET contract_version=contract_version+1 WHERE id IN (
 WITH RECURSIVE descendants(id) AS (SELECT NEW.id UNION SELECT d.issue_id FROM dependencies d JOIN descendants a ON d.depends_on_id=a.id WHERE d.type='parent-child') SELECT id FROM descendants);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER dependency_insert_versions AFTER INSERT ON dependencies  BEGIN
 UPDATE issues SET contract_version=contract_version+1 WHERE id=NEW.issue_id;
 UPDATE projects SET governance_generation=governance_generation+1 WHERE id=(SELECT project_id FROM issues WHERE id=NEW.issue_id) AND NEW.type='parent-child';
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER dependency_delete_versions AFTER DELETE ON dependencies  BEGIN
 UPDATE issues SET contract_version=contract_version+1 WHERE id=OLD.issue_id;
 UPDATE projects SET governance_generation=governance_generation+1 WHERE id=(SELECT project_id FROM issues WHERE id=OLD.issue_id) AND OLD.type='parent-child';
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER dependency_update_versions AFTER UPDATE ON dependencies WHEN OLD.type IS NOT NEW.type OR OLD.issue_id IS NOT NEW.issue_id OR OLD.depends_on_id IS NOT NEW.depends_on_id BEGIN
 UPDATE issues SET contract_version=contract_version+1 WHERE id=NEW.issue_id;
 UPDATE projects SET governance_generation=governance_generation+1 WHERE id=(SELECT project_id FROM issues WHERE id=NEW.issue_id) AND (OLD.type='parent-child' OR NEW.type='parent-child');
END;
-- +goose StatementEnd

-- +goose Down
-- Restore a coordinated pre-upgrade backup to remove retained governance history.
SELECT 1;
