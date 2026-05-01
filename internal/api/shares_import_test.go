package api //nolint:testpackage // tests use internal helpers that access unexported fields

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sentiolabs/arc/internal/types"
)

// TestImportLegacySharesJSONNoFile verifies that when the JSON file doesn't exist,
// the function returns (0, nil) with no side effects.
func TestImportLegacySharesJSONNoFile(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()

	tmpDir := t.TempDir()
	nonexistentPath := filepath.Join(tmpDir, "nonexistent.json")

	count, err := server.ImportLegacySharesJSON(context.Background(), nonexistentPath)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}

	// Verify file still doesn't exist
	if _, err := os.Stat(nonexistentPath); err == nil {
		t.Error("expected file to not exist")
	}
}

// TestImportLegacySharesJSONValidJSON verifies that a valid JSON file with 2 shares
// is imported successfully, returns (2, nil), and the file is renamed to .bak.
func TestImportLegacySharesJSONValidJSON(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()

	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "shares.json")

	// Create a valid legacy shares.json file with 2 shares
	legacyData := map[string]any{
		"shares": []map[string]any{
			{
				"id":         "share-legacy-1",
				"kind":       "shared",
				"url":        "https://example.com/paste/1",
				"key_b64url": "key1",
				"edit_token": "token1",
				"plan_file":  "/tmp/plan1.json",
				"created_at": time.Now().UTC(),
			},
			{
				"id":         "share-legacy-2",
				"kind":       "local",
				"url":        "https://example.com/paste/2",
				"key_b64url": "key2",
				"edit_token": "token2",
				"plan_file":  "",
				"created_at": time.Now().UTC(),
			},
		},
	}

	jsonBytes, _ := json.Marshal(legacyData)
	if err := os.WriteFile(jsonPath, jsonBytes, 0o644); err != nil {
		t.Fatalf("failed to write JSON file: %v", err)
	}

	// Import
	count, err := server.ImportLegacySharesJSON(context.Background(), jsonPath)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}

	// Verify file renamed to .bak
	if _, err := os.Stat(jsonPath); err == nil {
		t.Error("expected original file to be renamed")
	}
	bakPath := jsonPath + ".bak"
	if _, err := os.Stat(bakPath); err != nil {
		t.Errorf("expected .bak file to exist: %v", err)
	}

	// Verify shares were imported into the table
	shares, err := server.store.ListShares(context.Background())
	if err != nil {
		t.Fatalf("failed to list shares: %v", err)
	}
	if len(shares) != 2 {
		t.Errorf("expected 2 shares in table, got %d", len(shares))
	}

	// Verify the imported shares have correct values
	idToShare := make(map[string]*types.Share)
	for _, s := range shares {
		idToShare[s.ID] = s
	}

	if s, ok := idToShare["share-legacy-1"]; ok {
		if s.Kind != types.ShareKindShared {
			t.Errorf("share-legacy-1 kind = %q, want %q", s.Kind, types.ShareKindShared)
		}
		if s.URL != "https://example.com/paste/1" {
			t.Errorf("share-legacy-1 url = %q, want %q", s.URL, "https://example.com/paste/1")
		}
	} else {
		t.Error("share-legacy-1 not found in imported shares")
	}

	if s, ok := idToShare["share-legacy-2"]; ok {
		if s.Kind != types.ShareKindLocal {
			t.Errorf("share-legacy-2 kind = %q, want %q", s.Kind, types.ShareKindLocal)
		}
	} else {
		t.Error("share-legacy-2 not found in imported shares")
	}
}

// TestImportLegacySharesJSONNonEmptyTable verifies that if the shares table already
// has rows, the function returns (0, nil) without importing or renaming the file.
func TestImportLegacySharesJSONNonEmptyTable(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()

	// Pre-populate the table with one share
	existingShare := &types.Share{
		ID:        "existing-share",
		Kind:      types.ShareKindLocal,
		URL:       "https://example.com/existing",
		KeyB64Url: "key",
		EditToken: "token",
		CreatedAt: time.Now().UTC(),
	}
	if err := server.store.UpsertShare(context.Background(), existingShare); err != nil {
		t.Fatalf("failed to upsert existing share: %v", err)
	}

	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "shares.json")

	// Create a JSON file with different shares
	legacyData := map[string]any{
		"shares": []map[string]any{
			{
				"id":         "share-new",
				"kind":       "shared",
				"url":        "https://example.com/new",
				"key_b64url": "newkey",
				"edit_token": "newtoken",
				"created_at": time.Now().UTC(),
			},
		},
	}
	jsonBytes, _ := json.Marshal(legacyData)
	if err := os.WriteFile(jsonPath, jsonBytes, 0o644); err != nil {
		t.Fatalf("failed to write JSON file: %v", err)
	}

	// Attempt import
	count, err := server.ImportLegacySharesJSON(context.Background(), jsonPath)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0 (no import)", count)
	}

	// Verify JSON file was NOT renamed
	if _, err := os.Stat(jsonPath); err != nil {
		t.Errorf("expected original JSON file to still exist: %v", err)
	}
	bakPath := jsonPath + ".bak"
	if _, err := os.Stat(bakPath); err == nil {
		t.Error("expected .bak file to NOT be created when table is non-empty")
	}

	// Verify table still has only the existing share
	shares, err := server.store.ListShares(context.Background())
	if err != nil {
		t.Fatalf("failed to list shares: %v", err)
	}
	if len(shares) != 1 {
		t.Errorf("expected 1 share in table, got %d", len(shares))
	}
	if shares[0].ID != "existing-share" {
		t.Errorf("expected existing-share to be only share, got %q", shares[0].ID)
	}
}

// TestImportLegacySharesJSONMalformedJSON verifies that malformed JSON causes
// an error, with no file rename and no rows inserted.
func TestImportLegacySharesJSONMalformedJSON(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()

	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "shares.json")

	// Create a malformed JSON file
	if err := os.WriteFile(jsonPath, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("failed to write malformed JSON: %v", err)
	}

	// Attempt import
	count, err := server.ImportLegacySharesJSON(context.Background(), jsonPath)
	if err == nil {
		t.Error("expected error for malformed JSON, got nil")
	}
	if count != 0 {
		t.Errorf("count = %d, want 0 on error", count)
	}

	// Verify JSON file was NOT renamed
	if _, err := os.Stat(jsonPath); err != nil {
		t.Errorf("expected original file to still exist after parse error: %v", err)
	}
	bakPath := jsonPath + ".bak"
	if _, err := os.Stat(bakPath); err == nil {
		t.Error("expected .bak file to NOT be created on parse error")
	}

	// Verify no rows were inserted
	shares, err := server.store.ListShares(context.Background())
	if err != nil {
		t.Fatalf("failed to list shares: %v", err)
	}
	if len(shares) != 0 {
		t.Errorf("expected 0 shares in table after parse error, got %d", len(shares))
	}
}

// TestImportLegacySharesJSONValidationFailureMidImport verifies that when a validation
// error occurs mid-import (e.g., empty ID), the function returns (partial_count, err),
// the file is NOT renamed, and the inserted shares remain in the table.
func TestImportLegacySharesJSONValidationFailureMidImport(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()

	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "shares.json")

	// Create a JSON file where the second share has an empty ID (will fail validation)
	legacyData := map[string]any{
		"shares": []map[string]any{
			{
				"id":         "share-valid",
				"kind":       "shared",
				"url":        "https://example.com/valid",
				"key_b64url": "key1",
				"edit_token": "token1",
				"created_at": time.Now().UTC(),
			},
			{
				"id":         "",
				"kind":       "shared",
				"url":        "https://example.com/invalid",
				"key_b64url": "key2",
				"edit_token": "token2",
				"created_at": time.Now().UTC(),
			},
		},
	}
	jsonBytes, _ := json.Marshal(legacyData)
	if err := os.WriteFile(jsonPath, jsonBytes, 0o644); err != nil {
		t.Fatalf("failed to write JSON file: %v", err)
	}

	// Attempt import
	count, err := server.ImportLegacySharesJSON(context.Background(), jsonPath)
	if err == nil {
		t.Error("expected error for validation failure, got nil")
	}
	if count != 1 {
		t.Errorf("count = %d, want 1 (first share inserted before error)", count)
	}

	// Verify JSON file was NOT renamed (crash-safe: allows retry)
	if _, err := os.Stat(jsonPath); err != nil {
		t.Errorf("expected original file to still exist after mid-import error: %v", err)
	}
	bakPath := jsonPath + ".bak"
	if _, err := os.Stat(bakPath); err == nil {
		t.Error("expected .bak file to NOT be created on mid-import error")
	}

	// Verify only the first (valid) share was inserted
	shares, err := server.store.ListShares(context.Background())
	if err != nil {
		t.Fatalf("failed to list shares: %v", err)
	}
	if len(shares) != 1 {
		t.Errorf("expected 1 share in table (first one inserted), got %d", len(shares))
	}
	if shares[0].ID != "share-valid" {
		t.Errorf("expected share-valid to be in table, got %q", shares[0].ID)
	}
}

// TestImportLegacySharesJSONBakAlreadyExists verifies that if the .bak file
// already exists, os.Rename overwrites it successfully on import.
func TestImportLegacySharesJSONBakAlreadyExists(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()

	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "shares.json")
	bakPath := jsonPath + ".bak"

	// Pre-create the .bak file
	if err := os.WriteFile(bakPath, []byte("old backup"), 0o644); err != nil {
		t.Fatalf("failed to create .bak file: %v", err)
	}

	// Create a valid legacy shares.json file
	legacyData := map[string]any{
		"shares": []map[string]any{
			{
				"id":         "share-new",
				"kind":       "shared",
				"url":        "https://example.com/new",
				"key_b64url": "key",
				"edit_token": "token",
				"created_at": time.Now().UTC(),
			},
		},
	}
	jsonBytes, _ := json.Marshal(legacyData)
	if err := os.WriteFile(jsonPath, jsonBytes, 0o644); err != nil {
		t.Fatalf("failed to write JSON file: %v", err)
	}

	// Attempt import (should succeed and overwrite .bak on Linux)
	count, err := server.ImportLegacySharesJSON(context.Background(), jsonPath)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}

	// Verify original JSON file no longer exists
	if _, err := os.Stat(jsonPath); err == nil {
		t.Error("expected original file to be renamed")
	}

	// Verify .bak file now contains the JSON (overwritten)
	if _, err := os.Stat(bakPath); err != nil {
		t.Errorf("expected .bak file to exist: %v", err)
	}

	// Verify shares were imported
	shares, err := server.store.ListShares(context.Background())
	if err != nil {
		t.Fatalf("failed to list shares: %v", err)
	}
	if len(shares) != 1 {
		t.Errorf("expected 1 share in table, got %d", len(shares))
	}
}
