package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/sentiolabs/arc/internal/project"
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

	// Delete by the resolved ID
	err = s.queries.DeleteProject(ctx, p.ID)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}

	return nil
}

// MergeProjects moves all issues and plans from source projects into the
// target project, deletes the sources, and returns a summary. The entire
// operation runs inside a single transaction for atomicity.
func (s *Store) MergeProjects(
	ctx context.Context, targetID string, sourceIDs []string, actor string,
) (*types.MergeResult, error) {
	// Collect issue IDs from source projects before the transaction (for FTS rebuild + audit).
	// Must happen before BeginTx to avoid SQLite single-connection deadlock.
	var movedIssueIDs []string
	for _, srcID := range sourceIDs {
		srcIssues, err := s.ListIssues(ctx, types.IssueFilter{ProjectID: srcID})
		if err == nil {
			for _, issue := range srcIssues {
				movedIssueIDs = append(movedIssueIDs, issue.ID)
			}
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	qtx := s.queries.WithTx(tx)

	// Validate target exists
	if _, err := qtx.GetProject(ctx, targetID); err != nil {
		return nil, fmt.Errorf("target project not found: %s", targetID)
	}

	var totalIssues int64
	var deletedSources []string

	for _, srcID := range sourceIDs {
		issues, err := mergeOneSource(ctx, qtx, targetID, srcID)
		if err != nil {
			return nil, err
		}
		totalIssues += issues
		deletedSources = append(deletedSources, srcID)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit merge: %w", err)
	}

	// Best-effort post-commit work (outside transaction)
	for _, issueID := range movedIssueIDs {
		s.rebuildFTSForIssue(ctx, issueID)
		newValue := "merged into " + targetID
		s.recordEvent(ctx, issueID, types.EventMerged, actor, nil, &newValue)
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
		ID:          row.ID,
		Name:        row.Name,
		Description: fromNullString(row.Description),
		Prefix:      row.Prefix,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
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
