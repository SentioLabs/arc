package service

import (
	"context"
	"fmt"

	"github.com/sentiolabs/arc/internal/repository"
	"github.com/sentiolabs/arc/internal/types"
)

// CommentService handles comment and event business logic.
type CommentService struct {
	comments repository.CommentRepository
	events   repository.EventRepository
}

// NewCommentService creates a new comment service.
func NewCommentService(comments repository.CommentRepository, events repository.EventRepository) *CommentService {
	return &CommentService{
		comments: comments,
		events:   events,
	}
}

// Add adds a comment to an issue.
func (s *CommentService) Add(ctx context.Context, issueID, author, text string) (*types.Comment, error) {
	if text == "" {
		return nil, fmt.Errorf("comment text is required")
	}
	return s.comments.Add(ctx, issueID, author, text)
}

// Get retrieves a comment by ID.
func (s *CommentService) Get(ctx context.Context, commentID int64) (*types.Comment, error) {
	return s.comments.Get(ctx, commentID)
}

// List returns all comments for an issue.
func (s *CommentService) List(ctx context.Context, issueID string) ([]*types.Comment, error) {
	return s.comments.List(ctx, issueID)
}

// Update updates a comment.
func (s *CommentService) Update(ctx context.Context, commentID int64, text string) error {
	if text == "" {
		return fmt.Errorf("comment text is required")
	}
	return s.comments.Update(ctx, commentID, text)
}

// Delete deletes a comment.
func (s *CommentService) Delete(ctx context.Context, commentID int64) error {
	return s.comments.Delete(ctx, commentID)
}

// GetEvents returns the event history for an issue.
func (s *CommentService) GetEvents(ctx context.Context, issueID string, limit int) ([]*types.Event, error) {
	return s.events.List(ctx, issueID, limit)
}
