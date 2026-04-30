package gittest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sentiolabs/arc/internal/testutil/gittest"
)

// TestRun verifies that Run executes git commands successfully in the given dir.
func TestRun(t *testing.T) {
	dir := t.TempDir()
	// git init should succeed
	gittest.Run(t, dir, "init", "-q")
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Fatalf("expected .git directory after init: %v", err)
	}
}

// TestRun_EmptyWorkdir verifies that Run with "" uses the current process CWD.
func TestRun_EmptyWorkdir(t *testing.T) {
	// Just ensure it doesn't panic or fail on a simple no-op command.
	// git --version works from any directory.
	gittest.Run(t, "", "version")
}

// TestInitRepo verifies that InitRepo creates a git repository with one commit.
func TestInitRepo(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "repo")
	gittest.InitRepo(t, dir)

	// The dir must exist and have a .git directory.
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Fatalf("expected .git directory after InitRepo: %v", err)
	}

	// There must be at least one commit (so worktree add works).
	gittest.Run(t, dir, "log", "--oneline")
}

// TestAddWorktree verifies that AddWorktree creates a linked worktree.
func TestAddWorktree(t *testing.T) {
	root := t.TempDir()
	mainDir := filepath.Join(root, "main")
	wtDir := filepath.Join(root, "wt")

	gittest.InitRepo(t, mainDir)
	gittest.AddWorktree(t, mainDir, wtDir, "feature-test")

	// The worktree dir must exist and contain a .git file (not directory).
	info, err := os.Stat(filepath.Join(wtDir, ".git"))
	if err != nil {
		t.Fatalf("expected .git in worktree: %v", err)
	}
	if info.IsDir() {
		t.Errorf(".git in linked worktree should be a file, not a directory")
	}
}
