package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/sentiolabs/arc/internal/storage/sqlite/db"
	"github.com/sentiolabs/arc/internal/types"
)

// WorkspaceRepository implements repository.WorkspaceRepository.
type WorkspaceRepository struct {
	repo *Repository
}

// Create creates a new workspace.
func (r *WorkspaceRepository) Create(ctx context.Context, workspace *types.Workspace) error {
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

	err := r.repo.queries.CreateWorkspace(ctx, db.CreateWorkspaceParams{
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

// Get retrieves a workspace by ID.
func (r *WorkspaceRepository) Get(ctx context.Context, id string) (*types.Workspace, error) {
	row, err := r.repo.queries.GetWorkspace(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("workspace not found: %s", id)
		}
		return nil, fmt.Errorf("get workspace: %w", err)
	}

	return r.dbWorkspaceToType(row), nil
}

// GetByName retrieves a workspace by name.
func (r *WorkspaceRepository) GetByName(ctx context.Context, name string) (*types.Workspace, error) {
	row, err := r.repo.queries.GetWorkspaceByName(ctx, name)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("workspace not found: %s", name)
		}
		return nil, fmt.Errorf("get workspace by name: %w", err)
	}

	return r.dbWorkspaceToType(row), nil
}

// List returns all workspaces.
func (r *WorkspaceRepository) List(ctx context.Context) ([]*types.Workspace, error) {
	rows, err := r.repo.queries.ListWorkspaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}

	workspaces := make([]*types.Workspace, len(rows))
	for i, row := range rows {
		workspaces[i] = r.dbWorkspaceToType(row)
	}

	return workspaces, nil
}

// Update updates a workspace.
func (r *WorkspaceRepository) Update(ctx context.Context, workspace *types.Workspace) error {
	workspace.UpdatedAt = time.Now()

	err := r.repo.queries.UpdateWorkspace(ctx, db.UpdateWorkspaceParams{
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

// Delete deletes a workspace and all its issues.
func (r *WorkspaceRepository) Delete(ctx context.Context, id string) error {
	err := r.repo.queries.DeleteWorkspace(ctx, id)
	if err != nil {
		return fmt.Errorf("delete workspace: %w", err)
	}

	return nil
}

// GetStatistics returns aggregate statistics for a workspace.
func (r *WorkspaceRepository) GetStatistics(ctx context.Context, workspaceID string) (*types.Statistics, error) {
	stats, err := r.repo.queries.GetWorkspaceStats(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("get workspace stats: %w", err)
	}

	readyCount, err := r.repo.queries.GetReadyIssueCount(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("get ready issue count: %w", err)
	}

	avgLeadTime, err := r.repo.queries.GetAverageLeadTime(ctx, workspaceID)
	if err != nil && !strings.Contains(err.Error(), "no rows") {
		return nil, fmt.Errorf("get average lead time: %w", err)
	}

	return &types.Statistics{
		WorkspaceID:      workspaceID,
		TotalIssues:      int(stats.TotalIssues),
		OpenIssues:       int(stats.OpenIssues),
		InProgressIssues: int(stats.InProgressIssues),
		ClosedIssues:     int(stats.ClosedIssues),
		BlockedIssues:    int(stats.BlockedIssues),
		DeferredIssues:   int(stats.DeferredIssues),
		ReadyIssues:      int(readyCount),
		AvgLeadTimeHours: avgLeadTime.Float64,
	}, nil
}

// dbWorkspaceToType converts a database workspace to a types.Workspace.
func (r *WorkspaceRepository) dbWorkspaceToType(row *db.Workspace) *types.Workspace {
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
