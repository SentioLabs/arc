package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/sentiolabs/arc/internal/project"
	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/storage/sqlite/db"
	"github.com/sentiolabs/arc/internal/types"
)

// issueMutationTx is the shared executor for issue mutations and staged adoption.
// Its helpers never acquire another connection or begin a nested transaction.
type issueMutationTx struct {
	tx                *sql.Tx
	queries           *db.Queries
	expected          *types.ExpectedGovernance
	completionChecked string
}

// withIssueMutation owns the sole commit boundary. SQLite uses immediate
// transactions, so snapshots and checks cannot race another writer.
func (s *Store) withIssueMutation(ctx context.Context, apply func(*issueMutationTx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if err := apply(&issueMutationTx{tx: tx, queries: s.queries.WithTx(tx)}); err != nil {
		return err
	}
	return tx.Commit()
}

// GetIssue reads on the transaction connection, avoiding pool reentry.
func (s *issueMutationTx) GetIssue(ctx context.Context, id string) (*types.Issue, error) {
	row, err := s.queries.GetIssue(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %s", storage.ErrIssueNotFound, id)
		}
		return nil, err
	}
	return dbIssueToType(row), nil
}

// GetProject reads identity on the transaction connection.
func (s *issueMutationTx) GetProject(ctx context.Context, id string) (*types.Project, error) {
	row, err := s.queries.GetProject(ctx, id)
	if err != nil {
		return nil, err
	}
	return dbProjectToType(row), nil
}

// GetOpenChildIssues keeps close-child validation under the write lock.
func (s *issueMutationTx) GetOpenChildIssues(ctx context.Context, id string) ([]*types.Issue, error) {
	rows, err := s.queries.GetOpenChildIssues(ctx, id)
	if err != nil {
		return nil, err
	}
	result := make([]*types.Issue, len(rows))
	for i, row := range rows {
		result[i] = dbIssueToType(row)
	}
	return result, nil
}

// recordEvent treats audit failures as mutation failures; actor data is retained.
//
//nolint:revive // Audit records require issue, event, actor and both values.
func (s *issueMutationTx) recordEvent(
	ctx context.Context, id string, event types.EventType, actor string, oldValue, newValue *string,
) error {
	return s.queries.CreateEvent(ctx, db.CreateEventParams{
		IssueID: id, EventType: string(event), Actor: actor,
		OldValue: toNullString(
			ptrToString(oldValue),
		), NewValue: toNullString(ptrToString(newValue)), CreatedAt: time.Now(),
	})
}

// rebuildFTSForIssue replaces the indexed content atomically with the issue.
// Returning errors prevents a successful response with a missing search entry.
func (s *issueMutationTx) rebuildFTSForIssue(ctx context.Context, id string) error {
	row, err := s.queries.GetIssue(ctx, id)
	if err != nil {
		return err
	}
	if err := s.deleteFTSForIssue(ctx, id); err != nil {
		return err
	}
	_, err = s.tx.ExecContext(ctx,
		`INSERT INTO issues_fts(id,title,description) VALUES (?,?,?)`, id, row.Title, fromNullString(row.Description),
	)
	return err
}

// deleteFTSForIssue is rolled back along with deletion or index replacement.
func (s *issueMutationTx) deleteFTSForIssue(ctx context.Context, id string) error {
	_, err := s.tx.ExecContext(ctx, `DELETE FROM issues_fts WHERE id=?`, id)
	return err
}

// getNextChildNumber reserves the next suffix inside the mutation transaction.
func (s *issueMutationTx) getNextChildNumber(ctx context.Context, parentID string) (int, error) {
	var nextChild int
	err := s.tx.QueryRowContext(ctx, `
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
func (s *issueMutationTx) GetNextChildID(ctx context.Context, parentID string) (string, error) {
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
func (s *issueMutationTx) CreateIssue(ctx context.Context, issue *types.Issue, actor string) error {
	if err := s.prepareIssue(ctx, issue); err != nil {
		return err
	}

	now := time.Now()
	issue.CreatedAt = now
	issue.UpdatedAt = now

	err := s.queries.CreateIssue(ctx, db.CreateIssueParams{
		ID:          issue.ID,
		ProjectID:   issue.ProjectID,
		Title:       issue.Title,
		Description: toNullString(issue.Description),
		Status:      string(issue.Status),
		Priority:    int64(issue.Priority),
		IssueType:   string(issue.IssueType),
		AiSessionID: toNullString(issue.AISessionID),
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
	if err := s.recordEvent(ctx, issue.ID, types.EventCreated, actor, nil, &issue.Title); err != nil {
		return err
	}

	// Auto-create parent-child dependency if this is a child issue
	if issue.ParentID != "" {
		dep := &types.Dependency{
			IssueID:     issue.ID,
			DependsOnID: issue.ParentID,
			Type:        types.DepParentChild,
			CreatedAt:   now,
			CreatedBy:   actor,
		}
		if err := s.insertDependency(ctx, dep, actor); err != nil {
			return err
		}
		governing, err := s.resolveGoverningPlan(ctx, issue.ProjectID, issue.ID)
		if err != nil {
			return err
		}
		if issue.Status == types.StatusClosed && governing != nil {
			return storage.ErrExecutionPrecondition
		}
	}

	if err := s.rebuildFTSForIssue(ctx, issue.ID); err != nil {
		return err
	}

	row, err := s.GetIssue(ctx, issue.ID)
	if err != nil {
		return err
	}
	issue.ContractVersion = row.ContractVersion
	return nil
}

// writeIssueFields applies staged contract fields and their audit/index effects.
// Adoption may call this after validating the complete staged graph.
func (s *issueMutationTx) writeIssueFields(ctx context.Context,
	id string,
	updates map[string]any,
	actor string,
) error {
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
			err = s.changeStatus(ctx, id, value.(string), actor)

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
		case "ai_session_id":
			err = s.queries.UpdateIssueAISessionID(ctx, db.UpdateIssueAISessionIDParams{
				AiSessionID: toNullString(value.(string)),
				UpdatedAt:   now,
				ID:          id,
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

	if err := s.recordEvent(ctx, id, types.EventUpdated, actor, nil, nil); err != nil {
		return err
	}
	if err := s.rebuildFTSForIssue(ctx, id); err != nil {
		return err
	}
	return nil
}

// CloseIssue closes an issue.
// When cascade is false, it checks for open child issues and returns an
// *types.OpenChildrenError if any are found. When cascade is true, it
// recursively closes all open descendants leaf-first before closing the
// target issue. Each cascade-closed child gets a reason of
// "<reason> (cascade closed by <parent-id>)" where parent-id is the
func (s *issueMutationTx) CloseIssue(ctx context.Context,
	id string,
	reason string,
	cascade bool,
	actor string,
) error {
	if s.completionChecked != id {
		if _, err := s.checkExecutionExpected(ctx, id, s.expected); err != nil {
			return err
		}
	}
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
		if err := s.validateCascadeGraph(ctx, id); err != nil {
			return err
		}
		// Recursively collect and close all open descendants leaf-first
		if err := s.cascadeCloseDescendants(ctx, id, id, reason, actor); err != nil {
			return err
		}
	}

	// Close the target issue itself
	return s.closeIssueSingle(ctx, id, reason, actor)
}

// cascadeCloseDescendants recursively closes all open descendants of parentID
func (s *issueMutationTx) cascadeCloseDescendants(ctx context.Context,
	parentID,
	rootID,
	reason,
	actor string,
) error {
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

func (s *issueMutationTx) closeIssueSingle(ctx context.Context, id string, reason string, actor string) error {
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

	if err := s.recordEvent(ctx, id, types.EventClosed, actor, nil, &reason); err != nil {
		return err
	}
	return nil
}

func (s *issueMutationTx) ReopenIssue(ctx context.Context, id string, actor string) error {
	now := time.Now()
	err := s.queries.ReopenIssue(ctx, db.ReopenIssueParams{
		UpdatedAt: now,
		ID:        id,
	})
	if err != nil {
		return fmt.Errorf("reopen issue: %w", err)
	}

	if err := s.recordEvent(ctx, id, types.EventReopened, actor, nil, nil); err != nil {
		return err
	}
	return nil
}

// eraseIssue deletes the index and relational data in the caller transaction.
func (s *issueMutationTx) eraseIssue(ctx context.Context, id string) error {
	// Delete from FTS index before removing the issue
	if err := s.deleteFTSForIssue(ctx, id); err != nil {
		return err
	}

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

// insertDependency is the transaction-bound write used by creation and adoption.
// Ordinary callers enter through AddDependency to compare effective governance.
func (s *issueMutationTx) insertDependency(ctx context.Context, dep *types.Dependency, actor string) error {
	if dep.IssueID == dep.DependsOnID {
		return errors.New("issue cannot depend on itself")
	}

	if !dep.Type.IsValid() {
		return fmt.Errorf("invalid dependency type: %s", dep.Type)
	}

	now := time.Now()
	dep.CreatedAt = now
	dep.CreatedBy = actor

	err := s.queries.AddDependency(ctx, db.AddDependencyParams{
		IssueID:     dep.IssueID,
		DependsOnID: dep.DependsOnID,
		Type:        string(dep.Type),
		CreatedAt:   now,
		CreatedBy:   toNullString(actor),
	})
	if err != nil {
		return fmt.Errorf("add dependency: %w", err)
	}

	// Record event
	newVal := fmt.Sprintf("%s depends on %s (%s)", dep.IssueID, dep.DependsOnID, dep.Type)
	if err := s.recordEvent(ctx, dep.IssueID, types.EventDependencyAdded, actor, nil, &newVal); err != nil {
		return err
	}

	return nil
}

// deleteDependency records removal under the same lock as the edge deletion.
func (s *issueMutationTx) deleteDependency(ctx context.Context, issueID, dependsOnID string, actor string) error {
	err := s.queries.RemoveDependency(ctx, db.RemoveDependencyParams{
		IssueID:     issueID,
		DependsOnID: dependsOnID,
	})
	if err != nil {
		return fmt.Errorf("remove dependency: %w", err)
	}

	// Record event
	oldVal := fmt.Sprintf("%s no longer depends on %s", issueID, dependsOnID)
	if err := s.recordEvent(ctx, issueID, types.EventDependencyRemoved, actor, &oldVal, nil); err != nil {
		return err
	}

	return nil
}

// AddDependency rejects effective chain changes on the ordinary mutation path.
func (s *issueMutationTx) AddDependency(ctx context.Context, dep *types.Dependency, actor string) error {
	issue, err := s.GetIssue(ctx, dep.IssueID)
	if err != nil {
		return err
	}
	parent, err := s.GetIssue(ctx, dep.DependsOnID)
	if err != nil {
		return err
	}
	if issue.ProjectID != parent.ProjectID && dep.Type == types.DepParentChild {
		return errors.New("dependency crosses project boundary")
	}
	return s.guardGovernanceMutation(ctx, issue.ProjectID, dep.IssueID, func() error {
		return s.insertDependency(ctx, dep, actor)
	})
}

// RemoveDependency compares both old and new affected descendant sets.
func (s *issueMutationTx) RemoveDependency(ctx context.Context, issueID, dependsOnID, actor string) error {
	issue, err := s.GetIssue(ctx, issueID)
	if err != nil {
		return err
	}
	return s.guardGovernanceMutation(ctx, issue.ProjectID, issueID, func() error {
		return s.deleteDependency(ctx, issueID, dependsOnID, actor)
	})
}

// UpdateIssue guards type changes before applying the ordinary field mutation.
func (s *issueMutationTx) UpdateIssue(ctx context.Context, id string, updates map[string]any, actor string) error {
	issue, err := s.GetIssue(ctx, id)
	if err != nil {
		return err
	}
	if updates["status"] == string(types.StatusClosed) {
		if _, err := s.checkExecutionExpected(ctx, id, s.expected); err != nil {
			return err
		}
		s.completionChecked = id
	}
	apply := func() error { return s.writeIssueFields(ctx, id, updates, actor) }
	if typ, ok := updates["issue_type"]; ok {
		if issue.GoverningPlan != nil && typ != string(issue.IssueType) {
			return fmt.Errorf("%w: pinned container type", storage.ErrGovernanceReconciliation)
		}
		return s.guardGovernanceMutation(ctx, issue.ProjectID, id, apply)
	}
	return apply()
}

// DeleteIssue protects active governance and retained container provenance.
func (s *issueMutationTx) DeleteIssue(ctx context.Context, id string) error {
	// The retained marker survives pin removal and protects historic containers.
	var retained int
	if err := s.tx.QueryRowContext(ctx,
		`SELECT count(*) FROM issue_governance_history WHERE issue_id=?`, id,
	).Scan(&retained); err != nil {
		return err
	}
	if retained > 0 {
		return fmt.Errorf("%w: retained container history; close it instead", storage.ErrGovernanceReconciliation)
	}
	issue, err := s.GetIssue(ctx, id)
	if err != nil {
		return err
	}
	governing, err := s.resolveGoverningPlan(ctx, issue.ProjectID, id)
	if err != nil {
		return err
	}
	if governing != nil {
		return fmt.Errorf("%w: cannot delete governed issue; close it instead",
			storage.ErrGovernanceReconciliation)
	}
	return s.guardGovernanceMutation(ctx, issue.ProjectID, id, func() error {
		return s.eraseIssue(ctx, id)
	})
}

// UpdateIssueAndGet captures post-mutation governance and contract version before
// releasing the write lock, including for claims through the existing PUT routes.
func (s *Store) UpdateIssueAndGet(
	ctx context.Context, id string, updates map[string]any, actor string,
) (*types.IssueDetails, error) {
	var result *types.IssueDetails
	err := s.withIssueMutation(ctx, func(m *issueMutationTx) error {
		if err := m.UpdateIssue(ctx, id, updates, actor); err != nil {
			return err
		}
		issue, err := m.GetIssue(ctx, id)
		if err != nil {
			return err
		}
		governing, err := m.resolveGoverningPlan(ctx, issue.ProjectID, id)
		if err != nil {
			return err
		}
		result = &types.IssueDetails{Issue: *issue, ResolvedGovernance: governing}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// prepareIssue validates ownership and allocates a hierarchical counter under
// the same transaction as the issue and parent edge. A failed write consumes no ID.
func (s *issueMutationTx) prepareIssue(ctx context.Context, issue *types.Issue) error {
	if issue.GoverningPlan != nil {
		return fmt.Errorf("%w: explicit pins require plan adoption", storage.ErrGovernanceReconciliation)
	}
	issue.SetDefaults()

	if err := issue.Validate(); err != nil {
		return fmt.Errorf("validate issue: %w", err)
	}

	// Get project prefix for ID generation
	proj, err := s.GetProject(ctx, issue.ProjectID)
	if err != nil {
		return fmt.Errorf("get project for ID generation: %w", err)
	}

	if issue.ParentID != "" {
		parent, err := s.GetIssue(ctx, issue.ParentID)
		if err != nil {
			return err
		}
		if parent.ProjectID != issue.ProjectID {
			return errors.New("parent belongs to another project")
		}
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

	return nil
}

// changeStatus makes status-to-closed updates obey the same child checks as the
// explicit close endpoint and clears close metadata when reopening.
func (s *issueMutationTx) changeStatus(ctx context.Context, id, status, actor string) error {
	if status == string(types.StatusClosed) {
		return s.CloseIssue(ctx, id, "", false, actor)
	}
	current, err := s.GetIssue(ctx, id)
	if err != nil {
		return err
	}
	if status == string(types.StatusOpen) && current.Status == types.StatusClosed {
		return s.ReopenIssue(ctx, id, actor)
	}
	if err := s.queries.UpdateIssueStatus(ctx,
		db.UpdateIssueStatusParams{Status: status, UpdatedAt: time.Now(), ID: id}); err != nil {
		return err
	}
	return s.recordEvent(ctx, id, types.EventStatusChanged, actor, nil, &status)
}

// validateCascadeGraph checks every descendant's ancestry before recursive close.
// Legacy cycles must return an explicit error rather than loop until cancellation.
func (s *issueMutationTx) validateCascadeGraph(ctx context.Context, id string) error {
	issue, err := s.GetIssue(ctx, id)
	if err != nil {
		return err
	}
	graph, err := s.governanceGraph(ctx, issue.ProjectID)
	if err != nil {
		return err
	}
	for descendant := range graph.descendants(id) {
		governing, err := graph.resolve(descendant)
		if err != nil {
			return err
		}
		if descendant != id && governing != nil {
			child, err := s.GetIssue(ctx, descendant)
			if err != nil {
				return err
			}
			if child.Status != types.StatusClosed {
				return fmt.Errorf("%w: close governed descendants individually", storage.ErrExecutionConflict)
			}
		}
	}
	return nil
}
