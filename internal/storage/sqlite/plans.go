package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/sentiolabs/arc/internal/project"
	"github.com/sentiolabs/arc/internal/storage/sqlite/db"
	"github.com/sentiolabs/arc/internal/types"
)

// CreateOrUpdatePlan creates a new plan or updates an existing one bound to the same issue.
// If plan.ID is empty, a new ID is generated using project.GeneratePlanID.
func (s *Store) CreateOrUpdatePlan(ctx context.Context, plan *types.Plan) error {
	now := time.Now()

	if plan.ID == "" {
		plan.ID = project.GeneratePlanID(plan.Title)
	}

	row, err := s.queries.UpsertPlan(ctx, db.UpsertPlanParams{
		ID:        plan.ID,
		ProjectID: plan.ProjectID,
		IssueID:   toNullString(plan.IssueID),
		Title:     plan.Title,
		Content:   plan.Content,
		Status:    plan.Status,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return fmt.Errorf("create or update plan: %w", err)
	}

	plan.ID = row.ID
	plan.Status = row.Status
	plan.CreatedAt = row.CreatedAt
	plan.UpdatedAt = row.UpdatedAt
	return nil
}

// GetPlanByIssueID retrieves a plan by issue ID. Returns an error if not found.
func (s *Store) GetPlanByIssueID(ctx context.Context, issueID string) (*types.Plan, error) {
	row, err := s.queries.GetPlanByIssueID(ctx, toNullString(issueID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("no plan found for issue: %s", issueID)
		}
		return nil, fmt.Errorf("get plan by issue ID: %w", err)
	}
	return dbPlanToType(row), nil
}

// GetPlan retrieves a plan by plan ID.
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

// ListPlans returns all plans for a project. If status is non-empty, filters by status.
func (s *Store) ListPlans(ctx context.Context, projectID string, status string) ([]*types.Plan, error) {
	rows, err := s.queries.ListPlans(ctx, db.ListPlansParams{
		ProjectID: projectID,
		Column2:   status,
		Status:    status,
	})
	if err != nil {
		return nil, fmt.Errorf("list plans: %w", err)
	}

	plans := make([]*types.Plan, len(rows))
	for i, row := range rows {
		plans[i] = dbPlanToType(row)
	}
	return plans, nil
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

// UpdatePlanContent updates a plan's title and content.
func (s *Store) UpdatePlanContent(ctx context.Context, id string, title string, content string) error {
	err := s.queries.UpdatePlanContent(ctx, db.UpdatePlanContentParams{
		ID:        id,
		Title:     title,
		Content:   content,
		UpdatedAt: time.Now(),
	})
	if err != nil {
		return fmt.Errorf("update plan content: %w", err)
	}
	return nil
}

// ListAllPlans returns all plans across all projects. If status is non-empty, filters by status.
func (s *Store) ListAllPlans(ctx context.Context, status string) ([]*types.Plan, error) {
	rows, err := s.queries.ListAllPlans(ctx, db.ListAllPlansParams{
		Column1: status,
		Status:  status,
	})
	if err != nil {
		return nil, fmt.Errorf("list all plans: %w", err)
	}

	plans := make([]*types.Plan, len(rows))
	for i, row := range rows {
		plans[i] = dbPlanToType(row)
	}
	return plans, nil
}

// UpdatePlanIssueID updates the issue_id of a plan. Pass empty string to unlink.
func (s *Store) UpdatePlanIssueID(ctx context.Context, id string, issueID string) error {
	err := s.queries.UpdatePlanIssueID(ctx, db.UpdatePlanIssueIDParams{
		ID:        id,
		IssueID:   toNullString(issueID),
		UpdatedAt: time.Now(),
	})
	if err != nil {
		return fmt.Errorf("update plan issue ID: %w", err)
	}
	return nil
}

// DeletePlan deletes a plan.
func (s *Store) DeletePlan(ctx context.Context, id string) error {
	err := s.queries.DeletePlan(ctx, id)
	if err != nil {
		return fmt.Errorf("delete plan: %w", err)
	}
	return nil
}

// CountPlansByStatus returns the count of plans with a given status in a project.
func (s *Store) CountPlansByStatus(ctx context.Context, projectID string, status string) (int, error) {
	count, err := s.queries.CountPlansByStatus(ctx, db.CountPlansByStatusParams{
		ProjectID: projectID,
		Status:    status,
	})
	if err != nil {
		return 0, fmt.Errorf("count plans by status: %w", err)
	}
	return int(count), nil
}

// dbPlanToType converts a db.Plan to types.Plan.
func dbPlanToType(row *db.Plan) *types.Plan {
	return &types.Plan{
		ID:        row.ID,
		ProjectID: row.ProjectID,
		Title:     row.Title,
		Content:   row.Content,
		Status:    row.Status,
		IssueID:   fromNullString(row.IssueID),
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}
