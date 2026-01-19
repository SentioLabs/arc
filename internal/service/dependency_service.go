package service

import (
	"context"
	"fmt"

	"github.com/sentiolabs/arc/internal/repository"
	"github.com/sentiolabs/arc/internal/types"
)

// DependencyService handles dependency business logic.
type DependencyService struct {
	dependencies repository.DependencyRepository
}

// NewDependencyService creates a new dependency service.
func NewDependencyService(dependencies repository.DependencyRepository) *DependencyService {
	return &DependencyService{dependencies: dependencies}
}

// Add adds a dependency between issues.
func (s *DependencyService) Add(ctx context.Context, issueID, dependsOnID string, depType types.DependencyType, actor string) (*types.Dependency, error) {
	if !depType.IsValid() {
		return nil, fmt.Errorf("invalid dependency type: %s", depType)
	}

	dep := &types.Dependency{
		IssueID:     issueID,
		DependsOnID: dependsOnID,
		Type:        depType,
	}

	if err := s.dependencies.Add(ctx, dep, actor); err != nil {
		return nil, err
	}

	return dep, nil
}

// Remove removes a dependency between issues.
func (s *DependencyService) Remove(ctx context.Context, issueID, dependsOnID string, actor string) error {
	return s.dependencies.Remove(ctx, issueID, dependsOnID, actor)
}

// DependencyGraph contains dependencies and dependents for an issue.
type DependencyGraph struct {
	Dependencies []*types.Dependency `json:"dependencies"`
	Dependents   []*types.Dependency `json:"dependents"`
}

// GetGraph returns the dependency graph for an issue.
func (s *DependencyService) GetGraph(ctx context.Context, issueID string) (*DependencyGraph, error) {
	deps, err := s.dependencies.GetForIssue(ctx, issueID)
	if err != nil {
		return nil, err
	}

	dependents, err := s.dependencies.GetDependents(ctx, issueID)
	if err != nil {
		return nil, err
	}

	return &DependencyGraph{
		Dependencies: deps,
		Dependents:   dependents,
	}, nil
}
