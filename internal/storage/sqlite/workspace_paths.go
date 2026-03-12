package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/sentiolabs/arc/internal/storage/sqlite/db"
	"github.com/sentiolabs/arc/internal/types"
	"github.com/sentiolabs/arc/internal/workspace"
)

// CreateWorkspacePath creates a new workspace path entry.
func (s *Store) CreateWorkspacePath(ctx context.Context, wp *types.WorkspacePath) error {
	if err := wp.Validate(); err != nil {
		return fmt.Errorf("validate workspace path: %w", err)
	}

	if wp.ID == "" {
		wp.ID = workspace.GenerateWorkspaceID("wp", wp.Path)
	}

	now := time.Now()
	wp.CreatedAt = now
	wp.UpdatedAt = now

	if wp.PathType == "" {
		wp.PathType = "canonical"
	}

	err := s.queries.CreateWorkspacePath(ctx, db.CreateWorkspacePathParams{
		ID:             wp.ID,
		WorkspaceID:    wp.WorkspaceID,
		Path:           wp.Path,
		Label:          toNullString(wp.Label),
		Hostname:       toNullString(wp.Hostname),
		GitRemote:      toNullString(wp.GitRemote),
		PathType:       wp.PathType,
		LastAccessedAt: toNullTime(wp.LastAccessedAt),
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	if err != nil {
		return fmt.Errorf("create workspace path: %w", err)
	}

	return nil
}

// GetWorkspacePath retrieves a workspace path by ID.
func (s *Store) GetWorkspacePath(ctx context.Context, id string) (*types.WorkspacePath, error) {
	row, err := s.queries.GetWorkspacePath(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("workspace path not found: %s", id)
		}
		return nil, fmt.Errorf("get workspace path: %w", err)
	}

	return dbWorkspacePathToType(row), nil
}

// ListWorkspacePaths returns all paths for a workspace.
func (s *Store) ListWorkspacePaths(ctx context.Context, workspaceID string) ([]*types.WorkspacePath, error) {
	rows, err := s.queries.ListWorkspacePaths(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list workspace paths: %w", err)
	}

	result := make([]*types.WorkspacePath, len(rows))
	for i, row := range rows {
		result[i] = dbWorkspacePathToType(row)
	}

	return result, nil
}

// UpdateWorkspacePath updates a workspace path entry.
func (s *Store) UpdateWorkspacePath(ctx context.Context, wp *types.WorkspacePath) error {
	wp.UpdatedAt = time.Now()

	if wp.PathType == "" {
		wp.PathType = "canonical"
	}

	err := s.queries.UpdateWorkspacePath(ctx, db.UpdateWorkspacePathParams{
		Label:     toNullString(wp.Label),
		Hostname:  toNullString(wp.Hostname),
		GitRemote: toNullString(wp.GitRemote),
		PathType:  wp.PathType,
		UpdatedAt: wp.UpdatedAt,
		ID:        wp.ID,
	})
	if err != nil {
		return fmt.Errorf("update workspace path: %w", err)
	}

	return nil
}

// DeleteWorkspacePath removes a workspace path entry.
func (s *Store) DeleteWorkspacePath(ctx context.Context, id string) error {
	err := s.queries.DeleteWorkspacePath(ctx, id)
	if err != nil {
		return fmt.Errorf("delete workspace path: %w", err)
	}

	return nil
}

// ResolveWorkspaceByPath finds a workspace path entry by filesystem path.
func (s *Store) ResolveWorkspaceByPath(ctx context.Context, path string) (*types.WorkspacePath, error) {
	row, err := s.queries.ResolveWorkspaceByPath(ctx, path)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("workspace path not found for path: %s", path)
		}
		return nil, fmt.Errorf("resolve workspace by path: %w", err)
	}

	return dbWorkspacePathToType(row), nil
}

// UpdatePathLastAccessed updates the last_accessed_at timestamp for a workspace path.
func (s *Store) UpdatePathLastAccessed(ctx context.Context, id string) error {
	now := time.Now()

	err := s.queries.UpdatePathLastAccessed(ctx, db.UpdatePathLastAccessedParams{
		LastAccessedAt: toNullTime(&now),
		UpdatedAt:      now,
		ID:             id,
	})
	if err != nil {
		return fmt.Errorf("update path last accessed: %w", err)
	}

	return nil
}

// dbWorkspacePathToType converts a database WorkspacePath to a types.WorkspacePath.
func dbWorkspacePathToType(row *db.WorkspacePath) *types.WorkspacePath {
	return &types.WorkspacePath{
		ID:             row.ID,
		WorkspaceID:    row.WorkspaceID,
		Path:           row.Path,
		Label:          fromNullString(row.Label),
		Hostname:       fromNullString(row.Hostname),
		GitRemote:      fromNullString(row.GitRemote),
		PathType:       row.PathType,
		LastAccessedAt: fromNullTime(row.LastAccessedAt),
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}
