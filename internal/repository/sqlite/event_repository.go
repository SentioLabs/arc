package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/sentiolabs/arc/internal/storage/sqlite/db"
	"github.com/sentiolabs/arc/internal/types"
)

// EventRepository implements repository.EventRepository.
type EventRepository struct {
	repo *Repository
}

// List returns the event history for an issue.
func (r *EventRepository) List(ctx context.Context, issueID string, limit int) ([]*types.Event, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := r.repo.queries.GetEvents(ctx, db.GetEventsParams{
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

// Record records an event in the audit trail.
func (r *EventRepository) Record(ctx context.Context, issueID string, eventType types.EventType, actor string, oldValue, newValue *string) error {
	err := r.repo.queries.CreateEvent(ctx, db.CreateEventParams{
		IssueID:   issueID,
		EventType: string(eventType),
		Actor:     actor,
		OldValue:  toNullString(ptrToString(oldValue)),
		NewValue:  toNullString(ptrToString(newValue)),
		CreatedAt: time.Now(),
	})
	if err != nil {
		return fmt.Errorf("record event: %w", err)
	}
	return nil
}
