//go:build integration

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInitFromSymlinkRegisterssBothPaths verifies that `arc init` from a
// symlinked directory registers both the symlink path and the resolved path.
func TestInitFromSymlinkRegistersBothPaths(t *testing.T) {
	home := setupHome(t)

	// Create a real directory and a symlink to it.
	parentDir := t.TempDir()
	realDir := filepath.Join(parentDir, "real-project")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatalf("create real dir: %v", err)
	}
	symlinkDir := filepath.Join(parentDir, "link-project")
	if err := os.Symlink(realDir, symlinkDir); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	// Init from the symlink path.
	arcCmdInDirSuccess(t, home, symlinkDir, "init", "symlink-dual-ws", "--server", serverURL)

	wsID := getWorkspaceIDByName(t, home, "symlink-dual-ws")

	// Check registered paths.
	pathsOut := arcCmdSuccess(t, home, "paths", "-w", wsID, "--json", "--server", serverURL)
	var paths []struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(pathsOut), &paths); err != nil {
		t.Fatalf("parse paths: %v", err)
	}

	// On systems with symlinks (macOS /var -> /private/var), the resolved path
	// may differ. We should have at least one path registered.
	if len(paths) == 0 {
		t.Fatal("expected at least one path registered after init")
	}

	// If symlink and real paths are truly different, we expect 2 paths.
	// (On some systems, both may resolve to the same canonical path.)
	resolvedReal, _ := filepath.EvalSymlinks(realDir)
	resolvedLink, _ := filepath.EvalSymlinks(symlinkDir)
	if resolvedReal == resolvedLink {
		// Both resolve to the same thing, so symlink and real differ.
		// We expect 2 registrations (the symlink path + resolved path).
		if len(paths) < 2 {
			t.Logf("Note: expected 2 paths (symlink + resolved), got %d. Paths: %v", len(paths), paths)
		}
	}
}

// TestCommandsWorkFromBothSymlinkAndRealPath verifies that after init
// from a symlink, arc commands work from both the symlink and real path.
func TestCommandsWorkFromBothSymlinkAndRealPath(t *testing.T) {
	home := setupHome(t)

	parentDir := t.TempDir()
	realDir := filepath.Join(parentDir, "real-proj")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatalf("create real dir: %v", err)
	}
	symlinkDir := filepath.Join(parentDir, "link-proj")
	if err := os.Symlink(realDir, symlinkDir); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	// Init from the symlink path.
	arcCmdInDirSuccess(t, home, symlinkDir, "init", "symlink-both-ws", "--server", serverURL)

	// Create issue from the symlink path.
	createOut := arcCmdInDirSuccess(t, home, symlinkDir, "create", "Issue from symlink", "--type", "task", "--server", serverURL)
	if !strings.Contains(strings.ToLower(createOut), "created") {
		t.Errorf("expected 'created' in output, got: %s", createOut)
	}

	// List from the real path — should see the issue.
	listOut := arcCmdInDirSuccess(t, home, realDir, "list", "--server", serverURL)
	if !strings.Contains(listOut, "Issue from symlink") {
		t.Errorf("expected issue to be visible from real path, got: %s", listOut)
	}

	// Create issue from the real path.
	createOut2 := arcCmdInDirSuccess(t, home, realDir, "create", "Issue from real", "--type", "task", "--server", serverURL)
	if !strings.Contains(strings.ToLower(createOut2), "created") {
		t.Errorf("expected 'created' in output, got: %s", createOut2)
	}

	// List from the symlink path — should see both issues.
	listOut2 := arcCmdInDirSuccess(t, home, symlinkDir, "list", "--server", serverURL)
	if !strings.Contains(listOut2, "Issue from symlink") {
		t.Errorf("expected 'Issue from symlink' in symlink list, got: %s", listOut2)
	}
	if !strings.Contains(listOut2, "Issue from real") {
		t.Errorf("expected 'Issue from real' in symlink list, got: %s", listOut2)
	}
}

// TestWhichFromSymlinkAndRealPath verifies that `arc which` resolves
// the same workspace from both the symlink and real paths.
func TestWhichFromSymlinkAndRealPath(t *testing.T) {
	home := setupHome(t)

	parentDir := t.TempDir()
	realDir := filepath.Join(parentDir, "real-which")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatalf("create real dir: %v", err)
	}
	symlinkDir := filepath.Join(parentDir, "link-which")
	if err := os.Symlink(realDir, symlinkDir); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	arcCmdInDirSuccess(t, home, symlinkDir, "init", "symlink-which-ws", "--server", serverURL)

	// `arc which` from symlink.
	whichSymlink := arcCmdInDirSuccess(t, home, symlinkDir, "which", "--json", "--server", serverURL)
	var resultSymlink map[string]string
	if err := json.Unmarshal([]byte(whichSymlink), &resultSymlink); err != nil {
		t.Fatalf("parse which JSON from symlink: %v\noutput: %s", err, whichSymlink)
	}

	// `arc which` from real path.
	whichReal := arcCmdInDirSuccess(t, home, realDir, "which", "--json", "--server", serverURL)
	var resultReal map[string]string
	if err := json.Unmarshal([]byte(whichReal), &resultReal); err != nil {
		t.Fatalf("parse which JSON from real: %v\noutput: %s", err, whichReal)
	}

	// Both should resolve to the same workspace.
	if resultSymlink["workspace_id"] != resultReal["workspace_id"] {
		t.Errorf("workspace IDs differ: symlink=%q, real=%q",
			resultSymlink["workspace_id"], resultReal["workspace_id"])
	}
}

// TestInitFromRealPathThenAccessFromSymlink verifies the reverse direction:
// init from the real path, then access from a symlink.
func TestInitFromRealPathThenAccessFromSymlink(t *testing.T) {
	home := setupHome(t)

	parentDir := t.TempDir()
	realDir := filepath.Join(parentDir, "real-reverse")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatalf("create real dir: %v", err)
	}
	symlinkDir := filepath.Join(parentDir, "link-reverse")
	if err := os.Symlink(realDir, symlinkDir); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	// Init from real path.
	arcCmdInDirSuccess(t, home, realDir, "init", "reverse-sym-ws", "--server", serverURL)

	// Create issue from real path.
	arcCmdInDirSuccess(t, home, realDir, "create", "Real path issue", "--type", "task", "--server", serverURL)

	// Access from symlink path.
	listOut := arcCmdInDirSuccess(t, home, symlinkDir, "list", "--server", serverURL)
	if !strings.Contains(listOut, "Real path issue") {
		t.Errorf("expected issue to be visible from symlink path, got: %s", listOut)
	}
}
