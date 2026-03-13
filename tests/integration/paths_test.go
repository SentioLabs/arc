//go:build integration

package integration

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestPathsListAfterInit verifies that `arc paths` shows the path
// registered by `arc init`.
func TestPathsListAfterInit(t *testing.T) {
	home := setupHome(t)
	dir := t.TempDir()

	arcCmdInDirSuccess(t, home, dir, "init", "paths-list-proj", "--server", serverURL)

	out := arcCmdInDirSuccess(t, home, dir, "paths", "--server", serverURL)
	if !strings.Contains(out, dir) && !strings.Contains(out, "PATH") {
		t.Errorf("expected paths output to contain the registered directory or header, got: %s", out)
	}
}

// TestPathsListEmpty verifies that `arc paths` on a project with no paths
// prints a friendly message (use -p flag to skip path-based resolution).
func TestPathsListEmpty(t *testing.T) {
	home := setupHome(t)

	// Create project directly (not via init, so no path is registered).
	arcCmdSuccess(t, home, "project", "create", "empty-paths-proj", "--server", serverURL)

	// Get project ID from JSON output.
	jsonOut := arcCmdSuccess(t, home, "project", "list", "--json", "--server", serverURL)
	var projects []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &projects); err != nil {
		t.Fatalf("parse project list JSON: %v", err)
	}
	var projID string
	for _, proj := range projects {
		if proj.Name == "empty-paths-proj" {
			projID = proj.ID
			break
		}
	}
	if projID == "" {
		t.Fatal("could not find project empty-paths-proj")
	}

	out := arcCmdSuccess(t, home, "paths", "--project", projID, "--server", serverURL)
	if !strings.Contains(strings.ToLower(out), "no paths") {
		t.Errorf("expected 'no paths' message for project with no paths, got: %s", out)
	}
}

// TestPathsAddAndRemove verifies adding a path to a project and then
// removing it.
func TestPathsAddAndRemove(t *testing.T) {
	home := setupHome(t)
	dir := t.TempDir()

	arcCmdInDirSuccess(t, home, dir, "init", "paths-add-rm-proj", "--server", serverURL)

	// Add a second path.
	extraDir := t.TempDir()
	addOut := arcCmdInDirSuccess(t, home, dir, "paths", "add", extraDir, "--label", "extra", "--server", serverURL)
	if !strings.Contains(strings.ToLower(addOut), "registered") {
		t.Errorf("expected 'registered' in add output, got: %s", addOut)
	}

	// List should now show the extra path.
	listOut := arcCmdInDirSuccess(t, home, dir, "paths", "--server", serverURL)
	if !strings.Contains(listOut, "extra") {
		t.Errorf("expected label 'extra' in paths list, got: %s", listOut)
	}

	// Remove the extra path by path string.
	rmOut := arcCmdInDirSuccess(t, home, dir, "paths", "remove", extraDir, "--server", serverURL)
	if !strings.Contains(strings.ToLower(rmOut), "removed") {
		t.Errorf("expected 'removed' in remove output, got: %s", rmOut)
	}

	// List should no longer contain the extra path.
	listAfter := arcCmdInDirSuccess(t, home, dir, "paths", "--server", serverURL)
	if strings.Contains(listAfter, "extra") {
		t.Errorf("extra path should be removed, but still appears: %s", listAfter)
	}
}

// TestPathsAddWithLabel verifies that --label is stored and displayed.
func TestPathsAddWithLabel(t *testing.T) {
	home := setupHome(t)
	dir := t.TempDir()

	arcCmdInDirSuccess(t, home, dir, "init", "paths-label-proj", "--server", serverURL)

	extraDir := t.TempDir()
	arcCmdInDirSuccess(t, home, dir, "paths", "add", extraDir, "--label", "my-custom-label", "--server", serverURL)

	out := arcCmdInDirSuccess(t, home, dir, "paths", "--server", serverURL)
	if !strings.Contains(out, "my-custom-label") {
		t.Errorf("expected label 'my-custom-label' in paths list, got: %s", out)
	}
}

// TestPathsListAll verifies that `arc paths list --all` shows paths
// across multiple projects.
func TestPathsListAll(t *testing.T) {
	home := setupHome(t)

	dirA := t.TempDir()
	dirB := t.TempDir()

	arcCmdInDirSuccess(t, home, dirA, "init", "paths-all-proj-a", "--server", serverURL)
	arcCmdInDirSuccess(t, home, dirB, "init", "paths-all-proj-b", "--server", serverURL)

	out := arcCmdSuccess(t, home, "paths", "list", "--all", "--server", serverURL)

	// Should show paths from both projects.
	if !strings.Contains(out, "paths-all-proj-a") {
		t.Errorf("expected project A in --all output, got: %s", out)
	}
	if !strings.Contains(out, "paths-all-proj-b") {
		t.Errorf("expected project B in --all output, got: %s", out)
	}
}

// TestPathsJsonOutput verifies that `arc paths --json` returns valid JSON.
func TestPathsJsonOutput(t *testing.T) {
	home := setupHome(t)
	dir := t.TempDir()

	arcCmdInDirSuccess(t, home, dir, "init", "paths-json-proj", "--server", serverURL)

	out := arcCmdInDirSuccess(t, home, dir, "paths", "--json", "--server", serverURL)

	var paths []map[string]interface{}
	if err := json.Unmarshal([]byte(out), &paths); err != nil {
		t.Fatalf("expected valid JSON from paths --json, got parse error: %v\noutput: %s", err, out)
	}

	if len(paths) == 0 {
		t.Error("expected at least one path in JSON output")
	}

	// Verify structure has expected fields.
	first := paths[0]
	if _, ok := first["path"]; !ok {
		t.Error("expected 'path' field in JSON output")
	}
	if _, ok := first["id"]; !ok {
		t.Error("expected 'id' field in JSON output")
	}
}

// TestPathsAddJsonOutput verifies that `arc paths add --json` returns valid JSON.
func TestPathsAddJsonOutput(t *testing.T) {
	home := setupHome(t)
	dir := t.TempDir()

	arcCmdInDirSuccess(t, home, dir, "init", "paths-addjson-proj", "--server", serverURL)

	extraDir := t.TempDir()
	out := arcCmdInDirSuccess(t, home, dir, "paths", "add", extraDir, "--json", "--server", serverURL)

	var path map[string]interface{}
	if err := json.Unmarshal([]byte(out), &path); err != nil {
		t.Fatalf("expected valid JSON from paths add --json, got parse error: %v\noutput: %s", err, out)
	}

	if _, ok := path["id"]; !ok {
		t.Error("expected 'id' field in JSON output")
	}
	if _, ok := path["path"]; !ok {
		t.Error("expected 'path' field in JSON output")
	}
}

// TestPathsListAllJsonOutput verifies that `arc paths list --all --json`
// returns valid JSON.
func TestPathsListAllJsonOutput(t *testing.T) {
	home := setupHome(t)
	dir := t.TempDir()

	arcCmdInDirSuccess(t, home, dir, "init", "paths-alljson-proj", "--server", serverURL)

	out := arcCmdSuccess(t, home, "paths", "list", "--all", "--json", "--server", serverURL)

	var paths []map[string]interface{}
	if err := json.Unmarshal([]byte(out), &paths); err != nil {
		t.Fatalf("expected valid JSON from paths list --all --json, got parse error: %v\noutput: %s", err, out)
	}
}
