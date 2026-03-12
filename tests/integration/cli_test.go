//go:build integration

package integration

import (
	"regexp"
	"strings"
	"testing"
)

// issueIDPattern matches arc issue IDs like "abc-0def.a1b2c3".
var issueIDPattern = regexp.MustCompile(`[a-z0-9]+-[a-z0-9]+\.[a-z0-9]+`)

// extractID finds the first arc-style issue ID in the given text.
// Returns the ID and true if found, or empty string and false otherwise.
func extractID(text string) (string, bool) {
	match := issueIDPattern.FindString(text)
	return match, match != ""
}

// TestProjectCreateAndList verifies that `arc init` creates a project
// and that subsequent commands can operate within it.
func TestProjectCreateAndList(t *testing.T) {
	home := setupHome(t)

	// Initialize a project.
	output := arcCmdSuccess(t, home, "init", "integration-test-proj", "--server", serverURL)
	if !strings.Contains(strings.ToLower(output), "project") {
		t.Errorf("expected project mention in init output, got: %s", output)
	}

	// List issues in the project (should be empty but command should succeed).
	listOutput := arcCmdSuccess(t, home, "list", "--server", serverURL)
	// An empty project should not error; output may be empty or show a header.
	_ = listOutput
}

// TestIssueCreateShowClose exercises the basic issue lifecycle: create an
// issue, show its details, then close it.
func TestIssueCreateShowClose(t *testing.T) {
	home := setupHome(t)

	// Set up project first.
	arcCmdSuccess(t, home, "init", "integration-crud", "--server", serverURL)

	// Create an issue.
	createOutput := arcCmdSuccess(t, home, "create", "Test issue for CRUD", "--type", "task", "--server", serverURL)

	id, ok := extractID(createOutput)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", createOutput)
	}

	// Show the issue.
	showOutput := arcCmdSuccess(t, home, "show", id, "--server", serverURL)
	if !strings.Contains(showOutput, "Test issue for CRUD") {
		t.Errorf("show output should contain issue title, got: %s", showOutput)
	}

	// Close the issue.
	closeOutput := arcCmdSuccess(t, home, "close", id, "--server", serverURL)
	if !strings.Contains(strings.ToLower(closeOutput), "closed") && !strings.Contains(strings.ToLower(closeOutput), "close") {
		t.Errorf("close output should mention closed status, got: %s", closeOutput)
	}

	// Verify it shows as closed.
	showAfter := arcCmdSuccess(t, home, "show", id, "--server", serverURL)
	if !strings.Contains(strings.ToLower(showAfter), "closed") {
		t.Errorf("issue should be closed after arc close, got: %s", showAfter)
	}
}

// TestListAndReady verifies that `arc list` returns created issues and
// `arc ready` shows actionable work.
func TestListAndReady(t *testing.T) {
	home := setupHome(t)

	// Set up project.
	arcCmdSuccess(t, home, "init", "integration-ready", "--server", serverURL)

	// Create a couple of issues.
	arcCmdSuccess(t, home, "create", "Ready issue one", "--type", "task", "--server", serverURL)
	arcCmdSuccess(t, home, "create", "Ready issue two", "--type", "task", "--server", serverURL)

	// List should show both issues.
	listOutput := arcCmdSuccess(t, home, "list", "--server", serverURL)
	if !strings.Contains(listOutput, "Ready issue one") {
		t.Errorf("list output should contain first issue, got: %s", listOutput)
	}
	if !strings.Contains(listOutput, "Ready issue two") {
		t.Errorf("list output should contain second issue, got: %s", listOutput)
	}

	// Ready should show actionable work.
	readyOutput := arcCmdSuccess(t, home, "ready", "--server", serverURL)
	// Ready returns open issues sorted by priority; both should appear.
	if !strings.Contains(readyOutput, "Ready issue") {
		t.Errorf("ready output should contain at least one issue, got: %s", readyOutput)
	}
}
