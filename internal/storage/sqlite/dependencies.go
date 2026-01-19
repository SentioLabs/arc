package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/sentiolabs/arc/internal/storage/sqlite/db"
	"github.com/sentiolabs/arc/internal/types"
)

// AddDependency adds a dependency between two issues.
func (s *Store) AddDependency(ctx context.Context, dep *types.Dependency, actor string) error {
	if dep.IssueID == dep.DependsOnID {
		return fmt.Errorf("issue cannot depend on itself")
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
	s.recordEvent(ctx, dep.IssueID, types.EventDependencyAdded, actor, nil, &newVal)

	return nil
}

// RemoveDependency removes a dependency between two issues.
func (s *Store) RemoveDependency(ctx context.Context, issueID, dependsOnID string, actor string) error {
	err := s.queries.RemoveDependency(ctx, db.RemoveDependencyParams{
		IssueID:     issueID,
		DependsOnID: dependsOnID,
	})
	if err != nil {
		return fmt.Errorf("remove dependency: %w", err)
	}

	// Record event
	oldVal := fmt.Sprintf("%s no longer depends on %s", issueID, dependsOnID)
	s.recordEvent(ctx, issueID, types.EventDependencyRemoved, actor, &oldVal, nil)

	return nil
}

// GetDependencies returns the dependencies of an issue.
func (s *Store) GetDependencies(ctx context.Context, issueID string) ([]*types.Dependency, error) {
	rows, err := s.queries.GetDependencyRecords(ctx, issueID)
	if err != nil {
		return nil, fmt.Errorf("get dependencies: %w", err)
	}

	deps := make([]*types.Dependency, len(rows))
	for i, row := range rows {
		deps[i] = &types.Dependency{
			IssueID:     row.IssueID,
			DependsOnID: row.DependsOnID,
			Type:        types.DependencyType(row.Type),
			CreatedAt:   row.CreatedAt,
			CreatedBy:   fromNullString(row.CreatedBy),
		}
	}

	return deps, nil
}

// GetDependents returns issues that depend on the given issue.
func (s *Store) GetDependents(ctx context.Context, issueID string) ([]*types.Dependency, error) {
	rows, err := s.queries.GetDependentRecords(ctx, issueID)
	if err != nil {
		return nil, fmt.Errorf("get dependents: %w", err)
	}

	deps := make([]*types.Dependency, len(rows))
	for i, row := range rows {
		deps[i] = &types.Dependency{
			IssueID:     row.IssueID,
			DependsOnID: row.DependsOnID,
			Type:        types.DependencyType(row.Type),
			CreatedAt:   row.CreatedAt,
			CreatedBy:   fromNullString(row.CreatedBy),
		}
	}

	return deps, nil
}
