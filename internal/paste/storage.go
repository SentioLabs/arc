package paste

import (
	"context"
	"errors"
)

var ErrShareNotFound = errors.New("paste share not found")
var ErrInvalidEditToken = errors.New("invalid edit token")

type Storage interface {
	CreateShare(ctx context.Context, s PasteShare, editToken string) error
	GetShare(ctx context.Context, id string) (*PasteShare, error)
	UpdateSharePlan(ctx context.Context, id string, planBlob, iv []byte, editToken string) error
	DeleteShare(ctx context.Context, id string, editToken string) error
	AppendEvent(ctx context.Context, e PasteEvent) error
	ListEvents(ctx context.Context, shareID string) ([]PasteEvent, error)
	VerifyEditToken(ctx context.Context, id, token string) (bool, error)
}
