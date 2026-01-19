package service

import (
	"context"
	"fmt"

	"github.com/sentiolabs/arc/internal/repository"
	"github.com/sentiolabs/arc/internal/types"
)

// LabelService handles label business logic.
type LabelService struct {
	labels repository.LabelRepository
}

// NewLabelService creates a new label service.
func NewLabelService(labels repository.LabelRepository) *LabelService {
	return &LabelService{labels: labels}
}

// Create creates a new label.
func (s *LabelService) Create(ctx context.Context, workspaceID, name, color, description string) (*types.Label, error) {
	if name == "" {
		return nil, fmt.Errorf("label name is required")
	}

	label := &types.Label{
		WorkspaceID: workspaceID,
		Name:        name,
		Color:       color,
		Description: description,
	}

	if err := s.labels.Create(ctx, label); err != nil {
		return nil, err
	}

	return label, nil
}

// Get retrieves a label by name.
func (s *LabelService) Get(ctx context.Context, workspaceID, name string) (*types.Label, error) {
	return s.labels.Get(ctx, workspaceID, name)
}

// List returns all labels for a workspace.
func (s *LabelService) List(ctx context.Context, workspaceID string) ([]*types.Label, error) {
	return s.labels.List(ctx, workspaceID)
}

// Update updates a label.
func (s *LabelService) Update(ctx context.Context, workspaceID, name string, color, description *string) (*types.Label, error) {
	label, err := s.labels.Get(ctx, workspaceID, name)
	if err != nil {
		return nil, err
	}

	if color != nil {
		label.Color = *color
	}
	if description != nil {
		label.Description = *description
	}

	if err := s.labels.Update(ctx, label); err != nil {
		return nil, err
	}

	return label, nil
}

// Delete deletes a label.
func (s *LabelService) Delete(ctx context.Context, workspaceID, name string) error {
	return s.labels.Delete(ctx, workspaceID, name)
}

// AddToIssue adds a label to an issue.
func (s *LabelService) AddToIssue(ctx context.Context, issueID, label, actor string) error {
	return s.labels.AddToIssue(ctx, issueID, label, actor)
}

// RemoveFromIssue removes a label from an issue.
func (s *LabelService) RemoveFromIssue(ctx context.Context, issueID, label, actor string) error {
	return s.labels.RemoveFromIssue(ctx, issueID, label, actor)
}

// GetIssueLabels returns all labels for an issue.
func (s *LabelService) GetIssueLabels(ctx context.Context, issueID string) ([]string, error) {
	return s.labels.GetIssueLabels(ctx, issueID)
}
