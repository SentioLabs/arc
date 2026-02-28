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

// ErrNoPlan is returned when no inline plan exists for an issue.
var ErrNoPlan = errors.New("no inline plan exists")

// CreatePlan creates a new shared plan.
func (s *Store) CreatePlan(ctx context.Context, plan *types.Plan) error {
	now := time.Now()
	_, err := s.queries.CreatePlan(ctx, db.CreatePlanParams{
		ID:          plan.ID,
		WorkspaceID: plan.WorkspaceID,
		Title:       plan.Title,
		Content:     plan.Content,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		return fmt.Errorf("create plan: %w", err)
	}
	plan.CreatedAt = now
	plan.UpdatedAt = now
	return nil
}

// GetPlan retrieves a plan by ID.
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

// ListPlans returns all plans in a workspace.
func (s *Store) ListPlans(ctx context.Context, workspaceID string) ([]*types.Plan, error) {
	rows, err := s.queries.ListPlans(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list plans: %w", err)
	}

	plans := make([]*types.Plan, len(rows))
	for i, row := range rows {
		plans[i] = dbPlanToType(row)
	}

	return plans, nil
}

// UpdatePlan updates a plan's title and content.
func (s *Store) UpdatePlan(ctx context.Context, id, title, content string) error {
	now := time.Now()
	err := s.queries.UpdatePlan(ctx, db.UpdatePlanParams{
		ID:        id,
		Title:     title,
		Content:   content,
		UpdatedAt: now,
	})
	if err != nil {
		return fmt.Errorf("update plan: %w", err)
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

// LinkIssueToPlan creates a link between an issue and a plan.
func (s *Store) LinkIssueToPlan(ctx context.Context, issueID, planID string) error {
	now := time.Now()
	err := s.queries.LinkIssueToPlan(ctx, db.LinkIssueToPlanParams{
		IssueID:   issueID,
		PlanID:    planID,
		CreatedAt: now,
	})
	if err != nil {
		return fmt.Errorf("link issue to plan: %w", err)
	}
	return nil
}

// UnlinkIssueFromPlan removes a link between an issue and a plan.
func (s *Store) UnlinkIssueFromPlan(ctx context.Context, issueID, planID string) error {
	err := s.queries.UnlinkIssueFromPlan(ctx, db.UnlinkIssueFromPlanParams{
		IssueID: issueID,
		PlanID:  planID,
	})
	if err != nil {
		return fmt.Errorf("unlink issue from plan: %w", err)
	}
	return nil
}

// GetLinkedPlans returns all plans linked to an issue.
func (s *Store) GetLinkedPlans(ctx context.Context, issueID string) ([]*types.Plan, error) {
	rows, err := s.queries.GetLinkedPlans(ctx, issueID)
	if err != nil {
		return nil, fmt.Errorf("get linked plans: %w", err)
	}

	plans := make([]*types.Plan, len(rows))
	for i, row := range rows {
		plans[i] = dbPlanToType(row)
	}

	return plans, nil
}

// GetLinkedIssues returns all issue IDs linked to a plan.
func (s *Store) GetLinkedIssues(ctx context.Context, planID string) ([]string, error) {
	ids, err := s.queries.GetLinkedIssues(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf("get linked issues: %w", err)
	}
	return ids, nil
}

// SetInlinePlan sets or updates an inline plan comment on an issue.
// If a plan already exists, a new version is created (preserving history).
func (s *Store) SetInlinePlan(ctx context.Context, issueID, author, text string) (*types.Comment, error) {
	now := time.Now()
	result, err := s.queries.CreateComment(ctx, db.CreateCommentParams{
		IssueID:     issueID,
		Author:      author,
		Text:        text,
		CommentType: string(types.CommentTypePlan),
		CreatedAt:   now,
	})
	if err != nil {
		return nil, fmt.Errorf("set inline plan: %w", err)
	}

	return &types.Comment{
		ID:          result.ID,
		IssueID:     result.IssueID,
		Author:      result.Author,
		Text:        result.Text,
		CommentType: types.CommentType(result.CommentType),
		CreatedAt:   result.CreatedAt,
	}, nil
}

// GetInlinePlan returns the latest inline plan for an issue.
func (s *Store) GetInlinePlan(ctx context.Context, issueID string) (*types.Comment, error) {
	row, err := s.queries.GetLatestPlanComment(ctx, issueID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoPlan
		}
		return nil, fmt.Errorf("get inline plan: %w", err)
	}

	return dbCommentToType(row), nil
}

// GetPlanHistory returns all plan versions for an issue.
func (s *Store) GetPlanHistory(ctx context.Context, issueID string) ([]*types.Comment, error) {
	rows, err := s.queries.GetPlanHistory(ctx, issueID)
	if err != nil {
		return nil, fmt.Errorf("get plan history: %w", err)
	}

	comments := make([]*types.Comment, len(rows))
	for i, row := range rows {
		comments[i] = dbCommentToType(row)
	}

	return comments, nil
}

// GetPlanContext returns the complete plan context for an issue.
// This includes inline plan, parent plan (from parent-child deps), and shared plans.
func (s *Store) GetPlanContext(ctx context.Context, issueID string) (*types.PlanContext, error) {
	pc := &types.PlanContext{}

	// 1. Get inline plan
	inlinePlan, err := s.GetInlinePlan(ctx, issueID)
	if err != nil && !errors.Is(err, ErrNoPlan) {
		return nil, fmt.Errorf("get inline plan: %w", err)
	}
	if err == nil {
		pc.InlinePlan = inlinePlan
	}

	// 2. Check for parent plan via parent-child dependency
	deps, err := s.GetDependencies(ctx, issueID)
	if err != nil {
		return nil, fmt.Errorf("get dependencies: %w", err)
	}

	for _, dep := range deps {
		if dep.Type == types.DepParentChild {
			// This issue depends on a parent - check if parent has a plan
			parentPlan, err := s.GetInlinePlan(ctx, dep.DependsOnID)
			if err != nil && !errors.Is(err, ErrNoPlan) {
				return nil, fmt.Errorf("get parent plan: %w", err)
			}
			if err == nil && parentPlan != nil {
				pc.ParentPlan = parentPlan
				pc.ParentIssueID = dep.DependsOnID
				break // Only use first parent's plan
			}
		}
	}

	// 3. Get shared plans
	sharedPlans, err := s.GetLinkedPlans(ctx, issueID)
	if err != nil {
		return nil, fmt.Errorf("get shared plans: %w", err)
	}
	pc.SharedPlans = sharedPlans

	return pc, nil
}

// dbPlanToType converts a db.Plan to types.Plan.
func dbPlanToType(row *db.Plan) *types.Plan {
	return &types.Plan{
		ID:          row.ID,
		WorkspaceID: row.WorkspaceID,
		Title:       row.Title,
		Content:     row.Content,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

// dbCommentToType converts a db.Comment to types.Comment.
func dbCommentToType(row *db.Comment) *types.Comment {
	var updatedAt time.Time
	if row.UpdatedAt.Valid {
		updatedAt = row.UpdatedAt.Time
	}
	return &types.Comment{
		ID:          row.ID,
		IssueID:     row.IssueID,
		Author:      row.Author,
		Text:        row.Text,
		CommentType: types.CommentType(row.CommentType),
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   updatedAt,
	}
}
