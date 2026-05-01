package api //nolint:testpackage // tests use internal helpers that access unexported fields

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/types"
)

// shareFixture returns a valid Share payload for use in tests.
func shareFixture() types.Share {
	return types.Share{
		ID:        "share-abc123",
		Kind:      types.ShareKindShared,
		URL:       "https://example.com/paste/abc123",
		KeyB64Url: "c2VjcmV0a2V5",
		EditToken: "tok-xyz",
		PlanFile:  "/tmp/plan.json",
	}
}

// doShareRequest sends a JSON request to the echo instance and returns the recorder.
func doShareRequest(e *echo.Echo, method, path string, body any) *httptest.ResponseRecorder {
	var bodyBytes []byte
	if body != nil {
		bodyBytes, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(bodyBytes))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// TestListSharesEmpty verifies GET /api/v1/shares returns 200 with empty list when no shares exist.
func TestListSharesEmpty(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()

	rec := doShareRequest(server.echo, http.MethodGet, "/api/v1/shares", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var shares []*types.Share
	if err := json.Unmarshal(rec.Body.Bytes(), &shares); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(shares) != 0 {
		t.Errorf("expected empty list, got %d shares", len(shares))
	}
}

// TestUpsertShareValid verifies POST /api/v1/shares with a valid payload returns 200 and the stored record.
func TestUpsertShareValid(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()

	share := shareFixture()
	rec := doShareRequest(server.echo, http.MethodPost, "/api/v1/shares", share)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var stored types.Share
	if err := json.Unmarshal(rec.Body.Bytes(), &stored); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if stored.ID != share.ID {
		t.Errorf("id = %q, want %q", stored.ID, share.ID)
	}
	if stored.Kind != share.Kind {
		t.Errorf("kind = %q, want %q", stored.Kind, share.Kind)
	}
	if stored.URL != share.URL {
		t.Errorf("url = %q, want %q", stored.URL, share.URL)
	}
}

// TestUpsertShareInvalidKind verifies POST /api/v1/shares with an invalid kind returns 400.
func TestUpsertShareInvalidKind(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()

	share := shareFixture()
	share.Kind = "bogus"
	rec := doShareRequest(server.echo, http.MethodPost, "/api/v1/shares", share)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestUpsertShareMissingRequiredFields verifies POST /api/v1/shares with missing required fields returns 400.
func TestUpsertShareMissingRequiredFields(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()

	// Missing URL, KeyB64Url, EditToken
	share := types.Share{
		ID:   "share-missing",
		Kind: types.ShareKindLocal,
	}
	rec := doShareRequest(server.echo, http.MethodPost, "/api/v1/shares", share)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestUpsertShareReplaces verifies POST /api/v1/shares with same ID replaces the existing share.
func TestUpsertShareReplaces(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()

	share := shareFixture()
	// First POST: create
	rec := doShareRequest(server.echo, http.MethodPost, "/api/v1/shares", share)
	if rec.Code != http.StatusOK {
		t.Fatalf("first upsert: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Second POST: replace with updated URL
	share.URL = "https://example.com/paste/replaced"
	rec = doShareRequest(server.echo, http.MethodPost, "/api/v1/shares", share)
	if rec.Code != http.StatusOK {
		t.Fatalf("second upsert: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify replacement via GET
	rec = doShareRequest(server.echo, http.MethodGet, "/api/v1/shares/"+share.ID, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET after upsert: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var stored types.Share
	if err := json.Unmarshal(rec.Body.Bytes(), &stored); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if stored.URL != "https://example.com/paste/replaced" {
		t.Errorf("url = %q, want %q", stored.URL, "https://example.com/paste/replaced")
	}
}

// TestGetShareFound verifies GET /api/v1/shares/:id returns 200 with the share.
func TestGetShareFound(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()

	share := shareFixture()
	// Insert
	rec := doShareRequest(server.echo, http.MethodPost, "/api/v1/shares", share)
	if rec.Code != http.StatusOK {
		t.Fatalf("upsert: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Fetch
	rec = doShareRequest(server.echo, http.MethodGet, "/api/v1/shares/"+share.ID, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var stored types.Share
	if err := json.Unmarshal(rec.Body.Bytes(), &stored); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if stored.ID != share.ID {
		t.Errorf("id = %q, want %q", stored.ID, share.ID)
	}
}

// TestGetShareNotFound verifies GET /api/v1/shares/:id returns 404 for unknown ID.
func TestGetShareNotFound(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()

	rec := doShareRequest(server.echo, http.MethodGet, "/api/v1/shares/nonexistent", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestDeleteShareExisting verifies DELETE /api/v1/shares/:id returns 204, then GET returns 404.
func TestDeleteShareExisting(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()

	share := shareFixture()
	// Insert
	rec := doShareRequest(server.echo, http.MethodPost, "/api/v1/shares", share)
	if rec.Code != http.StatusOK {
		t.Fatalf("upsert: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Delete
	rec = doShareRequest(server.echo, http.MethodDelete, "/api/v1/shares/"+share.ID, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify gone
	rec = doShareRequest(server.echo, http.MethodGet, "/api/v1/shares/"+share.ID, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("after delete, expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestDeleteShareIdempotent verifies DELETE /api/v1/shares/:id returns 204 even for unknown ID.
func TestDeleteShareIdempotent(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()

	rec := doShareRequest(server.echo, http.MethodDelete, "/api/v1/shares/does-not-exist", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 (idempotent), got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestListSharesNewestFirst verifies GET /api/v1/shares returns shares newest first after multiple POSTs.
func TestListSharesNewestFirst(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()

	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	type entry struct {
		id        string
		createdAt time.Time
	}
	entries := []entry{
		{"share-first", baseTime},
		{"share-second", baseTime.Add(time.Hour)},
		{"share-third", baseTime.Add(2 * time.Hour)},
	}
	for _, e := range entries {
		s := shareFixture()
		s.ID = e.id
		s.CreatedAt = e.createdAt
		rec := doShareRequest(server.echo, http.MethodPost, "/api/v1/shares", s)
		if rec.Code != http.StatusOK {
			t.Fatalf("upsert %s: expected 200, got %d: %s", e.id, rec.Code, rec.Body.String())
		}
	}

	rec := doShareRequest(server.echo, http.MethodGet, "/api/v1/shares", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var shares []*types.Share
	if err := json.Unmarshal(rec.Body.Bytes(), &shares); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(shares) != 3 {
		t.Fatalf("expected 3 shares, got %d", len(shares))
	}

	// Newest first means creation order reversed: share-third should be first
	if shares[0].ID != "share-third" {
		t.Errorf("first share = %q, want %q (newest first)", shares[0].ID, "share-third")
	}
	if shares[2].ID != "share-first" {
		t.Errorf("last share = %q, want %q (newest first)", shares[2].ID, "share-first")
	}
}
