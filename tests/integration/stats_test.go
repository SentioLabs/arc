//go:build integration

package integration

import (
	"fmt"
	"strings"
	"testing"
)

// TestStatsAllStatuses creates issues in various statuses and verifies the
// stats command reports correct counts for each status category.
func TestStatsAllStatuses(t *testing.T) {
	home := setupHome(t)
	wsName := fmt.Sprintf("stats-all-%d", uniqueCounter())

	arcCmdSuccess(t, home, "init", wsName, "--server", serverURL)

	// Issue 1: leave open.
	arcCmdSuccess(t, home, "create", "Stats open issue", "--type", "task", "--server", serverURL)

	// Issue 2: set to in_progress.
	out2 := arcCmdSuccess(t, home, "create", "Stats in-progress issue", "--type", "task", "--server", serverURL)
	id2, ok := extractID(out2)
	if !ok {
		t.Fatalf("could not extract issue ID: %s", out2)
	}
	arcCmdSuccess(t, home, "update", id2, "--status", "in_progress", "--server", serverURL)

	// Issues 3 and 4: create a dependency so issue 3 is blocked by issue 4.
	out3 := arcCmdSuccess(t, home, "create", "Stats blocked issue", "--type", "task", "--server", serverURL)
	id3, ok := extractID(out3)
	if !ok {
		t.Fatalf("could not extract issue ID: %s", out3)
	}
	out4 := arcCmdSuccess(t, home, "create", "Stats blocker issue", "--type", "task", "--server", serverURL)
	id4, ok := extractID(out4)
	if !ok {
		t.Fatalf("could not extract issue ID: %s", out4)
	}
	arcCmdSuccess(t, home, "dep", "add", id3, id4, "--server", serverURL)

	// Issue 5: close it.
	out5 := arcCmdSuccess(t, home, "create", "Stats closed issue", "--type", "task", "--server", serverURL)
	id5, ok := extractID(out5)
	if !ok {
		t.Fatalf("could not extract issue ID: %s", out5)
	}
	arcCmdSuccess(t, home, "close", id5, "--server", serverURL)

	// Run stats.
	statsOut := arcCmdSuccess(t, home, "stats", "--server", serverURL)

	if !strings.Contains(statsOut, "Total:       5") {
		t.Errorf("expected Total: 5 in stats output, got: %s", statsOut)
	}
	if !strings.Contains(statsOut, "In Progress: 1") {
		t.Errorf("expected In Progress: 1 in stats output, got: %s", statsOut)
	}
	if !strings.Contains(statsOut, "Blocked:     1") {
		t.Errorf("expected Blocked: 1 in stats output, got: %s", statsOut)
	}
	if !strings.Contains(statsOut, "Closed:      1") {
		t.Errorf("expected Closed: 1 in stats output, got: %s", statsOut)
	}
	// Open count should be at least 1 (the open issue; the blocker issue is also open).
	if !strings.Contains(statsOut, "Open:") {
		t.Errorf("expected Open: line in stats output, got: %s", statsOut)
	}
}

// TestStatsAfterCloseAll creates three issues, closes all of them, and
// verifies that Closed equals Total and Open is 0.
func TestStatsAfterCloseAll(t *testing.T) {
	home := setupHome(t)
	wsName := fmt.Sprintf("stats-closeall-%d", uniqueCounter())

	arcCmdSuccess(t, home, "init", wsName, "--server", serverURL)

	ids := make([]string, 3)
	for i := 0; i < 3; i++ {
		out := arcCmdSuccess(t, home, "create", fmt.Sprintf("Close-all issue %d", i+1), "--type", "task", "--server", serverURL)
		id, ok := extractID(out)
		if !ok {
			t.Fatalf("could not extract issue ID: %s", out)
		}
		ids[i] = id
	}

	for _, id := range ids {
		arcCmdSuccess(t, home, "close", id, "--server", serverURL)
	}

	statsOut := arcCmdSuccess(t, home, "stats", "--server", serverURL)

	if !strings.Contains(statsOut, "Total:       3") {
		t.Errorf("expected Total: 3 in stats output, got: %s", statsOut)
	}
	if !strings.Contains(statsOut, "Closed:      3") {
		t.Errorf("expected Closed: 3 in stats output, got: %s", statsOut)
	}
	if !strings.Contains(statsOut, "Open:        0") {
		t.Errorf("expected Open: 0 in stats output, got: %s", statsOut)
	}
}

// TestStatsDeferred creates an issue, updates it to deferred status, and
// verifies the stats command shows Deferred: 1.
func TestStatsDeferred(t *testing.T) {
	home := setupHome(t)
	wsName := fmt.Sprintf("stats-deferred-%d", uniqueCounter())

	arcCmdSuccess(t, home, "init", wsName, "--server", serverURL)

	out := arcCmdSuccess(t, home, "create", "Deferred stats issue", "--type", "task", "--server", serverURL)
	id, ok := extractID(out)
	if !ok {
		t.Fatalf("could not extract issue ID: %s", out)
	}

	arcCmdSuccess(t, home, "update", id, "--status", "deferred", "--server", serverURL)

	statsOut := arcCmdSuccess(t, home, "stats", "--server", serverURL)

	if !strings.Contains(statsOut, "Deferred:    1") {
		t.Errorf("expected Deferred: 1 in stats output, got: %s", statsOut)
	}
}

// TestStatsReadyCount creates three open issues with no dependencies and
// verifies that all three are counted as ready.
func TestStatsReadyCount(t *testing.T) {
	home := setupHome(t)
	wsName := fmt.Sprintf("stats-ready-%d", uniqueCounter())

	arcCmdSuccess(t, home, "init", wsName, "--server", serverURL)

	for i := 0; i < 3; i++ {
		arcCmdSuccess(t, home, "create", fmt.Sprintf("Ready stats issue %d", i+1), "--type", "task", "--server", serverURL)
	}

	statsOut := arcCmdSuccess(t, home, "stats", "--server", serverURL)

	if !strings.Contains(statsOut, "Ready:       3") {
		t.Errorf("expected Ready: 3 in stats output, got: %s", statsOut)
	}
}

// TestStatsMultipleRuns creates issues incrementally and checks stats after
// each addition to verify counts update correctly.
func TestStatsMultipleRuns(t *testing.T) {
	home := setupHome(t)
	wsName := fmt.Sprintf("stats-multi-%d", uniqueCounter())

	arcCmdSuccess(t, home, "init", wsName, "--server", serverURL)

	// After 0 issues.
	statsOut := arcCmdSuccess(t, home, "stats", "--server", serverURL)
	if !strings.Contains(statsOut, "Total:       0") {
		t.Errorf("expected Total: 0 initially, got: %s", statsOut)
	}

	// Create first issue.
	arcCmdSuccess(t, home, "create", "Multi-run issue 1", "--type", "task", "--server", serverURL)

	statsOut = arcCmdSuccess(t, home, "stats", "--server", serverURL)
	if !strings.Contains(statsOut, "Total:       1") {
		t.Errorf("expected Total: 1 after first issue, got: %s", statsOut)
	}
	if !strings.Contains(statsOut, "Open:        1") {
		t.Errorf("expected Open: 1 after first issue, got: %s", statsOut)
	}

	// Create second issue.
	out2 := arcCmdSuccess(t, home, "create", "Multi-run issue 2", "--type", "task", "--server", serverURL)
	id2, ok := extractID(out2)
	if !ok {
		t.Fatalf("could not extract issue ID: %s", out2)
	}

	statsOut = arcCmdSuccess(t, home, "stats", "--server", serverURL)
	if !strings.Contains(statsOut, "Total:       2") {
		t.Errorf("expected Total: 2 after second issue, got: %s", statsOut)
	}

	// Close the second issue.
	arcCmdSuccess(t, home, "close", id2, "--server", serverURL)

	statsOut = arcCmdSuccess(t, home, "stats", "--server", serverURL)
	if !strings.Contains(statsOut, "Total:       2") {
		t.Errorf("expected Total: 2 after closing one, got: %s", statsOut)
	}
	if !strings.Contains(statsOut, "Closed:      1") {
		t.Errorf("expected Closed: 1 after closing one, got: %s", statsOut)
	}
	if !strings.Contains(statsOut, "Open:        1") {
		t.Errorf("expected Open: 1 after closing one, got: %s", statsOut)
	}
}
