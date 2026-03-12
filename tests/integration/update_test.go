//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// TestUpdateStatus creates an issue and updates its status to in_progress,
// then verifies that arc show reflects the change.
func TestUpdateStatus(t *testing.T) {
	home := setupHome(t)

	arcCmdSuccess(t, home, "init", "update-status-proj", "--server", serverURL)

	createOut := arcCmdSuccess(t, home, "create", "Status update test", "--type", "task", "--server", serverURL)
	id, ok := extractID(createOut)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", createOut)
	}

	arcCmdSuccess(t, home, "update", id, "--status", "in_progress", "--server", serverURL)

	showOut := arcCmdSuccess(t, home, "show", id, "--server", serverURL)
	if !strings.Contains(strings.ToLower(showOut), "in_progress") && !strings.Contains(strings.ToLower(showOut), "in progress") {
		t.Errorf("expected status in_progress in show output, got: %s", showOut)
	}
}

// TestUpdateTitle creates an issue and updates its title, then verifies
// that arc show reflects the new title.
func TestUpdateTitle(t *testing.T) {
	home := setupHome(t)

	arcCmdSuccess(t, home, "init", "update-title-proj", "--server", serverURL)

	createOut := arcCmdSuccess(t, home, "create", "Original Title", "--type", "task", "--server", serverURL)
	id, ok := extractID(createOut)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", createOut)
	}

	arcCmdSuccess(t, home, "update", id, "--title", "New Title", "--server", serverURL)

	showOut := arcCmdSuccess(t, home, "show", id, "--server", serverURL)
	if !strings.Contains(showOut, "New Title") {
		t.Errorf("expected 'New Title' in show output, got: %s", showOut)
	}
}

// TestUpdatePriority creates an issue and updates its priority to 1, then
// verifies that arc show shows P1.
func TestUpdatePriority(t *testing.T) {
	home := setupHome(t)

	arcCmdSuccess(t, home, "init", "update-priority-proj", "--server", serverURL)

	createOut := arcCmdSuccess(t, home, "create", "Priority update test", "--type", "task", "--server", serverURL)
	id, ok := extractID(createOut)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", createOut)
	}

	arcCmdSuccess(t, home, "update", id, "--priority", "1", "--server", serverURL)

	showOut := arcCmdSuccess(t, home, "show", id, "--server", serverURL)
	if !strings.Contains(showOut, "P1") {
		t.Errorf("expected P1 in show output, got: %s", showOut)
	}
}

// TestUpdateAssignee creates an issue and updates its assignee to alice,
// then verifies that arc show shows alice.
func TestUpdateAssignee(t *testing.T) {
	home := setupHome(t)

	arcCmdSuccess(t, home, "init", "update-assignee-proj", "--server", serverURL)

	createOut := arcCmdSuccess(t, home, "create", "Assignee update test", "--type", "task", "--server", serverURL)
	id, ok := extractID(createOut)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", createOut)
	}

	arcCmdSuccess(t, home, "update", id, "--assignee", "alice", "--server", serverURL)

	showOut := arcCmdSuccess(t, home, "show", id, "--server", serverURL)
	if !strings.Contains(showOut, "alice") {
		t.Errorf("expected 'alice' in show output, got: %s", showOut)
	}
}

// TestUpdateType creates an issue as a task, updates it to bug, then
// verifies that arc show reflects the bug type.
func TestUpdateType(t *testing.T) {
	home := setupHome(t)

	arcCmdSuccess(t, home, "init", "update-type-proj", "--server", serverURL)

	createOut := arcCmdSuccess(t, home, "create", "Type update test", "--type", "task", "--server", serverURL)
	id, ok := extractID(createOut)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", createOut)
	}

	arcCmdSuccess(t, home, "update", id, "--type", "bug", "--server", serverURL)

	showOut := arcCmdSuccess(t, home, "show", id, "--server", serverURL)
	if !strings.Contains(strings.ToLower(showOut), "bug") {
		t.Errorf("expected 'bug' type in show output, got: %s", showOut)
	}
}

// TestUpdateDescription creates an issue, updates its description, then
// verifies that arc show contains the new description.
func TestUpdateDescription(t *testing.T) {
	home := setupHome(t)

	arcCmdSuccess(t, home, "init", "update-desc-proj", "--server", serverURL)

	createOut := arcCmdSuccess(t, home, "create", "Description update test", "--type", "task", "--server", serverURL)
	id, ok := extractID(createOut)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", createOut)
	}

	arcCmdSuccess(t, home, "update", id, "--description", "New desc", "--server", serverURL)

	showOut := arcCmdSuccess(t, home, "show", id, "--server", serverURL)
	if !strings.Contains(showOut, "New desc") {
		t.Errorf("expected 'New desc' in show output, got: %s", showOut)
	}
}

// TestUpdateMultipleFields creates an issue and updates multiple fields in
// a single command, then verifies all changes are applied.
func TestUpdateMultipleFields(t *testing.T) {
	home := setupHome(t)

	arcCmdSuccess(t, home, "init", "update-multi-proj", "--server", serverURL)

	createOut := arcCmdSuccess(t, home, "create", "Multi-field update test", "--type", "task", "--server", serverURL)
	id, ok := extractID(createOut)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", createOut)
	}

	arcCmdSuccess(t, home, "update", id,
		"--status", "in_progress",
		"--priority", "1",
		"--assignee", "bob",
		"--server", serverURL,
	)

	showOut := arcCmdSuccess(t, home, "show", id, "--server", serverURL)
	lower := strings.ToLower(showOut)

	if !strings.Contains(lower, "in_progress") && !strings.Contains(lower, "in progress") {
		t.Errorf("expected status in_progress in show output, got: %s", showOut)
	}
	if !strings.Contains(showOut, "P1") {
		t.Errorf("expected P1 in show output, got: %s", showOut)
	}
	if !strings.Contains(showOut, "bob") {
		t.Errorf("expected 'bob' in show output, got: %s", showOut)
	}
}

// TestUpdateNoChanges creates an issue and runs arc update with no flags,
// which should fail with an error about no updates specified.
func TestUpdateNoChanges(t *testing.T) {
	home := setupHome(t)

	arcCmdSuccess(t, home, "init", "update-noop-proj", "--server", serverURL)

	createOut := arcCmdSuccess(t, home, "create", "No-change update test", "--type", "task", "--server", serverURL)
	id, ok := extractID(createOut)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", createOut)
	}

	output, err := arcCmd(t, home, "update", id, "--server", serverURL)
	if err == nil {
		t.Fatalf("expected error when running update with no flags, but command succeeded with output: %s", output)
	}
	if !strings.Contains(strings.ToLower(output), "no updates") {
		t.Errorf("expected 'no updates' error message, got: %s", output)
	}
}

// TestIssueReopen creates an issue, closes it, then uses the HTTP API
// directly to reopen it, and verifies the status is open again.
func TestIssueReopen(t *testing.T) {
	home := setupHome(t)

	arcCmdSuccess(t, home, "init", "reopen-proj", "--server", serverURL)

	createOut := arcCmdSuccess(t, home, "create", "Reopen test issue", "--type", "task", "--server", serverURL)
	id, ok := extractID(createOut)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", createOut)
	}

	// Close the issue.
	arcCmdSuccess(t, home, "close", id, "--server", serverURL)

	// Get project ID from JSON output.
	jsonOut := arcCmdSuccess(t, home, "show", id, "--json", "--server", serverURL)
	var issueData map[string]interface{}
	if err := json.Unmarshal([]byte(jsonOut), &issueData); err != nil {
		t.Fatalf("failed to parse show --json output: %v\noutput: %s", err, jsonOut)
	}
	projID, ok := issueData["project_id"].(string)
	if !ok || projID == "" {
		t.Fatalf("could not extract project_id from JSON: %v", issueData)
	}

	// Reopen via HTTP API.
	reopenURL := fmt.Sprintf("%s/api/v1/projects/%s/issues/%s/reopen", serverURL, projID, id)
	req, err := http.NewRequest(http.MethodPost, reopenURL, nil)
	if err != nil {
		t.Fatalf("failed to create reopen request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("reopen request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from reopen endpoint, got %d", resp.StatusCode)
	}

	// Verify the issue is open again.
	showOut := arcCmdSuccess(t, home, "show", id, "--server", serverURL)
	lower := strings.ToLower(showOut)
	if !strings.Contains(lower, "open") {
		t.Errorf("expected issue to be open after reopen, got: %s", showOut)
	}
	if strings.Contains(lower, "closed") {
		t.Errorf("issue should not be closed after reopen, got: %s", showOut)
	}
}

// TestIssueJsonOutput creates an issue and verifies that arc show --json
// returns valid JSON containing the expected issue fields.
func TestIssueJsonOutput(t *testing.T) {
	home := setupHome(t)

	arcCmdSuccess(t, home, "init", "json-output-proj", "--server", serverURL)

	createOut := arcCmdSuccess(t, home, "create", "JSON output test", "--type", "task", "--server", serverURL)
	id, ok := extractID(createOut)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", createOut)
	}

	jsonOut := arcCmdSuccess(t, home, "show", id, "--json", "--server", serverURL)

	var issueData map[string]interface{}
	if err := json.Unmarshal([]byte(jsonOut), &issueData); err != nil {
		t.Fatalf("show --json output is not valid JSON: %v\noutput: %s", err, jsonOut)
	}

	// Verify expected fields are present.
	for _, field := range []string{"id", "title", "status", "priority", "issue_type", "project_id"} {
		if _, exists := issueData[field]; !exists {
			t.Errorf("expected field %q in JSON output, got keys: %v", field, keys(issueData))
		}
	}

	// Verify field values.
	if got, _ := issueData["title"].(string); got != "JSON output test" {
		t.Errorf("expected title 'JSON output test', got %q", got)
	}
	if got, _ := issueData["id"].(string); got != id {
		t.Errorf("expected id %q, got %q", id, got)
	}
}

// keys returns the keys of a map as a sorted slice for diagnostic output.
func keys(m map[string]interface{}) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}
