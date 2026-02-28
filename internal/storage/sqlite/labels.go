// Package sqlite implements the storage interface using SQLite.
// This file handles label operations including label definitions and issue-label associations.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/sentiolabs/arc/internal/storage/sqlite/db"
	"github.com/sentiolabs/arc/internal/types"
)

// CreateLabel creates a new global label definition.
func (s *Store) CreateLabel(ctx context.Context, label *types.Label) error {
	err := s.queries.CreateLabel(ctx, db.CreateLabelParams{
		Name:        label.Name,
		Color:       toNullString(label.Color),
		Description: toNullString(label.Description),
	})
	if err != nil {
		return fmt.Errorf("create label: %w", err)
	}

	return nil
}

// GetLabel retrieves a label by name.
func (s *Store) GetLabel(ctx context.Context, name string) (*types.Label, error) {
	row, err := s.queries.GetLabel(ctx, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("label not found: %s", name)
		}
		return nil, fmt.Errorf("get label: %w", err)
	}

	return &types.Label{
		Name:        row.Name,
		Color:       fromNullString(row.Color),
		Description: fromNullString(row.Description),
	}, nil
}

// ListLabels returns all global labels.
func (s *Store) ListLabels(ctx context.Context) ([]*types.Label, error) {
	rows, err := s.queries.ListLabels(ctx)
	if err != nil {
		return nil, fmt.Errorf("list labels: %w", err)
	}

	labels := make([]*types.Label, len(rows))
	for i, row := range rows {
		labels[i] = &types.Label{
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
		Name:        label.Name,
	})
	if err != nil {
		return fmt.Errorf("update label: %w", err)
	}

	return nil
}

// DeleteLabel deletes a label.
func (s *Store) DeleteLabel(ctx context.Context, name string) error {
	err := s.queries.DeleteLabel(ctx, name)
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
	copy(labels, rows)

	return labels, nil
}

// GetLabelsForIssues fetches labels for multiple issues in a single query.
// Returns a map of issue_id -> []labels
func (s *Store) GetLabelsForIssues(ctx context.Context, issueIDs []string) (map[string][]string, error) {
	if len(issueIDs) == 0 {
		return make(map[string][]string), nil
	}

	// Build placeholders for IN clause
	placeholders := make([]any, len(issueIDs))
	marks := make([]string, len(issueIDs))
	for i, id := range issueIDs {
		placeholders[i] = id
		marks[i] = "?"
	}

	//nolint:gosec // G202: placeholders are parameterized; IN clause built from integer indices
	query := `SELECT issue_id, label FROM issue_labels WHERE issue_id IN (` +
		strings.Join(marks, ",") + `) ORDER BY issue_id, label`

	rows, err := s.db.QueryContext(ctx, query, placeholders...)
	if err != nil {
		return nil, fmt.Errorf("batch get labels: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]string)
	for rows.Next() {
		var issueID, label string
		if err := rows.Scan(&issueID, &label); err != nil {
			return nil, err
		}
		result[issueID] = append(result[issueID], label)
	}

	return result, nil
}
