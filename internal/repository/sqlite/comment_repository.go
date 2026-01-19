package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/sentiolabs/arc/internal/storage/sqlite/db"
	"github.com/sentiolabs/arc/internal/types"
)

// CommentRepository implements repository.CommentRepository.
type CommentRepository struct {
	repo *Repository
}

// Add adds a comment to an issue.
func (r *CommentRepository) Add(ctx context.Context, issueID, author, text string) (*types.Comment, error) {
	now := time.Now()
	result, err := r.repo.queries.CreateComment(ctx, db.CreateCommentParams{
		IssueID:   issueID,
		Author:    author,
		Text:      text,
		CreatedAt: now,
	})
	if err != nil {
		return nil, fmt.Errorf("add comment: %w", err)
	}

	r.repo.events.Record(ctx, issueID, types.EventCommented, author, nil, &text)

	return &types.Comment{
		ID:        result.ID,
		IssueID:   result.IssueID,
		Author:    result.Author,
		Text:      result.Text,
		CreatedAt: result.CreatedAt,
	}, nil
}

// Get retrieves a comment by ID.
func (r *CommentRepository) Get(ctx context.Context, commentID int64) (*types.Comment, error) {
	row, err := r.repo.queries.GetComment(ctx, commentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("comment not found: %d", commentID)
		}
		return nil, fmt.Errorf("get comment: %w", err)
	}

	var updatedAt time.Time
	if row.UpdatedAt.Valid {
		updatedAt = row.UpdatedAt.Time
	}

	return &types.Comment{
		ID:        row.ID,
		IssueID:   row.IssueID,
		Author:    row.Author,
		Text:      row.Text,
		CreatedAt: row.CreatedAt,
		UpdatedAt: updatedAt,
	}, nil
}

// List returns all comments for an issue.
func (r *CommentRepository) List(ctx context.Context, issueID string) ([]*types.Comment, error) {
	rows, err := r.repo.queries.ListComments(ctx, issueID)
	if err != nil {
		return nil, fmt.Errorf("get comments: %w", err)
	}

	comments := make([]*types.Comment, len(rows))
	for i, row := range rows {
		var updatedAt time.Time
		if row.UpdatedAt.Valid {
			updatedAt = row.UpdatedAt.Time
		}
		comments[i] = &types.Comment{
			ID:        row.ID,
			IssueID:   row.IssueID,
			Author:    row.Author,
			Text:      row.Text,
			CreatedAt: row.CreatedAt,
			UpdatedAt: updatedAt,
		}
	}

	return comments, nil
}

// Update updates a comment's text.
func (r *CommentRepository) Update(ctx context.Context, commentID int64, text string) error {
	now := time.Now()
	err := r.repo.queries.UpdateComment(ctx, db.UpdateCommentParams{
		Text:      text,
		UpdatedAt: toNullTime(&now),
		ID:        commentID,
	})
	if err != nil {
		return fmt.Errorf("update comment: %w", err)
	}

	return nil
}

// Delete deletes a comment.
func (r *CommentRepository) Delete(ctx context.Context, commentID int64) error {
	err := r.repo.queries.DeleteComment(ctx, commentID)
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}

	return nil
}
