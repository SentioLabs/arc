// Durable plan storage preserves exact revisions and append-only review evidence.
// Every mutation validates project ownership on its transaction-bound connection.
// Filesystem content is published before SQLite can commit a reference to it.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sentiolabs/arc/internal/planfiles"
	"github.com/sentiolabs/arc/internal/project"
	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/storage/sqlite/db"
	"github.com/sentiolabs/arc/internal/types"
)

const (
	planLifecycleActive   = "active"
	planLifecycleArchived = "archived"
	maxPlanKeyBytes       = 256
)

var _ storage.DurablePlans = (*Store)(nil)

// durablePlan maps only public metadata; private content paths remain in revision rows.
// Neither provenance nor local draft paths may become public plan identity.
func durablePlan(row *db.Plan) *types.Plan {
	return &types.Plan{
		ID:              row.ID,
		ProjectID:       row.ProjectID,
		Title:           row.Title,
		Lifecycle:       row.Lifecycle,
		HeadRevision:    row.HeadRevision,
		Version:         row.Version,
		FeedbackVersion: row.FeedbackVersion,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

// durableRevision preserves the independent review version of each content revision.
// Saving a new head never resets these historical fields.
func durableRevision(row *db.PlanRevision) *types.PlanRevision {
	return &types.PlanRevision{
		PlanID:        row.PlanID,
		Revision:      row.Revision,
		ContentSHA256: row.ContentSha256,
		ContentBytes:  row.ContentBytes,
		ReviewStatus:  row.ReviewStatus,
		ReviewVersion: row.ReviewVersion,
		CreatedAt:     row.CreatedAt,
	}
}

// revisionBlob reconstructs the private publication capability metadata for verification.
// Read checks the retained digest and size before exposing content.
func revisionBlob(row *db.PlanRevision) planfiles.Blob {
	return planfiles.Blob{
		RelativePath: row.ContentPath,
		SHA256:       row.ContentSha256,
		Bytes:        row.ContentBytes,
	}
}

// getDurablePlan accepts transaction-bound queries to avoid single-connection deadlocks.
// Identity and project ownership are checked by the same query.
func getDurablePlan(ctx context.Context, q *db.Queries, projectID, id string) (*types.Plan, error) {
	row, err := q.GetDurablePlan(ctx, db.GetDurablePlanParams{ProjectID: projectID, ID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrPlanNotFound
	}
	if err != nil {
		return nil, err
	}
	return durablePlan(row), nil
}

// getRevision reads a precise revision using the caller transaction when present.
// Its parent ownership must already have been established by getDurablePlan.
func getRevision(
	ctx context.Context,
	q *db.Queries,
	id string,
	revision int64,
) (*db.PlanRevision, error) {
	row, err := q.GetPlanRevision(ctx, db.GetPlanRevisionParams{PlanID: id, Revision: revision})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrPlanNotFound
	}
	return row, err
}

// GetDurablePlan returns project-scoped public metadata for direct service consumers.
// Transactional mutations use getDurablePlan with their own query handle instead.
func (s *Store) GetDurablePlan(ctx context.Context, projectID, id string) (*types.Plan, error) {
	return getDurablePlan(ctx, s.queries, projectID, id)
}

// checkPage rejects unbounded history reads at the service boundary.
// API defaults are conveniences; callers still must choose an explicit bound.
func checkPage(limit, offset int) error {
	if limit < 1 || limit > 200 || offset < 0 {
		return fmt.Errorf("%w: limit must be 1..200 and offset nonnegative", storage.ErrPlanInvalid)
	}
	return nil
}

// ListDurablePlans lists one lifecycle within a project with bounded pagination.
// Unknown projects and empty known projects remain distinct results.
func (s *Store) ListDurablePlans(
	ctx context.Context,
	projectID string,
	archived bool,
	limit, offset int,
) ([]*types.Plan, error) {
	if err := checkPage(limit, offset); err != nil {
		return nil, err
	}
	if _, err := s.queries.GetProject(ctx, projectID); errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrPlanNotFound
	} else if err != nil {
		return nil, err
	}
	lifecycle := planLifecycleActive
	if archived {
		lifecycle = planLifecycleArchived
	}
	rows, err := s.queries.ListDurablePlans(
		ctx,
		db.ListDurablePlansParams{
			ProjectID: projectID,
			Lifecycle: lifecycle,
			Limit:     int64(limit),
			Offset:    int64(offset),
		},
	)
	if err != nil {
		return nil, err
	}
	result := make([]*types.Plan, 0, len(rows))
	for _, row := range rows {
		result = append(result, durablePlan(row))
	}
	return result, nil
}

// ListPlanRevisions exposes immutable revision metadata ordered newest first.
// Project validation happens before accessing the parent revision collection.
func (s *Store) ListPlanRevisions(
	ctx context.Context,
	projectID, id string,
	limit, offset int,
) ([]*types.PlanRevision, error) {
	if err := checkPage(limit, offset); err != nil {
		return nil, err
	}
	if _, err := s.GetDurablePlan(ctx, projectID, id); err != nil {
		return nil, err
	}
	rows, err := s.queries.ListPlanRevisions(
		ctx,
		db.ListPlanRevisionsParams{PlanID: id, Limit: int64(limit), Offset: int64(offset)},
	)
	if err != nil {
		return nil, err
	}
	result := make([]*types.PlanRevision, 0, len(rows))
	for _, row := range rows {
		result = append(result, durableRevision(row))
	}
	return result, nil
}

// ReadPlanRevision verifies retained content on every read.
// Archive preserves access; missing or tampered content cannot be substituted.
func (s *Store) ReadPlanRevision(
	ctx context.Context,
	projectID, id string,
	revision int64,
) (*types.PlanRevisionWithContent, error) {
	if _, err := s.GetDurablePlan(ctx, projectID, id); err != nil {
		return nil, err
	}
	row, err := getRevision(ctx, s.queries, id, revision)
	if err != nil {
		return nil, err
	}
	if s.planFiles == nil {
		return nil, errors.New("plan publisher unavailable")
	}
	content, err := s.planFiles.Read(ctx, revisionBlob(row))
	if err != nil {
		return nil, err
	}
	return &types.PlanRevisionWithContent{
		PlanRevision: *durableRevision(row),
		Content:      string(content),
	}, nil
}

// validatePlanContent runs before publication or idempotency reservation.
// Empty content is valid, while invalid UTF-8 and oversized uploads are rejected.
func validatePlanContent(content string) error {
	if len(content) > planfiles.MaxContentBytes || !utf8.ValidString(content) {
		return fmt.Errorf("%w: content must be UTF-8 and at most 10 MiB", storage.ErrPlanInvalid)
	}
	return nil
}

// validatePlanKey bounds the retained request identity without transforming it.
// Failed validation never reserves a key in the database.
func validatePlanKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return storage.ErrPlanPrecondition
	}
	if len(key) > maxPlanKeyBytes {
		return fmt.Errorf("%w: idempotency key exceeds 256 bytes", storage.ErrPlanInvalid)
	}
	return nil
}

// CreateDurablePlan publishes bytes before inserting any durable reference.
// It checks committed replay both before publication and inside the write transaction.
// Failed or uncertain commits retain unreferenced blobs for operator inspection.
func (s *Store) CreateDurablePlan(
	ctx context.Context,
	projectID, key string,
	req storage.PlanUpload,
) (*storage.PlanWriteResult, error) {
	if err := validatePlanKey(key); err != nil {
		return nil, err
	}
	if err := validatePlanContent(req.Content); err != nil {
		return nil, err
	}
	fingerprint, err := planFingerprint(req)
	if err != nil {
		return nil, err
	}
	scope := "create"
	identity := planRequestIdentity{
		projectID:   projectID,
		scope:       scope,
		key:         key,
		fingerprint: fingerprint,
	}
	if replay, found, err := lookupPlanRequest(ctx, s.db, identity); err != nil ||
		found {
		return replay, err
	}
	if _, err := s.queries.GetProject(ctx, projectID); errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrPlanNotFound
	} else if err != nil {
		return nil, err
	}
	id := project.GeneratePlanID(projectID + key)
	if s.planFiles == nil {
		return nil, errors.New("plan publisher unavailable")
	}
	blob, err := s.planFiles.Publish(ctx, projectID, id, 1, []byte(req.Content))
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck
	if replay, found, err := lookupPlanRequest(ctx, tx, identity); err != nil ||
		found {
		return replay, err
	}
	q := s.queries.WithTx(tx)
	if _, err := q.GetProject(ctx, projectID); errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrPlanNotFound
	} else if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	_, err = tx.ExecContext(
		ctx,
		"INSERT INTO plans(id,project_id,title,head_revision,created_at,updated_at) VALUES(?,?,?,1,?,?)",
		id,
		projectID,
		req.Title,
		now,
		now,
	)
	if err != nil {
		return nil, err
	}
	return commitPlanWrite(
		ctx,
		tx,
		q,
		planPublication{
			identity:   identity,
			id:         id,
			revision:   1,
			content:    req.Content,
			sourceName: req.SourceName,
			blob:       blob,
			createdAt:  now,
		},
	)
}

// SavePlanRevision creates a new draft for every accepted explicit save.
// The expected head is revalidated after filesystem publication in the write transaction.
// Identical retries return their historical result even after later state changes.
func (s *Store) SavePlanRevision(
	ctx context.Context,
	projectID, id, key string,
	req storage.PlanSave,
) (*storage.PlanWriteResult, error) {
	if err := validatePlanKey(key); err != nil {
		return nil, err
	}
	if err := validatePlanContent(req.Content); err != nil {
		return nil, err
	}
	if req.ExpectedRevision < 1 {
		return nil, storage.ErrPlanInvalid
	}
	fingerprint, err := planFingerprint(req)
	if err != nil {
		return nil, err
	}
	scope := "save:" + id
	identity := planRequestIdentity{
		projectID:   projectID,
		scope:       scope,
		key:         key,
		fingerprint: fingerprint,
	}
	if replay, found, err := s.planSavePreflight(ctx, identity, id, req.ExpectedRevision); err != nil ||
		found {
		return replay, err
	}

	if s.planFiles == nil {
		return nil, errors.New("plan publisher unavailable")
	}
	revision := req.ExpectedRevision + 1
	blob, err := s.planFiles.Publish(ctx, projectID, id, revision, []byte(req.Content))
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck
	if replay, found, err := lookupPlanRequest(ctx, tx, identity); err != nil ||
		found {
		return replay, err
	}
	q := s.queries.WithTx(tx)
	plan, err := getDurablePlan(ctx, q, projectID, id)
	if err != nil {
		return nil, err
	}
	if err := checkWritablePlan(plan, req.ExpectedRevision); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	_, err = tx.ExecContext(
		ctx,
		"UPDATE plans SET head_revision=?,version=version+1,updated_at=? WHERE id=?",
		revision,
		now,
		id,
	)
	if err != nil {
		return nil, err
	}
	return commitPlanWrite(
		ctx,
		tx,
		q,
		planPublication{
			identity:   identity,
			id:         id,
			revision:   revision,
			content:    req.Content,
			sourceName: req.SourceName,
			blob:       blob,
			createdAt:  now,
		},
	)
}

// checkWritablePlan enforces active lifecycle and the supplied editing base.
// Callers must check committed replay first, because historical results remain valid.
func checkWritablePlan(plan *types.Plan, head int64) error {
	if plan.Lifecycle != planLifecycleActive || plan.HeadRevision != head {
		return fmt.Errorf(
			"%w: current head %d, lifecycle %s",
			storage.ErrPlanConflict,
			plan.HeadRevision,
			plan.Lifecycle,
		)
	}
	return nil
}

// planPublication carries the durable file and validated request identity into commit.
type planPublication struct {
	identity            planRequestIdentity
	id                  string
	revision            int64
	content, sourceName string
	blob                planfiles.Blob
	createdAt           time.Time
}

// commitPlanWrite records publication metadata and its exact replay result atomically.
// Only successful commit permits acknowledgment; rollback never deletes a blob.
// All metadata reads use the supplied transaction-bound query handle.
func commitPlanWrite(
	ctx context.Context,
	tx *sql.Tx,
	q *db.Queries,
	publication planPublication,
) (*storage.PlanWriteResult, error) {
	id, revision := publication.id, publication.revision
	content, sourceName := publication.content, publication.sourceName
	blob, now := publication.blob, publication.createdAt
	projectID := publication.identity.projectID

	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO plan_revisions(plan_id,revision,content_path,content_sha256,content_bytes,source_name,created_at)
		VALUES (?,?,?,?,?,?,?)`,
		id,
		revision,
		blob.RelativePath,
		blob.SHA256,
		blob.Bytes,
		sourceName,
		now,
	)
	if err != nil {
		return nil, err
	}
	plan, err := getDurablePlan(ctx, q, projectID, id)
	if err != nil {
		return nil, err
	}
	row, err := getRevision(ctx, q, id, revision)
	if err != nil {
		return nil, err
	}
	result := &storage.PlanWriteResult{
		Plan: *plan,
		Revision: types.PlanRevisionWithContent{
			PlanRevision: *durableRevision(row),
			Content:      content,
		},
	}
	if err := recordPlanRequest(ctx, tx, publication.identity, result); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

// UpdateDurablePlan serializes title and lifecycle edits with content/review mutations.
// Restore changes lifecycle without changing any historical review decisions.
func (s *Store) UpdateDurablePlan(
	ctx context.Context,
	projectID, id string,
	req storage.PlanMetadataUpdate,
) (*types.Plan, error) {
	if req.ExpectedVersion < 1 {
		return nil, storage.ErrPlanInvalid
	}
	if req.Lifecycle != "" && req.Lifecycle != planLifecycleActive &&
		req.Lifecycle != planLifecycleArchived {
		return nil, storage.ErrPlanInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck
	q := s.queries.WithTx(tx)
	plan, err := getDurablePlan(ctx, q, projectID, id)
	if err != nil {
		return nil, err
	}
	if plan.Version != req.ExpectedVersion {
		return nil, fmt.Errorf("%w: current version %d", storage.ErrPlanConflict, plan.Version)
	}
	if req.Title != nil {
		plan.Title = *req.Title
	}
	if req.Lifecycle != "" {
		plan.Lifecycle = req.Lifecycle
	}
	plan.Version++
	plan.UpdatedAt = time.Now().UTC()
	_, err = tx.ExecContext(
		ctx,
		"UPDATE plans SET title=?,lifecycle=?,version=?,updated_at=? WHERE id=?",
		plan.Title,
		plan.Lifecycle,
		plan.Version,
		plan.UpdatedAt,
		id,
	)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return plan, nil
}

// planSavePreflight reads replay and mutable head from one coherent snapshot.
// An identical writer cannot commit between a missed replay lookup and the head
// check. The final write transaction repeats these checks after publication.
func (s *Store) planSavePreflight(
	ctx context.Context,
	identity planRequestIdentity,
	id string,
	head int64,
) (*storage.PlanWriteResult, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback() //nolint:errcheck
	if replay, found, err := lookupPlanRequest(ctx, tx, identity); err != nil || found {
		return replay, found, err
	}
	plan, err := getDurablePlan(ctx, s.queries.WithTx(tx), identity.projectID, id)
	if err != nil {
		return nil, false, err
	}
	return nil, false, checkWritablePlan(plan, head)
}
