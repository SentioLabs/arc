//go:build integration

package integration

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestWhichFromInitDir verifies that `arc which` shows the project
// resolved via server path match after `arc init`.
func TestWhichFromInitDir(t *testing.T) {
	home := setupHome(t)
	dir := t.TempDir()

	arcCmdInDirSuccess(t, home, dir, "init", "which-test-proj", "--server", serverURL)

	out := arcCmdInDirSuccess(t, home, dir, "which", "--server", serverURL)
	if !strings.Contains(out, "which-test-proj") {
		t.Errorf("expected project name in which output, got: %s", out)
	}
	if !strings.Contains(strings.ToLower(out), "server path") {
		t.Errorf("expected 'server path' source in which output, got: %s", out)
	}
}

// TestWhichWithFlag verifies that `arc which -p <id>` shows the flag
// as the resolution source.
func TestWhichWithFlag(t *testing.T) {
	home := setupHome(t)

	// Create a project to get its ID.
	arcCmdSuccess(t, home, "project", "create", "which-flag-proj", "--server", serverURL)
	projID := getProjectIDByName(t, home, "which-flag-proj")

	out := arcCmdSuccess(t, home, "which", "-p", projID, "--server", serverURL)
	if !strings.Contains(out, projID) {
		t.Errorf("expected project ID in which output, got: %s", out)
	}
	if !strings.Contains(strings.ToLower(out), "flag") {
		t.Errorf("expected 'flag' source in which output, got: %s", out)
	}
}

// TestWhichJsonOutput verifies that `arc which --json` returns valid JSON
// with project_id and source fields.
func TestWhichJsonOutput(t *testing.T) {
	home := setupHome(t)
	dir := t.TempDir()

	arcCmdInDirSuccess(t, home, dir, "init", "which-json-proj", "--server", serverURL)

	out := arcCmdInDirSuccess(t, home, dir, "which", "--json", "--server", serverURL)

	var result map[string]string
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("expected valid JSON from which --json, got parse error: %v\noutput: %s", err, out)
	}

	if result["project_id"] == "" {
		t.Error("expected non-empty project_id in JSON output")
	}
	if result["source"] == "" {
		t.Error("expected non-empty source in JSON output")
	}
	if result["project_name"] != "which-json-proj" {
		t.Errorf("expected project_name 'which-json-proj', got %q", result["project_name"])
	}
}

// TestWhichNoProject verifies that `arc which` fails with a helpful
// message when no project is configured.
func TestWhichNoProject(t *testing.T) {
	home := setupHome(t)
	dir := t.TempDir()

	out, err := arcCmdInDir(t, home, dir, "which", "--server", serverURL)
	if err == nil {
		t.Fatalf("expected arc which to fail in unconfigured dir, but it succeeded: %s", out)
	}
	if !strings.Contains(strings.ToLower(out), "no project") {
		t.Errorf("expected 'no project' error message, got: %s", out)
	}
}
