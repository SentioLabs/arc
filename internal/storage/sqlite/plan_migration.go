package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/sentiolabs/arc/internal/planfiles"
	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/storage/sqlite/db"
)

const legacyImportScope = "legacy-import"

// OpenPlanOperator opens an existing, upgraded database without running schema
// migrations, FTS population, cleanup or counters. Read-only mode never creates a
// missing DB. Operators must pair this handle with the configured blob root.
// mode=ro honors committed WAL rather than treating a live database as immutable.
// SQLite may maintain transient shared-index coordination files during reads.
func OpenPlanOperator(path string, writable bool) (*Store, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("operator database path must be absolute")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("operator database must be a regular file")
	}
	// Never create missing databases or run migration startup side effects.
	mode := "ro"
	if writable {
		mode = "rw"
	}
	u := url.URL{Scheme: "file", Path: path}
	sqlDB, err := sql.Open(
		"sqlite",
		u.String()+"?mode="+mode+
			"&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_txlock=immediate",
	)
	if err != nil {
		return nil, err
	}
	// Match Store transaction serialization on the operator connection.
	sqlDB.SetMaxOpenConns(1)
	s := &Store{db: sqlDB, queries: db.New(sqlDB), path: path}
	// Refuse unmigrated schemas instead of implicitly upgrading operator input.
	var version int
	err = sqlDB.QueryRow("SELECT MAX(version_id) FROM goose_db_version WHERE is_applied=1").Scan(&version)
	if err == nil && version < 20 {
		err = errors.New("upgrade the database schema with the matching server before plan operations")
	}
	if err != nil {
		return nil, errors.Join(err, sqlDB.Close())
	}
	return s, nil
}

// ListLegacyPlans exposes original paths and discussions without opening files.
func (s *Store) ListLegacyPlans(ctx context.Context, limit, offset int) ([]storage.LegacyPlanInventory, error) {
	if err := checkPage(limit, offset); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT l.id,COALESCE(p.project_id,'') FROM legacy_plans l
 LEFT JOIN plans p ON p.id=l.id ORDER BY l.id LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	result := make([]storage.LegacyPlanInventory, 0)
	for rows.Next() {
		var item storage.LegacyPlanInventory
		if err := rows.Scan(&item.ID, &item.ImportedProjectID); err != nil {
			return nil, errors.Join(err, rows.Close())
		}
		result = append(result, item)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, err
	}
	// Close the row cursor before dependent queries on the one-connection pool.
	for i := range result {
		legacy, err := s.GetPlan(ctx, result[i].ID)
		if err != nil {
			return nil, err
		}
		result[i].LegacyPlan = *legacy
		result[i].Comments, err = s.ListPlanComments(ctx, legacy.ID)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// LegacyImportAssignment reads the committed marker across projects, preventing
// a retry from assigning a preserved ID to a different project or source path.
// The marker is committed atomically with revision one and never expires.
func (s *Store) LegacyImportAssignment(ctx context.Context, id string) (*storage.LegacyPlanImport, error) {
	var encoded string
	err := s.db.QueryRowContext(
		ctx,
		"SELECT fingerprint FROM plan_idempotency WHERE scope=? AND key=?",
		legacyImportScope,
		id,
	).Scan(&encoded)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // No committed import is distinct from a read failure.
	}
	if err != nil {
		return nil, err
	}
	var assignment storage.LegacyPlanImport
	if err := json.Unmarshal([]byte(encoded), &assignment); err != nil {
		return nil, err
	}
	return &assignment, nil
}

// ImportLegacyPlan retains legacy IDs and comment snapshots in one transaction.
// The idempotency record is also the migration marker; publication precedes it.
func (s *Store) ImportLegacyPlan(ctx context.Context, req storage.LegacyPlanImport) (*storage.PlanWriteResult, error) {
	if err := validatePlanContent(req.Content); err != nil {
		return nil, err
	}
	if req.LegacyID == "" || req.ProjectID == "" || !filepath.IsAbs(req.SourceFile) {
		return nil, storage.ErrPlanInvalid
	}
	// Bind exact bytes and explicit ownership/source assignment to the replay marker.
	encoded, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	identity := planRequestIdentity{
		projectID:   req.ProjectID,
		scope:       legacyImportScope,
		key:         req.LegacyID,
		fingerprint: string(encoded),
	}
	if replay, found, err := lookupPlanRequest(ctx, s.db, identity); err != nil || found {
		return replay, err
	}
	if _, err := s.GetProject(ctx, req.ProjectID); err != nil {
		return nil, err
	}
	if _, err := s.GetPlan(ctx, req.LegacyID); err != nil {
		return nil, err
	}
	if s.planFiles == nil {
		return nil, errors.New("plan publisher unavailable")
	}
	// The publisher lifetime lock spans this publication and the transaction below.
	blob, err := s.planFiles.Publish(ctx, req.ProjectID, req.LegacyID, 1, []byte(req.Content))
	if err != nil {
		return nil, err
	}
	// Recheck identity after publication: competing imports may already have committed.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck
	if replay, found, err := lookupPlanRequest(ctx, tx, identity); err != nil || found {
		return replay, err
	}
	q := s.queries.WithTx(tx)
	legacy, err := q.GetPlan(ctx, req.LegacyID)
	if err != nil {
		return nil, err
	}
	// Preserved IDs cannot overwrite a durable plan or cross-project assignment.
	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM plans WHERE id=?", req.LegacyID).Scan(&exists); err != nil {
		return nil, err
	}
	if exists != 0 {
		return nil, storage.ErrPlanConflict
	}
	// Legacy creation time belongs to the plan; revision creation time is the import time.
	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, `INSERT INTO plans
 (id,project_id,title,head_revision,legacy_status_unverified,created_at,updated_at)
 VALUES(?,?,?,1,?,?,?)`, req.LegacyID, req.ProjectID, req.LegacyID, legacy.Status, legacy.CreatedAt, now)
	if err != nil {
		return nil, err
	}
	if err := importLegacyComments(ctx, tx, q, req.LegacyID); err != nil {
		return nil, err
	}
	// The marker and all imported comments commit with the first draft revision.
	return commitPlanWrite(ctx, tx, q, planPublication{
		identity: identity, id: req.LegacyID, revision: 1, content: req.Content,
		sourceName: req.SourceFile, blob: blob, createdAt: now,
	})
}

// importLegacyComments keeps unknown revision provenance and original timestamps.
// Generic legacy resolution cannot invent reviewed disposition evidence.
func importLegacyComments(ctx context.Context, tx *sql.Tx, q *db.Queries, id string) error {
	comments, err := q.ListPlanComments(ctx, id)
	if err != nil {
		return err
	}
	for _, row := range comments {
		comment := dbPlanCommentToType(row)
		comment.Version = 1
		body, err := json.Marshal(comment)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(
			ctx,
			"INSERT INTO plan_comments(id,plan_id,revision,version,body) VALUES(?,?,NULL,1,?)",
			comment.ID,
			id,
			string(body),
		)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(
			ctx,
			"INSERT INTO plan_comment_events(comment_id,version,body,created_at) VALUES(?,1,?,?)",
			comment.ID,
			string(body),
			comment.CreatedAt,
		)
		if err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, "UPDATE plans SET feedback_version=? WHERE id=?", len(comments), id)
	return err
}

// PlanBlobs returns all referenced artifacts, including archived history.
func (s *Store) PlanBlobs(ctx context.Context) ([]planfiles.Blob, error) {
	rows, err := s.db.QueryContext(
		ctx,
		"SELECT content_path,content_sha256,content_bytes FROM plan_revisions ORDER BY content_path",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	blobs := make([]planfiles.Blob, 0)
	for rows.Next() {
		var blob planfiles.Blob
		if err := rows.Scan(&blob.RelativePath, &blob.SHA256, &blob.Bytes); err != nil {
			return nil, err
		}
		blobs = append(blobs, blob)
	}
	return blobs, rows.Err()
}

// BackupPlansDatabase uses SQLite's consistent VACUUM INTO snapshot, including
// legacy metadata. Caller holds maintenance exclusion across DB and file copy.
func (s *Store) BackupPlansDatabase(ctx context.Context, destination string) error {
	if !filepath.IsAbs(destination) {
		return errors.New("backup destination must be absolute")
	}
	_, err := s.db.ExecContext(ctx, "VACUUM INTO ?", destination)
	return err
}
