package paste

import (
	"testing"
	"time"
)

func TestPasteShareContract(t *testing.T) {
	var s PasteShare
	var _ string = s.ID
	var _ []byte = s.PlanBlob
	var _ []byte = s.PlanIV
	var _ int = s.SchemaVer
	var _ time.Time = s.CreatedAt
	var _ time.Time = s.UpdatedAt
	var _ *time.Time = s.ExpiresAt
}

func TestPasteEventContract(t *testing.T) {
	var e PasteEvent
	var _ string = e.ID
	var _ string = e.ShareID
	var _ []byte = e.Blob
	var _ []byte = e.IV
	var _ time.Time = e.CreatedAt
}

func TestCreatePasteResponseContract(t *testing.T) {
	var r CreatePasteResponse
	var _ string = r.ID
	var _ string = r.EditToken
}
