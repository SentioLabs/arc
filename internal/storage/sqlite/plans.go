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

	err := s.queries.CreatePlanComment(ctx, db.CreatePlanCommentParams{
		ID:         comment.ID,
		PlanID:     comment.PlanID,
		LineNumber: lineNumber,
		Content:    comment.Content,
		CreatedAt:  comment.CreatedAt,
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

	return &types.PlanComment{
		ID:         row.ID,
		PlanID:     row.PlanID,
		LineNumber: lineNumber,
		Content:    row.Content,
		CreatedAt:  row.CreatedAt,
	}
}
