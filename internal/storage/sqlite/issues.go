package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

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

	// Get workspace prefix for ID generation
	ws, err := s.GetWorkspace(ctx, issue.WorkspaceID)
	if err != nil {
		return fmt.Errorf("get workspace for ID generation: %w", err)
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
			issue.ID = generateID(ws.Prefix, issue.Title)
		}
	}

	now := time.Now()
	issue.CreatedAt = now
	issue.UpdatedAt = now

	err = s.queries.CreateIssue(ctx, db.CreateIssueParams{
		ID:                 issue.ID,
		WorkspaceID:        issue.WorkspaceID,
		Title:              issue.Title,
		Description:        toNullString(issue.Description),
		AcceptanceCriteria: toNullString(issue.AcceptanceCriteria),
		Notes:              toNullString(issue.Notes),
		Status:             string(issue.Status),
		Priority:           int64(issue.Priority),
		IssueType:          string(issue.IssueType),
		Assignee:           toNullString(issue.Assignee),
		ExternalRef:        toNullString(issue.ExternalRef),
		CreatedAt:          now,
		UpdatedAt:          now,
		ClosedAt:           toNullTime(issue.ClosedAt),
		CloseReason:        toNullString(issue.CloseReason),
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
		if err := s.AddDependency(ctx, dep, actor); err != nil {
			// Log but don't fail - the issue was created successfully
			// The dependency creation failure shouldn't rollback the issue
		}
	}

	return nil
}

// GetIssue retrieves an issue by ID.
func (s *Store) GetIssue(ctx context.Context, id string) (*types.Issue, error) {
	row, err := s.queries.GetIssue(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
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
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("issue not found with external ref: %s", externalRef)
		}
		return nil, fmt.Errorf("get issue by external ref: %w", err)
	}

	return dbIssueToType(row), nil
}

// ListIssues returns issues matching the filter.
func (s *Store) ListIssues(ctx context.Context, filter types.IssueFilter) ([]*types.Issue, error) {
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
		rows, err = s.queries.SearchIssues(ctx, db.SearchIssuesParams{
			WorkspaceID: filter.WorkspaceID,
			Title:       searchPattern,
			Description: toNullString(searchPattern),
			Limit:       int64(limit),
			Offset:      int64(offset),
		})
	case filter.Status != nil:
		rows, err = s.queries.ListIssuesByStatus(ctx, db.ListIssuesByStatusParams{
			WorkspaceID: filter.WorkspaceID,
			Status:      string(*filter.Status),
			Limit:       int64(limit),
			Offset:      int64(offset),
		})
	case filter.Assignee != nil:
		rows, err = s.queries.ListIssuesByAssignee(ctx, db.ListIssuesByAssigneeParams{
			WorkspaceID: filter.WorkspaceID,
			Assignee:    toNullString(*filter.Assignee),
			Limit:       int64(limit),
			Offset:      int64(offset),
		})
	case filter.IssueType != nil:
		rows, err = s.queries.ListIssuesByType(ctx, db.ListIssuesByTypeParams{
			WorkspaceID: filter.WorkspaceID,
			IssueType:   string(*filter.IssueType),
			Limit:       int64(limit),
			Offset:      int64(offset),
		})
	default:
		rows, err = s.queries.ListIssuesByWorkspace(ctx, db.ListIssuesByWorkspaceParams{
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
		issues[i] = dbIssueToType(row)
	}

	return issues, nil
}

// UpdateIssue updates an issue with the given updates.
func (s *Store) UpdateIssue(ctx context.Context, id string, updates map[string]interface{}, actor string) error {
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
		case "notes":
			err = s.queries.UpdateIssueNotes(ctx, db.UpdateIssueNotesParams{
				Notes:     toNullString(value.(string)),
				UpdatedAt: now,
				ID:        id,
			})
		case "acceptance_criteria":
			err = s.queries.UpdateIssueAcceptanceCriteria(ctx, db.UpdateIssueAcceptanceCriteriaParams{
				AcceptanceCriteria: toNullString(value.(string)),
				UpdatedAt:          now,
				ID:                 id,
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
	return nil
}

// CloseIssue closes an issue.
func (s *Store) CloseIssue(ctx context.Context, id string, reason string, actor string) error {
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
		ID:                 row.ID,
		WorkspaceID:        row.WorkspaceID,
		Title:              row.Title,
		Description:        fromNullString(row.Description),
		AcceptanceCriteria: fromNullString(row.AcceptanceCriteria),
		Notes:              fromNullString(row.Notes),
		Status:             types.Status(row.Status),
		Priority:           int(row.Priority),
		Rank:               int(row.Rank),
		IssueType:          types.IssueType(row.IssueType),
		Assignee:           fromNullString(row.Assignee),
		ExternalRef:        fromNullString(row.ExternalRef),
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
		ClosedAt:           fromNullTime(row.ClosedAt),
		CloseReason:        fromNullString(row.CloseReason),
	}
}

// recordEvent records an event in the audit trail.
func (s *Store) recordEvent(ctx context.Context, issueID string, eventType types.EventType, actor string, oldValue, newValue *string) {
	s.queries.CreateEvent(ctx, db.CreateEventParams{
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
