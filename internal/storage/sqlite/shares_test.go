package sqlite_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/types"
)

func makeTestShare(id string) *types.Share {
	return &types.Share{
		ID:        id,
		Kind:      types.ShareKindLocal,
		URL:       "https://example.com/paste/" + id,
		KeyB64Url: "dGVzdGtleQ==",
		EditToken: "edit-token-" + id,
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
}

func TestUpsertShare_Insert(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	share := makeTestShare("share-insert-1")

	err := store.UpsertShare(ctx, share)
	if err != nil {
		t.Fatalf("UpsertShare() error = %v", err)
	}

	got, err := store.GetShare(ctx, share.ID)
	if err != nil {
		t.Fatalf("GetShare() after insert error = %v", err)
	}

	if got.ID != share.ID {
		t.Errorf("ID = %q, want %q", got.ID, share.ID)
	}
	if got.Kind != share.Kind {
		t.Errorf("Kind = %q, want %q", got.Kind, share.Kind)
	}
	if got.URL != share.URL {
		t.Errorf("URL = %q, want %q", got.URL, share.URL)
	}
	if got.KeyB64Url != share.KeyB64Url {
		t.Errorf("KeyB64Url = %q, want %q", got.KeyB64Url, share.KeyB64Url)
	}
	if got.EditToken != share.EditToken {
		t.Errorf("EditToken = %q, want %q", got.EditToken, share.EditToken)
	}
	if got.PlanFile != share.PlanFile {
		t.Errorf("PlanFile = %q, want %q", got.PlanFile, share.PlanFile)
	}
	if !got.CreatedAt.Equal(share.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, share.CreatedAt)
	}
}

func TestUpsertShare_Replace(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	share := makeTestShare("share-replace-1")

	// Insert first
	if err := store.UpsertShare(ctx, share); err != nil {
		t.Fatalf("UpsertShare() first insert error = %v", err)
	}

	// Update the same ID with different fields
	updated := &types.Share{
		ID:        share.ID,
		Kind:      types.ShareKindShared,
		URL:       "https://example.com/paste/updated",
		KeyB64Url: "dXBkYXRlZGtleQ==",
		EditToken: "updated-edit-token",
		PlanFile:  "/path/to/plan.md",
		CreatedAt: share.CreatedAt.Add(time.Minute),
	}
	if err := store.UpsertShare(ctx, updated); err != nil {
		t.Fatalf("UpsertShare() second upsert error = %v", err)
	}

	// GetShare should return the second version
	got, err := store.GetShare(ctx, share.ID)
	if err != nil {
		t.Fatalf("GetShare() after upsert error = %v", err)
	}

	if got.Kind != updated.Kind {
		t.Errorf("Kind = %q, want %q", got.Kind, updated.Kind)
	}
	if got.URL != updated.URL {
		t.Errorf("URL = %q, want %q", got.URL, updated.URL)
	}
	if got.KeyB64Url != updated.KeyB64Url {
		t.Errorf("KeyB64Url = %q, want %q", got.KeyB64Url, updated.KeyB64Url)
	}
	if got.EditToken != updated.EditToken {
		t.Errorf("EditToken = %q, want %q", got.EditToken, updated.EditToken)
	}
	if got.PlanFile != updated.PlanFile {
		t.Errorf("PlanFile = %q, want %q", got.PlanFile, updated.PlanFile)
	}
}

func TestUpsertShare_ValidationError(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Empty ID should fail validation
	invalid := &types.Share{
		ID:        "",
		Kind:      types.ShareKindLocal,
		URL:       "https://example.com/paste/x",
		KeyB64Url: "dGVzdA==",
		EditToken: "tok",
	}
	err := store.UpsertShare(ctx, invalid)
	if err == nil {
		t.Fatal("UpsertShare() expected error for empty ID, got nil")
	}
}

func TestGetShare_Found(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	share := makeTestShare("share-get-1")

	if err := store.UpsertShare(ctx, share); err != nil {
		t.Fatalf("UpsertShare() error = %v", err)
	}

	got, err := store.GetShare(ctx, share.ID)
	if err != nil {
		t.Fatalf("GetShare() error = %v", err)
	}
	if got.ID != share.ID {
		t.Errorf("ID = %q, want %q", got.ID, share.ID)
	}
}

func TestGetShare_NotFound(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	_, err := store.GetShare(ctx, "nonexistent-share-id")
	if err == nil {
		t.Fatal("GetShare() expected error for missing ID, got nil")
	}
	if !errors.Is(err, storage.ErrShareNotFound) {
		t.Errorf("GetShare() error = %v, want errors.Is(err, storage.ErrShareNotFound) to be true", err)
	}
}

func TestListShares_OrderedByCreatedAtDesc(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	older := &types.Share{
		ID:        "share-older",
		Kind:      types.ShareKindLocal,
		URL:       "https://example.com/paste/older",
		KeyB64Url: "b2xkZXJrZXk=",
		EditToken: "edit-older",
		CreatedAt: baseTime,
	}
	newer := &types.Share{
		ID:        "share-newer",
		Kind:      types.ShareKindShared,
		URL:       "https://example.com/paste/newer",
		KeyB64Url: "bmV3ZXJrZXk=",
		EditToken: "edit-newer",
		CreatedAt: baseTime.Add(time.Hour),
	}

	// Insert older first, then newer
	if err := store.UpsertShare(ctx, older); err != nil {
		t.Fatalf("UpsertShare(older) error = %v", err)
	}
	if err := store.UpsertShare(ctx, newer); err != nil {
		t.Fatalf("UpsertShare(newer) error = %v", err)
	}

	list, err := store.ListShares(ctx)
	if err != nil {
		t.Fatalf("ListShares() error = %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListShares() returned %d items, want 2", len(list))
	}

	// Newest first
	if list[0].ID != newer.ID {
		t.Errorf("list[0].ID = %q, want %q (newest first)", list[0].ID, newer.ID)
	}
	if list[1].ID != older.ID {
		t.Errorf("list[1].ID = %q, want %q (oldest last)", list[1].ID, older.ID)
	}
}

func TestListShares_Empty(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	list, err := store.ListShares(ctx)
	if err != nil {
		t.Fatalf("ListShares() empty table error = %v", err)
	}
	if list == nil {
		t.Error("ListShares() returned nil slice, want empty non-nil slice")
	}
	if len(list) != 0 {
		t.Errorf("ListShares() returned %d items, want 0", len(list))
	}
}

func TestDeleteShare_Removes(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	share := makeTestShare("share-delete-1")

	if err := store.UpsertShare(ctx, share); err != nil {
		t.Fatalf("UpsertShare() error = %v", err)
	}

	// Verify it exists
	if _, err := store.GetShare(ctx, share.ID); err != nil {
		t.Fatalf("GetShare() before delete error = %v", err)
	}

	// Delete
	if err := store.DeleteShare(ctx, share.ID); err != nil {
		t.Fatalf("DeleteShare() error = %v", err)
	}

	// Verify it's gone
	_, err := store.GetShare(ctx, share.ID)
	if !errors.Is(err, storage.ErrShareNotFound) {
		t.Errorf("GetShare() after delete: got %v, want storage.ErrShareNotFound", err)
	}
}

func TestDeleteShare_Idempotent(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Delete an ID that never existed — should not error
	err := store.DeleteShare(ctx, "nonexistent-share-id")
	if err != nil {
		t.Errorf("DeleteShare() on missing ID error = %v, want nil", err)
	}
}
