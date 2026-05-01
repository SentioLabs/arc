package client_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/types"
)

func TestListShares(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	// Initially empty
	shares, err := c.ListShares()
	require.NoError(t, err)
	assert.Empty(t, shares)
}

func TestUpsertShare(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	share := &types.Share{
		ID:        "share-123",
		Kind:      types.ShareKindShared,
		URL:       "https://example.com/share/abc",
		KeyB64Url: "key_b64url_value",
		EditToken: "token_value",
		PlanFile:  "plan.md",
	}

	stored, err := c.UpsertShare(share)
	require.NoError(t, err)
	assert.Equal(t, "share-123", stored.ID)
	assert.Equal(t, types.ShareKindShared, stored.Kind)
	assert.Equal(t, "https://example.com/share/abc", stored.URL)
	assert.Equal(t, "key_b64url_value", stored.KeyB64Url)
	assert.Equal(t, "token_value", stored.EditToken)
	assert.Equal(t, "plan.md", stored.PlanFile)
	// CreatedAt should be set by server
	assert.False(t, stored.CreatedAt.IsZero())
}

func TestGetShare(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	share := &types.Share{
		ID:        "share-456",
		Kind:      types.ShareKindLocal,
		URL:       "https://example.com/share/def",
		KeyB64Url: "key_b64url_value",
		EditToken: "token_value",
	}

	_, err := c.UpsertShare(share)
	require.NoError(t, err)

	retrieved, err := c.GetShare("share-456")
	require.NoError(t, err)
	assert.Equal(t, "share-456", retrieved.ID)
	assert.Equal(t, types.ShareKindLocal, retrieved.Kind)
	assert.Equal(t, "https://example.com/share/def", retrieved.URL)
}

func TestGetShareNotFound(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	_, err := c.GetShare("nonexistent-id")
	require.Error(t, err)
	assert.True(t, errors.Is(err, client.ErrShareNotFound))
}

func TestDeleteShare(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	share := &types.Share{
		ID:        "share-789",
		Kind:      types.ShareKindShared,
		URL:       "https://example.com/share/ghi",
		KeyB64Url: "key_b64url_value",
		EditToken: "token_value",
	}

	_, err := c.UpsertShare(share)
	require.NoError(t, err)

	err = c.DeleteShare("share-789")
	require.NoError(t, err)

	// Verify it's deleted
	_, err = c.GetShare("share-789")
	require.Error(t, err)
	assert.True(t, errors.Is(err, client.ErrShareNotFound))
}

func TestDeleteShareIdempotent(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	// Delete nonexistent share should not error (idempotent)
	err := c.DeleteShare("nonexistent-id")
	require.NoError(t, err)
}

func TestListSharesMultiple(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	share1 := &types.Share{
		ID:        "share-1",
		Kind:      types.ShareKindLocal,
		URL:       "https://example.com/1",
		KeyB64Url: "key1",
		EditToken: "token1",
	}

	share2 := &types.Share{
		ID:        "share-2",
		Kind:      types.ShareKindShared,
		URL:       "https://example.com/2",
		KeyB64Url: "key2",
		EditToken: "token2",
	}

	_, err := c.UpsertShare(share1)
	require.NoError(t, err)

	_, err = c.UpsertShare(share2)
	require.NoError(t, err)

	shares, err := c.ListShares()
	require.NoError(t, err)
	assert.Len(t, shares, 2)
}
