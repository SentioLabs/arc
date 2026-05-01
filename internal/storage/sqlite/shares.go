// Package sqlite stub implementations for Share keyring methods.
// TODO(T1): replace stubs with real implementations backed by sqlc queries.
package sqlite

import (
	"context"
	"errors"

	"github.com/sentiolabs/arc/internal/types"
)

func (s *Store) UpsertShare(ctx context.Context, share *types.Share) error {
	return errors.New("UpsertShare not yet implemented")
}

func (s *Store) GetShare(ctx context.Context, id string) (*types.Share, error) {
	return nil, errors.New("GetShare not yet implemented")
}

func (s *Store) ListShares(ctx context.Context) ([]*types.Share, error) {
	return nil, errors.New("ListShares not yet implemented")
}

func (s *Store) DeleteShare(ctx context.Context, id string) error {
	return errors.New("DeleteShare not yet implemented")
}
