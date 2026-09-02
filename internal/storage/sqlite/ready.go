// Package sqlite implements the storage interface using SQLite.
// This file handles ready work queries, blocked issue detection, and project statistics.
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

// maxAncestorDepth caps hierarchy walks so cyclic parent-child edges terminate.
const maxAncestorDepth = 20

// GetReadyWork returns issues that are ready to work on: unblocked, ungated by
// their roadmap ancestors, and never a container type.
// Results are sorted according to the filter's SortPolicy (hybrid, priority, or oldest).
// Additional filters for issue type, priority, and status are applied in-memory.
func (s *Store) GetReadyWork(ctx context.Context, filter types.WorkFilter) ([]*types.ReadyIssue, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = defaultWorkLimit
	}

	// Default to hybrid sort policy if not specified
	sortPolicy := filter.SortPolicy
	if sortPolicy == "" || !sortPolicy.IsValid() {
		sortPolicy = types.SortPolicyHybrid
	}

	args := []any{filter.ProjectID, filter.ProjectID, filter.ProjectID}
	if filter.Under != "" {
		args = append(args, filter.Under)
	}
	args = append(args, int64(limit))

	rows, err := s.db.QueryContext(ctx, readyBaseSQL(sortPolicy, filter.Under != ""), args...)
	if err != nil {
		return nil, fmt.Errorf("get ready work: %w", err)
	}
	defer rows.Close()

	issues := []*types.ReadyIssue{}
	for rows.Next() {
		var row db.Issue
		var effectivePriority int64
		if err := rows.Scan(
			&row.ID, &row.ProjectID, &row.Title, &row.Description,
			&row.Status, &row.Priority, &row.IssueType,
			&row.AiSessionID, &row.ExternalRef, &row.Rank,
			&row.CreatedAt, &row.UpdatedAt, &row.ClosedAt, &row.CloseReason,
			&effectivePriority,
		); err != nil {
			return nil, fmt.Errorf("scan ready issue: %w", err)
		}

		issue := dbIssueToType(&row)

		// Apply additional in-memory filters
		if filter.IssueType != nil && issue.IssueType != *filter.IssueType {
			continue
		}
		if filter.Priority != nil && issue.Priority != *filter.Priority {
			continue
		}
		if filter.Status != nil && issue.Status != *filter.Status {
			continue
		}

		issues = append(issues, &types.ReadyIssue{
			Issue:             *issue,
			EffectivePriority: int(effectivePriority),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ready work rows: %w", err)
	}

	if err := s.buildPaths(ctx, filter.ProjectID, issues); err != nil {
		return nil, err
	}

	return issues, nil
}

// readyBaseSQL builds the roadmap-aware ready-work query for a sort policy.
// We use hand-built SQL because sqlc cannot express a recursive CTE with
// computed columns, the same reason ListIssues is hand-built.
// Placeholders bind in order: project ID three times, the ancestor ID when
// under is true, then the row limit.
func readyBaseSQL(policy types.SortPolicy, under bool) string {
	var orderBy string
	switch policy {
	case types.SortPolicyPriority:
		orderBy = `e.effective_priority ASC,
  CASE WHEN i.rank = 0 THEN 999999 ELSE i.rank END ASC,
  i.created_at ASC`
	case types.SortPolicyOldest:
		orderBy = `i.created_at ASC`
	default: // SortPolicyHybrid: freshness band first, effective priority within both bands.
		orderBy = `CASE WHEN i.updated_at >= datetime('now', '-48 hours') THEN 0 ELSE 1 END ASC,
  e.effective_priority ASC,
  CASE WHEN i.rank = 0 THEN 999999 ELSE i.rank END ASC,
  i.created_at ASC`
	}

	var underClause string
	if under {
		underClause = "AND i.id IN (SELECT issue_id FROM anc WHERE ancestor_id = ?)"
	}

	// The excluded issue types mirror types.IssueType.IsContainer.
	return fmt.Sprintf(`
WITH RECURSIVE anc(issue_id, ancestor_id, depth) AS (
    SELECT d.issue_id, d.depends_on_id, 1
    FROM dependencies d
    JOIN issues p ON p.id = d.depends_on_id
    WHERE d.type = 'parent-child' AND p.project_id = ?
    UNION ALL
    SELECT a.issue_id, d.depends_on_id, a.depth + 1
    FROM anc a
    JOIN dependencies d ON d.issue_id = a.ancestor_id AND d.type = 'parent-child'
    WHERE a.depth < %d
),
gated AS (
    SELECT DISTINCT a.issue_id
    FROM anc a
    JOIN issues anci ON anci.id = a.ancestor_id
    LEFT JOIN dependencies bd ON bd.issue_id = anci.id AND bd.type = 'blocks'
    LEFT JOIN issues blocker ON blocker.id = bd.depends_on_id AND blocker.status != 'closed'
    WHERE anci.status IN ('deferred', 'blocked') OR blocker.id IS NOT NULL
),
eff AS (
    SELECT i.id AS issue_id,
           CASE WHEN MIN(x.priority) IS NULL OR i.priority < MIN(x.priority)
                THEN i.priority ELSE MIN(x.priority) END AS effective_priority
    FROM issues i
    LEFT JOIN anc a ON a.issue_id = i.id
    LEFT JOIN issues x ON x.id = a.ancestor_id
    WHERE i.project_id = ?
    GROUP BY i.id
)
SELECT i.id, i.project_id, i.title, i.description, i.status, i.priority,
       i.issue_type, i.ai_session_id, i.external_ref, i.rank,
       i.created_at, i.updated_at, i.closed_at, i.close_reason,
       e.effective_priority
FROM issues i
JOIN eff e ON e.issue_id = i.id
LEFT JOIN dependencies d ON d.issue_id = i.id AND d.type = 'blocks'
LEFT JOIN issues blocker ON blocker.id = d.depends_on_id AND blocker.status != 'closed'
WHERE i.project_id = ?
  AND i.status IN ('open', 'in_progress')
  AND i.issue_type NOT IN ('epic', 'release', 'milestone')
  AND i.id NOT IN (SELECT issue_id FROM gated)
  %s
GROUP BY i.id
HAVING COUNT(blocker.id) = 0
ORDER BY %s
LIMIT ?
`, maxAncestorDepth, underClause, orderBy)
}

// buildPaths fills each issue's ancestry title path, root-first, from the
// project's parent-child edges. One batch query serves every issue and the
// walk is depth-capped so cyclic edges terminate.
func (s *Store) buildPaths(ctx context.Context, projectID string, issues []*types.ReadyIssue) error {
	if len(issues) == 0 {
		return nil
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT d.issue_id, d.depends_on_id, p.title
FROM dependencies d
JOIN issues p ON p.id = d.depends_on_id
WHERE d.type = 'parent-child' AND p.project_id = ?`, projectID)
	if err != nil {
		return fmt.Errorf("load ancestry: %w", err)
	}
	defer rows.Close()

	parentOf := map[string]string{}
	titleOf := map[string]string{}
	for rows.Next() {
		var issueID, parentID, title string
		if err := rows.Scan(&issueID, &parentID, &title); err != nil {
			return fmt.Errorf("scan ancestry: %w", err)
		}
		parentOf[issueID] = parentID
		titleOf[parentID] = title
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("ancestry rows: %w", err)
	}

	for _, issue := range issues {
		var path []string
		id := parentOf[issue.ID]
		for depth := 0; id != "" && depth < maxAncestorDepth; depth++ {
			path = append([]string{titleOf[id]}, path...)
			id = parentOf[id]
		}
		issue.Path = path
	}

	return nil
}

// GetBlockedIssues returns issues that are blocked by other issues.
// For each blocked issue, it also fetches the IDs of the issues blocking it.
func (s *Store) GetBlockedIssues(ctx context.Context, filter types.WorkFilter) ([]*types.BlockedIssue, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = defaultWorkLimit
	}

	rows, err := s.queries.GetBlockedIssuesInProject(ctx, db.GetBlockedIssuesInProjectParams{
		ProjectID: filter.ProjectID,
		Limit:     int64(limit),
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
				ProjectID:   row.ProjectID,
				Title:       row.Title,
				Description: fromNullString(row.Description),
				Status:      types.Status(row.Status),
				Priority:    int(row.Priority),
				IssueType:   types.IssueType(row.IssueType),
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

// GetStatistics returns aggregate statistics for a project.
// Includes counts by status, ready issue count, and average lead time.
func (s *Store) GetStatistics(ctx context.Context, projectID string) (*types.Statistics, error) {
	stats, err := s.queries.GetProjectStats(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("get project stats: %w", err)
	}

	// TODO(roadmap): align GetReadyIssueCount with gating
	readyCount, err := s.queries.GetReadyIssueCount(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("get ready issue count: %w", err)
	}

	avgLeadTime, err := s.queries.GetAverageLeadTime(ctx, projectID)
	if err != nil && !strings.Contains(err.Error(), "no rows") {
		return nil, fmt.Errorf("get average lead time: %w", err)
	}

	return &types.Statistics{
		ProjectID:        projectID,
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
