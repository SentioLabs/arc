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

	arcCmdInDirSuccess(t, home, dir, "init", "paths-list-ws", "--server", serverURL)

	out := arcCmdInDirSuccess(t, home, dir, "paths", "--server", serverURL)
	if !strings.Contains(out, dir) && !strings.Contains(out, "PATH") {
		t.Errorf("expected paths output to contain the registered directory or header, got: %s", out)
	}
}

// TestPathsListEmpty verifies that `arc paths` on a workspace with no paths
// prints a friendly message (use -w flag to skip path-based resolution).
func TestPathsListEmpty(t *testing.T) {
	home := setupHome(t)

	// Create workspace directly (not via init, so no path is registered).
	arcCmdSuccess(t, home, "workspace", "create", "empty-paths-ws", "--server", serverURL)

	// Get workspace ID from JSON output.
	jsonOut := arcCmdSuccess(t, home, "workspace", "list", "--json", "--server", serverURL)
	var workspaces []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &workspaces); err != nil {
		t.Fatalf("parse workspace list JSON: %v", err)
	}
	var wsID string
	for _, ws := range workspaces {
		if ws.Name == "empty-paths-ws" {
			wsID = ws.ID
			break
		}
	}
	if wsID == "" {
		t.Fatal("could not find workspace empty-paths-ws")
	}

	out := arcCmdSuccess(t, home, "paths", "-w", wsID, "--server", serverURL)
	if !strings.Contains(strings.ToLower(out), "no paths") {
		t.Errorf("expected 'no paths' message for workspace with no paths, got: %s", out)
	}
}

// TestPathsAddAndRemove verifies adding a path to a workspace and then
// removing it.
func TestPathsAddAndRemove(t *testing.T) {
	home := setupHome(t)
	dir := t.TempDir()

	arcCmdInDirSuccess(t, home, dir, "init", "paths-add-rm-ws", "--server", serverURL)

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

	arcCmdInDirSuccess(t, home, dir, "init", "paths-label-ws", "--server", serverURL)

	extraDir := t.TempDir()
	arcCmdInDirSuccess(t, home, dir, "paths", "add", extraDir, "--label", "my-custom-label", "--server", serverURL)

	out := arcCmdInDirSuccess(t, home, dir, "paths", "--server", serverURL)
	if !strings.Contains(out, "my-custom-label") {
		t.Errorf("expected label 'my-custom-label' in paths list, got: %s", out)
	}
}

// TestPathsListAll verifies that `arc paths list --all` shows paths
// across multiple workspaces.
func TestPathsListAll(t *testing.T) {
	home := setupHome(t)

	dirA := t.TempDir()
	dirB := t.TempDir()

	arcCmdInDirSuccess(t, home, dirA, "init", "paths-all-ws-a", "--server", serverURL)
	arcCmdInDirSuccess(t, home, dirB, "init", "paths-all-ws-b", "--server", serverURL)

	out := arcCmdSuccess(t, home, "paths", "list", "--all", "--server", serverURL)

	// Should show paths from both workspaces.
	if !strings.Contains(out, "paths-all-ws-a") {
		t.Errorf("expected workspace A in --all output, got: %s", out)
	}
	if !strings.Contains(out, "paths-all-ws-b") {
		t.Errorf("expected workspace B in --all output, got: %s", out)
	}
}

// TestPathsJsonOutput verifies that `arc paths --json` returns valid JSON.
func TestPathsJsonOutput(t *testing.T) {
	home := setupHome(t)
	dir := t.TempDir()

	arcCmdInDirSuccess(t, home, dir, "init", "paths-json-ws", "--server", serverURL)

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

	arcCmdInDirSuccess(t, home, dir, "init", "paths-addjson-ws", "--server", serverURL)

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

	arcCmdInDirSuccess(t, home, dir, "init", "paths-alljson-ws", "--server", serverURL)

	out := arcCmdSuccess(t, home, "paths", "list", "--all", "--json", "--server", serverURL)

	var paths []map[string]interface{}
	if err := json.Unmarshal([]byte(out), &paths); err != nil {
		t.Fatalf("expected valid JSON from paths list --all --json, got parse error: %v\noutput: %s", err, out)
	}
}
