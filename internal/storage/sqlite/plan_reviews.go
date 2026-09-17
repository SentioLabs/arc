// Durable plan storage preserves exact revisions and append-only review evidence.
// Every mutation validates project ownership on its transaction-bound connection.
// Filesystem content is published before SQLite can commit a reference to it.
package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/types"
)

// DecidePlanRevision serializes review evidence with both content and feedback.
func (s *Store) DecidePlanRevision(
	ctx context.Context,
	projectID, id string,
	revision int64,
	req types.PlanReviewRequest,
) (*types.PlanRevision, error) {
	// Immediate transactions serialize this snapshot with independent Store handles.
	// All reads below use tx; Store-level reads would deadlock its single connection.
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
	row, err := getRevision(ctx, q, id, revision)
	if err != nil {
		return nil, err
	}
	if err := checkWritablePlan(plan, req.ExpectedHead); err != nil {
		return nil, err
	}
	if revision != plan.HeadRevision || row.ReviewVersion != req.ExpectedReviewVersion ||
		plan.FeedbackVersion != req.ExpectedFeedbackVersion {
		return nil, fmt.Errorf(
			"%w: head %d, review version %d, feedback version %d",
			storage.ErrPlanConflict,
			plan.HeadRevision,
			row.ReviewVersion,
			plan.FeedbackVersion,
		)
	}
	if !validReviewTransition(row.ReviewStatus, req.Status) {
		return nil, fmt.Errorf(
			"%w: invalid review transition %s to %s",
			storage.ErrPlanConflict,
			row.ReviewStatus,
			req.Status,
		)
	}

	// Decision evidence records immutable disposition IDs rather than a mutable
	// resolved flag. Each referenced disposition also identifies a comment version.
	dispositions := []string{}
	// Content integrity and the complete feedback set are part of this decision.
	// Late feedback after commit belongs to the next review, not retroactive revocation.
	if req.Status == types.PlanStatusApproved {
		if s.planFiles == nil {
			return nil, errors.New("plan publisher unavailable")
		}
		if _, err := s.planFiles.Read(ctx, revisionBlob(row)); err != nil {
			return nil, err
		}
		dispositions, err = approvalDispositions(ctx, tx, id, revision)
		if err != nil {
			return nil, err
		}
	}
	encoded, err := json.Marshal(dispositions)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO plan_review_events(id,plan_id,revision,status,review_version,feedback_version,
 disposition_ids,created_at,actor,session_id)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		rand.Text(),
		id,
		revision,
		req.Status,
		row.ReviewVersion+1,
		plan.FeedbackVersion,
		string(encoded),
		now,
		storage.PlanProvenanceFromContext(ctx).Actor,
		storage.PlanProvenanceFromContext(ctx).SessionID,
	)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(
		ctx,
		"UPDATE plan_revisions SET review_status=?,review_version=review_version+1 WHERE plan_id=? AND revision=?",
		req.Status,
		id,
		revision,
	)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(
		ctx,
		"UPDATE plans SET version=version+1,updated_at=? WHERE id=?",
		now,
		id,
	)
	if err != nil {
		return nil, err
	}
	// Acknowledgment follows commit; no audit record is exposed on rollback.
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	result := durableRevision(row)
	result.ReviewStatus = req.Status
	result.ReviewVersion++
	return result, nil
}

// Deleted feedback remains an obligation. Addressed dispositions carry forward
// only for the same version; deferrals apply to exactly one target revision.
func approvalDispositions(
	ctx context.Context,
	tx *sql.Tx,
	id string,
	revision int64,
) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT c.id, (
 SELECT d.id FROM plan_dispositions d WHERE d.plan_id=c.plan_id AND d.comment_id=c.id AND d.comment_version=c.version
 AND d.target_revision<=? AND (d.disposition='addressed' OR d.target_revision=?) AND length(trim(d.reason))>0
 ORDER BY d.target_revision DESC,d.created_at DESC,d.id DESC LIMIT 1)
 FROM plan_comments c WHERE c.plan_id=? AND (c.revision<=? OR c.revision IS NULL)
 ORDER BY c.id`, revision, revision, id, revision)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// Null results represent obligations with no valid disposition for this target.
	// Tombstones and unknown legacy revisions are intentionally included above.
	result := []string{}
	for rows.Next() {
		var comment string
		var disposition sql.NullString
		if err := rows.Scan(&comment, &disposition); err != nil {
			return nil, err
		}
		if !disposition.Valid {
			return nil, fmt.Errorf("%w: %s", storage.ErrUnresolvedFeedback, comment)
		}
		result = append(result, disposition.String)
	}
	return result, rows.Err()
}

// validateComment preserves the established quote and line-range anchor semantics.
// It validates service callers as well as decoded HTTP requests.
// Empty feedback cannot create an approval obligation with no readable explanation.
func validateComment(req storage.PlanCommentCreate) error {
	if strings.TrimSpace(req.Content) == "" || len(req.Content) > 1024*1024 {
		return fmt.Errorf(
			"%w: comment content must be nonempty and at most 1 MiB",
			storage.ErrPlanInvalid,
		)
	}
	if req.LineNumber != nil && *req.LineNumber < 1 {
		return fmt.Errorf("%w: line_number must be positive", storage.ErrPlanInvalid)
	}
	if a := req.Anchor; a != nil &&
		(a.LineStart < 1 || a.LineEnd < a.LineStart || a.Occurrence < 0 || strings.TrimSpace(a.QuotedText) == "") {
		return fmt.Errorf("%w: invalid comment anchor", storage.ErrPlanInvalid)
	}
	return nil
}

// CreateRevisionComment binds feedback to an exact retained revision.
// Comments on decided revisions are permitted and enter later reviews.
// The creation event and feedback counter commit together with the comment.
func (s *Store) CreateRevisionComment(
	ctx context.Context,
	projectID, id string,
	revision int64,
	req storage.PlanCommentCreate,
) (*types.PlanComment, error) {
	if err := validateComment(req); err != nil {
		return nil, err
	}
	// Immediate transactions serialize this snapshot with independent Store handles.
	// All reads below use tx; Store-level reads would deadlock its single connection.
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
	if plan.Lifecycle != planLifecycleActive {
		return nil, storage.ErrPlanConflict
	}
	if _, err := getRevision(ctx, q, id, revision); err != nil {
		return nil, err
	}
	comment := &types.PlanComment{
		ID:         "pc." + rand.Text(),
		PlanID:     id,
		Revision:   &revision,
		Version:    1,
		Content:    req.Content,
		LineNumber: req.LineNumber,
		Anchor:     req.Anchor,
		CreatedAt:  time.Now().UTC(),
	}
	if req.Anchor != nil {
		v := req.Anchor.LineStart
		comment.LineNumber = &v
	}
	body, err := json.Marshal(comment)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(
		ctx,
		"INSERT INTO plan_comments(id,plan_id,revision,version,body) VALUES(?,?,?,?,?)",
		comment.ID,
		id,
		revision,
		comment.Version,
		string(body),
	)
	if err != nil {
		return nil, err
	}
	if err := recordCommentVersion(ctx, tx, comment); err != nil {
		return nil, err
	}
	// Acknowledgment follows commit; no audit record is exposed on rollback.
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return comment, nil
}

// recordCommentVersion appends the complete comment snapshot for audit.
// Prior text, anchors, and deletion state remain recoverable by version.
// Every snapshot changes the plan feedback version in the same transaction.
func recordCommentVersion(ctx context.Context, tx *sql.Tx, comment *types.PlanComment) error {
	body, err := json.Marshal(comment)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(
		ctx,
		"INSERT INTO plan_comment_events(comment_id,version,body,created_at,actor,session_id) VALUES(?,?,?,?,?,?)",
		comment.ID,
		comment.Version,
		string(body),
		time.Now().UTC(),
		storage.PlanProvenanceFromContext(ctx).Actor,
		storage.PlanProvenanceFromContext(ctx).SessionID,
	)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(
		ctx,
		"UPDATE plans SET feedback_version=feedback_version+1,updated_at=? WHERE id=?",
		time.Now().UTC(),
		comment.PlanID,
	)
	return err
}

// UpdateRevisionComment checks the original comment version before applying an edit.
// Editing, reopening, or deleting invalidates dispositions bound to older versions.
// Deletion retains original text and anchor in a tombstone and append-only event.
// No generic resolved flag can remove an outstanding approval obligation.
//
//revive:disable-next-line:argument-limit // Explicit project, plan, revision and comment/pagination scope.
func (s *Store) UpdateRevisionComment(
	ctx context.Context,
	projectID, id string,
	revision int64,
	commentID string,
	req storage.PlanCommentUpdate,
) (*types.PlanComment, error) {
	if req.ExpectedVersion < 1 {
		return nil, storage.ErrPlanInvalid
	}
	// Immediate transactions serialize this snapshot with independent Store handles.
	// All reads below use tx; Store-level reads would deadlock its single connection.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck
	plan, err := getDurablePlan(ctx, s.queries.WithTx(tx), projectID, id)
	if err != nil {
		return nil, err
	}
	if plan.Lifecycle != planLifecycleActive {
		return nil, storage.ErrPlanConflict
	}
	var body string
	err = tx.QueryRowContext(
		ctx,
		"SELECT body FROM plan_comments WHERE id=? AND plan_id=? AND revision=?",
		commentID,
		id,
		revision,
	).
		Scan(
			&body,
		)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrPlanNotFound
	}
	if err != nil {
		return nil, err
	}
	var comment types.PlanComment
	if err := json.Unmarshal([]byte(body), &comment); err != nil {
		return nil, err
	}
	if comment.Version != req.ExpectedVersion || comment.DeletedAt != nil {
		return nil, storage.ErrPlanConflict
	}
	if req.Content != nil {
		comment.Content = *req.Content
	}
	if req.Anchor != nil {
		comment.Anchor = req.Anchor
		v := req.Anchor.LineStart
		comment.LineNumber = &v
	}
	if err := validateComment(storage.PlanCommentCreate{
		Content:    comment.Content,
		Anchor:     comment.Anchor,
		LineNumber: comment.LineNumber,
	}); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	comment.UpdatedAt = &now
	comment.ResolvedAt = nil
	comment.Version++
	if req.Delete {
		comment.DeletedAt = &now
	}
	encoded, err := json.Marshal(comment)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(
		ctx,
		"UPDATE plan_comments SET body=?,version=? WHERE id=?",
		string(encoded),
		comment.Version,
		commentID,
	)
	if err != nil {
		return nil, err
	}
	if err := recordCommentVersion(ctx, tx, &comment); err != nil {
		return nil, err
	}
	// Acknowledgment follows commit; no audit record is exposed on rollback.
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &comment, nil
}

// ListRevisionComments returns bounded original feedback without re-anchoring it.
// includePrior also exposes imported legacy discussion with unknown revision.
// Deleted comments remain visible for audit and disposition assessment.
//
//revive:disable-next-line:argument-limit // Explicit project, plan, revision and comment/pagination scope.
func (s *Store) ListRevisionComments(
	ctx context.Context,
	projectID, id string,
	revision int64,
	includePrior bool,
	limit, offset int,
) ([]*types.PlanComment, error) {
	if err := checkPage(limit, offset); err != nil {
		return nil, err
	}
	if _, err := s.GetDurablePlan(ctx, projectID, id); err != nil {
		return nil, err
	}
	if _, err := getRevision(ctx, s.queries, id, revision); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT body FROM plan_comments WHERE plan_id=?
 AND (revision=? OR (? AND (revision<=? OR revision IS NULL))) ORDER BY revision,id LIMIT ? OFFSET ?`,
		id,
		revision,
		includePrior,
		revision,
		limit,
		offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	comments := []*types.PlanComment{}
	for rows.Next() {
		var body string
		if err := rows.Scan(&body); err != nil {
			return nil, err
		}
		var comment types.PlanComment
		if err := json.Unmarshal([]byte(body), &comment); err != nil {
			return nil, err
		}
		comments = append(comments, &comment)
	}
	return comments, rows.Err()
}

// AddPlanDisposition appends an addressed or deferred assessment with a reason.
// It checks both the comment version and plan feedback version atomically.
// Approved and rejected revisions retain their decision evidence unchanged.
// Legacy comments with unknown content provenance can still receive dispositions.
func (s *Store) AddPlanDisposition(
	ctx context.Context,
	projectID, id string,
	revision int64,
	req storage.PlanDispositionRequest,
) (*types.PlanFeedbackDisposition, error) {
	if req.Disposition != "addressed" && req.Disposition != "deferred" ||
		strings.TrimSpace(req.Reason) == "" {
		return nil, storage.ErrPlanInvalid
	}
	// Immediate transactions serialize this snapshot with independent Store handles.
	// All reads below use tx; Store-level reads would deadlock its single connection.
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
	if err := checkWritablePlan(plan, revision); err != nil {
		return nil, err
	}
	row, err := getRevision(ctx, q, id, revision)
	if err != nil {
		return nil, err
	}
	if row.ReviewStatus == types.PlanStatusApproved ||
		row.ReviewStatus == types.PlanStatusRejected ||
		plan.FeedbackVersion != req.ExpectedFeedbackVersion {
		return nil, storage.ErrPlanConflict
	}
	var version int64
	err = tx.QueryRowContext(
		ctx,
		"SELECT version FROM plan_comments WHERE id=? AND plan_id=? AND (revision<=? OR revision"+
			" IS NULL)",
		req.CommentID,
		id,
		revision,
	).
		Scan(
			&version,
		)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrPlanNotFound
	}
	if err != nil {
		return nil, err
	}
	if version != req.ExpectedCommentVersion {
		return nil, storage.ErrPlanConflict
	}
	d := &types.PlanFeedbackDisposition{
		ID:             "pd." + rand.Text(),
		PlanID:         id,
		TargetRevision: revision,
		CommentID:      req.CommentID,
		CommentVersion: version,
		Disposition:    req.Disposition,
		Reason:         req.Reason,
		CreatedAt:      time.Now().UTC(),
	}
	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO plan_dispositions(id,plan_id,target_revision,comment_id,comment_version,
 disposition,reason,created_at,actor,session_id)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		d.ID,
		id,
		revision,
		d.CommentID,
		version,
		d.Disposition,
		d.Reason,
		d.CreatedAt,
		storage.PlanProvenanceFromContext(ctx).Actor,
		storage.PlanProvenanceFromContext(ctx).SessionID,
	)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(
		ctx,
		"UPDATE plans SET feedback_version=feedback_version+1,updated_at=? WHERE id=?",
		d.CreatedAt,
		id,
	)
	if err != nil {
		return nil, err
	}
	// Acknowledgment follows commit; no audit record is exposed on rollback.
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return d, nil
}

// ListPlanDispositions retains every assessment through the selected revision.
// Callers can inspect superseded reasons and original comment versions.
// The rows are append-only, so approval event identifiers remain stable.
//
//revive:disable-next-line:argument-limit // Explicit project, plan, revision and comment/pagination scope.
func (s *Store) ListPlanDispositions(
	ctx context.Context,
	projectID, id string,
	revision int64,
	limit, offset int,
) ([]*types.PlanFeedbackDisposition, error) {
	if err := checkPage(limit, offset); err != nil {
		return nil, err
	}
	if _, err := s.GetDurablePlan(ctx, projectID, id); err != nil {
		return nil, err
	}
	if _, err := getRevision(ctx, s.queries, id, revision); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id,plan_id,target_revision,comment_id,comment_version,disposition,reason,created_at
		FROM  plan_dispositions
		WHERE  plan_id=? AND target_revision<=? ORDER BY created_at,id LIMIT ? OFFSET ?`,
		id,
		revision,
		limit,
		offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPlanDispositions(rows)
}

// validReviewTransition keeps terminal decisions immutable and requires submission
// before any decision. A changes-requested revision can be resubmitted unchanged.
func validReviewTransition(from, to string) bool {
	switch from {
	case types.PlanStatusDraft, types.PlanStatusChangesRequested:
		return to == types.PlanStatusInReview
	case types.PlanStatusInReview:
		return to == types.PlanStatusApproved || to == types.PlanStatusRejected ||
			to == types.PlanStatusChangesRequested
	default:
		return false
	}
}

// retainedReviewEvent holds the original ID set while the event cursor is open.
// Disposition reads happen after that cursor closes to respect the single pool connection.
type retainedReviewEvent struct {
	event          *storage.PlanReviewEvent
	dispositionIDs string
}

// ListPlanReviewEvents exposes exact historical decisions for one retained revision.
// It deliberately ignores current head/lifecycle and current feedback obligations.
// Paging selects events before expanding the disposition set captured at decision time.
//
//revive:disable-next-line:argument-limit // Explicit project, plan, revision and pagination scope.
func (s *Store) ListPlanReviewEvents(
	ctx context.Context, projectID, id string, revision int64, limit, offset int,
) ([]*storage.PlanReviewEvent, error) {
	if err := checkPage(limit, offset); err != nil {
		return nil, err
	}
	if _, err := s.GetDurablePlan(ctx, projectID, id); err != nil {
		return nil, err
	}
	if _, err := getRevision(ctx, s.queries, id, revision); err != nil {
		return nil, err
	}
	records, err := s.readPlanReviewEvents(ctx, id, revision, limit, offset)
	if err != nil {
		return nil, err
	}
	result := make([]*storage.PlanReviewEvent, 0, len(records))
	for _, record := range records {
		// Resolve immutable IDs, never infer the approved set from today's comments.
		dispositions, err := s.readReviewEventDispositions(ctx, id, record.dispositionIDs)
		if err != nil {
			return nil, err
		}
		record.event.Dispositions = dispositions
		result = append(result, record.event)
	}
	return result, nil
}

// readPlanReviewEvents collects rows and closes them before dependent queries.
// Public callers establish ownership and revision existence before this read.
func (s *Store) readPlanReviewEvents(
	ctx context.Context, id string, revision int64, limit, offset int,
) ([]retainedReviewEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,plan_id,revision,status,review_version,
 feedback_version,disposition_ids,created_at,actor,session_id FROM plan_review_events
 WHERE plan_id=? AND revision=? ORDER BY review_version,id LIMIT ? OFFSET ?`, id, revision, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []retainedReviewEvent{}
	for rows.Next() {
		var event storage.PlanReviewEvent
		var ids string
		if err := rows.Scan(&event.ID, &event.PlanID, &event.Revision, &event.Status, &event.ReviewVersion,
			&event.FeedbackVersion, &ids, &event.CreatedAt, &event.Actor, &event.SessionID); err != nil {
			return nil, err
		}
		result = append(result, retainedReviewEvent{event: &event, dispositionIDs: ids})
	}
	return result, rows.Err()
}

// readReviewEventDispositions uses the append-only ID array stored with the event.
// Ordering follows that array, including addressed dispositions from prior revisions.
func (s *Store) readReviewEventDispositions(
	ctx context.Context, id, encodedIDs string,
) ([]*types.PlanFeedbackDisposition, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT d.id,d.plan_id,d.target_revision,d.comment_id,
 d.comment_version,d.disposition,d.reason,d.created_at FROM json_each(?) selected
 JOIN plan_dispositions d ON d.id=selected.value AND d.plan_id=?
 ORDER BY CAST(selected.key AS INTEGER)`, encodedIDs, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPlanDispositions(rows)
}

// ListPlanCommentVersions returns original snapshots, including pre-edit anchors.
// A comment must belong to both the requested plan and the exact requested revision.
// Archive and tombstones preserve this read path; neither rewrites event snapshots.
//
//revive:disable-next-line:argument-limit // Explicit project, plan, revision, comment and pagination scope.
func (s *Store) ListPlanCommentVersions(
	ctx context.Context, projectID, id string, revision int64, commentID string, limit, offset int,
) ([]*storage.PlanCommentVersion, error) {
	if err := checkPage(limit, offset); err != nil {
		return nil, err
	}
	if _, err := s.GetDurablePlan(ctx, projectID, id); err != nil {
		return nil, err
	}
	if _, err := getRevision(ctx, s.queries, id, revision); err != nil {
		return nil, err
	}
	var found string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM plan_comments
 WHERE id=? AND plan_id=? AND revision=?`, commentID, id, revision).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrPlanNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT body,created_at,actor,session_id FROM plan_comment_events
 WHERE comment_id=? ORDER BY version LIMIT ? OFFSET ?`,
		commentID,
		limit,
		offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []*storage.PlanCommentVersion{}
	for rows.Next() {
		var version storage.PlanCommentVersion
		var body string
		if err := rows.Scan(&body, &version.CreatedAt, &version.Actor, &version.SessionID); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(body), &version.Comment); err != nil {
			return nil, err
		}
		result = append(result, &version)
	}
	return result, rows.Err()
}

// scanPlanDispositions shares the immutable disposition mapping across history readers.
// The caller owns closing its row cursor.
func scanPlanDispositions(rows *sql.Rows) ([]*types.PlanFeedbackDisposition, error) {
	result := []*types.PlanFeedbackDisposition{}
	for rows.Next() {
		var d types.PlanFeedbackDisposition
		if err := rows.Scan(
			&d.ID,
			&d.PlanID,
			&d.TargetRevision,
			&d.CommentID,
			&d.CommentVersion,
			&d.Disposition,
			&d.Reason,
			&d.CreatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, &d)
	}
	return result, rows.Err()
}
