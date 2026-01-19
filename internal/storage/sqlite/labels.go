package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/sentiolabs/arc/internal/storage/sqlite/db"
	"github.com/sentiolabs/arc/internal/types"
)

// CreateLabel creates a new label definition.
func (s *Store) CreateLabel(ctx context.Context, label *types.Label) error {
	err := s.queries.CreateLabel(ctx, db.CreateLabelParams{
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

// GetLabel retrieves a label by workspace and name.
func (s *Store) GetLabel(ctx context.Context, workspaceID, name string) (*types.Label, error) {
	row, err := s.queries.GetLabel(ctx, db.GetLabelParams{
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

// ListLabels returns all labels for a workspace.
func (s *Store) ListLabels(ctx context.Context, workspaceID string) ([]*types.Label, error) {
	rows, err := s.queries.ListLabels(ctx, workspaceID)
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

// UpdateLabel updates a label.
func (s *Store) UpdateLabel(ctx context.Context, label *types.Label) error {
	err := s.queries.UpdateLabel(ctx, db.UpdateLabelParams{
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

// DeleteLabel deletes a label.
func (s *Store) DeleteLabel(ctx context.Context, workspaceID, name string) error {
	err := s.queries.DeleteLabel(ctx, db.DeleteLabelParams{
		WorkspaceID: workspaceID,
		Name:        name,
	})
	if err != nil {
		return fmt.Errorf("delete label: %w", err)
	}

	return nil
}

// AddLabelToIssue adds a label to an issue.
func (s *Store) AddLabelToIssue(ctx context.Context, issueID, label, actor string) error {
	err := s.queries.AddLabelToIssue(ctx, db.AddLabelToIssueParams{
		IssueID: issueID,
		Label:   label,
	})
	if err != nil {
		return fmt.Errorf("add label to issue: %w", err)
	}

	s.recordEvent(ctx, issueID, types.EventLabelAdded, actor, nil, &label)
	return nil
}

// RemoveLabelFromIssue removes a label from an issue.
func (s *Store) RemoveLabelFromIssue(ctx context.Context, issueID, label, actor string) error {
	err := s.queries.RemoveLabelFromIssue(ctx, db.RemoveLabelFromIssueParams{
		IssueID: issueID,
		Label:   label,
	})
	if err != nil {
		return fmt.Errorf("remove label from issue: %w", err)
	}

	s.recordEvent(ctx, issueID, types.EventLabelRemoved, actor, &label, nil)
	return nil
}

// GetIssueLabels returns all labels for an issue.
func (s *Store) GetIssueLabels(ctx context.Context, issueID string) ([]string, error) {
	rows, err := s.queries.GetIssueLabels(ctx, issueID)
	if err != nil {
		return nil, fmt.Errorf("get issue labels: %w", err)
	}

	labels := make([]string, len(rows))
	for i, row := range rows {
		labels[i] = row
	}

	return labels, nil
}
