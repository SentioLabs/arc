package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/sentiolabs/arc/internal/storage/sqlite/db"
	"github.com/sentiolabs/arc/internal/types"
)

// LabelRepository implements repository.LabelRepository.
type LabelRepository struct {
	repo *Repository
}

// Create creates a new label definition.
func (r *LabelRepository) Create(ctx context.Context, label *types.Label) error {
	err := r.repo.queries.CreateLabel(ctx, db.CreateLabelParams{
		WorkspaceID: label.WorkspaceID,
		Name:        label.Name,
		Color:       toNullString(label.Color),
		Description: toNullString(label.Description),
	})
	if err != nil {
		return fmt.Errorf("create label: %w", err)
	}

	return nil
}

// Get retrieves a label by workspace and name.
func (r *LabelRepository) Get(ctx context.Context, workspaceID, name string) (*types.Label, error) {
	row, err := r.repo.queries.GetLabel(ctx, db.GetLabelParams{
		WorkspaceID: workspaceID,
		Name:        name,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("label not found: %s", name)
		}
		return nil, fmt.Errorf("get label: %w", err)
	}

	return &types.Label{
		WorkspaceID: row.WorkspaceID,
		Name:        row.Name,
		Color:       fromNullString(row.Color),
		Description: fromNullString(row.Description),
	}, nil
}

// List returns all labels for a workspace.
func (r *LabelRepository) List(ctx context.Context, workspaceID string) ([]*types.Label, error) {
	rows, err := r.repo.queries.ListLabels(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list labels: %w", err)
	}

	labels := make([]*types.Label, len(rows))
	for i, row := range rows {
		labels[i] = &types.Label{
			WorkspaceID: row.WorkspaceID,
			Name:        row.Name,
			Color:       fromNullString(row.Color),
			Description: fromNullString(row.Description),
		}
	}

	return labels, nil
}

// Update updates a label.
func (r *LabelRepository) Update(ctx context.Context, label *types.Label) error {
	err := r.repo.queries.UpdateLabel(ctx, db.UpdateLabelParams{
		Color:       toNullString(label.Color),
		Description: toNullString(label.Description),
		WorkspaceID: label.WorkspaceID,
		Name:        label.Name,
	})
	if err != nil {
		return fmt.Errorf("update label: %w", err)
	}

	return nil
}

// Delete deletes a label.
func (r *LabelRepository) Delete(ctx context.Context, workspaceID, name string) error {
	err := r.repo.queries.DeleteLabel(ctx, db.DeleteLabelParams{
		WorkspaceID: workspaceID,
		Name:        name,
	})
	if err != nil {
		return fmt.Errorf("delete label: %w", err)
	}

	return nil
}

// AddToIssue adds a label to an issue.
func (r *LabelRepository) AddToIssue(ctx context.Context, issueID, label, actor string) error {
	err := r.repo.queries.AddLabelToIssue(ctx, db.AddLabelToIssueParams{
		IssueID: issueID,
		Label:   label,
	})
	if err != nil {
		return fmt.Errorf("add label to issue: %w", err)
	}

	r.repo.events.Record(ctx, issueID, types.EventLabelAdded, actor, nil, &label)
	return nil
}

// RemoveFromIssue removes a label from an issue.
func (r *LabelRepository) RemoveFromIssue(ctx context.Context, issueID, label, actor string) error {
	err := r.repo.queries.RemoveLabelFromIssue(ctx, db.RemoveLabelFromIssueParams{
		IssueID: issueID,
		Label:   label,
	})
	if err != nil {
		return fmt.Errorf("remove label from issue: %w", err)
	}

	r.repo.events.Record(ctx, issueID, types.EventLabelRemoved, actor, &label, nil)
	return nil
}

// GetIssueLabels returns all labels for an issue.
func (r *LabelRepository) GetIssueLabels(ctx context.Context, issueID string) ([]string, error) {
	rows, err := r.repo.queries.GetIssueLabels(ctx, issueID)
	if err != nil {
		return nil, fmt.Errorf("get issue labels: %w", err)
	}

	labels := make([]string, len(rows))
	for i, row := range rows {
		labels[i] = row
	}

	return labels, nil
}
