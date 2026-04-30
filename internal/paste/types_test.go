package paste_test

import (
	"testing"

	"github.com/sentiolabs/arc/internal/paste"
)

func TestShareContract(t *testing.T) {
	var s paste.Share
	_ = s.ID
	_ = s.PlanBlob
	_ = s.PlanIV
	_ = s.SchemaVer
	_ = s.CreatedAt
	_ = s.UpdatedAt
	_ = s.ExpiresAt
}

func TestEventContract(t *testing.T) {
	var e paste.Event
	_ = e.ID
	_ = e.ShareID
	_ = e.Blob
	_ = e.IV
	_ = e.CreatedAt
}

func TestCreatePasteResponseContract(t *testing.T) {
	var r paste.CreatePasteResponse
	_ = r.ID
	_ = r.EditToken
}
