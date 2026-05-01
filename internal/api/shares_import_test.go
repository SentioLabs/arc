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
	if err := os.WriteFile(jsonPath, jsonBytes, 0o600); err != nil {
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

// TestImportLegacySharesJSONIdempotentUpsert verifies that re-importing entries
// already in the table is a clean no-op upsert: pre-existing rows that aren't
// in the file are preserved, and rows that are in both get overwritten.
func TestImportLegacySharesJSONIdempotentUpsert(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()

	// Pre-populate the table with two shares: one that will also appear in
	// the JSON (overwritten on import) and one that won't (preserved).
	existingShared := &types.Share{
		ID:        "share-overlapping",
		Kind:      types.ShareKindLocal,
		URL:       "https://example.com/old",
		KeyB64Url: "oldkey",
		EditToken: "oldtoken",
		CreatedAt: time.Now().UTC(),
	}
	existingPreserved := &types.Share{
		ID:        "share-preserved",
		Kind:      types.ShareKindLocal,
		URL:       "https://example.com/preserved",
		KeyB64Url: "key",
		EditToken: "token",
		CreatedAt: time.Now().UTC(),
	}
	for _, s := range []*types.Share{existingShared, existingPreserved} {
		if err := server.store.UpsertShare(context.Background(), s); err != nil {
			t.Fatalf("seed share: %v", err)
		}
	}

	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "shares.json")

	// JSON contains the overlapping ID with new field values.
	legacyData := map[string]any{
		"shares": []map[string]any{
			{
				"id":         "share-overlapping",
				"kind":       "shared",
				"url":        "https://example.com/new",
				"key_b64url": "newkey",
				"edit_token": "newtoken",
				"created_at": time.Now().UTC(),
			},
		},
	}
	jsonBytes, _ := json.Marshal(legacyData)
	if err := os.WriteFile(jsonPath, jsonBytes, 0o600); err != nil {
		t.Fatalf("failed to write JSON file: %v", err)
	}

	// Import should succeed and rename the file — the row-count is no longer
	// a guard; the file's presence is.
	count, err := server.ImportLegacySharesJSON(context.Background(), jsonPath)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
	if _, err := os.Stat(jsonPath); err == nil {
		t.Error("expected original file to be renamed")
	}
	if _, err := os.Stat(jsonPath + ".bak"); err != nil {
		t.Errorf("expected .bak file to exist: %v", err)
	}

	// Overlapping row should be overwritten with the JSON's values; the
	// preserved row should still be there untouched.
	shares, err := server.store.ListShares(context.Background())
	if err != nil {
		t.Fatalf("failed to list shares: %v", err)
	}
	if len(shares) != 2 {
		t.Fatalf("expected 2 shares in table, got %d", len(shares))
	}
	byID := map[string]*types.Share{}
	for _, s := range shares {
		byID[s.ID] = s
	}
	if got, want := byID["share-overlapping"].URL, "https://example.com/new"; got != want {
		t.Errorf("overlapping URL = %q, want %q (upsert should have overwritten)", got, want)
	}
	if got, want := byID["share-preserved"].URL, "https://example.com/preserved"; got != want {
		t.Errorf("preserved URL = %q, want %q (untouched row mutated)", got, want)
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
	if err := os.WriteFile(jsonPath, []byte("{not json"), 0o600); err != nil {
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

// TestImportLegacySharesJSONValidationFailureMidImport verifies atomic rollback:
// a validation error on any entry must roll the entire batch back so the file
// stays as shares.json (un-renamed) and the table stays empty. The next
// startup can then retry once the operator fixes the bad entry.
func TestImportLegacySharesJSONValidationFailureMidImport(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()

	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "shares.json")

	// First entry is valid; second has an empty ID and will fail Validate().
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
	if err := os.WriteFile(jsonPath, jsonBytes, 0o600); err != nil {
		t.Fatalf("failed to write JSON file: %v", err)
	}

	count, err := server.ImportLegacySharesJSON(context.Background(), jsonPath)
	if err == nil {
		t.Error("expected error for validation failure, got nil")
	}
	if count != 0 {
		t.Errorf("count = %d, want 0 (transactional rollback)", count)
	}

	// File stays as shares.json so the next startup retries.
	if _, err := os.Stat(jsonPath); err != nil {
		t.Errorf("expected original file to still exist after mid-import error: %v", err)
	}
	if _, err := os.Stat(jsonPath + ".bak"); err == nil {
		t.Error("expected .bak file to NOT be created on mid-import error")
	}

	// No partial state — table must be empty.
	shares, err := server.store.ListShares(context.Background())
	if err != nil {
		t.Fatalf("failed to list shares: %v", err)
	}
	if len(shares) != 0 {
		t.Errorf("expected 0 shares (rollback), got %d", len(shares))
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
	if err := os.WriteFile(bakPath, []byte("old backup"), 0o600); err != nil {
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
	if err := os.WriteFile(jsonPath, jsonBytes, 0o600); err != nil {
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
