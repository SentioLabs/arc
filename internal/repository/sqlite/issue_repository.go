package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/sentiolabs/arc/internal/storage/sqlite/db"
	"github.com/sentiolabs/arc/internal/types"
)

// IssueRepository implements repository.IssueRepository.
type IssueRepository struct {
	repo *Repository
}

// Create creates a new issue.
func (r *IssueRepository) Create(ctx context.Context, issue *types.Issue, actor string) error {
	issue.SetDefaults()

	if err := issue.Validate(); err != nil {
		return fmt.Errorf("validate issue: %w", err)
	}

	// Get workspace prefix for ID generation
	ws, err := r.repo.workspaces.Get(ctx, issue.WorkspaceID)
	if err != nil {
		return fmt.Errorf("get workspace for ID generation: %w", err)
	}

	// Generate ID if not provided
	if issue.ID == "" {
		issue.ID = generateID(ws.Prefix, issue.Title)
	}

	now := time.Now()
	issue.CreatedAt = now
	issue.UpdatedAt = now

	err = r.repo.queries.CreateIssue(ctx, db.CreateIssueParams{
		ID:          issue.ID,
		WorkspaceID: issue.WorkspaceID,
		Title:       issue.Title,
		Description: toNullString(issue.Description),
		Status:      string(issue.Status),
		Priority:    int64(issue.Priority),
		IssueType:   string(issue.IssueType),
		Assignee:    toNullString(issue.Assignee),
		ExternalRef: toNullString(issue.ExternalRef),
		CreatedAt:   now,
		UpdatedAt:   now,
		ClosedAt:    toNullTime(issue.ClosedAt),
		CloseReason: toNullString(issue.CloseReason),
	})
	if err != nil {
		return fmt.Errorf("create issue: %w", err)
	}

	// Record creation event
	r.repo.events.Record(ctx, issue.ID, types.EventCreated, actor, nil, &issue.Title)

	return nil
}

// Get retrieves an issue by ID.
func (r *IssueRepository) Get(ctx context.Context, id string) (*types.Issue, error) {
	row, err := r.repo.queries.GetIssue(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("issue not found: %s", id)
		}
		return nil, fmt.Errorf("get issue: %w", err)
	}

	return r.dbIssueToType(row), nil
}

// GetByExternalRef retrieves an issue by its external reference.
func (r *IssueRepository) GetByExternalRef(ctx context.Context, externalRef string) (*types.Issue, error) {
	row, err := r.repo.queries.GetIssueByExternalRef(ctx, toNullString(externalRef))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("issue not found with external ref: %s", externalRef)
		}
		return nil, fmt.Errorf("get issue by external ref: %w", err)
	}

	return r.dbIssueToType(row), nil
}

// List returns issues matching the filter.
func (r *IssueRepository) List(ctx context.Context, filter types.IssueFilter) ([]*types.Issue, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	var rows []*db.Issue
	var err error

	switch {
	case filter.Query != "":
		searchPattern := "%" + filter.Query + "%"
		rows, err = r.repo.queries.SearchIssues(ctx, db.SearchIssuesParams{
			WorkspaceID: filter.WorkspaceID,
			Title:       searchPattern,
			Description: toNullString(searchPattern),
			Limit:       int64(limit),
			Offset:      int64(offset),
		})
	case filter.Status != nil:
		rows, err = r.repo.queries.ListIssuesByStatus(ctx, db.ListIssuesByStatusParams{
			WorkspaceID: filter.WorkspaceID,
			Status:      string(*filter.Status),
			Limit:       int64(limit),
			Offset:      int64(offset),
		})
	case filter.Assignee != nil:
		rows, err = r.repo.queries.ListIssuesByAssignee(ctx, db.ListIssuesByAssigneeParams{
			WorkspaceID: filter.WorkspaceID,
			Assignee:    toNullString(*filter.Assignee),
			Limit:       int64(limit),
			Offset:      int64(offset),
		})
	case filter.IssueType != nil:
		rows, err = r.repo.queries.ListIssuesByType(ctx, db.ListIssuesByTypeParams{
			WorkspaceID: filter.WorkspaceID,
			IssueType:   string(*filter.IssueType),
			Limit:       int64(limit),
			Offset:      int64(offset),
		})
	default:
		rows, err = r.repo.queries.ListIssuesByWorkspace(ctx, db.ListIssuesByWorkspaceParams{
			WorkspaceID: filter.WorkspaceID,
			Limit:       int64(limit),
			Offset:      int64(offset),
		})
	}

	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}

	issues := make([]*types.Issue, len(rows))
	for i, row := range rows {
		issues[i] = r.dbIssueToType(row)
	}

	return issues, nil
}

// Update updates an issue with the given updates.
func (r *IssueRepository) Update(ctx context.Context, id string, updates map[string]any, actor string) error {
	now := time.Now()

	for field, value := range updates {
		var err error
		switch field {
		case "title":
			err = r.repo.queries.UpdateIssueTitle(ctx, db.UpdateIssueTitleParams{
				Title:     value.(string),
				UpdatedAt: now,
				ID:        id,
			})
		case "description":
			err = r.repo.queries.UpdateIssueDescription(ctx, db.UpdateIssueDescriptionParams{
				Description: toNullString(value.(string)),
				UpdatedAt:   now,
				ID:          id,
			})
		case "status":
			status := value.(string)
			err = r.repo.queries.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{
				Status:    status,
				UpdatedAt: now,
				ID:        id,
			})
			r.repo.events.Record(ctx, id, types.EventStatusChanged, actor, nil, &status)
		case "priority":
			err = r.repo.queries.UpdateIssuePriority(ctx, db.UpdateIssuePriorityParams{
				Priority:  int64(value.(int)),
				UpdatedAt: now,
				ID:        id,
			})
		case "issue_type":
			err = r.repo.queries.UpdateIssueType(ctx, db.UpdateIssueTypeParams{
				IssueType: value.(string),
				UpdatedAt: now,
				ID:        id,
			})
		case "assignee":
			err = r.repo.queries.UpdateIssueAssignee(ctx, db.UpdateIssueAssigneeParams{
				Assignee:  toNullString(value.(string)),
				UpdatedAt: now,
				ID:        id,
			})
		case "external_ref":
			err = r.repo.queries.UpdateIssueExternalRef(ctx, db.UpdateIssueExternalRefParams{
				ExternalRef: toNullString(value.(string)),
				UpdatedAt:   now,
				ID:          id,
			})
		default:
			return fmt.Errorf("unknown field: %s", field)
		}
		if err != nil {
			return fmt.Errorf("update %s: %w", field, err)
		}
	}

	r.repo.events.Record(ctx, id, types.EventUpdated, actor, nil, nil)
	return nil
}

// Delete deletes an issue.
func (r *IssueRepository) Delete(ctx context.Context, id string) error {
	// Delete dependencies first
	err := r.repo.queries.DeleteDependenciesByIssue(ctx, db.DeleteDependenciesByIssueParams{
		IssueID:     id,
		DependsOnID: id,
	})
	if err != nil {
		return fmt.Errorf("delete dependencies: %w", err)
	}

	// Delete labels
	err = r.repo.queries.DeleteIssueLabels(ctx, id)
	if err != nil {
		return fmt.Errorf("delete labels: %w", err)
	}

	// Delete events
	err = r.repo.queries.DeleteEventsByIssue(ctx, id)
	if err != nil {
		return fmt.Errorf("delete events: %w", err)
	}

	// Delete issue
	err = r.repo.queries.DeleteIssue(ctx, id)
	if err != nil {
		return fmt.Errorf("delete issue: %w", err)
	}

	return nil
}

// Close closes an issue.
func (r *IssueRepository) Close(ctx context.Context, id string, reason string, actor string) error {
	now := time.Now()
	err := r.repo.queries.CloseIssue(ctx, db.CloseIssueParams{
		ClosedAt:    toNullTime(&now),
		CloseReason: toNullString(reason),
		UpdatedAt:   now,
		ID:          id,
	})
	if err != nil {
		return fmt.Errorf("close issue: %w", err)
	}

	r.repo.events.Record(ctx, id, types.EventClosed, actor, nil, &reason)
	return nil
}

// Reopen reopens a closed issue.
func (r *IssueRepository) Reopen(ctx context.Context, id string, actor string) error {
	now := time.Now()
	err := r.repo.queries.ReopenIssue(ctx, db.ReopenIssueParams{
		UpdatedAt: now,
		ID:        id,
	})
	if err != nil {
		return fmt.Errorf("reopen issue: %w", err)
	}

	r.repo.events.Record(ctx, id, types.EventReopened, actor, nil, nil)
	return nil
}

// GetDetails retrieves an issue with all its relational data.
func (r *IssueRepository) GetDetails(ctx context.Context, id string) (*types.IssueDetails, error) {
	issue, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	labels, err := r.repo.labels.GetIssueLabels(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get labels: %w", err)
	}

	deps, err := r.repo.dependencies.GetForIssue(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get dependencies: %w", err)
	}

	dependents, err := r.repo.dependencies.GetDependents(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get dependents: %w", err)
	}

	comments, err := r.repo.comments.List(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get comments: %w", err)
	}

	return &types.IssueDetails{
		Issue:        *issue,
		Labels:       labels,
		Dependencies: deps,
		Dependents:   dependents,
		Comments:     comments,
	}, nil
}

// GetReadyWork returns issues that are ready to work on (not blocked).
func (r *IssueRepository) GetReadyWork(ctx context.Context, filter types.WorkFilter) ([]*types.Issue, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}

	rows, err := r.repo.queries.GetOpenNonBlockedIssues(ctx, db.GetOpenNonBlockedIssuesParams{
		WorkspaceID: filter.WorkspaceID,
		Limit:       int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("get ready work: %w", err)
	}

	issues := make([]*types.Issue, 0, len(rows))
	for _, row := range rows {
		issue := r.dbIssueToType(row)

		// Apply additional filters
		if filter.IssueType != nil && issue.IssueType != *filter.IssueType {
			continue
		}
		if filter.Priority != nil && issue.Priority != *filter.Priority {
			continue
		}
		if filter.Assignee != nil && issue.Assignee != *filter.Assignee {
			continue
		}
		if filter.Unassigned && issue.Assignee != "" {
			continue
		}
		if filter.Status != nil && issue.Status != *filter.Status {
			continue
		}

		issues = append(issues, issue)
	}

	return issues, nil
}

// GetBlockedIssues returns issues that are blocked by other issues.
func (r *IssueRepository) GetBlockedIssues(ctx context.Context, filter types.WorkFilter) ([]*types.BlockedIssue, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}

	rows, err := r.repo.queries.GetBlockedIssuesInWorkspace(ctx, db.GetBlockedIssuesInWorkspaceParams{
		WorkspaceID: filter.WorkspaceID,
		Limit:       int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("get blocked issues: %w", err)
	}

	issues := make([]*types.BlockedIssue, 0, len(rows))
	for _, row := range rows {
		// Get blocking issue IDs
		blockingIDs := []string{}
		blockingIssues, _ := r.repo.queries.GetBlockingIssues(ctx, row.ID)
		for _, bi := range blockingIssues {
			blockingIDs = append(blockingIDs, bi.ID)
		}

		blocked := &types.BlockedIssue{
			Issue: types.Issue{
				ID:          row.ID,
				WorkspaceID: row.WorkspaceID,
				Title:       row.Title,
				Description: fromNullString(row.Description),
				Status:      types.Status(row.Status),
				Priority:    int(row.Priority),
				IssueType:   types.IssueType(row.IssueType),
				Assignee:    fromNullString(row.Assignee),
				ExternalRef: fromNullString(row.ExternalRef),
				CreatedAt:   row.CreatedAt,
				UpdatedAt:   row.UpdatedAt,
				ClosedAt:    fromNullTime(row.ClosedAt),
				CloseReason: fromNullString(row.CloseReason),
			},
			BlockedByCount: int(row.BlockedByCount),
			BlockedBy:      blockingIDs,
		}
		issues = append(issues, blocked)
	}

	return issues, nil
}

// IsBlocked checks if an issue is blocked by any open issues.
func (r *IssueRepository) IsBlocked(ctx context.Context, issueID string) (bool, []string, error) {
	blockingIssues, err := r.repo.queries.GetBlockingIssues(ctx, issueID)
	if err != nil {
		return false, nil, fmt.Errorf("get blocking issues: %w", err)
	}

	if len(blockingIssues) == 0 {
		return false, nil, nil
	}

	blockerIDs := make([]string, len(blockingIssues))
	for i, bi := range blockingIssues {
		blockerIDs[i] = bi.ID
	}

	return true, blockerIDs, nil
}

// dbIssueToType converts a database issue to a types.Issue.
func (r *IssueRepository) dbIssueToType(row *db.Issue) *types.Issue {
	return &types.Issue{
		ID:          row.ID,
		WorkspaceID: row.WorkspaceID,
		Title:       row.Title,
		Description: fromNullString(row.Description),
		Status:      types.Status(row.Status),
		Priority:    int(row.Priority),
		IssueType:   types.IssueType(row.IssueType),
		Assignee:    fromNullString(row.Assignee),
		ExternalRef: fromNullString(row.ExternalRef),
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		ClosedAt:    fromNullTime(row.ClosedAt),
		CloseReason: fromNullString(row.CloseReason),
	}
}
