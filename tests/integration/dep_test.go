//go:build integration

package integration

import (
	"fmt"
	"strings"
	"testing"
)

// TestDepAddBlocks creates two issues and verifies that adding a "blocks"
// dependency between them is reflected in the show output.
func TestDepAddBlocks(t *testing.T) {
	home := setupHome(t)
	wsName := fmt.Sprintf("dep-blocks-%d", uniqueCounter())

	arcCmdSuccess(t, home, "init", wsName, "--server", serverURL)

	outA := arcCmdSuccess(t, home, "create", "Issue A blocks test", "--type", "task", "--server", serverURL)
	idA, ok := extractID(outA)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", outA)
	}

	outB := arcCmdSuccess(t, home, "create", "Issue B blocks test", "--type", "task", "--server", serverURL)
	idB, ok := extractID(outB)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", outB)
	}

	// A depends on B (B blocks A).
	depOut := arcCmdSuccess(t, home, "dep", "add", idA, idB, "--server", serverURL)
	if !strings.Contains(depOut, "Added") {
		t.Errorf("expected 'Added' in dep add output, got: %s", depOut)
	}

	// Show A should mention B as a dependency.
	showOut := arcCmdSuccess(t, home, "show", idA, "--server", serverURL)
	if !strings.Contains(showOut, idB) {
		t.Errorf("expected show output for %s to mention dependency %s, got: %s", idA, idB, showOut)
	}
}

// TestDepAddRemove creates two issues, adds a dependency, then removes it.
func TestDepAddRemove(t *testing.T) {
	home := setupHome(t)
	wsName := fmt.Sprintf("dep-remove-%d", uniqueCounter())

	arcCmdSuccess(t, home, "init", wsName, "--server", serverURL)

	outA := arcCmdSuccess(t, home, "create", "Issue A remove test", "--type", "task", "--server", serverURL)
	idA, ok := extractID(outA)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", outA)
	}

	outB := arcCmdSuccess(t, home, "create", "Issue B remove test", "--type", "task", "--server", serverURL)
	idB, ok := extractID(outB)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", outB)
	}

	arcCmdSuccess(t, home, "dep", "add", idA, idB, "--server", serverURL)

	removeOut := arcCmdSuccess(t, home, "dep", "remove", idA, idB, "--server", serverURL)
	if !strings.Contains(removeOut, "Removed") {
		t.Errorf("expected 'Removed' in dep remove output, got: %s", removeOut)
	}
}

// TestDepAddParentChild verifies that a parent-child dependency can be created.
func TestDepAddParentChild(t *testing.T) {
	home := setupHome(t)
	wsName := fmt.Sprintf("dep-parent-%d", uniqueCounter())

	arcCmdSuccess(t, home, "init", wsName, "--server", serverURL)

	outParent := arcCmdSuccess(t, home, "create", "Parent issue", "--type", "task", "--server", serverURL)
	idParent, ok := extractID(outParent)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", outParent)
	}

	outChild := arcCmdSuccess(t, home, "create", "Child issue", "--type", "task", "--server", serverURL)
	idChild, ok := extractID(outChild)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", outChild)
	}

	depOut := arcCmdSuccess(t, home, "dep", "add", idChild, idParent, "--type", "parent-child", "--server", serverURL)
	if !strings.Contains(depOut, "Added") {
		t.Errorf("expected 'Added' in dep add output, got: %s", depOut)
	}
	if !strings.Contains(depOut, "parent-child") {
		t.Errorf("expected 'parent-child' type in dep add output, got: %s", depOut)
	}
}

// TestBlockedShowsBlockedIssues creates a dependency and checks that the
// blocked command lists the blocked issue, then verifies it disappears after
// closing the blocker.
func TestBlockedShowsBlockedIssues(t *testing.T) {
	home := setupHome(t)
	wsName := fmt.Sprintf("dep-blocked-%d", uniqueCounter())

	arcCmdSuccess(t, home, "init", wsName, "--server", serverURL)

	outA := arcCmdSuccess(t, home, "create", "Blocked issue A", "--type", "task", "--server", serverURL)
	idA, ok := extractID(outA)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", outA)
	}

	outB := arcCmdSuccess(t, home, "create", "Blocker issue B", "--type", "task", "--server", serverURL)
	idB, ok := extractID(outB)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", outB)
	}

	// A depends on B.
	arcCmdSuccess(t, home, "dep", "add", idA, idB, "--server", serverURL)

	// A should appear in blocked list.
	blockedOut := arcCmdSuccess(t, home, "blocked", "--server", serverURL)
	if !strings.Contains(blockedOut, "Blocked issue A") {
		t.Errorf("expected 'Blocked issue A' in blocked output, got: %s", blockedOut)
	}

	// Close B (the blocker).
	arcCmdSuccess(t, home, "close", idB, "--server", serverURL)

	// A should no longer appear in blocked list.
	blockedOut2 := arcCmdSuccess(t, home, "blocked", "--server", serverURL)
	if strings.Contains(blockedOut2, "Blocked issue A") {
		t.Errorf("expected 'Blocked issue A' to disappear after closing blocker, got: %s", blockedOut2)
	}
}

// TestBlockedEmpty verifies that "arc blocked" reports no blocked issues when
// there are no dependencies.
func TestBlockedEmpty(t *testing.T) {
	home := setupHome(t)
	wsName := fmt.Sprintf("dep-empty-%d", uniqueCounter())

	arcCmdSuccess(t, home, "init", wsName, "--server", serverURL)

	blockedOut := arcCmdSuccess(t, home, "blocked", "--server", serverURL)
	if !strings.Contains(blockedOut, "No blocked issues") {
		t.Errorf("expected 'No blocked issues' in output, got: %s", blockedOut)
	}
}

// TestDepAffectsReady verifies that dependencies influence which issues appear
// in the "ready" list.
func TestDepAffectsReady(t *testing.T) {
	home := setupHome(t)
	wsName := fmt.Sprintf("dep-ready-%d", uniqueCounter())

	arcCmdSuccess(t, home, "init", wsName, "--server", serverURL)

	outA := arcCmdSuccess(t, home, "create", "Ready issue A", "--type", "task", "--server", serverURL)
	idA, ok := extractID(outA)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", outA)
	}

	outB := arcCmdSuccess(t, home, "create", "Ready issue B", "--type", "task", "--server", serverURL)
	idB, ok := extractID(outB)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", outB)
	}

	// A depends on B, so A is blocked.
	arcCmdSuccess(t, home, "dep", "add", idA, idB, "--server", serverURL)

	// A should NOT be in ready list (it's blocked). B should be.
	readyOut := arcCmdSuccess(t, home, "ready", "--server", serverURL)
	if strings.Contains(readyOut, "Ready issue A") {
		t.Errorf("expected 'Ready issue A' to NOT appear in ready (blocked), got: %s", readyOut)
	}
	if !strings.Contains(readyOut, "Ready issue B") {
		t.Errorf("expected 'Ready issue B' to appear in ready, got: %s", readyOut)
	}

	// Close B (the blocker).
	arcCmdSuccess(t, home, "close", idB, "--server", serverURL)

	// Now A should appear in ready.
	readyOut2 := arcCmdSuccess(t, home, "ready", "--server", serverURL)
	if !strings.Contains(readyOut2, "Ready issue A") {
		t.Errorf("expected 'Ready issue A' to appear in ready after closing blocker, got: %s", readyOut2)
	}
}

// TestDepAddRelated verifies that a "related" dependency can be created.
func TestDepAddRelated(t *testing.T) {
	home := setupHome(t)
	wsName := fmt.Sprintf("dep-related-%d", uniqueCounter())

	arcCmdSuccess(t, home, "init", wsName, "--server", serverURL)

	outA := arcCmdSuccess(t, home, "create", "Related issue A", "--type", "task", "--server", serverURL)
	idA, ok := extractID(outA)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", outA)
	}

	outB := arcCmdSuccess(t, home, "create", "Related issue B", "--type", "task", "--server", serverURL)
	idB, ok := extractID(outB)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", outB)
	}

	depOut := arcCmdSuccess(t, home, "dep", "add", idA, idB, "--type", "related", "--server", serverURL)
	if !strings.Contains(depOut, "Added") {
		t.Errorf("expected 'Added' in dep add output, got: %s", depOut)
	}
	if !strings.Contains(depOut, "related") {
		t.Errorf("expected 'related' in dep add output, got: %s", depOut)
	}
}
