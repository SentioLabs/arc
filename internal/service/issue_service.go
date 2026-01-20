package service

import (
	"context"
	"fmt"

	"github.com/sentiolabs/arc/internal/repository"
	"github.com/sentiolabs/arc/internal/types"
)

// IssueService handles issue business logic.
type IssueService struct {
	issues repository.IssueRepository
}

// NewIssueService creates a new issue service.
func NewIssueService(issues repository.IssueRepository) *IssueService {
	return &IssueService{issues: issues}
}

// CreateIssueInput contains the input for creating an issue.
type CreateIssueInput struct {
	WorkspaceID string
	Title       string
	Description string
	Status      *types.Status
	Priority    *int
	IssueType   *types.IssueType
	Assignee    string
	ExternalRef string
}

// Create creates a new issue.
func (s *IssueService) Create(ctx context.Context, input CreateIssueInput, actor string) (*types.Issue, error) {
	issue := &types.Issue{
		WorkspaceID: input.WorkspaceID,
		Title:       input.Title,
		Description: input.Description,
		Assignee:    input.Assignee,
		ExternalRef: input.ExternalRef,
	}

	// Apply optional fields
	if input.Status != nil {
		issue.Status = *input.Status
	}
	if input.Priority != nil {
		issue.Priority = *input.Priority
	}
	if input.IssueType != nil {
		issue.IssueType = *input.IssueType
	}

	// SetDefaults and Validate are called in repository

	if err := s.issues.Create(ctx, issue, actor); err != nil {
		return nil, err
	}

	return issue, nil
}

// Get retrieves an issue by ID.
func (s *IssueService) Get(ctx context.Context, id string) (*types.Issue, error) {
	return s.issues.Get(ctx, id)
}

// GetDetails retrieves an issue with full details.
func (s *IssueService) GetDetails(ctx context.Context, id string) (*types.IssueDetails, error) {
	return s.issues.GetDetails(ctx, id)
}

// GetByExternalRef retrieves an issue by external reference.
func (s *IssueService) GetByExternalRef(ctx context.Context, externalRef string) (*types.Issue, error) {
	return s.issues.GetByExternalRef(ctx, externalRef)
}

// List returns issues matching the filter.
func (s *IssueService) List(ctx context.Context, filter types.IssueFilter) ([]*types.Issue, error) {
	return s.issues.List(ctx, filter)
}

// UpdateIssueInput contains the input for updating an issue.
type UpdateIssueInput struct {
	Title       *string
	Description *string
	Status      *string
	Priority    *int
	IssueType   *string
	Assignee    *string
	ExternalRef *string
}

// Update updates an issue.
func (s *IssueService) Update(ctx context.Context, id string, input UpdateIssueInput, actor string) (*types.Issue, error) {
	updates := make(map[string]any)

	if input.Title != nil {
		updates["title"] = *input.Title
	}
	if input.Description != nil {
		updates["description"] = *input.Description
	}
	if input.Status != nil {
		// Validate status
		status := types.Status(*input.Status)
		if !status.IsValid() {
			return nil, fmt.Errorf("invalid status: %s", *input.Status)
		}
		updates["status"] = *input.Status
	}
	if input.Priority != nil {
		if *input.Priority < 0 || *input.Priority > 4 {
			return nil, fmt.Errorf("priority must be between 0 and 4")
		}
		updates["priority"] = *input.Priority
	}
	if input.IssueType != nil {
		// Validate issue type
		it := types.IssueType(*input.IssueType)
		if !it.IsValid() {
			return nil, fmt.Errorf("invalid issue type: %s", *input.IssueType)
		}
		updates["issue_type"] = *input.IssueType
	}
	if input.Assignee != nil {
		updates["assignee"] = *input.Assignee
	}
	if input.ExternalRef != nil {
		updates["external_ref"] = *input.ExternalRef
	}

	if len(updates) == 0 {
		return nil, fmt.Errorf("no updates provided")
	}

	if err := s.issues.Update(ctx, id, updates, actor); err != nil {
		return nil, err
	}

	return s.issues.Get(ctx, id)
}

// Delete deletes an issue.
func (s *IssueService) Delete(ctx context.Context, id string) error {
	return s.issues.Delete(ctx, id)
}

// Close closes an issue.
func (s *IssueService) Close(ctx context.Context, id string, reason string, actor string) (*types.Issue, error) {
	if err := s.issues.Close(ctx, id, reason, actor); err != nil {
		return nil, err
	}
	return s.issues.Get(ctx, id)
}

// Reopen reopens a closed issue.
func (s *IssueService) Reopen(ctx context.Context, id string, actor string) (*types.Issue, error) {
	if err := s.issues.Reopen(ctx, id, actor); err != nil {
		return nil, err
	}
	return s.issues.Get(ctx, id)
}

// GetReadyWork returns issues ready to work on.
func (s *IssueService) GetReadyWork(ctx context.Context, filter types.WorkFilter) ([]*types.Issue, error) {
	return s.issues.GetReadyWork(ctx, filter)
}

// GetBlockedIssues returns blocked issues.
func (s *IssueService) GetBlockedIssues(ctx context.Context, filter types.WorkFilter) ([]*types.BlockedIssue, error) {
	return s.issues.GetBlockedIssues(ctx, filter)
}

// IsBlocked checks if an issue is blocked.
func (s *IssueService) IsBlocked(ctx context.Context, issueID string) (bool, []string, error) {
	return s.issues.IsBlocked(ctx, issueID)
}
