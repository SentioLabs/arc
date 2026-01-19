package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/sentiolabs/beads-central/internal/storage/sqlite/db"
	"github.com/sentiolabs/beads-central/internal/types"
)

func nullFloat64ToInt(nf sql.NullFloat64) int {
	if nf.Valid {
		return int(nf.Float64)
	}
	return 0
}

// GetReadyWork returns issues that are ready to work on (not blocked).
func (s *Store) GetReadyWork(ctx context.Context, filter types.WorkFilter) ([]*types.Issue, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.queries.GetOpenNonBlockedIssues(ctx, db.GetOpenNonBlockedIssuesParams{
		WorkspaceID: filter.WorkspaceID,
		Limit:       int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("get ready work: %w", err)
	}

	issues := make([]*types.Issue, 0, len(rows))
	for _, row := range rows {
		issue := dbIssueToType(row)

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
func (s *Store) GetBlockedIssues(ctx context.Context, filter types.WorkFilter) ([]*types.BlockedIssue, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
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
		// Get blocking issue IDs
		blockingIDs := []string{}
		blockingIssues, _ := s.queries.GetBlockingIssues(ctx, row.ID)
		for _, bi := range blockingIssues {
			blockingIDs = append(blockingIDs, bi.ID)
		}

		blocked := &types.BlockedIssue{
			Issue: types.Issue{
				ID:                 row.ID,
				WorkspaceID:        row.WorkspaceID,
				Title:              row.Title,
				Description:        fromNullString(row.Description),
				AcceptanceCriteria: fromNullString(row.AcceptanceCriteria),
				Notes:              fromNullString(row.Notes),
				Status:             types.Status(row.Status),
				Priority:           int(row.Priority),
				IssueType:          types.IssueType(row.IssueType),
				Assignee:           fromNullString(row.Assignee),
				ExternalRef:        fromNullString(row.ExternalRef),
				CreatedAt:          row.CreatedAt,
				UpdatedAt:          row.UpdatedAt,
				ClosedAt:           fromNullTime(row.ClosedAt),
				CloseReason:        fromNullString(row.CloseReason),
			},
			BlockedByCount: int(row.BlockedByCount),
			BlockedBy:      blockingIDs,
		}
		issues = append(issues, blocked)
	}

	return issues, nil
}

// IsBlocked checks if an issue is blocked by any open issues.
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
		OpenIssues:       nullFloat64ToInt(stats.OpenIssues),
		InProgressIssues: nullFloat64ToInt(stats.InProgressIssues),
		ClosedIssues:     nullFloat64ToInt(stats.ClosedIssues),
		BlockedIssues:    nullFloat64ToInt(stats.BlockedIssues),
		DeferredIssues:   nullFloat64ToInt(stats.DeferredIssues),
		ReadyIssues:      int(readyCount),
		AvgLeadTimeHours: avgLeadTime.Float64,
	}, nil
}
