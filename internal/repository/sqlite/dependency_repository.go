package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/sentiolabs/arc/internal/storage/sqlite/db"
	"github.com/sentiolabs/arc/internal/types"
)

// DependencyRepository implements repository.DependencyRepository.
type DependencyRepository struct {
	repo *Repository
}

// Add adds a dependency between two issues.
func (r *DependencyRepository) Add(ctx context.Context, dep *types.Dependency, actor string) error {
	if dep.IssueID == dep.DependsOnID {
		return fmt.Errorf("issue cannot depend on itself")
	}

	if !dep.Type.IsValid() {
		return fmt.Errorf("invalid dependency type: %s", dep.Type)
	}

	now := time.Now()
	dep.CreatedAt = now
	dep.CreatedBy = actor

	err := r.repo.queries.AddDependency(ctx, db.AddDependencyParams{
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
	r.repo.events.Record(ctx, dep.IssueID, types.EventDependencyAdded, actor, nil, &newVal)

	return nil
}

// Remove removes a dependency between two issues.
func (r *DependencyRepository) Remove(ctx context.Context, issueID, dependsOnID string, actor string) error {
	err := r.repo.queries.RemoveDependency(ctx, db.RemoveDependencyParams{
		IssueID:     issueID,
		DependsOnID: dependsOnID,
	})
	if err != nil {
		return fmt.Errorf("remove dependency: %w", err)
	}

	// Record event
	oldVal := fmt.Sprintf("%s no longer depends on %s", issueID, dependsOnID)
	r.repo.events.Record(ctx, issueID, types.EventDependencyRemoved, actor, &oldVal, nil)

	return nil
}

// GetForIssue returns the dependencies of an issue.
func (r *DependencyRepository) GetForIssue(ctx context.Context, issueID string) ([]*types.Dependency, error) {
	rows, err := r.repo.queries.GetDependencyRecords(ctx, issueID)
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
func (r *DependencyRepository) GetDependents(ctx context.Context, issueID string) ([]*types.Dependency, error) {
	rows, err := r.repo.queries.GetDependentRecords(ctx, issueID)
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
