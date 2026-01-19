package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/sentiolabs/arc/internal/storage/sqlite/db"
	"github.com/sentiolabs/arc/internal/types"
)

// AddComment adds a comment to an issue.
func (s *Store) AddComment(ctx context.Context, issueID, author, text string) (*types.Comment, error) {
	now := time.Now()
	result, err := s.queries.CreateComment(ctx, db.CreateCommentParams{
		IssueID:   issueID,
		Author:    author,
		Text:      text,
		CreatedAt: now,
	})
	if err != nil {
		return nil, fmt.Errorf("add comment: %w", err)
	}

	s.recordEvent(ctx, issueID, types.EventCommented, author, nil, &text)

	return &types.Comment{
		ID:        result.ID,
		IssueID:   result.IssueID,
		Author:    result.Author,
		Text:      result.Text,
		CreatedAt: result.CreatedAt,
	}, nil
}

// GetComments returns all comments for an issue.
func (s *Store) GetComments(ctx context.Context, issueID string) ([]*types.Comment, error) {
	rows, err := s.queries.ListComments(ctx, issueID)
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

// UpdateComment updates a comment's text.
func (s *Store) UpdateComment(ctx context.Context, commentID int64, text string) error {
	now := time.Now()
	err := s.queries.UpdateComment(ctx, db.UpdateCommentParams{
		Text:      text,
		UpdatedAt: toNullTime(&now),
		ID:        commentID,
	})
	if err != nil {
		return fmt.Errorf("update comment: %w", err)
	}

	return nil
}

// DeleteComment deletes a comment.
func (s *Store) DeleteComment(ctx context.Context, commentID int64) error {
	err := s.queries.DeleteComment(ctx, commentID)
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}

	return nil
}

// GetEvents returns the event history for an issue.
func (s *Store) GetEvents(ctx context.Context, issueID string, limit int) ([]*types.Event, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.queries.GetEvents(ctx, db.GetEventsParams{
		IssueID: issueID,
		Limit:   int64(limit),
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return []*types.Event{}, nil
		}
		return nil, fmt.Errorf("get events: %w", err)
	}

	events := make([]*types.Event, len(rows))
	for i, row := range rows {
		events[i] = &types.Event{
			ID:        row.ID,
			IssueID:   row.IssueID,
			EventType: types.EventType(row.EventType),
			Actor:     row.Actor,
			OldValue:  nullStringToPtr(row.OldValue),
			NewValue:  nullStringToPtr(row.NewValue),
			Comment:   nullStringToPtr(row.Comment),
			CreatedAt: row.CreatedAt,
		}
	}

	return events, nil
}

func nullStringToPtr(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
}
