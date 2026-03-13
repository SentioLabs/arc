//go:build integration

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLegacyAutoMigrationOnList verifies that `arc list` transparently
// auto-migrates a legacy config when no server paths are registered.
func TestLegacyAutoMigrationOnList(t *testing.T) {
	home := setupHome(t)
	dir := t.TempDir()

	// Step 1: Create a project via init so it exists on the server,
	// then create an issue in it.
	arcCmdInDirSuccess(t, home, dir, "init", "legacy-auto-proj", "--server", serverURL)
	arcCmdInDirSuccess(t, home, dir, "create", "Legacy auto issue", "--type", "task", "--server", serverURL)

	// Step 2: Get the project ID.
	projID := getProjectIDByName(t, home, "legacy-auto-proj")

	// Step 3: Delete all server-side paths for this project, simulating
	// a state where only a legacy config exists.
	pathsJSON := arcCmdSuccess(t, home, "paths", "--project", projID, "--json", "--server", serverURL)
	var paths []struct {
		ID   string `json:"id"`
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(pathsJSON), &paths); err != nil {
		t.Fatalf("parse paths: %v", err)
	}
	for _, p := range paths {
		arcCmdSuccess(t, home, "paths", "remove", p.ID, "--project", projID, "--server", serverURL)
	}

	// Step 4: Write a legacy config pointing to this project.
	writeLegacyConfig(t, home, dir, projID, "legacy-auto-proj")

	// Step 5: Run `arc list` from the directory — should auto-migrate and work.
	listOut := arcCmdInDirSuccess(t, home, dir, "list", "--server", serverURL)
	if !strings.Contains(listOut, "Legacy auto issue") {
		t.Errorf("expected auto-migrated list to show issue, got: %s", listOut)
	}

	// Step 6: Verify the legacy config was cleaned up.
	dirName := strings.ReplaceAll(dir, "/", "-")
	cfgDir := filepath.Join(home, ".arc", "projects", dirName)
	if _, err := os.Stat(cfgDir); err == nil {
		t.Errorf("legacy config should be cleaned up after auto-migration, but still exists: %s", cfgDir)
	}

	// Step 7: Verify paths were registered on the server.
	pathsAfter := arcCmdSuccess(t, home, "paths", "--project", projID, "--json", "--server", serverURL)
	var afterPaths []map[string]interface{}
	if err := json.Unmarshal([]byte(pathsAfter), &afterPaths); err != nil {
		t.Fatalf("parse paths after migration: %v", err)
	}
	if len(afterPaths) == 0 {
		t.Error("expected paths to be registered after auto-migration")
	}
}

// TestLegacyAutoMigrationOnCreate verifies that `arc create` also triggers
// auto-migration transparently.
func TestLegacyAutoMigrationOnCreate(t *testing.T) {
	home := setupHome(t)
	dir := t.TempDir()

	// Create project on server.
	arcCmdSuccess(t, home, "project", "create", "legacy-create-proj", "--server", serverURL)
	projID := getProjectIDByName(t, home, "legacy-create-proj")

	// Write legacy config (no server paths exist).
	writeLegacyConfig(t, home, dir, projID, "legacy-create-proj")

	// `arc create` from the directory should auto-migrate and succeed.
	createOut := arcCmdInDirSuccess(t, home, dir, "create", "Issue via legacy", "--type", "task", "--server", serverURL)
	if !strings.Contains(strings.ToLower(createOut), "created") {
		t.Errorf("expected 'created' in output, got: %s", createOut)
	}

	// Verify the issue exists.
	listOut := arcCmdSuccess(t, home, "list", "--project", projID, "--server", serverURL)
	if !strings.Contains(listOut, "Issue via legacy") {
		t.Errorf("expected issue in list after legacy auto-migration, got: %s", listOut)
	}
}

// TestLegacyAutoMigrationSubsequentCallSkipsMigration verifies that after
// auto-migration, subsequent commands resolve via server path (not legacy).
func TestLegacyAutoMigrationSubsequentCallSkipsMigration(t *testing.T) {
	home := setupHome(t)
	dir := t.TempDir()

	// Create project on server.
	arcCmdSuccess(t, home, "project", "create", "legacy-skip-proj", "--server", serverURL)
	projID := getProjectIDByName(t, home, "legacy-skip-proj")

	// Write legacy config.
	writeLegacyConfig(t, home, dir, projID, "legacy-skip-proj")

	// First call triggers auto-migration.
	arcCmdInDirSuccess(t, home, dir, "create", "First issue", "--type", "task", "--server", serverURL)

	// Remove the legacy config manually to ensure the second call
	// doesn't depend on it.
	projDir := filepath.Join(home, ".arc", "projects")
	os.RemoveAll(projDir)

	// Second call should still work (paths are now on the server).
	arcCmdInDirSuccess(t, home, dir, "create", "Second issue", "--type", "task", "--server", serverURL)

	// Both issues should be visible.
	listOut := arcCmdSuccess(t, home, "list", "--project", projID, "--server", serverURL)
	if !strings.Contains(listOut, "First issue") {
		t.Errorf("expected 'First issue' in list, got: %s", listOut)
	}
	if !strings.Contains(listOut, "Second issue") {
		t.Errorf("expected 'Second issue' in list, got: %s", listOut)
	}
}

// TestLegacyAutoMigrationWhichShowsSource verifies that `arc which`
// shows the correct source when resolved via legacy auto-migration.
func TestLegacyAutoMigrationWhichShowsSource(t *testing.T) {
	home := setupHome(t)
	dir := t.TempDir()

	// Create project on server.
	arcCmdSuccess(t, home, "project", "create", "legacy-which-proj", "--server", serverURL)
	projID := getProjectIDByName(t, home, "legacy-which-proj")

	// Write legacy config.
	writeLegacyConfig(t, home, dir, projID, "legacy-which-proj")

	// First `arc which` should resolve via legacy and auto-migrate.
	out := arcCmdInDirSuccess(t, home, dir, "which", "--server", serverURL)
	if !strings.Contains(out, projID) && !strings.Contains(out, "legacy-which-proj") {
		t.Errorf("expected project info in which output, got: %s", out)
	}
}
