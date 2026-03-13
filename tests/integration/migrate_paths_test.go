//go:build integration

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeLegacyConfig creates a legacy ~/.arc/projects/<dir>/config.json file.
func writeLegacyConfig(t *testing.T, home, projectRoot, projID, projName string) {
	t.Helper()

	// Legacy config dirs use the project root with slashes replaced by dashes.
	dirName := strings.ReplaceAll(projectRoot, "/", "-")
	cfgDir := filepath.Join(home, ".arc", "projects", dirName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("create legacy config dir: %v", err)
	}

	cfg := map[string]string{
		"workspace_id":   projID,
		"workspace_name": projName,
		"project_root":   projectRoot,
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal legacy config: %v", err)
	}

	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), data, 0o600); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}
}

// getProjectIDByName extracts the project ID from `arc project list --json`
// for a project with the given name.
func getProjectIDByName(t *testing.T, home, name string) string {
	t.Helper()
	jsonOut := arcCmdSuccess(t, home, "project", "list", "--json", "--server", serverURL)
	var projects []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &projects); err != nil {
		t.Fatalf("parse project list: %v", err)
	}
	for _, proj := range projects {
		if proj.Name == name {
			return proj.ID
		}
	}
	t.Fatalf("project %q not found", name)
	return ""
}

// TestMigratePathsDryRun verifies that --dry-run shows what would be
// migrated without actually making changes.
func TestMigratePathsDryRun(t *testing.T) {
	home := setupHome(t)

	// Create a project on the server first (legacy configs reference existing projects).
	arcCmdSuccess(t, home, "project", "create", "migrate-dry-proj", "--server", serverURL)
	projID := getProjectIDByName(t, home, "migrate-dry-proj")

	// Write a legacy config pointing to this project.
	projectDir := t.TempDir()
	writeLegacyConfig(t, home, projectDir, projID, "migrate-dry-proj")

	// Run dry-run.
	out := arcCmdSuccess(t, home, "migrate-paths", "--dry-run", "--server", serverURL)
	if !strings.Contains(strings.ToLower(out), "would migrate") {
		t.Errorf("expected 'would migrate' in dry-run output, got: %s", out)
	}

	// Verify the legacy config was NOT removed.
	entries, err := os.ReadDir(filepath.Join(home, ".arc", "projects"))
	if err != nil {
		t.Fatalf("read projects dir after dry-run: %v", err)
	}
	if len(entries) == 0 {
		t.Error("dry-run should not remove legacy configs, but projects/ is empty")
	}
}

// TestMigratePathsActual verifies that migrate-paths registers paths
// on the server and cleans up legacy configs.
func TestMigratePathsActual(t *testing.T) {
	home := setupHome(t)

	// Create project on server.
	arcCmdSuccess(t, home, "project", "create", "migrate-actual-proj", "--server", serverURL)
	projID := getProjectIDByName(t, home, "migrate-actual-proj")

	// Write legacy config.
	projectDir := t.TempDir()
	writeLegacyConfig(t, home, projectDir, projID, "migrate-actual-proj")

	// Run migration.
	out := arcCmdSuccess(t, home, "migrate-paths", "--server", serverURL)
	if !strings.Contains(strings.ToLower(out), "migrated") {
		t.Errorf("expected 'migrated' in output, got: %s", out)
	}

	// Verify paths were registered: list paths for the project.
	pathsOut := arcCmdSuccess(t, home, "paths", "--project", projID, "--json", "--server", serverURL)
	if !strings.Contains(pathsOut, projectDir) {
		// The path may be normalized/resolved — just verify something was registered.
		var paths []map[string]interface{}
		if err := json.Unmarshal([]byte(pathsOut), &paths); err != nil {
			t.Fatalf("parse paths JSON: %v", err)
		}
		if len(paths) == 0 {
			t.Error("expected at least one path after migration, got none")
		}
	}

	// Verify legacy config was cleaned up.
	dirName := strings.ReplaceAll(projectDir, "/", "-")
	cfgDir := filepath.Join(home, ".arc", "projects", dirName)
	if _, err := os.Stat(cfgDir); err == nil {
		t.Errorf("legacy config dir should be removed after migration, but still exists: %s", cfgDir)
	}
}

// TestMigratePathsNoConfigs verifies graceful handling when there are
// no legacy configs to migrate.
func TestMigratePathsNoConfigs(t *testing.T) {
	home := setupHome(t)

	out := arcCmdSuccess(t, home, "migrate-paths", "--server", serverURL)
	if !strings.Contains(strings.ToLower(out), "no project configs") {
		t.Errorf("expected 'no project configs' message, got: %s", out)
	}
}

// TestMigratePathsMultiple verifies that multiple legacy configs are
// migrated and cleaned up individually.
func TestMigratePathsMultiple(t *testing.T) {
	home := setupHome(t)

	// Create two projects.
	arcCmdSuccess(t, home, "project", "create", "migrate-multi-a", "--server", serverURL)
	arcCmdSuccess(t, home, "project", "create", "migrate-multi-b", "--server", serverURL)
	projIDA := getProjectIDByName(t, home, "migrate-multi-a")
	projIDB := getProjectIDByName(t, home, "migrate-multi-b")

	// Write legacy configs for both.
	dirA := t.TempDir()
	dirB := t.TempDir()
	writeLegacyConfig(t, home, dirA, projIDA, "migrate-multi-a")
	writeLegacyConfig(t, home, dirB, projIDB, "migrate-multi-b")

	// Run migration.
	out := arcCmdSuccess(t, home, "migrate-paths", "--server", serverURL)
	if !strings.Contains(out, "Migrated 2") && !strings.Contains(out, "Migrated: ") {
		// At minimum, both should show as migrated.
		lines := strings.Count(out, "Migrated:")
		if lines < 2 {
			t.Errorf("expected 2 migrations, got output: %s", out)
		}
	}

	// Verify both projects have paths registered.
	pathsA := arcCmdSuccess(t, home, "paths", "--project", projIDA, "--json", "--server", serverURL)
	var pA []map[string]interface{}
	if err := json.Unmarshal([]byte(pathsA), &pA); err != nil {
		t.Fatalf("parse paths A: %v", err)
	}
	if len(pA) == 0 {
		t.Error("expected paths for project A after migration")
	}

	pathsB := arcCmdSuccess(t, home, "paths", "--project", projIDB, "--json", "--server", serverURL)
	var pB []map[string]interface{}
	if err := json.Unmarshal([]byte(pathsB), &pB); err != nil {
		t.Fatalf("parse paths B: %v", err)
	}
	if len(pB) == 0 {
		t.Error("expected paths for project B after migration")
	}
}
