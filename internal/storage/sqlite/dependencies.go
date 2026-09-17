// Package sqlite implements the storage interface using SQLite.
// This file handles dependency (relationship) operations between issues.
package sqlite

import (
	"context"
	"fmt"

	"github.com/sentiolabs/arc/internal/types"
)

// AddDependency adds a dependency between two issues.
// It validates that the issue does not depend on itself and that the dependency type is valid.

// RemoveDependency removes a dependency between two issues.

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

// GetOpenChildIssues returns open (non-closed) child issues of a given parent
// via parent-child dependencies.
func (s *Store) GetOpenChildIssues(ctx context.Context, parentID string) ([]*types.Issue, error) {
	rows, err := s.queries.GetOpenChildIssues(ctx, parentID)
	if err != nil {
		return nil, fmt.Errorf("get open child issues: %w", err)
	}

	issues := make([]*types.Issue, len(rows))
	for i, row := range rows {
		issues[i] = dbIssueToType(row)
	}

	return issues, nil
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

func (s *Store) AddDependency(ctx context.Context, dep *types.Dependency, actor string) error {
	return s.withIssueMutation(ctx, func(m *issueMutationTx) error {
		return m.AddDependency(ctx, dep, actor)
	})
}

func (s *Store) RemoveDependency(ctx context.Context, issueID, dependsOnID, actor string) error {
	return s.withIssueMutation(ctx, func(m *issueMutationTx) error {
		return m.RemoveDependency(ctx, issueID, dependsOnID, actor)
	})
}
