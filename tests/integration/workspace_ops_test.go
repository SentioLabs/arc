//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestWorkspaceDelete creates a workspace, verifies it appears in list,
// deletes it, and verifies it no longer appears.
func TestWorkspaceDelete(t *testing.T) {
	home := setupHome(t)

	wsName := fmt.Sprintf("ws-delete-%d", time.Now().UnixNano())

	// Create workspace with --json to extract the ID.
	createOut := arcCmdSuccess(t, home, "workspace", "create", wsName, "--json", "--server", serverURL)

	var created map[string]interface{}
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("failed to parse workspace create JSON: %v\noutput: %s", err, createOut)
	}
	wsID, ok := created["id"].(string)
	if !ok || wsID == "" {
		t.Fatalf("workspace create JSON missing id field: %s", createOut)
	}

	// Verify workspace appears in list.
	listOut := arcCmdSuccess(t, home, "workspace", "list", "--server", serverURL)
	if !strings.Contains(listOut, wsName) {
		t.Fatalf("expected workspace %q in list output, got: %s", wsName, listOut)
	}

	// Delete workspace by ID.
	arcCmdSuccess(t, home, "workspace", "delete", wsID, "--server", serverURL)

	// Verify workspace no longer appears in list.
	listAfter := arcCmdSuccess(t, home, "workspace", "list", "--server", serverURL)
	if strings.Contains(listAfter, wsName) {
		t.Errorf("workspace %q should not appear after deletion, got: %s", wsName, listAfter)
	}
}

// TestWorkspaceDeleteNonexistent tries to delete a workspace that does not
// exist and verifies the command fails.
func TestWorkspaceDeleteNonexistent(t *testing.T) {
	home := setupHome(t)

	_, err := arcCmd(t, home, "workspace", "delete", "nonexistent-ws-id-12345", "--server", serverURL)
	if err == nil {
		t.Error("expected workspace delete of nonexistent workspace to fail, but it succeeded")
	}
}

// TestWorkspaceCreateWithDescription creates a workspace with --description
// and verifies the description appears in the workspace list or JSON output.
func TestWorkspaceCreateWithDescription(t *testing.T) {
	home := setupHome(t)

	wsName := fmt.Sprintf("ws-desc-%d", time.Now().UnixNano())
	description := "My project workspace"

	// Create workspace with description.
	createOut := arcCmdSuccess(t, home, "workspace", "create", wsName,
		"--description", description, "--json", "--server", serverURL)

	var created map[string]interface{}
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("failed to parse workspace create JSON: %v\noutput: %s", err, createOut)
	}

	// Verify description in create response.
	if desc, ok := created["description"].(string); !ok || desc != description {
		t.Errorf("expected description %q in create response, got: %v", description, created["description"])
	}

	// List workspaces and verify description appears.
	listOut := arcCmdSuccess(t, home, "workspace", "list", "--server", serverURL)
	if !strings.Contains(listOut, description) {
		t.Errorf("expected description %q in workspace list output, got: %s", description, listOut)
	}
}

// TestWorkspaceStats creates a workspace with issues in different statuses
// and verifies the stats output contains the expected counts.
func TestWorkspaceStats(t *testing.T) {
	home := setupHome(t)

	dir := t.TempDir()
	wsName := fmt.Sprintf("ws-stats-%d", time.Now().UnixNano())

	// Initialize workspace.
	arcCmdInDirSuccess(t, home, dir, "init", wsName, "--server", serverURL)

	// Create 3 issues.
	create1 := arcCmdInDirSuccess(t, home, dir, "create", "Stats issue open", "--type", "task", "--server", serverURL)
	create2 := arcCmdInDirSuccess(t, home, dir, "create", "Stats issue progress", "--type", "task", "--server", serverURL)
	create3 := arcCmdInDirSuccess(t, home, dir, "create", "Stats issue closed", "--type", "task", "--server", serverURL)

	// Set issue 2 to in_progress.
	id2, ok := extractID(create2)
	if !ok {
		t.Fatalf("could not extract ID from: %s", create2)
	}
	arcCmdInDirSuccess(t, home, dir, "update", id2, "--status", "in_progress", "--server", serverURL)

	// Close issue 3.
	id3, ok := extractID(create3)
	if !ok {
		t.Fatalf("could not extract ID from: %s", create3)
	}
	arcCmdInDirSuccess(t, home, dir, "close", id3, "--server", serverURL)

	// Verify issue 1 was created (open by default).
	_, ok = extractID(create1)
	if !ok {
		t.Fatalf("could not extract ID from: %s", create1)
	}

	// Run stats.
	statsOut := arcCmdInDirSuccess(t, home, dir, "stats", "--server", serverURL)

	// Verify expected counts.
	if !strings.Contains(statsOut, "Total:       3") {
		t.Errorf("expected Total: 3 in stats output, got: %s", statsOut)
	}
	if !strings.Contains(statsOut, "Open:        1") {
		t.Errorf("expected Open: 1 in stats output, got: %s", statsOut)
	}
	if !strings.Contains(statsOut, "In Progress: 1") {
		t.Errorf("expected In Progress: 1 in stats output, got: %s", statsOut)
	}
	if !strings.Contains(statsOut, "Closed:      1") {
		t.Errorf("expected Closed: 1 in stats output, got: %s", statsOut)
	}
}

// TestWorkspaceStatsEmpty creates an empty workspace and verifies stats shows
// Total: 0.
func TestWorkspaceStatsEmpty(t *testing.T) {
	home := setupHome(t)

	dir := t.TempDir()
	wsName := fmt.Sprintf("ws-stats-empty-%d", time.Now().UnixNano())

	// Initialize workspace.
	arcCmdInDirSuccess(t, home, dir, "init", wsName, "--server", serverURL)

	// Run stats on the empty workspace.
	statsOut := arcCmdInDirSuccess(t, home, dir, "stats", "--server", serverURL)

	if !strings.Contains(statsOut, "Total:       0") {
		t.Errorf("expected Total: 0 in stats output for empty workspace, got: %s", statsOut)
	}
}

// TestWorkspaceStatsJsonOutput creates a workspace with issues and verifies
// that `arc stats --json` returns valid JSON with numeric fields.
func TestWorkspaceStatsJsonOutput(t *testing.T) {
	home := setupHome(t)

	dir := t.TempDir()
	wsName := fmt.Sprintf("ws-stats-json-%d", time.Now().UnixNano())

	// Initialize workspace and create an issue.
	arcCmdInDirSuccess(t, home, dir, "init", wsName, "--server", serverURL)
	arcCmdInDirSuccess(t, home, dir, "create", "JSON stats issue", "--type", "task", "--server", serverURL)

	// Run stats with --json.
	statsOut := arcCmdInDirSuccess(t, home, dir, "stats", "--json", "--server", serverURL)

	var stats map[string]interface{}
	if err := json.Unmarshal([]byte(statsOut), &stats); err != nil {
		t.Fatalf("stats --json output is not valid JSON: %v\noutput: %s", err, statsOut)
	}

	// Verify numeric fields exist.
	for _, field := range []string{"total_issues", "open_issues", "closed_issues"} {
		val, ok := stats[field]
		if !ok {
			t.Errorf("expected field %q in stats JSON, got keys: %v", field, statsKeys(stats))
			continue
		}
		if _, ok := val.(float64); !ok {
			t.Errorf("expected field %q to be numeric, got %T: %v", field, val, val)
		}
	}

	// total_issues should be at least 1 since we created one issue.
	if total, ok := stats["total_issues"].(float64); ok && total < 1 {
		t.Errorf("expected total_issues >= 1, got: %v", total)
	}
}

// TestWorkspaceListJsonOutput creates 2 workspaces and verifies that
// `arc workspace list --json` returns a valid JSON array.
func TestWorkspaceListJsonOutput(t *testing.T) {
	home := setupHome(t)

	wsName1 := fmt.Sprintf("ws-listjson-a-%d", time.Now().UnixNano())
	wsName2 := fmt.Sprintf("ws-listjson-b-%d", time.Now().UnixNano())

	arcCmdSuccess(t, home, "workspace", "create", wsName1, "--server", serverURL)
	arcCmdSuccess(t, home, "workspace", "create", wsName2, "--server", serverURL)

	// List workspaces with --json.
	listOut := arcCmdSuccess(t, home, "workspace", "list", "--json", "--server", serverURL)

	var workspaces []map[string]interface{}
	if err := json.Unmarshal([]byte(listOut), &workspaces); err != nil {
		t.Fatalf("workspace list --json output is not valid JSON array: %v\noutput: %s", err, listOut)
	}

	// Verify we got at least 2 workspaces (there may be others from parallel tests).
	if len(workspaces) < 2 {
		t.Errorf("expected at least 2 workspaces in JSON output, got %d", len(workspaces))
	}

	// Verify our created workspaces are present.
	found1, found2 := false, false
	for _, ws := range workspaces {
		name, _ := ws["name"].(string)
		if name == wsName1 {
			found1 = true
		}
		if name == wsName2 {
			found2 = true
		}
	}
	if !found1 {
		t.Errorf("workspace %q not found in JSON list output", wsName1)
	}
	if !found2 {
		t.Errorf("workspace %q not found in JSON list output", wsName2)
	}
}

// statsKeys returns the keys from a map for diagnostic output.
func statsKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
