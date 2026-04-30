package paste

import (
	"context"
	"errors"
)

// Sentinel errors returned by Storage implementations. Handlers translate
// these into HTTP status codes; CLI clients pattern-match on them too.
var (
	// ErrShareNotFound is returned when no paste share exists with the given ID.
	ErrShareNotFound = errors.New("paste share not found")
	// ErrInvalidEditToken is returned when an edit/delete request's bearer
	// token doesn't match the share's stored edit token.
	ErrInvalidEditToken = errors.New("invalid edit token")
)

// Storage is the persistence interface backing the paste service. It captures
// the full lifecycle of a share (create, read, update plan, delete) plus the
// append-only event log used for review comments.
type Storage interface {
	CreateShare(ctx context.Context, s Share, editToken string) error
	GetShare(ctx context.Context, id string) (*Share, error)
	UpdateSharePlan(ctx context.Context, id string, planBlob, iv []byte, editToken string) error
	DeleteShare(ctx context.Context, id string, editToken string) error
	AppendEvent(ctx context.Context, e Event) error
	ListEvents(ctx context.Context, shareID string) ([]Event, error)
	VerifyEditToken(ctx context.Context, id, token string) (bool, error)
}
