//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestProjectDelete creates a project, verifies it appears in list,
// deletes it, and verifies it no longer appears.
func TestProjectDelete(t *testing.T) {
	home := setupHome(t)

	projName := fmt.Sprintf("proj-delete-%d", time.Now().UnixNano())

	// Create project with --json to extract the ID.
	createOut := arcCmdSuccess(t, home, "project", "create", projName, "--json", "--server", serverURL)

	var created map[string]interface{}
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("failed to parse project create JSON: %v\noutput: %s", err, createOut)
	}
	projID, ok := created["id"].(string)
	if !ok || projID == "" {
		t.Fatalf("project create JSON missing id field: %s", createOut)
	}

	// Verify project appears in list.
	listOut := arcCmdSuccess(t, home, "project", "list", "--server", serverURL)
	if !strings.Contains(listOut, projName) {
		t.Fatalf("expected project %q in list output, got: %s", projName, listOut)
	}

	// Delete project by ID.
	arcCmdSuccess(t, home, "project", "delete", projID, "--server", serverURL)

	// Verify project no longer appears in list.
	listAfter := arcCmdSuccess(t, home, "project", "list", "--server", serverURL)
	if strings.Contains(listAfter, projName) {
		t.Errorf("project %q should not appear after deletion, got: %s", projName, listAfter)
	}
}

// TestProjectDeleteNonexistent tries to delete a project that does not
// exist and verifies the command fails.
func TestProjectDeleteNonexistent(t *testing.T) {
	home := setupHome(t)

	_, err := arcCmd(t, home, "project", "delete", "nonexistent-proj-id-12345", "--server", serverURL)
	if err == nil {
		t.Error("expected project delete of nonexistent project to fail, but it succeeded")
	}
}

// TestProjectCreateWithDescription creates a project with --description
// and verifies the description appears in the project list or JSON output.
func TestProjectCreateWithDescription(t *testing.T) {
	home := setupHome(t)

	projName := fmt.Sprintf("proj-desc-%d", time.Now().UnixNano())
	description := "My project description"

	// Create project with description.
	createOut := arcCmdSuccess(t, home, "project", "create", projName,
		"--description", description, "--json", "--server", serverURL)

	var created map[string]interface{}
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("failed to parse project create JSON: %v\noutput: %s", err, createOut)
	}

	// Verify description in create response.
	if desc, ok := created["description"].(string); !ok || desc != description {
		t.Errorf("expected description %q in create response, got: %v", description, created["description"])
	}

	// List projects and verify description appears.
	listOut := arcCmdSuccess(t, home, "project", "list", "--server", serverURL)
	if !strings.Contains(listOut, description) {
		t.Errorf("expected description %q in project list output, got: %s", description, listOut)
	}
}

// TestProjectStats creates a project with issues in different statuses
// and verifies the stats output contains the expected counts.
func TestProjectStats(t *testing.T) {
	home := setupHome(t)

	dir := t.TempDir()
	projName := fmt.Sprintf("proj-stats-%d", time.Now().UnixNano())

	// Initialize project.
	arcCmdInDirSuccess(t, home, dir, "init", projName, "--server", serverURL)

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

// TestProjectStatsEmpty creates an empty project and verifies stats shows
// Total: 0.
func TestProjectStatsEmpty(t *testing.T) {
	home := setupHome(t)

	dir := t.TempDir()
	projName := fmt.Sprintf("proj-stats-empty-%d", time.Now().UnixNano())

	// Initialize project.
	arcCmdInDirSuccess(t, home, dir, "init", projName, "--server", serverURL)

	// Run stats on the empty project.
	statsOut := arcCmdInDirSuccess(t, home, dir, "stats", "--server", serverURL)

	if !strings.Contains(statsOut, "Total:       0") {
		t.Errorf("expected Total: 0 in stats output for empty project, got: %s", statsOut)
	}
}

// TestProjectStatsJsonOutput creates a project with issues and verifies
// that `arc stats --json` returns valid JSON with numeric fields.
func TestProjectStatsJsonOutput(t *testing.T) {
	home := setupHome(t)

	dir := t.TempDir()
	projName := fmt.Sprintf("proj-stats-json-%d", time.Now().UnixNano())

	// Initialize project and create an issue.
	arcCmdInDirSuccess(t, home, dir, "init", projName, "--server", serverURL)
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

// TestProjectListJsonOutput creates 2 projects and verifies that
// `arc project list --json` returns a valid JSON array.
func TestProjectListJsonOutput(t *testing.T) {
	home := setupHome(t)

	projName1 := fmt.Sprintf("proj-listjson-a-%d", time.Now().UnixNano())
	projName2 := fmt.Sprintf("proj-listjson-b-%d", time.Now().UnixNano())

	arcCmdSuccess(t, home, "project", "create", projName1, "--server", serverURL)
	arcCmdSuccess(t, home, "project", "create", projName2, "--server", serverURL)

	// List projects with --json.
	listOut := arcCmdSuccess(t, home, "project", "list", "--json", "--server", serverURL)

	var projects []map[string]interface{}
	if err := json.Unmarshal([]byte(listOut), &projects); err != nil {
		t.Fatalf("project list --json output is not valid JSON array: %v\noutput: %s", err, listOut)
	}

	// Verify we got at least 2 projects (there may be others from parallel tests).
	if len(projects) < 2 {
		t.Errorf("expected at least 2 projects in JSON output, got %d", len(projects))
	}

	// Verify our created projects are present.
	found1, found2 := false, false
	for _, proj := range projects {
		name, _ := proj["name"].(string)
		if name == projName1 {
			found1 = true
		}
		if name == projName2 {
			found2 = true
		}
	}
	if !found1 {
		t.Errorf("project %q not found in JSON list output", projName1)
	}
	if !found2 {
		t.Errorf("project %q not found in JSON list output", projName2)
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
