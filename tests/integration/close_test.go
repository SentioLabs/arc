//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// TestCloseWithReason verifies that closing an issue with --reason succeeds
// and the issue ends up in a closed state.
func TestCloseWithReason(t *testing.T) {
	home := setupHome(t)
	ws := fmt.Sprintf("close-reason-%d", uniqueCounter())

	arcCmdSuccess(t, home, "init", ws, "--server", serverURL)

	createOut := arcCmdSuccess(t, home, "create", "Issue to close with reason", "--type", "task", "--server", serverURL)
	id, ok := extractID(createOut)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", createOut)
	}

	// Close with a reason.
	arcCmdSuccess(t, home, "close", id, "--reason", "Done for now", "--server", serverURL)

	// Verify the issue is closed via JSON output.
	showOut := arcCmdSuccess(t, home, "show", id, "--json", "--server", serverURL)

	var issue map[string]interface{}
	if err := json.Unmarshal([]byte(showOut), &issue); err != nil {
		t.Fatalf("failed to parse show --json output: %v\noutput: %s", err, showOut)
	}

	status, _ := issue["status"].(string)
	if !strings.EqualFold(status, "closed") {
		t.Errorf("expected status 'closed', got %q", status)
	}
}

// TestCloseMultipleIssues verifies that multiple issues can be closed in a
// single arc close command.
func TestCloseMultipleIssues(t *testing.T) {
	home := setupHome(t)
	ws := fmt.Sprintf("close-multi-%d", uniqueCounter())

	arcCmdSuccess(t, home, "init", ws, "--server", serverURL)

	ids := make([]string, 3)
	for i := 0; i < 3; i++ {
		title := fmt.Sprintf("Multi-close issue %d", i+1)
		createOut := arcCmdSuccess(t, home, "create", title, "--type", "task", "--server", serverURL)
		id, ok := extractID(createOut)
		if !ok {
			t.Fatalf("could not extract issue ID from create output: %s", createOut)
		}
		ids[i] = id
	}

	// Close all three in one command.
	args := append([]string{"close"}, ids...)
	args = append(args, "--server", serverURL)
	arcCmdSuccess(t, home, args...)

	// Verify all three are closed.
	for _, id := range ids {
		showOut := arcCmdSuccess(t, home, "show", id, "--json", "--server", serverURL)

		var issue map[string]interface{}
		if err := json.Unmarshal([]byte(showOut), &issue); err != nil {
			t.Fatalf("failed to parse show --json output for %s: %v\noutput: %s", id, err, showOut)
		}

		status, _ := issue["status"].(string)
		if !strings.EqualFold(status, "closed") {
			t.Errorf("expected issue %s status 'closed', got %q", id, status)
		}
	}
}

// TestCloseCascade verifies that --cascade closes a parent and all its open
// children, and that closing a parent without --cascade fails when children
// are open.
func TestCloseCascade(t *testing.T) {
	home := setupHome(t)
	ws := fmt.Sprintf("close-cascade-%d", uniqueCounter())

	arcCmdSuccess(t, home, "init", ws, "--server", serverURL)

	// Create parent issue.
	parentOut := arcCmdSuccess(t, home, "create", "Cascade parent", "--type", "epic", "--server", serverURL)
	parentID, ok := extractID(parentOut)
	if !ok {
		t.Fatalf("could not extract parent ID from create output: %s", parentOut)
	}

	// Create two child issues.
	child1Out := arcCmdSuccess(t, home, "create", "Cascade child 1", "--type", "task", "--server", serverURL)
	child1ID, ok := extractID(child1Out)
	if !ok {
		t.Fatalf("could not extract child1 ID from create output: %s", child1Out)
	}

	child2Out := arcCmdSuccess(t, home, "create", "Cascade child 2", "--type", "task", "--server", serverURL)
	child2ID, ok := extractID(child2Out)
	if !ok {
		t.Fatalf("could not extract child2 ID from create output: %s", child2Out)
	}

	// Add parent-child dependencies.
	arcCmdSuccess(t, home, "dep", "add", child1ID, parentID, "--type", "parent-child", "--server", serverURL)
	arcCmdSuccess(t, home, "dep", "add", child2ID, parentID, "--type", "parent-child", "--server", serverURL)

	// Attempt to close parent WITHOUT --cascade — should fail.
	failOut, err := arcCmd(t, home, "close", parentID, "--server", serverURL)
	if err == nil {
		t.Errorf("expected close without --cascade to fail when children are open, but it succeeded: %s", failOut)
	}
	if !strings.Contains(strings.ToLower(failOut), "child") {
		t.Errorf("expected error to mention open children, got: %s", failOut)
	}

	// Close with --cascade — should succeed.
	arcCmdSuccess(t, home, "close", parentID, "--cascade", "--server", serverURL)

	// Verify parent and both children are closed.
	for _, id := range []string{parentID, child1ID, child2ID} {
		showOut := arcCmdSuccess(t, home, "show", id, "--json", "--server", serverURL)

		var issue map[string]interface{}
		if err := json.Unmarshal([]byte(showOut), &issue); err != nil {
			t.Fatalf("failed to parse show --json output for %s: %v\noutput: %s", id, err, showOut)
		}

		status, _ := issue["status"].(string)
		if !strings.EqualFold(status, "closed") {
			t.Errorf("expected issue %s status 'closed', got %q", id, status)
		}
	}
}

// TestCloseAlreadyClosed verifies that closing an already-closed issue does
// not crash or produce an unexpected error.
func TestCloseAlreadyClosed(t *testing.T) {
	home := setupHome(t)
	ws := fmt.Sprintf("close-already-%d", uniqueCounter())

	arcCmdSuccess(t, home, "init", ws, "--server", serverURL)

	createOut := arcCmdSuccess(t, home, "create", "Already closed issue", "--type", "task", "--server", serverURL)
	id, ok := extractID(createOut)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", createOut)
	}

	// Close it the first time.
	arcCmdSuccess(t, home, "close", id, "--server", serverURL)

	// Close it again — should not crash. We accept either success or a
	// non-panic error.
	output, err := arcCmd(t, home, "close", id, "--server", serverURL)
	if err != nil {
		// Command returned non-zero, which is acceptable as long as it
		// didn't crash (panics would show a stack trace).
		if strings.Contains(output, "panic") || strings.Contains(output, "goroutine") {
			t.Fatalf("closing an already-closed issue caused a panic: %s", output)
		}
	}
	// Either way, verify the issue is still closed.
	showOut := arcCmdSuccess(t, home, "show", id, "--json", "--server", serverURL)

	var issue map[string]interface{}
	if err := json.Unmarshal([]byte(showOut), &issue); err != nil {
		t.Fatalf("failed to parse show --json output: %v\noutput: %s", err, showOut)
	}

	status, _ := issue["status"].(string)
	if !strings.EqualFold(status, "closed") {
		t.Errorf("expected status 'closed', got %q", status)
	}
}

// TestCloseNonexistentIssue verifies that closing a nonexistent issue
// returns an error.
func TestCloseNonexistentIssue(t *testing.T) {
	home := setupHome(t)
	ws := fmt.Sprintf("close-nonexist-%d", uniqueCounter())

	arcCmdSuccess(t, home, "init", ws, "--server", serverURL)

	output, err := arcCmd(t, home, "close", "nonexistent-id-xyz", "--server", serverURL)
	if err == nil {
		t.Errorf("expected close of nonexistent issue to fail, but it succeeded: %s", output)
	}
	if len(output) == 0 {
		t.Error("expected error output when closing nonexistent issue, got empty output")
	}
}
