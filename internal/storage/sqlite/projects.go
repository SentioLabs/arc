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

// CreateProject creates a new project.
func (s *Store) CreateProject(ctx context.Context, p *types.Project) error {
	if err := p.Validate(); err != nil {
		return fmt.Errorf("validate project: %w", err)
	}

	// Generate ID if not provided
	if p.ID == "" {
		p.ID = project.GenerateProjectID("proj", p.Name)
	}

	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now

	err := s.queries.CreateProject(ctx, db.CreateProjectParams{
		ID:          p.ID,
		Name:        p.Name,
		Description: toNullString(p.Description),
		Prefix:      p.Prefix,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		return fmt.Errorf("create project: %w", err)
	}

	return nil
}

// GetProject retrieves a project by ID.
func (s *Store) GetProject(ctx context.Context, id string) (*types.Project, error) {
	row, err := s.queries.GetProject(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("project not found: %s", id)
		}
		return nil, fmt.Errorf("get project: %w", err)
	}

	return dbProjectToType(row), nil
}

// GetProjectByName retrieves a project by name.
func (s *Store) GetProjectByName(ctx context.Context, name string) (*types.Project, error) {
	row, err := s.queries.GetProjectByName(ctx, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("project not found: %s", name)
		}
		return nil, fmt.Errorf("get project by name: %w", err)
	}

	return dbProjectToType(row), nil
}

// ListProjects returns all projects.
func (s *Store) ListProjects(ctx context.Context) ([]*types.Project, error) {
	rows, err := s.queries.ListProjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}

	projects := make([]*types.Project, len(rows))
	for i, row := range rows {
		projects[i] = dbProjectToType(row)
	}

	return projects, nil
}

// UpdateProject updates a project.
func (s *Store) UpdateProject(ctx context.Context, p *types.Project) error {
	p.UpdatedAt = time.Now()

	err := s.queries.UpdateProject(ctx, db.UpdateProjectParams{
		Name:        p.Name,
		Description: toNullString(p.Description),
		UpdatedAt:   p.UpdatedAt,
		ID:          p.ID,
	})
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}

	return nil
}

// DeleteProject deletes a project and all its issues.
// Accepts either project ID (e.g., "proj-00blnw") or name (e.g., "my-project-a1b2c3").
func (s *Store) DeleteProject(ctx context.Context, idOrName string) error {
	// Try to resolve by ID first
	p, err := s.GetProject(ctx, idOrName)
	if err != nil {
		// If not found by ID, try by name
		p, err = s.GetProjectByName(ctx, idOrName)
		if err != nil {
			return fmt.Errorf("project not found: %s", idOrName)
		}
	}

	// Ownership checks and deletion share a write transaction so a concurrent
	// upload cannot publish a database reference after ownership was checked.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	q := s.queries.WithTx(tx)
	if err := guardDurableOwnership(ctx, q, p.ID); err != nil {
		return err
	}
	// Remove search rows before the cascading project delete, in the same transaction.
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM issues_fts WHERE id IN (SELECT id FROM issues WHERE project_id=?)`, p.ID,
	); err != nil {
		return err
	}
	if err := q.DeleteProject(ctx, p.ID); err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	return tx.Commit()
}

// MergeProjects moves all issues and plans from source projects into the
// target project, deletes the sources, and returns a summary. The entire
// operation runs inside a single transaction for atomicity.
func (s *Store) MergeProjects(
	ctx context.Context, targetID string, sourceIDs []string, actor string,
) (*types.MergeResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	qtx := s.queries.WithTx(tx)
	mutation := &issueMutationTx{tx: tx, queries: qtx}

	// Validate target exists
	if _, err := qtx.GetProject(ctx, targetID); err != nil {
		return nil, fmt.Errorf("target project not found: %s", targetID)
	}

	// Check both ends of every incident ancestry edge before moving any source.
	// Looking only at source children misses target/third-project descendants.
	if err := mutation.guardMergeAncestry(ctx, targetID, sourceIDs); err != nil {
		return nil, err
	}

	var totalIssues int64
	var deletedSources []string

	// Enumerate every source issue under the write lock, without list pagination.
	// Audit and FTS writes below must succeed before any source deletion commits.
	for _, srcID := range sourceIDs {
		movedIssueIDs, err := mutation.mergeSourceIssueIDs(ctx, srcID)
		if err != nil {
			return nil, err
		}

		issues, err := mergeOneSource(ctx, qtx, targetID, srcID)
		if err != nil {
			return nil, err
		}
		if err := mutation.recordMerge(ctx, targetID, movedIssueIDs, actor); err != nil {
			return nil, err
		}

		totalIssues += issues
		deletedSources = append(deletedSources, srcID)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit merge: %w", err)
	}

	target, err := s.GetProject(ctx, targetID)
	if err != nil {
		return nil, fmt.Errorf("fetch merged project: %w", err)
	}
	return &types.MergeResult{
		TargetProject:  target,
		IssuesMoved:    int(totalIssues),
		SourcesDeleted: deletedSources,
	}, nil
}

// mergeOneSource moves all issues from a single source project
// into the target, deletes the source config and project, and returns counts.
func mergeOneSource(
	ctx context.Context, qtx *db.Queries, targetID, srcID string,
) (issues int64, err error) {
	if srcID == targetID {
		return 0, fmt.Errorf("source project cannot be the same as target: %s", srcID)
	}
	if _, err := qtx.GetProject(ctx, srcID); err != nil {
		return 0, fmt.Errorf("source project not found: %s", srcID)
	}

	if err := guardDurableOwnership(ctx, qtx, srcID); err != nil {
		return 0, err
	}

	res, err := qtx.MoveIssuesToProject(ctx, db.MoveIssuesToProjectParams{
		ProjectID:   targetID,
		ProjectID_2: srcID,
	})
	if err != nil {
		return 0, fmt.Errorf("move issues from %s: %w", srcID, err)
	}
	issues, _ = res.RowsAffected()

	if err := qtx.DeleteConfigByProject(ctx, srcID); err != nil {
		return 0, fmt.Errorf("delete config for %s: %w", srcID, err)
	}
	if err := qtx.DeleteProject(ctx, srcID); err != nil {
		return 0, fmt.Errorf("delete project %s: %w", srcID, err)
	}
	return issues, nil
}

// dbProjectToType converts a database project row to a types.Project.
// It maps nullable SQL fields to their Go equivalents.
func dbProjectToType(row *db.Project) *types.Project {
	return &types.Project{
		GovernanceGeneration: row.GovernanceGeneration,
		ID:                   row.ID,
		Name:                 row.Name,
		Description:          fromNullString(row.Description),
		Prefix:               row.Prefix,
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
	}
}

// Helper functions for nullable fields
func toNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func fromNullString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func toNullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

func fromNullTime(nt sql.NullTime) *time.Time {
	if nt.Valid {
		return &nt.Time
	}
	return nil
}

// Archived plans retain project ownership and cannot be erased by project operations.
func guardDurableOwnership(ctx context.Context, q *db.Queries, projectID string) error {
	ids, err := q.ListOwnedPlanIDs(ctx, projectID)
	if err != nil {
		return err
	}
	if len(ids) > 0 {
		return fmt.Errorf(
			"project owns retained durable plans %v; archive preserves ownership; "+
				"deletion and source merge are unavailable",
			ids,
		)
	}
	return nil
}

// mergeSourceIssueIDs preserves the source's unlinked contract. A legacy cross-
// project edge cannot silently become governing ancestry through a project merge.
// Historical pin records are also protected by the project ownership guard/FKs.
func (s *issueMutationTx) mergeSourceIssueIDs(ctx context.Context, projectID string) ([]string, error) {
	if err := guardDurableOwnership(ctx, s.queries, projectID); err != nil {
		return nil, err
	}
	graph, err := s.governanceGraph(ctx, projectID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(graph.issues))
	for id := range graph.issues {
		governing, err := graph.resolve(id)
		if err != nil {
			return nil, err
		}
		if governing != nil {
			return nil, fmt.Errorf("%w: source has governed work", storage.ErrGovernanceReconciliation)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// recordMerge keeps audit, index and membership generation atomic with ownership
// transfer. There is no post-commit best-effort work or partial-page enumeration.
func (s *issueMutationTx) recordMerge(
	ctx context.Context, targetID string, movedIssueIDs []string, actor string,
) error {
	for _, id := range movedIssueIDs {
		if err := s.rebuildFTSForIssue(ctx, id); err != nil {
			return err
		}
		value := "merged into " + targetID
		if err := s.recordEvent(ctx, id, types.EventMerged, actor, nil, &value); err != nil {
			return err
		}
	}
	if len(movedIssueIDs) > 0 {
		if _, err := s.tx.ExecContext(ctx,
			`UPDATE projects SET governance_generation=governance_generation+? WHERE id=?`,
			len(movedIssueIDs), targetID,
		); err != nil {
			return err
		}
	}
	return nil
}

// guardMergeAncestry rejects malformed cross-project hierarchy touching any
// merge participant. Incoming edges can change effective governance even when
// every source node is unlinked, and third-project descendants must not be moved
// indirectly to a different external ancestor. Non-governing blockers are valid.
func (s *issueMutationTx) guardMergeAncestry(ctx context.Context, targetID string, sourceIDs []string) error {
	for _, projectID := range append([]string{targetID}, sourceIDs...) {
		var childID, parentID string
		err := s.tx.QueryRowContext(ctx, `
 SELECT d.issue_id,d.depends_on_id
 FROM dependencies d
 JOIN issues child ON child.id=d.issue_id
 JOIN issues parent ON parent.id=d.depends_on_id
 WHERE d.type='parent-child' AND child.project_id<>parent.project_id
 AND (child.project_id=? OR parent.project_id=?)
 LIMIT 1`, projectID, projectID).Scan(&childID, &parentID)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		return fmt.Errorf("%w: cross-project parent-child edge %s -> %s",
			storage.ErrGovernanceReconciliation, childID, parentID)
	}
	return nil
}
