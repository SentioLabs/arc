// Package sqlite implements the storage interface using SQLite.
// This file handles share keyring operations.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/storage/sqlite/db"
	"github.com/sentiolabs/arc/internal/types"
)

// UpsertShare inserts or replaces a share record. Stamps CreatedAt to now
// when callers omit it, so HTTP handlers and the legacy import path don't
// each need their own default.
func (s *Store) UpsertShare(ctx context.Context, share *types.Share) error {
	return s.upsertShareWith(ctx, s.queries, share)
}

// UpsertShares atomically upserts a batch of shares in a single transaction.
// All-or-nothing: a validation or constraint failure on any entry rolls the
// whole batch back so callers can fix the bad entry and retry without first
// having to clean up partial state.
func (s *Store) UpsertShares(ctx context.Context, shares []*types.Share) error {
	if len(shares) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	qtx := s.queries.WithTx(tx)
	for _, share := range shares {
		if err := s.upsertShareWith(ctx, qtx, share); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// upsertShareWith is the shared body of UpsertShare and UpsertShares. The qtx
// argument lets the caller pick between the bare queries and a transactional
// view (queries.WithTx).
func (s *Store) upsertShareWith(ctx context.Context, qtx *db.Queries, share *types.Share) error {
	if share.CreatedAt.IsZero() {
		share.CreatedAt = time.Now().UTC()
	}
	if err := share.Validate(); err != nil {
		return fmt.Errorf("upsert share: %w", err)
	}
	err := qtx.UpsertShare(ctx, db.UpsertShareParams{
		ID:        share.ID,
		Kind:      string(share.Kind),
		Url:       share.URL,
		KeyB64url: share.KeyB64Url,
		EditToken: share.EditToken,
		PlanFile:  toNullString(share.PlanFile),
		CreatedAt: share.CreatedAt.UTC(),
	})
	if err != nil {
		return fmt.Errorf("upsert share: %w", err)
	}
	return nil
}

// GetShare retrieves a share by ID.
// Returns storage.ErrShareNotFound if the ID does not exist.
func (s *Store) GetShare(ctx context.Context, id string) (*types.Share, error) {
	row, err := s.queries.GetShare(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrShareNotFound
		}
		return nil, fmt.Errorf("get share: %w", err)
	}
	return rowToShare(row), nil
}

// ListShares returns all shares ordered by created_at DESC (newest first).
func (s *Store) ListShares(ctx context.Context) ([]*types.Share, error) {
	rows, err := s.queries.ListShares(ctx)
	if err != nil {
		return nil, fmt.Errorf("list shares: %w", err)
	}
	out := make([]*types.Share, len(rows))
	for i, r := range rows {
		out[i] = rowToShare(r)
	}
	return out, nil
}

// DeleteShare removes a share by ID.
// Idempotent: no error is returned if the ID does not exist.
func (s *Store) DeleteShare(ctx context.Context, id string) error {
	if err := s.queries.DeleteShare(ctx, id); err != nil {
		return fmt.Errorf("delete share: %w", err)
	}
	return nil
}

// rowToShare converts a db.Share row to a types.Share.
func rowToShare(r *db.Share) *types.Share {
	return &types.Share{
		ID:        r.ID,
		Kind:      types.ShareKind(r.Kind),
		URL:       r.Url,
		KeyB64Url: r.KeyB64url,
		EditToken: r.EditToken,
		PlanFile:  fromNullString(r.PlanFile),
		CreatedAt: r.CreatedAt.UTC(),
	}
}
