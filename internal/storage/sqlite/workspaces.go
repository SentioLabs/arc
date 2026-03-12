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

// CreateWorkspace creates a new workspace (directory path entry).
func (s *Store) CreateWorkspace(ctx context.Context, ws *types.Workspace) error {
	if err := ws.Validate(); err != nil {
		return fmt.Errorf("validate workspace: %w", err)
	}

	if ws.ID == "" {
		ws.ID = project.GenerateProjectID("ws", ws.Path)
	}

	now := time.Now()
	ws.CreatedAt = now
	ws.UpdatedAt = now

	if ws.PathType == "" {
		ws.PathType = "canonical"
	}

	err := s.queries.CreateWorkspace(ctx, db.CreateWorkspaceParams{
		ID:             ws.ID,
		ProjectID:      ws.ProjectID,
		Path:           ws.Path,
		Label:          toNullString(ws.Label),
		Hostname:       toNullString(ws.Hostname),
		GitRemote:      toNullString(ws.GitRemote),
		PathType:       ws.PathType,
		LastAccessedAt: toNullTime(ws.LastAccessedAt),
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	if err != nil {
		return fmt.Errorf("create workspace: %w", err)
	}

	return nil
}

// GetWorkspace retrieves a workspace by ID.
func (s *Store) GetWorkspace(ctx context.Context, id string) (*types.Workspace, error) {
	row, err := s.queries.GetWorkspace(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("workspace not found: %s", id)
		}
		return nil, fmt.Errorf("get workspace: %w", err)
	}

	return dbWorkspaceToType(row), nil
}

// ListWorkspaces returns all workspaces for a project.
func (s *Store) ListWorkspaces(ctx context.Context, projectID string) ([]*types.Workspace, error) {
	rows, err := s.queries.ListWorkspaces(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}

	result := make([]*types.Workspace, len(rows))
	for i, row := range rows {
		result[i] = dbWorkspaceToType(row)
	}

	return result, nil
}

// UpdateWorkspace updates a workspace entry.
func (s *Store) UpdateWorkspace(ctx context.Context, ws *types.Workspace) error {
	ws.UpdatedAt = time.Now()

	if ws.PathType == "" {
		ws.PathType = "canonical"
	}

	err := s.queries.UpdateWorkspace(ctx, db.UpdateWorkspaceParams{
		Label:     toNullString(ws.Label),
		Hostname:  toNullString(ws.Hostname),
		GitRemote: toNullString(ws.GitRemote),
		PathType:  ws.PathType,
		UpdatedAt: ws.UpdatedAt,
		ID:        ws.ID,
	})
	if err != nil {
		return fmt.Errorf("update workspace: %w", err)
	}

	return nil
}

// DeleteWorkspace removes a workspace entry.
func (s *Store) DeleteWorkspace(ctx context.Context, id string) error {
	err := s.queries.DeleteWorkspace(ctx, id)
	if err != nil {
		return fmt.Errorf("delete workspace: %w", err)
	}

	return nil
}

// ResolveProjectByPath finds a workspace entry by filesystem path.
func (s *Store) ResolveProjectByPath(ctx context.Context, path string) (*types.Workspace, error) {
	row, err := s.queries.ResolveProjectByPath(ctx, path)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("workspace not found for path: %s", path)
		}
		return nil, fmt.Errorf("resolve project by path: %w", err)
	}

	return dbWorkspaceToType(row), nil
}

// UpdateWorkspaceLastAccessed updates the last_accessed_at timestamp for a workspace.
func (s *Store) UpdateWorkspaceLastAccessed(ctx context.Context, id string) error {
	now := time.Now()

	err := s.queries.UpdateWorkspaceLastAccessed(ctx, db.UpdateWorkspaceLastAccessedParams{
		LastAccessedAt: toNullTime(&now),
		UpdatedAt:      now,
		ID:             id,
	})
	if err != nil {
		return fmt.Errorf("update workspace last accessed: %w", err)
	}

	return nil
}

// dbWorkspaceToType converts a database Workspace to a types.Workspace.
func dbWorkspaceToType(row *db.Workspace) *types.Workspace {
	return &types.Workspace{
		ID:             row.ID,
		ProjectID:      row.ProjectID,
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
