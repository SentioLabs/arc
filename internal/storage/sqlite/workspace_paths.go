package sqlite

import (
	"context"

	"github.com/sentiolabs/arc/internal/types"
)

// CreateWorkspacePath creates a new workspace path entry.
func (s *Store) CreateWorkspacePath(_ context.Context, _ *types.WorkspacePath) error {
	panic("not implemented")
}

// GetWorkspacePath retrieves a workspace path by ID.
func (s *Store) GetWorkspacePath(_ context.Context, _ string) (*types.WorkspacePath, error) {
	panic("not implemented")
}

// ListWorkspacePaths returns all paths for a workspace.
func (s *Store) ListWorkspacePaths(_ context.Context, _ string) ([]*types.WorkspacePath, error) {
	panic("not implemented")
}

// UpdateWorkspacePath updates a workspace path entry.
func (s *Store) UpdateWorkspacePath(_ context.Context, _ *types.WorkspacePath) error {
	panic("not implemented")
}

// DeleteWorkspacePath removes a workspace path entry.
func (s *Store) DeleteWorkspacePath(_ context.Context, _ string) error {
	panic("not implemented")
}

// ResolveWorkspaceByPath finds a workspace path entry by filesystem path.
func (s *Store) ResolveWorkspaceByPath(_ context.Context, _ string) (*types.WorkspacePath, error) {
	panic("not implemented")
}

// UpdatePathLastAccessed updates the last_accessed_at timestamp for a workspace path.
func (s *Store) UpdatePathLastAccessed(_ context.Context, _ string) error {
	panic("not implemented")
}
