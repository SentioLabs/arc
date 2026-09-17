package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sentiolabs/arc/internal/storage"
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

// GetIssue retrieves an issue by ID.
func (s *Store) GetIssue(ctx context.Context, id string) (*types.Issue, error) {
	row, err := s.queries.GetIssue(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %s", storage.ErrIssueNotFound, id)
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

// allStatuses contains every valid status string for use as a default filter.
var allStatuses = []string{"open", "in_progress", "blocked", "deferred", "closed"}

// allIssueTypes contains every valid issue type string for use as a default filter.
var allIssueTypes = issueTypeStrings(types.AllIssueTypes())

// issueTypeStrings converts issue types to their string values.
func issueTypeStrings(issueTypes []types.IssueType) []string {
	out := make([]string, len(issueTypes))
	for i, t := range issueTypes {
		out[i] = string(t)
	}
	return out
}

// allPriorities contains every valid priority value for use as a default filter.
var allPriorities = []int64{0, 1, 2, 3, 4}

// ListIssues returns issues matching the filter.
// All filter fields are composed with AND semantics so multiple filters
// (e.g. --parent + --status) work together via a dynamic SQL query.
// We use dynamic SQL because sqlc.slice and sqlc.narg positional placeholders
// are incompatible when mixed in the same query (positional ?N offsets shift
// when slice placeholders expand to multiple values).
func (s *Store) ListIssues(ctx context.Context, filter types.IssueFilter) ([]*types.Issue, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	offset := max(filter.Offset, 0)

	if filter.Query != "" {
		return s.searchIssuesFTS(ctx, filter.ProjectID, filter.Query, limit, offset)
	}

	query, args := buildListIssuesQuery(filter, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}
	defer rows.Close()

	var issues []*types.Issue
	for rows.Next() {
		var row db.Issue
		if err := rows.Scan(
			&row.ID, &row.ProjectID, &row.Title, &row.Description,
			&row.Status, &row.Priority, &row.IssueType,
			&row.AiSessionID, &row.ExternalRef, &row.Rank,
			&row.CreatedAt, &row.UpdatedAt, &row.ClosedAt, &row.CloseReason,
			&row.ContractVersion, &row.GoverningPlanID, &row.GoverningPlanRevision,
		); err != nil {
			return nil, fmt.Errorf("scan issue: %w", err)
		}
		issues = append(issues, dbIssueToType(&row))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list issues rows: %w", err)
	}

	return issues, nil
}

// buildListIssuesQuery constructs the dynamic SQL and args for ListIssues.
// All string interpolation uses only positional placeholder references (?N),
// never user-supplied values directly.
func buildListIssuesQuery(filter types.IssueFilter, limit, offset int) (string, []any) {
	statuses := toStringSliceOrDefault(filter.Statuses, allStatuses)
	issueTypes := toStringSliceOrDefault(filter.IssueTypes, allIssueTypes)
	priorities := toInt64SliceOrDefault(filter.Priorities, allPriorities)

	args := []any{filter.ProjectID}
	argIdx := 2

	statusPH := appendSlice(&args, &argIdx, statuses)
	typePH := appendSlice(&args, &argIdx, issueTypes)
	priorityPH := appendSlice(&args, &argIdx, priorities)

	var sessionClause, parentClause, parentJoin string

	if filter.AISessionID != nil {
		sessionClause = fmt.Sprintf("AND i.ai_session_id = ?%d", argIdx)
		args = append(args, *filter.AISessionID)
		argIdx++
	}
	if filter.ParentID != "" {
		parentJoin = "JOIN dependencies d ON d.issue_id = i.id AND d.type = 'parent-child'"
		parentClause = fmt.Sprintf("AND d.depends_on_id = ?%d", argIdx)
		args = append(args, filter.ParentID)
		argIdx++
	}

	offsetPH := fmt.Sprintf("?%d", argIdx)
	args = append(args, int64(offset))
	argIdx++
	limitPH := fmt.Sprintf("?%d", argIdx)
	args = append(args, int64(limit))

	query := fmt.Sprintf(`
SELECT i.id, i.project_id, i.title, i.description, i.status, i.priority,
       i.issue_type, i.ai_session_id, i.external_ref, i.rank,
       i.created_at, i.updated_at, i.closed_at, i.close_reason,
 i.contract_version, i.governing_plan_id, i.governing_plan_revision
FROM issues i
%s
WHERE i.project_id = ?1
  AND i.status IN (%s)
  AND i.issue_type IN (%s)
  AND i.priority IN (%s)
  %s
  %s
ORDER BY i.priority ASC, i.updated_at DESC
LIMIT %s OFFSET %s
`, parentJoin, statusPH, typePH, priorityPH,
		sessionClause, parentClause,
		limitPH, offsetPH)

	return query, args
}

// toStringSliceOrDefault converts a typed slice to []string, returning defaults if empty.
func toStringSliceOrDefault[T ~string](vals []T, defaults []string) []string {
	if len(vals) == 0 {
		return defaults
	}
	out := make([]string, len(vals))
	for i, v := range vals {
		out[i] = string(v)
	}
	return out
}

// toInt64SliceOrDefault converts []int to []int64, returning defaults if empty.
func toInt64SliceOrDefault(vals []int, defaults []int64) []int64 {
	if len(vals) == 0 {
		return defaults
	}
	out := make([]int64, len(vals))
	for i, v := range vals {
		out[i] = int64(v)
	}
	return out
}

// appendSlice adds slice values to args and returns the SQL placeholder string.
func appendSlice[T any](args *[]any, argIdx *int, vals []T) string {
	ph := buildPlaceholders(argIdx, len(vals))
	for _, v := range vals {
		*args = append(*args, v)
	}
	return ph
}

// buildPlaceholders generates a comma-separated string of SQL placeholders
// like "?2, ?3, ?4" and advances the argIdx accordingly.
func buildPlaceholders(argIdx *int, count int) string {
	placeholders := make([]string, count)
	for i := range count {
		placeholders[i] = fmt.Sprintf("?%d", *argIdx)
		*argIdx++
	}
	return strings.Join(placeholders, ", ")
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

	var governing *types.GoverningPlan
	err = s.withIssueMutation(ctx, func(m *issueMutationTx) error {
		var err error
		issue, err = m.GetIssue(ctx, id)
		if err != nil {
			return err
		}
		governing, err = m.resolveGoverningPlan(ctx, issue.ProjectID, id)

		return err
	})
	if err != nil {
		return nil, err
	}
	return &types.IssueDetails{
		ResolvedGovernance: governing,
		Issue:              *issue,
		Labels:             labels,
		Dependencies:       deps,
		Dependents:         dependents,
		Comments:           comments,
	}, nil
}

// dbIssueToType converts a database issue to a types.Issue.
func dbIssueToType(row *db.Issue) *types.Issue {
	return &types.Issue{
		ContractVersion: row.ContractVersion,
		GoverningPlan:   dbPlanReference(row.GoverningPlanID, row.GoverningPlanRevision),
		ID:              row.ID,
		ProjectID:       row.ProjectID,
		Title:           row.Title,
		Description:     fromNullString(row.Description),
		Status:          types.Status(row.Status),
		Priority:        int(row.Priority),
		Rank:            int(row.Rank),
		IssueType:       types.IssueType(row.IssueType),
		AISessionID:     fromNullString(row.AiSessionID),
		ExternalRef:     fromNullString(row.ExternalRef),
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		ClosedAt:        fromNullTime(row.ClosedAt),
		CloseReason:     fromNullString(row.CloseReason),
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

// GetNextChildID reserves a hierarchical suffix atomically. Creation uses the same transaction for the
// reservation and issue.
func (s *Store) GetNextChildID(ctx context.Context, parentID string) (string, error) {
	var id string
	err := s.withIssueMutation(ctx, func(m *issueMutationTx) error {
		var err error
		id, err = m.GetNextChildID(ctx, parentID)
		return err
	})
	return id, err
}

// CreateIssue commits the issue, parent edge, counter, audit and index together.
func (s *Store) CreateIssue(ctx context.Context, issue *types.Issue, actor string) error {
	candidate := *issue
	err := s.withIssueMutation(ctx, func(m *issueMutationTx) error { return m.CreateIssue(ctx, &candidate, actor) })
	if err == nil {
		*issue = candidate
	}
	return err
}

// UpdateIssue applies ordinary field updates under the shared mutation lock.
func (s *Store) UpdateIssue(ctx context.Context, id string, updates map[string]any, actor string) error {
	return s.withIssueMutation(ctx, func(m *issueMutationTx) error {
		return m.UpdateIssue(ctx, id, updates, actor)
	})
}

// CloseIssue preserves child checks and commits cascade close atomically.
func (s *Store) CloseIssue(ctx context.Context, id, reason string, cascade bool, actor string) error {
	return s.withIssueMutation(ctx, func(m *issueMutationTx) error {
		return m.CloseIssue(ctx, id, reason, cascade, actor)
	})
}

// ReopenIssue changes status and audit in one transaction.
func (s *Store) ReopenIssue(ctx context.Context, id, actor string) error {
	return s.withIssueMutation(ctx, func(m *issueMutationTx) error {
		return m.ReopenIssue(ctx, id, actor)
	})
}

// DeleteIssue rejects governance/history loss and deletes unlinked issues atomically.
func (s *Store) DeleteIssue(ctx context.Context, id string) error {
	return s.withIssueMutation(ctx, func(m *issueMutationTx) error {
		return m.DeleteIssue(ctx, id)
	})
}

func dbPlanReference(planID sql.NullString, revision sql.NullInt64) *types.PlanReference {
	if !planID.Valid || !revision.Valid {
		return nil
	}
	return &types.PlanReference{PlanID: planID.String, Revision: revision.Int64}
}
