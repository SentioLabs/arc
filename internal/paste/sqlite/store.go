package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/sentiolabs/arc/internal/paste"
)

// Store is the SQLite-backed paste.Storage implementation.
type Store struct {
	db *sql.DB
}

// New wraps an open *sql.DB as a Store. The caller still owns the connection
// and is responsible for closing it.
func New(db *sql.DB) *Store { return &Store{db: db} }

// CreateShare inserts a new share row plus its edit token. The token is
// stored in the same row so the table is the source of truth for both the
// public ID and the bearer credential needed to mutate it later.
func (s *Store) CreateShare(ctx context.Context, share paste.Share, editToken string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO paste_shares (id, edit_token, plan_blob, plan_iv, schema_ver, created_at, updated_at, expires_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		share.ID, editToken, share.PlanBlob, share.PlanIV, share.SchemaVer,
		share.CreatedAt, share.UpdatedAt, share.ExpiresAt,
	)
	return err
}

// GetShare returns the share row by ID. The edit_token column is intentionally
// excluded from the SELECT — only VerifyEditToken consults it, so untrusted
// reads can't accidentally leak the token via an error or log.
func (s *Store) GetShare(ctx context.Context, id string) (*paste.Share, error) {
	var sh paste.Share
	var expires sql.NullTime
	err := s.db.QueryRowContext(ctx,
		`SELECT id, plan_blob, plan_iv, schema_ver, created_at, updated_at, expires_at
         FROM paste_shares WHERE id = ?`, id).
		Scan(&sh.ID, &sh.PlanBlob, &sh.PlanIV, &sh.SchemaVer, &sh.CreatedAt, &sh.UpdatedAt, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, paste.ErrShareNotFound
	}
	if err != nil {
		return nil, err
	}
	if expires.Valid {
		t := expires.Time
		sh.ExpiresAt = &t
	}
	return &sh, nil
}

// UpdateSharePlan replaces the encrypted plan blob & nonce after verifying
// the edit token. updated_at is bumped so clients can detect changes.
func (s *Store) UpdateSharePlan(ctx context.Context, id string, planBlob, iv []byte, editToken string) error {
	ok, err := s.VerifyEditToken(ctx, id, editToken)
	if err != nil {
		return err
	}
	if !ok {
		return paste.ErrInvalidEditToken
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE paste_shares SET plan_blob = ?, plan_iv = ?, updated_at = ? WHERE id = ?`,
		planBlob, iv, time.Now(), id)
	return err
}

// DeleteShare removes the share (and, via cascade, its event log) after
// verifying the edit token.
func (s *Store) DeleteShare(ctx context.Context, id, editToken string) error {
	ok, err := s.VerifyEditToken(ctx, id, editToken)
	if err != nil {
		return err
	}
	if !ok {
		return paste.ErrInvalidEditToken
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM paste_shares WHERE id = ?`, id)
	return err
}

// AppendEvent appends a new event row to the share's append-only log. Events
// are only ever inserted — never updated or deleted — so the entire history
// is reproducible by replay.
func (s *Store) AppendEvent(ctx context.Context, e paste.Event) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO paste_events (id, share_id, blob, iv, created_at) VALUES (?, ?, ?, ?, ?)`,
		e.ID, e.ShareID, e.Blob, e.IV, e.CreatedAt)
	return err
}

// ListEvents returns every event for the share, ordered by created_at then
// id so callers can replay state deterministically (Go map iteration is
// randomized, so the deterministic order has to come from the SQL side).
func (s *Store) ListEvents(ctx context.Context, shareID string) ([]paste.Event, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, share_id, blob, iv, created_at FROM paste_events
         WHERE share_id = ? ORDER BY created_at ASC, id ASC`, shareID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []paste.Event
	for rows.Next() {
		var e paste.Event
		if err := rows.Scan(&e.ID, &e.ShareID, &e.Blob, &e.IV, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// VerifyEditToken checks token against the stored edit_token for share id.
// Returns (false, ErrShareNotFound) when the share doesn't exist; (false, nil)
// when the share exists but the token doesn't match; (true, nil) on success.
func (s *Store) VerifyEditToken(ctx context.Context, id, token string) (bool, error) {
	var stored string
	err := s.db.QueryRowContext(ctx,
		`SELECT edit_token FROM paste_shares WHERE id = ?`, id).Scan(&stored)
	if errors.Is(err, sql.ErrNoRows) {
		return false, paste.ErrShareNotFound
	}
	if err != nil {
		return false, err
	}
	return stored == token, nil
}
