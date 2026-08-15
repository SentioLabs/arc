// Plans implements the storage layer for plan CRUD operations
// and plan comment management.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/sentiolabs/arc/internal/storage/sqlite/db"
	"github.com/sentiolabs/arc/internal/types"
)

// CreatePlan persists a new plan. The caller must set plan.ID before calling.
func (s *Store) CreatePlan(ctx context.Context, plan *types.Plan) error {
	now := time.Now()
	plan.CreatedAt = now
	plan.UpdatedAt = now

	err := s.queries.CreatePlan(ctx, db.CreatePlanParams{
		ID:        plan.ID,
		FilePath:  plan.FilePath,
		Status:    plan.Status,
		CreatedAt: plan.CreatedAt,
		UpdatedAt: plan.UpdatedAt,
	})
	if err != nil {
		return fmt.Errorf("create plan: %w", err)
	}
	return nil
}

// GetPlan retrieves a plan by ID. Returns an error if not found.
func (s *Store) GetPlan(ctx context.Context, id string) (*types.Plan, error) {
	row, err := s.queries.GetPlan(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("plan not found: %s", id)
		}
		return nil, fmt.Errorf("get plan: %w", err)
	}
	return dbPlanToType(row), nil
}

// UpdatePlanStatus changes a plan's status.
func (s *Store) UpdatePlanStatus(ctx context.Context, id string, status string) error {
	err := s.queries.UpdatePlanStatus(ctx, db.UpdatePlanStatusParams{
		ID:        id,
		Status:    status,
		UpdatedAt: time.Now(),
	})
	if err != nil {
		return fmt.Errorf("update plan status: %w", err)
	}
	return nil
}

// DeletePlan deletes a plan and its associated comments.
// Comments are deleted explicitly to ensure cascade behavior regardless of
// whether the SQLite driver honours the ON DELETE CASCADE pragma.
func (s *Store) DeletePlan(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM plan_comments WHERE plan_id = ?", id); err != nil {
		return fmt.Errorf("delete plan comments for plan: %w", err)
	}
	err := s.queries.DeletePlan(ctx, id)
	if err != nil {
		return fmt.Errorf("delete plan: %w", err)
	}
	return nil
}

// CreatePlanComment persists a new comment on a plan.
func (s *Store) CreatePlanComment(ctx context.Context, comment *types.PlanComment) error {
	now := time.Now()
	comment.CreatedAt = now

	var lineNumber sql.NullInt64
	if comment.LineNumber != nil {
		lineNumber = sql.NullInt64{Int64: int64(*comment.LineNumber), Valid: true}
	}

	lineStart, lineEnd, occurrence, quoted, slug, before, after := planCommentAnchorParams(comment.Anchor)

	err := s.queries.CreatePlanComment(ctx, db.CreatePlanCommentParams{
		ID:            comment.ID,
		PlanID:        comment.PlanID,
		LineNumber:    lineNumber,
		Content:       comment.Content,
		CreatedAt:     comment.CreatedAt,
		LineStart:     lineStart,
		LineEnd:       lineEnd,
		QuotedText:    quoted,
		Occurrence:    occurrence,
		HeadingSlug:   slug,
		ContextBefore: before,
		ContextAfter:  after,
		UpdatedAt:     nullTime(comment.UpdatedAt),
		ResolvedAt:    nullTime(comment.ResolvedAt),
	})
	if err != nil {
		return fmt.Errorf("create plan comment: %w", err)
	}
	return nil
}

// ListPlanComments returns all comments for a plan, ordered by creation time.
func (s *Store) ListPlanComments(ctx context.Context, planID string) ([]*types.PlanComment, error) {
	rows, err := s.queries.ListPlanComments(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf("list plan comments: %w", err)
	}

	comments := make([]*types.PlanComment, len(rows))
	for i, row := range rows {
		comments[i] = dbPlanCommentToType(row)
	}
	return comments, nil
}

// GetPlanComment returns a single plan comment by ID.
func (s *Store) GetPlanComment(ctx context.Context, id string) (*types.PlanComment, error) {
	row, err := s.queries.GetPlanComment(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get plan comment: %w", err)
	}
	return dbPlanCommentToType(row), nil
}

// UpdatePlanComment overwrites the mutable fields of a plan comment.
func (s *Store) UpdatePlanComment(ctx context.Context, comment *types.PlanComment) error {
	var lineNumber sql.NullInt64
	if comment.LineNumber != nil {
		lineNumber = sql.NullInt64{Int64: int64(*comment.LineNumber), Valid: true}
	}
	lineStart, lineEnd, occurrence, quoted, slug, before, after := planCommentAnchorParams(comment.Anchor)
	err := s.queries.UpdatePlanComment(ctx, db.UpdatePlanCommentParams{
		Content:       comment.Content,
		LineNumber:    lineNumber,
		LineStart:     lineStart,
		LineEnd:       lineEnd,
		QuotedText:    quoted,
		Occurrence:    occurrence,
		HeadingSlug:   slug,
		ContextBefore: before,
		ContextAfter:  after,
		UpdatedAt:     nullTime(comment.UpdatedAt),
		ResolvedAt:    nullTime(comment.ResolvedAt),
		ID:            comment.ID,
	})
	if err != nil {
		return fmt.Errorf("update plan comment: %w", err)
	}
	return nil
}

// DeletePlanComment removes a plan comment.
func (s *Store) DeletePlanComment(ctx context.Context, id string) error {
	if err := s.queries.DeletePlanComment(ctx, id); err != nil {
		return fmt.Errorf("delete plan comment: %w", err)
	}
	return nil
}

// nullString returns a valid sql.NullString for non-empty s.
func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// nullTime returns a valid sql.NullTime when t is non-nil.
func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

// planCommentAnchorParams maps the anchor fields of a PlanComment to sqlc nullable params.
func planCommentAnchorParams(a *types.PlanCommentAnchor) (
	lineStart, lineEnd, occurrence sql.NullInt64,
	quoted, slug, before, after sql.NullString,
) {
	if a == nil {
		return
	}
	lineStart = sql.NullInt64{Int64: int64(a.LineStart), Valid: true}
	lineEnd = sql.NullInt64{Int64: int64(a.LineEnd), Valid: true}
	occurrence = sql.NullInt64{Int64: int64(a.Occurrence), Valid: true}
	quoted = sql.NullString{String: a.QuotedText, Valid: true} // quoted_text NOT NULL marks "anchored"
	slug = nullString(a.HeadingSlug)
	before = nullString(a.ContextBefore)
	after = nullString(a.ContextAfter)
	return
}

// dbPlanToType converts a db.Plan to types.Plan.
func dbPlanToType(row *db.Plan) *types.Plan {
	return &types.Plan{
		ID:        row.ID,
		FilePath:  row.FilePath,
		Status:    row.Status,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

// dbPlanCommentToType converts a db.PlanComment to types.PlanComment.
func dbPlanCommentToType(row *db.PlanComment) *types.PlanComment {
	var lineNumber *int
	if row.LineNumber.Valid {
		v := int(row.LineNumber.Int64)
		lineNumber = &v
	}

	var anchor *types.PlanCommentAnchor
	if row.QuotedText.Valid {
		anchor = &types.PlanCommentAnchor{
			LineStart:     int(row.LineStart.Int64),
			LineEnd:       int(row.LineEnd.Int64),
			QuotedText:    row.QuotedText.String,
			Occurrence:    int(row.Occurrence.Int64),
			HeadingSlug:   row.HeadingSlug.String,
			ContextBefore: row.ContextBefore.String,
			ContextAfter:  row.ContextAfter.String,
		}
	}

	var updatedAt, resolvedAt *time.Time
	if row.UpdatedAt.Valid {
		t := row.UpdatedAt.Time
		updatedAt = &t
	}
	if row.ResolvedAt.Valid {
		t := row.ResolvedAt.Time
		resolvedAt = &t
	}

	return &types.PlanComment{
		ID:         row.ID,
		PlanID:     row.PlanID,
		LineNumber: lineNumber,
		Content:    row.Content,
		Anchor:     anchor,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  updatedAt,
		ResolvedAt: resolvedAt,
	}
}
