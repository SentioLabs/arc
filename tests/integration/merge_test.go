//go:build integration

package integration

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestWorkspaceMerge verifies that merging a source workspace into a target
// moves all issues and deletes the source workspace.
func TestWorkspaceMerge(t *testing.T) {
	home := setupHome(t)

	// Create unique workspace names to avoid collisions with other tests.
	target := fmt.Sprintf("merge-target-%d", time.Now().UnixNano())
	source := fmt.Sprintf("merge-source-%d", time.Now().UnixNano())

	// Create target workspace.
	arcCmdSuccess(t, home, "workspace", "create", target, "--server", serverURL)

	// Create source workspace.
	arcCmdSuccess(t, home, "workspace", "create", source, "--server", serverURL)

	// Init target in a temp dir so we can verify issues later.
	targetDir := t.TempDir()
	arcCmdInDirSuccess(t, home, targetDir, "init", target, "--server", serverURL)

	// Init source in a different temp dir so we can create issues in it.
	sourceDir := t.TempDir()
	arcCmdInDirSuccess(t, home, sourceDir, "init", source, "--server", serverURL)

	// Create 2 issues in the source workspace.
	arcCmdInDirSuccess(t, home, sourceDir, "create", "Merge issue alpha", "--type", "task", "--server", serverURL)
	arcCmdInDirSuccess(t, home, sourceDir, "create", "Merge issue beta", "--type", "task", "--server", serverURL)

	// Merge source into target.
	mergeOut := arcCmdSuccess(t, home, "workspace", "merge", "--into", target, source, "--server", serverURL)

	// Assert output mentions "Merged".
	if !strings.Contains(mergeOut, "Merged") {
		t.Errorf("expected merge output to mention 'Merged', got: %s", mergeOut)
	}

	// Assert source workspace no longer exists in workspace list.
	listOut := arcCmdSuccess(t, home, "workspace", "list", "--server", serverURL)
	if strings.Contains(listOut, source) {
		t.Errorf("source workspace %q should have been deleted after merge, but still appears in list: %s", source, listOut)
	}

	// Assert issues are now visible from the target workspace directory.
	targetList := arcCmdInDirSuccess(t, home, targetDir, "list", "--server", serverURL)
	if !strings.Contains(targetList, "Merge issue alpha") {
		t.Errorf("expected 'Merge issue alpha' in target workspace list, got: %s", targetList)
	}
	if !strings.Contains(targetList, "Merge issue beta") {
		t.Errorf("expected 'Merge issue beta' in target workspace list, got: %s", targetList)
	}
}

// TestWorkspaceMergeBatch verifies that multiple source workspaces can be
// merged into a single target in one command.
func TestWorkspaceMergeBatch(t *testing.T) {
	home := setupHome(t)

	// Create unique workspace names.
	target := fmt.Sprintf("batch-target-%d", time.Now().UnixNano())
	source1 := fmt.Sprintf("batch-src1-%d", time.Now().UnixNano())
	source2 := fmt.Sprintf("batch-src2-%d", time.Now().UnixNano())

	// Create all workspaces.
	arcCmdSuccess(t, home, "workspace", "create", target, "--server", serverURL)
	arcCmdSuccess(t, home, "workspace", "create", source1, "--server", serverURL)
	arcCmdSuccess(t, home, "workspace", "create", source2, "--server", serverURL)

	// Init each workspace in its own temp dir.
	targetDir := t.TempDir()
	arcCmdInDirSuccess(t, home, targetDir, "init", target, "--server", serverURL)

	src1Dir := t.TempDir()
	arcCmdInDirSuccess(t, home, src1Dir, "init", source1, "--server", serverURL)

	src2Dir := t.TempDir()
	arcCmdInDirSuccess(t, home, src2Dir, "init", source2, "--server", serverURL)

	// Create issues in both source workspaces.
	arcCmdInDirSuccess(t, home, src1Dir, "create", "Batch issue from src1", "--type", "task", "--server", serverURL)
	arcCmdInDirSuccess(t, home, src2Dir, "create", "Batch issue from src2", "--type", "task", "--server", serverURL)

	// Merge both sources into target in a single command.
	mergeOut := arcCmdSuccess(t, home, "workspace", "merge", "--into", target, source1, source2, "--server", serverURL)

	// Assert output mentions "Merged".
	if !strings.Contains(mergeOut, "Merged") {
		t.Errorf("expected merge output to mention 'Merged', got: %s", mergeOut)
	}

	// Assert both source workspaces are deleted.
	listOut := arcCmdSuccess(t, home, "workspace", "list", "--server", serverURL)
	if strings.Contains(listOut, source1) {
		t.Errorf("source1 %q should have been deleted after merge, but still appears in list: %s", source1, listOut)
	}
	if strings.Contains(listOut, source2) {
		t.Errorf("source2 %q should have been deleted after merge, but still appears in list: %s", source2, listOut)
	}

	// Assert all issues are in the target.
	targetList := arcCmdInDirSuccess(t, home, targetDir, "list", "--server", serverURL)
	if !strings.Contains(targetList, "Batch issue from src1") {
		t.Errorf("expected 'Batch issue from src1' in target workspace list, got: %s", targetList)
	}
	if !strings.Contains(targetList, "Batch issue from src2") {
		t.Errorf("expected 'Batch issue from src2' in target workspace list, got: %s", targetList)
	}
}
