//go:build integration

package integration

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestWhichFromInitDir verifies that `arc which` shows the workspace
// resolved via server path match after `arc init`.
func TestWhichFromInitDir(t *testing.T) {
	home := setupHome(t)
	dir := t.TempDir()

	arcCmdInDirSuccess(t, home, dir, "init", "which-test-ws", "--server", serverURL)

	out := arcCmdInDirSuccess(t, home, dir, "which", "--server", serverURL)
	if !strings.Contains(out, "which-test-ws") {
		t.Errorf("expected workspace name in which output, got: %s", out)
	}
	if !strings.Contains(strings.ToLower(out), "server path") {
		t.Errorf("expected 'server path' source in which output, got: %s", out)
	}
}

// TestWhichWithFlag verifies that `arc which -w <id>` shows the flag
// as the resolution source.
func TestWhichWithFlag(t *testing.T) {
	home := setupHome(t)

	// Create a workspace to get its ID.
	arcCmdSuccess(t, home, "workspace", "create", "which-flag-ws", "--server", serverURL)
	wsID := getWorkspaceIDByName(t, home, "which-flag-ws")

	out := arcCmdSuccess(t, home, "which", "-w", wsID, "--server", serverURL)
	if !strings.Contains(out, wsID) {
		t.Errorf("expected workspace ID in which output, got: %s", out)
	}
	if !strings.Contains(strings.ToLower(out), "flag") {
		t.Errorf("expected 'flag' source in which output, got: %s", out)
	}
}

// TestWhichJsonOutput verifies that `arc which --json` returns valid JSON
// with workspace_id and source fields.
func TestWhichJsonOutput(t *testing.T) {
	home := setupHome(t)
	dir := t.TempDir()

	arcCmdInDirSuccess(t, home, dir, "init", "which-json-ws", "--server", serverURL)

	out := arcCmdInDirSuccess(t, home, dir, "which", "--json", "--server", serverURL)

	var result map[string]string
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("expected valid JSON from which --json, got parse error: %v\noutput: %s", err, out)
	}

	if result["workspace_id"] == "" {
		t.Error("expected non-empty workspace_id in JSON output")
	}
	if result["source"] == "" {
		t.Error("expected non-empty source in JSON output")
	}
	if result["workspace_name"] != "which-json-ws" {
		t.Errorf("expected workspace_name 'which-json-ws', got %q", result["workspace_name"])
	}
}

// TestWhichNoWorkspace verifies that `arc which` fails with a helpful
// message when no workspace is configured.
func TestWhichNoWorkspace(t *testing.T) {
	home := setupHome(t)
	dir := t.TempDir()

	out, err := arcCmdInDir(t, home, dir, "which", "--server", serverURL)
	if err == nil {
		t.Fatalf("expected arc which to fail in unconfigured dir, but it succeeded: %s", out)
	}
	if !strings.Contains(strings.ToLower(out), "no workspace") {
		t.Errorf("expected 'no workspace' error message, got: %s", out)
	}
}
