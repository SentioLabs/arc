package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sentiolabs/arc/internal/project"
	"github.com/sentiolabs/arc/internal/storage/sqlite/db"
	"github.com/sentiolabs/arc/internal/types"
)

// IsHierarchicalID checks if an issue ID is hierarchical (has a parent).
// Hierarchical IDs have the format {parentID}.{N} where N is a numeric child suffix.
// Returns true and the parent ID if hierarchical, false and empty string otherwise.
func IsHierarchicalID(id string) (isHierarchical bool, parentID string) {
	lastDot := strings.LastIndex(id, ".")
	if lastDot == -1 {
		return false, ""
	}

	// Check if the suffix after the last dot is purely numeric
	suffix := id[lastDot+1:]
	if len(suffix) == 0 {
		return false, ""
	}

	for _, c := range suffix {
		if c < '0' || c > '9' {
			return false, ""
		}
	}

	// It's hierarchical - parent is everything before the last dot
	return true, id[:lastDot]
}

// getNextChildNumber atomically increments and returns the next child counter for a parent.
func (s *Store) getNextChildNumber(ctx context.Context, parentID string) (int, error) {
	var nextChild int
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO child_counters (parent_id, last_child)
		VALUES (?, 1)
		ON CONFLICT(parent_id) DO UPDATE SET
			last_child = last_child + 1
		RETURNING last_child
	`, parentID).Scan(&nextChild)
	if err != nil {
		return 0, fmt.Errorf("failed to generate next child number for parent %s: %w", parentID, err)
	}
	return nextChild, nil
}

// GetNextChildID generates the next hierarchical child ID for a given parent.
// Returns formatted ID as parentID.{counter} (e.g., arc-a3f8e9.1)
func (s *Store) GetNextChildID(ctx context.Context, parentID string) (string, error) {
	// Validate parent exists
	_, err := s.GetIssue(ctx, parentID)
	if err != nil {
		return "", fmt.Errorf("parent issue not found: %s", parentID)
	}

	// Get next child number atomically
	nextNum, err := s.getNextChildNumber(ctx, parentID)
	if err != nil {
		return "", err
	}

	// Format as parentID.counter
	childID := fmt.Sprintf("%s.%d", parentID, nextNum)
	return childID, nil
}

// CreateIssue creates a new issue.
// If ParentID is set, generates a hierarchical child ID (e.g., parent.1) and
// automatically creates a parent-child dependency.
func (s *Store) CreateIssue(ctx context.Context, issue *types.Issue, actor string) error {
	issue.SetDefaults()

	if err := issue.Validate(); err != nil {
		return fmt.Errorf("validate issue: %w", err)
	}

	// Get project prefix for ID generation
	proj, err := s.GetProject(ctx, issue.ProjectID)
	if err != nil {
		return fmt.Errorf("get project for ID generation: %w", err)
	}

	// Generate ID - use hierarchical ID if parent is specified
	if issue.ID == "" {
		if issue.ParentID != "" {
			// Generate child ID from parent
			childID, err := s.GetNextChildID(ctx, issue.ParentID)
			if err != nil {
				return fmt.Errorf("generate child ID: %w", err)
			}
			issue.ID = childID
		} else {
			issue.ID = project.GenerateIssueID(proj.Prefix, issue.Title)
		}
	}

	now := time.Now()
	issue.CreatedAt = now
	issue.UpdatedAt = now

	err = s.queries.CreateIssue(ctx, db.CreateIssueParams{
		ID:          issue.ID,
		ProjectID:   issue.ProjectID,
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
	s.recordEvent(ctx, issue.ID, types.EventCreated, actor, nil, &issue.Title)

	// Auto-create parent-child dependency if this is a child issue
	if issue.ParentID != "" {
		dep := &types.Dependency{
			IssueID:     issue.ID,
			DependsOnID: issue.ParentID,
			Type:        types.DepParentChild,
			CreatedAt:   now,
			CreatedBy:   actor,
		}
		// Best-effort: dependency creation failure shouldn't rollback the issue
		_ = s.AddDependency(ctx, dep, actor)
	}

	s.rebuildFTSForIssue(ctx, issue.ID)

	return nil
}

// GetIssue retrieves an issue by ID.
func (s *Store) GetIssue(ctx context.Context, id string) (*types.Issue, error) {
	row, err := s.queries.GetIssue(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("issue not found: %s", id)
		}
		return nil, fmt.Errorf("get issue: %w", err)
	}

	return dbIssueToType(row), nil
}

// GetIssueByExternalRef retrieves an issue by its external reference.
func (s *Store) GetIssueByExternalRef(ctx context.Context, externalRef string) (*types.Issue, error) {
	row, err := s.queries.GetIssueByExternalRef(ctx, toNullString(externalRef))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("issue not found with external ref: %s", externalRef)
		}
		return nil, fmt.Errorf("get issue by external ref: %w", err)
	}

	return dbIssueToType(row), nil
}

// ListIssues returns issues matching the filter.
// All filter fields are composed with AND semantics so multiple filters
// (e.g. --parent + --status) work together via a single sqlc query.
func (s *Store) ListIssues(ctx context.Context, filter types.IssueFilter) ([]*types.Issue, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	offset := max(filter.Offset, 0)

	// Full-text search has its own dedicated path.
	if filter.Query != "" {
		return s.searchIssuesFTS(ctx, filter.ProjectID, filter.Query, limit, offset)
	}

	// Convert pointer filters to nil-able interface{} values for sqlc.narg params.
	// nil means "skip this filter", non-nil means "apply this filter".
	params := db.ListIssuesFilteredParams{
		ProjectID: filter.ProjectID,
		Limit:     int64(limit),
		Offset:    int64(offset),
	}
	if filter.Status != nil {
		params.Status = string(*filter.Status)
	}
	if filter.IssueType != nil {
		params.IssueType = string(*filter.IssueType)
	}
	if filter.Assignee != nil {
		params.Assignee = *filter.Assignee
	}
	if filter.Priority != nil {
		params.Priority = *filter.Priority
	}
	if filter.ParentID != "" {
		params.ParentID = filter.ParentID
	}

	rows, err := s.queries.ListIssuesFiltered(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}

	issues := make([]*types.Issue, len(rows))
	for i, row := range rows {
		issues[i] = dbIssueToType(row)
	}

	return issues, nil
}

// UpdateIssue updates an issue with the given updates.
func (s *Store) UpdateIssue(ctx context.Context, id string, updates map[string]any, actor string) error {
	now := time.Now()

	for field, value := range updates {
		var err error
		switch field {
		case "title":
			err = s.queries.UpdateIssueTitle(ctx, db.UpdateIssueTitleParams{
				Title:     value.(string),
				UpdatedAt: now,
				ID:        id,
			})
		case "description":
			err = s.queries.UpdateIssueDescription(ctx, db.UpdateIssueDescriptionParams{
				Description: toNullString(value.(string)),
				UpdatedAt:   now,
				ID:          id,
			})
		case "status":
			status := value.(string)
			err = s.queries.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{
				Status:    status,
				UpdatedAt: now,
				ID:        id,
			})
			s.recordEvent(ctx, id, types.EventStatusChanged, actor, nil, &status)
		case "priority":
			err = s.queries.UpdateIssuePriority(ctx, db.UpdateIssuePriorityParams{
				Priority:  int64(value.(int)),
				UpdatedAt: now,
				ID:        id,
			})
		case "issue_type":
			err = s.queries.UpdateIssueType(ctx, db.UpdateIssueTypeParams{
				IssueType: value.(string),
				UpdatedAt: now,
				ID:        id,
			})
		case "assignee":
			err = s.queries.UpdateIssueAssignee(ctx, db.UpdateIssueAssigneeParams{
				Assignee:  toNullString(value.(string)),
				UpdatedAt: now,
				ID:        id,
			})
		case "external_ref":
			err = s.queries.UpdateIssueExternalRef(ctx, db.UpdateIssueExternalRefParams{
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

	s.recordEvent(ctx, id, types.EventUpdated, actor, nil, nil)
	s.rebuildFTSForIssue(ctx, id)
	return nil
}

// CloseIssue closes an issue.
// When cascade is false, it checks for open child issues and returns an
// *types.OpenChildrenError if any are found. When cascade is true, it
// recursively closes all open descendants leaf-first before closing the
// target issue. Each cascade-closed child gets a reason of
// "<reason> (cascade closed by <parent-id>)" where parent-id is the
// original issue being closed.
func (s *Store) CloseIssue(ctx context.Context, id string, reason string, cascade bool, actor string) error {
	// Check for open children
	openChildren, err := s.GetOpenChildIssues(ctx, id)
	if err != nil {
		return fmt.Errorf("check open children: %w", err)
	}

	if len(openChildren) > 0 && !cascade {
		children := make([]types.Issue, len(openChildren))
		for i, c := range openChildren {
			children[i] = *c
		}
		return &types.OpenChildrenError{
			IssueID:  id,
			Children: children,
		}
	}

	if cascade {
		// Recursively collect and close all open descendants leaf-first
		if err := s.cascadeCloseDescendants(ctx, id, id, reason, actor); err != nil {
			return err
		}
	}

	// Close the target issue itself
	return s.closeIssueSingle(ctx, id, reason, actor)
}

// cascadeCloseDescendants recursively closes all open descendants of parentID
// in leaf-first order. rootID is the original issue being closed (for reason formatting).
func (s *Store) cascadeCloseDescendants(ctx context.Context, parentID, rootID, reason, actor string) error {
	openChildren, err := s.GetOpenChildIssues(ctx, parentID)
	if err != nil {
		return fmt.Errorf("get open children of %s: %w", parentID, err)
	}

	for _, child := range openChildren {
		// Recurse into grandchildren first (leaf-first closing)
		if err := s.cascadeCloseDescendants(ctx, child.ID, rootID, reason, actor); err != nil {
			return err
		}

		// Close this child with cascade reason
		cascadeReason := fmt.Sprintf("%s (cascade closed by %s)", reason, rootID)
		if err := s.closeIssueSingle(ctx, child.ID, cascadeReason, actor); err != nil {
			return fmt.Errorf("cascade close %s: %w", child.ID, err)
		}
	}

	return nil
}

// closeIssueSingle closes a single issue without any cascade logic.
func (s *Store) closeIssueSingle(ctx context.Context, id string, reason string, actor string) error {
	now := time.Now()
	err := s.queries.CloseIssue(ctx, db.CloseIssueParams{
		ClosedAt:    toNullTime(&now),
		CloseReason: toNullString(reason),
		UpdatedAt:   now,
		ID:          id,
	})
	if err != nil {
		return fmt.Errorf("close issue: %w", err)
	}

	s.recordEvent(ctx, id, types.EventClosed, actor, nil, &reason)
	return nil
}

// ReopenIssue reopens a closed issue.
func (s *Store) ReopenIssue(ctx context.Context, id string, actor string) error {
	now := time.Now()
	err := s.queries.ReopenIssue(ctx, db.ReopenIssueParams{
		UpdatedAt: now,
		ID:        id,
	})
	if err != nil {
		return fmt.Errorf("reopen issue: %w", err)
	}

	s.recordEvent(ctx, id, types.EventReopened, actor, nil, nil)
	return nil
}

// DeleteIssue deletes an issue.
func (s *Store) DeleteIssue(ctx context.Context, id string) error {
	// Delete from FTS index before removing the issue
	s.deleteFTSForIssue(ctx, id)

	// Delete dependencies first
	err := s.queries.DeleteDependenciesByIssue(ctx, db.DeleteDependenciesByIssueParams{
		IssueID:     id,
		DependsOnID: id,
	})
	if err != nil {
		return fmt.Errorf("delete dependencies: %w", err)
	}

	// Delete labels
	err = s.queries.DeleteIssueLabels(ctx, id)
	if err != nil {
		return fmt.Errorf("delete labels: %w", err)
	}

	// Delete events
	err = s.queries.DeleteEventsByIssue(ctx, id)
	if err != nil {
		return fmt.Errorf("delete events: %w", err)
	}

	// Delete issue
	err = s.queries.DeleteIssue(ctx, id)
	if err != nil {
		return fmt.Errorf("delete issue: %w", err)
	}

	return nil
}

// GetIssueDetails retrieves an issue with all its relational data.
func (s *Store) GetIssueDetails(ctx context.Context, id string) (*types.IssueDetails, error) {
	issue, err := s.GetIssue(ctx, id)
	if err != nil {
		return nil, err
	}

	labels, err := s.GetIssueLabels(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get labels: %w", err)
	}

	deps, err := s.GetDependencies(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get dependencies: %w", err)
	}

	dependents, err := s.GetDependents(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get dependents: %w", err)
	}

	comments, err := s.GetComments(ctx, id)
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

// dbIssueToType converts a database issue to a types.Issue.
func dbIssueToType(row *db.Issue) *types.Issue {
	return &types.Issue{
		ID:          row.ID,
		ProjectID:   row.ProjectID,
		Title:       row.Title,
		Description: fromNullString(row.Description),
		Status:      types.Status(row.Status),
		Priority:    int(row.Priority),
		Rank:        int(row.Rank),
		IssueType:   types.IssueType(row.IssueType),
		Assignee:    fromNullString(row.Assignee),
		ExternalRef: fromNullString(row.ExternalRef),
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		ClosedAt:    fromNullTime(row.ClosedAt),
		CloseReason: fromNullString(row.CloseReason),
	}
}

// recordEvent records an event in the audit trail.
// Errors are intentionally ignored because event recording is best-effort
// and should not fail the parent operation.
//
//nolint:revive,lll // argument-limit: event recording requires all these parameters
func (s *Store) recordEvent(ctx context.Context, issueID string, eventType types.EventType, actor string, oldValue, newValue *string) {
	_ = s.queries.CreateEvent(ctx, db.CreateEventParams{
		IssueID:   issueID,
		EventType: string(eventType),
		Actor:     actor,
		OldValue:  toNullString(ptrToString(oldValue)),
		NewValue:  toNullString(ptrToString(newValue)),
		CreatedAt: time.Now(),
	})
}

func ptrToString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
