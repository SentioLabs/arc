package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/sentiolabs/beads-central/internal/storage/sqlite/db"
	"github.com/sentiolabs/beads-central/internal/types"
)

// CreateWorkspace creates a new workspace.
func (s *Store) CreateWorkspace(ctx context.Context, workspace *types.Workspace) error {
	if err := workspace.Validate(); err != nil {
		return fmt.Errorf("validate workspace: %w", err)
	}

	// Generate ID if not provided
	if workspace.ID == "" {
		workspace.ID = generateID("ws", workspace.Name)
	}

	now := time.Now()
	workspace.CreatedAt = now
	workspace.UpdatedAt = now

	err := s.queries.CreateWorkspace(ctx, db.CreateWorkspaceParams{
		ID:          workspace.ID,
		Name:        workspace.Name,
		Path:        toNullString(workspace.Path),
		Description: toNullString(workspace.Description),
		Prefix:      workspace.Prefix,
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
func (s *Store) UpdateWorkspace(ctx context.Context, workspace *types.Workspace) error {
	workspace.UpdatedAt = time.Now()

	err := s.queries.UpdateWorkspace(ctx, db.UpdateWorkspaceParams{
		Name:        workspace.Name,
		Path:        toNullString(workspace.Path),
		Description: toNullString(workspace.Description),
		UpdatedAt:   workspace.UpdatedAt,
		ID:          workspace.ID,
	})
	if err != nil {
		return fmt.Errorf("update workspace: %w", err)
	}

	return nil
}

// DeleteWorkspace deletes a workspace and all its issues.
func (s *Store) DeleteWorkspace(ctx context.Context, id string) error {
	err := s.queries.DeleteWorkspace(ctx, id)
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
