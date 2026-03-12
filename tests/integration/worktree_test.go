//go:build integration

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runGit executes a git command in the given directory and returns stdout.
// It fails the test if the command exits with a non-zero status.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\noutput: %s", args, err, out)
	}
	return string(out)
}

// TestWorktreeAutoDetection verifies that arc resolves the project
// correctly when run from a git worktree linked to a repo that has an
// arc project configured.
func TestWorktreeAutoDetection(t *testing.T) {
	home := setupHome(t)

	// Create a parent temp directory that holds both the main repo and the worktree.
	parentDir := t.TempDir()
	repoDir := filepath.Join(parentDir, "main-repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}

	// Initialize a git repo.
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.email", "test@test.com")
	runGit(t, repoDir, "config", "user.name", "Test")

	// Create an initial commit (git worktree requires at least one commit).
	emptyFile := filepath.Join(repoDir, "README.md")
	if err := os.WriteFile(emptyFile, []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("write empty file: %v", err)
	}
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "initial commit")

	// Initialize an arc project in the main repo.
	arcCmdInDirSuccess(t, home, repoDir, "init", "test-worktree", "--server", serverURL)

	// Create a git worktree.
	worktreeDir := filepath.Join(parentDir, "wt-test")
	runGit(t, repoDir, "worktree", "add", worktreeDir, "-b", "wt-branch")

	// From the worktree, `arc which` should resolve the project.
	whichOut := arcCmdInDirSuccess(t, home, worktreeDir, "which", "--server", serverURL)
	if !strings.Contains(strings.ToLower(whichOut), "test-worktree") {
		t.Errorf("expected 'test-worktree' project in which output from worktree, got: %s", whichOut)
	}
}
