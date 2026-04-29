package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/sentiolabs/arc/internal/paste"
)

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) CreateShare(ctx context.Context, share paste.PasteShare, editToken string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO paste_shares (id, edit_token, plan_blob, plan_iv, schema_ver, created_at, updated_at, expires_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		share.ID, editToken, share.PlanBlob, share.PlanIV, share.SchemaVer,
		share.CreatedAt, share.UpdatedAt, share.ExpiresAt,
	)
	return err
}

func (s *Store) GetShare(ctx context.Context, id string) (*paste.PasteShare, error) {
	var sh paste.PasteShare
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

func (s *Store) AppendEvent(ctx context.Context, e paste.PasteEvent) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO paste_events (id, share_id, blob, iv, created_at) VALUES (?, ?, ?, ?, ?)`,
		e.ID, e.ShareID, e.Blob, e.IV, e.CreatedAt)
	return err
}

func (s *Store) ListEvents(ctx context.Context, shareID string) ([]paste.PasteEvent, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, share_id, blob, iv, created_at FROM paste_events
         WHERE share_id = ? ORDER BY created_at ASC, id ASC`, shareID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []paste.PasteEvent
	for rows.Next() {
		var e paste.PasteEvent
		if err := rows.Scan(&e.ID, &e.ShareID, &e.Blob, &e.IV, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

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
