package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/sentiolabs/arc/internal/storage/sqlite/db"
	"github.com/sentiolabs/arc/internal/types"
	"github.com/sentiolabs/arc/internal/workspace"
)

// CreateWorkspace creates a new workspace.
func (s *Store) CreateWorkspace(ctx context.Context, ws *types.Workspace) error {
	if err := ws.Validate(); err != nil {
		return fmt.Errorf("validate workspace: %w", err)
	}

	// Generate ID if not provided
	if ws.ID == "" {
		ws.ID = workspace.GenerateWorkspaceID("ws", ws.Name)
	}

	now := time.Now()
	ws.CreatedAt = now
	ws.UpdatedAt = now

	err := s.queries.CreateWorkspace(ctx, db.CreateWorkspaceParams{
		ID:          ws.ID,
		Name:        ws.Name,
		Path:        toNullString(ws.Path),
		Description: toNullString(ws.Description),
		Prefix:      ws.Prefix,
		CreatedAt:   now,
		UpdatedAt:   now,
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
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("workspace not found: %s", id)
		}
		return nil, fmt.Errorf("get workspace: %w", err)
	}

	return dbWorkspaceToType(row), nil
}

// GetWorkspaceByName retrieves a workspace by name.
func (s *Store) GetWorkspaceByName(ctx context.Context, name string) (*types.Workspace, error) {
	row, err := s.queries.GetWorkspaceByName(ctx, name)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("workspace not found: %s", name)
		}
		return nil, fmt.Errorf("get workspace by name: %w", err)
	}

	return dbWorkspaceToType(row), nil
}

// GetWorkspaceByPath retrieves a workspace by its file system path.
func (s *Store) GetWorkspaceByPath(ctx context.Context, path string) (*types.Workspace, error) {
	row, err := s.queries.GetWorkspaceByPath(ctx, toNullString(path))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("workspace not found for path: %s", path)
		}
		return nil, fmt.Errorf("get workspace by path: %w", err)
	}

	return dbWorkspaceToType(row), nil
}

// ListWorkspaces returns all workspaces.
func (s *Store) ListWorkspaces(ctx context.Context) ([]*types.Workspace, error) {
	rows, err := s.queries.ListWorkspaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}

	workspaces := make([]*types.Workspace, len(rows))
	for i, row := range rows {
		workspaces[i] = dbWorkspaceToType(row)
	}

	return workspaces, nil
}

// UpdateWorkspace updates a workspace.
func (s *Store) UpdateWorkspace(ctx context.Context, ws *types.Workspace) error {
	ws.UpdatedAt = time.Now()

	err := s.queries.UpdateWorkspace(ctx, db.UpdateWorkspaceParams{
		Name:        ws.Name,
		Path:        toNullString(ws.Path),
		Description: toNullString(ws.Description),
		UpdatedAt:   ws.UpdatedAt,
		ID:          ws.ID,
	})
	if err != nil {
		return fmt.Errorf("update workspace: %w", err)
	}

	return nil
}

// DeleteWorkspace deletes a workspace and all its issues.
// Accepts either workspace ID (e.g., "ws-00blnw") or name (e.g., "my-project-a1b2c3").
func (s *Store) DeleteWorkspace(ctx context.Context, idOrName string) error {
	// Try to resolve by ID first
	ws, err := s.GetWorkspace(ctx, idOrName)
	if err != nil {
		// If not found by ID, try by name
		ws, err = s.GetWorkspaceByName(ctx, idOrName)
		if err != nil {
			return fmt.Errorf("workspace not found: %s", idOrName)
		}
	}

	// Delete by the resolved ID
	err = s.queries.DeleteWorkspace(ctx, ws.ID)
	if err != nil {
		return fmt.Errorf("delete workspace: %w", err)
	}

	return nil
}

// dbWorkspaceToType converts a database workspace to a types.Workspace.
func dbWorkspaceToType(row *db.Workspace) *types.Workspace {
	return &types.Workspace{
		ID:          row.ID,
		Name:        row.Name,
		Path:        fromNullString(row.Path),
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
