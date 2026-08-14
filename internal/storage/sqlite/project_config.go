package sqlite

import (
	"context"
	"fmt"

	"github.com/sentiolabs/arc/internal/storage/sqlite/db"
)

// Per-project config is a generic key/value store scoped to a project.
// Keys are opaque here — callers own their meaning and validation.

// GetProjectConfig returns all per-project config rows as a key/value map.
// A project with no config rows yields an empty (non-nil) map.
func (s *Store) GetProjectConfig(ctx context.Context, projectID string) (map[string]string, error) {
	rows, err := s.queries.GetAllConfig(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("get project config: %w", err)
	}

	values := make(map[string]string, len(rows))
	for _, row := range rows {
		values[row.Key] = row.Value.String
	}
	return values, nil
}

// SetProjectConfig upserts a single per-project config key.
func (s *Store) SetProjectConfig(ctx context.Context, projectID, key, value string) error {
	err := s.queries.SetConfig(ctx, db.SetConfigParams{
		ProjectID: projectID,
		Key:       key,
		Value:     toNullString(value),
	})
	if err != nil {
		return fmt.Errorf("set project config: %w", err)
	}
	return nil
}

// DeleteProjectConfig removes a single per-project config key.
func (s *Store) DeleteProjectConfig(ctx context.Context, projectID, key string) error {
	err := s.queries.DeleteConfig(ctx, db.DeleteConfigParams{
		ProjectID: projectID,
		Key:       key,
	})
	if err != nil {
		return fmt.Errorf("delete project config: %w", err)
	}
	return nil
}
