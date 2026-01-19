// Package service provides business logic between API handlers and repositories.
package service

import (
	"context"
	"fmt"

	"github.com/sentiolabs/arc/internal/repository"
	"github.com/sentiolabs/arc/internal/types"
)

// WorkspaceService handles workspace business logic.
type WorkspaceService struct {
	workspaces repository.WorkspaceRepository
}

// NewWorkspaceService creates a new workspace service.
func NewWorkspaceService(workspaces repository.WorkspaceRepository) *WorkspaceService {
	return &WorkspaceService{workspaces: workspaces}
}

// Create creates a new workspace.
func (s *WorkspaceService) Create(ctx context.Context, name, path, description, prefix string) (*types.Workspace, error) {
	workspace := &types.Workspace{
		Name:        name,
		Path:        path,
		Description: description,
		Prefix:      prefix,
	}

	if err := workspace.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	if err := s.workspaces.Create(ctx, workspace); err != nil {
		return nil, err
	}

	return workspace, nil
}

// Get retrieves a workspace by ID.
func (s *WorkspaceService) Get(ctx context.Context, id string) (*types.Workspace, error) {
	return s.workspaces.Get(ctx, id)
}

// GetByName retrieves a workspace by name.
func (s *WorkspaceService) GetByName(ctx context.Context, name string) (*types.Workspace, error) {
	return s.workspaces.GetByName(ctx, name)
}

// List returns all workspaces.
func (s *WorkspaceService) List(ctx context.Context) ([]*types.Workspace, error) {
	return s.workspaces.List(ctx)
}

// Update updates a workspace.
func (s *WorkspaceService) Update(ctx context.Context, id string, name, path, description *string) (*types.Workspace, error) {
	workspace, err := s.workspaces.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if name != nil {
		workspace.Name = *name
	}
	if path != nil {
		workspace.Path = *path
	}
	if description != nil {
		workspace.Description = *description
	}

	if err := s.workspaces.Update(ctx, workspace); err != nil {
		return nil, err
	}

	return workspace, nil
}

// Delete deletes a workspace.
func (s *WorkspaceService) Delete(ctx context.Context, id string) error {
	return s.workspaces.Delete(ctx, id)
}

// GetStatistics returns workspace statistics.
func (s *WorkspaceService) GetStatistics(ctx context.Context, workspaceID string) (*types.Statistics, error) {
	return s.workspaces.GetStatistics(ctx, workspaceID)
}
