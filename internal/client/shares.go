// Share keyring client methods — wrap the /api/v1/shares endpoints used by
// arc share commands to read/write the author's local keyring.
package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/sentiolabs/arc/internal/types"
)

// ErrShareNotFound is returned by GetShare when the keyring has no entry
// for the given ID. Callers can use errors.Is to branch on this.
var ErrShareNotFound = errors.New("share not found")

// ListShares returns all keyring entries (newest first).
func (c *Client) ListShares() ([]*types.Share, error) {
	resp, err := c.get("/api/v1/shares")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var shares []*types.Share
	if err := json.NewDecoder(resp.Body).Decode(&shares); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return shares, nil
}

// GetShare returns the keyring entry for the given share ID.
// Returns ErrShareNotFound if the ID is not in the keyring.
func (c *Client) GetShare(id string) (*types.Share, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/shares/"+id, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Actor", c.actor)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrShareNotFound
	}

	if err := c.checkError(resp); err != nil {
		return nil, err
	}

	var share types.Share
	if err := json.NewDecoder(resp.Body).Decode(&share); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &share, nil
}

// UpsertShare inserts or replaces a keyring entry. Returns the stored record.
func (c *Client) UpsertShare(share *types.Share) (*types.Share, error) {
	resp, err := c.post("/api/v1/shares", share)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var stored types.Share
	if err := json.NewDecoder(resp.Body).Decode(&stored); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &stored, nil
}

// DeleteShare removes a keyring entry. Idempotent: no error if absent.
func (c *Client) DeleteShare(id string) error {
	req, err := http.NewRequest("DELETE", c.baseURL+"/api/v1/shares/"+id, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Actor", c.actor)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// 204 No Content is expected for successful delete
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	// 404 Not Found is also acceptable (idempotent delete)
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}

	// Any other non-2xx status is an error
	if err := c.checkError(resp); err != nil {
		return err
	}

	return nil
}
