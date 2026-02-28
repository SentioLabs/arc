// Package sqlite implements the storage interface using SQLite.
// This file handles ready work queries, blocked issue detection, and workspace statistics.
package sqlite

import (
	"context"
	"fmt"
	"strings"

	"github.com/sentiolabs/arc/internal/storage/sqlite/db"
	"github.com/sentiolabs/arc/internal/types"
)

// defaultWorkLimit is the default maximum number of issues returned by work queries.
const defaultWorkLimit = 100

// GetReadyWork returns issues that are ready to work on (not blocked).
// Results are sorted according to the filter's SortPolicy (hybrid, priority, or oldest).
// Additional filters for issue type, priority, assignee, and status are applied in-memory.
//
//nolint:revive // cognitive-complexity: filter application is straightforward sequential checks
func (s *Store) GetReadyWork(ctx context.Context, filter types.WorkFilter) ([]*types.Issue, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = defaultWorkLimit
	}

	// Default to hybrid sort policy if not specified
	sortPolicy := filter.SortPolicy
	if sortPolicy == "" || !sortPolicy.IsValid() {
		sortPolicy = types.SortPolicyHybrid
	}

	var rows []*db.Issue
	var err error

	// Fetch issues using the appropriate sort query
	switch sortPolicy {
	case types.SortPolicyPriority:
		rows, err = s.queries.GetReadyIssuesPriority(ctx, db.GetReadyIssuesPriorityParams{
			WorkspaceID: filter.WorkspaceID,
			Limit:       int64(limit),
		})
	case types.SortPolicyOldest:
		rows, err = s.queries.GetReadyIssuesOldest(ctx, db.GetReadyIssuesOldestParams{
			WorkspaceID: filter.WorkspaceID,
			Limit:       int64(limit),
		})
	default: // SortPolicyHybrid
		rows, err = s.queries.GetReadyIssuesHybrid(ctx, db.GetReadyIssuesHybridParams{
			WorkspaceID: filter.WorkspaceID,
			Limit:       int64(limit),
		})
	}

	if err != nil {
		return nil, fmt.Errorf("get ready work: %w", err)
	}

	// Apply additional in-memory filters
	issues := make([]*types.Issue, 0, len(rows))
	for _, row := range rows {
		issue := dbIssueToType(row)

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
// For each blocked issue, it also fetches the IDs of the issues blocking it.
func (s *Store) GetBlockedIssues(ctx context.Context, filter types.WorkFilter) ([]*types.BlockedIssue, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = defaultWorkLimit
	}

	rows, err := s.queries.GetBlockedIssuesInWorkspace(ctx, db.GetBlockedIssuesInWorkspaceParams{
		WorkspaceID: filter.WorkspaceID,
		Limit:       int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("get blocked issues: %w", err)
	}

	issues := make([]*types.BlockedIssue, 0, len(rows))
	for _, row := range rows {
		// Get blocking issue IDs for this blocked issue
		blockingIDs := []string{}
		blockingIssues, _ := s.queries.GetBlockingIssues(ctx, row.ID)
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
// Returns true and the list of blocking issue IDs if blocked, false otherwise.
func (s *Store) IsBlocked(ctx context.Context, issueID string) (bool, []string, error) {
	blockingIssues, err := s.queries.GetBlockingIssues(ctx, issueID)
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

// GetStatistics returns aggregate statistics for a workspace.
// Includes counts by status, ready issue count, and average lead time.
func (s *Store) GetStatistics(ctx context.Context, workspaceID string) (*types.Statistics, error) {
	stats, err := s.queries.GetWorkspaceStats(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("get workspace stats: %w", err)
	}

	readyCount, err := s.queries.GetReadyIssueCount(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("get ready issue count: %w", err)
	}

	avgLeadTime, err := s.queries.GetAverageLeadTime(ctx, workspaceID)
	if err != nil && !strings.Contains(err.Error(), "no rows") {
		return nil, fmt.Errorf("get average lead time: %w", err)
	}

	return &types.Statistics{
		WorkspaceID:      workspaceID,
		TotalIssues:      int(stats.TotalIssues),
		OpenIssues:       int(stats.OpenIssues),
		InProgressIssues: int(stats.InProgressIssues),
		ClosedIssues:     int(stats.ClosedIssues),
		BlockedIssues:    int(stats.BlockedIssues),
		DeferredIssues:   int(stats.DeferredIssues),
		ReadyIssues:      int(readyCount),
		AvgLeadTimeHours: avgLeadTime.Float64,
	}, nil
}
