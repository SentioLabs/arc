package sqlite_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/sentiolabs/arc/internal/paste"
	"github.com/sentiolabs/arc/internal/paste/sqlite"
)

var _ paste.Storage = (*sqlite.Store)(nil)

func newTestStore(t *testing.T) *sqlite.Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlite.Apply(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return sqlite.New(db)
}

func TestCreateAndGetShare(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	share := paste.PasteShare{
		ID:        "abc12345",
		PlanBlob:  []byte{1, 2, 3},
		PlanIV:    []byte{4, 5, 6},
		SchemaVer: 1,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateShare(context.Background(), share, "tok"); err != nil {
		t.Fatalf("CreateShare: %v", err)
	}
	got, err := s.GetShare(context.Background(), "abc12345")
	if err != nil {
		t.Fatalf("GetShare: %v", err)
	}
	if got.ID != share.ID || string(got.PlanBlob) != string(share.PlanBlob) {
		t.Errorf("got %+v, want %+v", got, share)
	}
}

func TestVerifyEditToken(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()
	_ = s.CreateShare(context.Background(), paste.PasteShare{
		ID:        "x",
		PlanBlob:  []byte{0},
		PlanIV:    []byte{0},
		SchemaVer: 1,
		CreatedAt: now,
		UpdatedAt: now,
	}, "good")
	ok, err := s.VerifyEditToken(context.Background(), "x", "good")
	if err != nil || !ok {
		t.Errorf("expected good token to verify, got ok=%v err=%v", ok, err)
	}
	ok, _ = s.VerifyEditToken(context.Background(), "x", "bad")
	if ok {
		t.Error("expected bad token to fail verify")
	}
}

func TestUpdateSharePlanRequiresToken(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()
	_ = s.CreateShare(context.Background(), paste.PasteShare{
		ID:        "x",
		PlanBlob:  []byte{0},
		PlanIV:    []byte{0},
		SchemaVer: 1,
		CreatedAt: now,
		UpdatedAt: now,
	}, "good")
	err := s.UpdateSharePlan(context.Background(), "x", []byte{9}, []byte{8}, "bad")
	if err != paste.ErrInvalidEditToken {
		t.Errorf("expected ErrInvalidEditToken, got %v", err)
	}
}

func TestAppendAndListEvents(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()
	_ = s.CreateShare(context.Background(), paste.PasteShare{
		ID:        "x",
		PlanBlob:  []byte{0},
		PlanIV:    []byte{0},
		SchemaVer: 1,
		CreatedAt: now,
		UpdatedAt: now,
	}, "tok")
	_ = s.AppendEvent(context.Background(), paste.PasteEvent{
		ID:        "e1",
		ShareID:   "x",
		Blob:      []byte{1},
		IV:        []byte{1},
		CreatedAt: now,
	})
	_ = s.AppendEvent(context.Background(), paste.PasteEvent{
		ID:        "e2",
		ShareID:   "x",
		Blob:      []byte{2},
		IV:        []byte{2},
		CreatedAt: now.Add(time.Second),
	})
	events, err := s.ListEvents(context.Background(), "x")
	if err != nil || len(events) != 2 {
		t.Fatalf("expected 2 events, got %d (err=%v)", len(events), err)
	}
	if events[0].ID != "e1" {
		t.Errorf("expected ordering by created_at; first event was %s", events[0].ID)
	}
}
