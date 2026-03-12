package project_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sentiolabs/arc/internal/project"
)

func TestWriteAndLoadConfig(t *testing.T) {
	// Use a temp dir as the arc home
	tmpDir := t.TempDir()

	cfg := &project.Config{
		WorkspaceID:   "ws-abc123",
		WorkspaceName: "my-project",
		ProjectRoot:   "/home/user/my-project",
	}

	err := project.WriteConfig(tmpDir, "/home/user/my-project", cfg)
	if err != nil {
		t.Fatalf("WriteConfig failed: %v", err)
	}

	loaded, err := project.LoadConfig(tmpDir, "/home/user/my-project")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if loaded.WorkspaceID != cfg.WorkspaceID {
		t.Errorf("WorkspaceID = %q, want %q", loaded.WorkspaceID, cfg.WorkspaceID)
	}
	if loaded.WorkspaceName != cfg.WorkspaceName {
		t.Errorf("WorkspaceName = %q, want %q", loaded.WorkspaceName, cfg.WorkspaceName)
	}
	if loaded.ProjectRoot != cfg.ProjectRoot {
		t.Errorf("ProjectRoot = %q, want %q", loaded.ProjectRoot, cfg.ProjectRoot)
	}
}

func TestLoadConfigNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := project.LoadConfig(tmpDir, "/nonexistent/path")
	if err == nil {
		t.Fatal("LoadConfig should fail for nonexistent project")
	}
}

func TestPathToProjectDir(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"simple path", "/home/user/project", "-home-user-project"},
		{"deep path", "/home/user/dev/org/repo", "-home-user-dev-org-repo"},
		{"root", "/", "-"},
		{"trailing slash stripped", "/home/user/project/", "-home-user-project"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := project.PathToProjectDir(tc.path)
			if result != tc.expected {
				t.Errorf("PathToProjectDir(%q) = %q, want %q", tc.path, result, tc.expected)
			}
		})
	}
}

func TestFindProjectRootViaGit(t *testing.T) {
	// Create a temp dir with a .git directory
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a nested subdirectory
	nested := filepath.Join(tmpDir, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	root, err := project.FindProjectRoot(nested)
	if err != nil {
		t.Fatalf("FindProjectRoot failed: %v", err)
	}

	if root != tmpDir {
		t.Errorf("FindProjectRoot = %q, want %q", root, tmpDir)
	}
}

func TestFindProjectRootViaPrefixWalk(t *testing.T) {
	// No .git dir — should fall back to prefix walk
	tmpDir := t.TempDir()
	arcHome := t.TempDir()

	// Register a project at tmpDir
	cfg := &project.Config{
		WorkspaceID:   "ws-test",
		WorkspaceName: "test",
		ProjectRoot:   tmpDir,
	}
	if err := project.WriteConfig(arcHome, tmpDir, cfg); err != nil {
		t.Fatal(err)
	}

	// Create nested dir
	nested := filepath.Join(tmpDir, "sub", "deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	root, err := project.FindProjectRootWithArcHome(nested, arcHome)
	if err != nil {
		t.Fatalf("FindProjectRootWithArcHome failed: %v", err)
	}

	if root != tmpDir {
		t.Errorf("FindProjectRootWithArcHome = %q, want %q", root, tmpDir)
	}
}

func TestFindProjectRootNoMatch(t *testing.T) {
	tmpDir := t.TempDir()
	arcHome := t.TempDir()

	_, err := project.FindProjectRootWithArcHome(tmpDir, arcHome)
	if err == nil {
		t.Fatal("FindProjectRootWithArcHome should fail when no project found")
	}
}

func TestCleanupWorkspaceConfigs(t *testing.T) {
	arcHome := t.TempDir()

	// Create three project configs: two for workspace "ws-target", one for "ws-other"
	cfg1 := &project.Config{WorkspaceID: "ws-target", WorkspaceName: "proj-a", ProjectRoot: "/home/user/proj-a"}
	cfg2 := &project.Config{WorkspaceID: "ws-target", WorkspaceName: "proj-b", ProjectRoot: "/home/user/proj-b"}
	cfg3 := &project.Config{WorkspaceID: "ws-other", WorkspaceName: "proj-c", ProjectRoot: "/home/user/proj-c"}

	if err := project.WriteConfig(arcHome, "/home/user/proj-a", cfg1); err != nil {
		t.Fatal(err)
	}
	if err := project.WriteConfig(arcHome, "/home/user/proj-b", cfg2); err != nil {
		t.Fatal(err)
	}
	if err := project.WriteConfig(arcHome, "/home/user/proj-c", cfg3); err != nil {
		t.Fatal(err)
	}

	// Clean up configs for ws-target
	removed, err := project.CleanupWorkspaceConfigs(arcHome, "ws-target")
	if err != nil {
		t.Fatalf("CleanupWorkspaceConfigs failed: %v", err)
	}

	if removed != 2 {
		t.Errorf("removed = %d, want 2", removed)
	}

	// ws-target configs should be gone
	if _, err := project.LoadConfig(arcHome, "/home/user/proj-a"); err == nil {
		t.Error("proj-a config should have been removed")
	}
	if _, err := project.LoadConfig(arcHome, "/home/user/proj-b"); err == nil {
		t.Error("proj-b config should have been removed")
	}

	// ws-other config should still exist
	loaded, err := project.LoadConfig(arcHome, "/home/user/proj-c")
	if err != nil {
		t.Fatalf("proj-c config should still exist: %v", err)
	}
	if loaded.WorkspaceID != "ws-other" {
		t.Errorf("proj-c WorkspaceID = %q, want %q", loaded.WorkspaceID, "ws-other")
	}
}

func TestDetectWorktreeMainRepo_Worktree(t *testing.T) {
	tmpDir := t.TempDir()

	// Simulate a worktree: .git is a file, not a directory
	mainRepo := filepath.Join(tmpDir, "main-repo")
	if err := os.MkdirAll(mainRepo, 0o755); err != nil {
		t.Fatal(err)
	}

	worktree := filepath.Join(tmpDir, "worktree")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}

	gitdirPath := filepath.Join(mainRepo, ".git", "worktrees", "feature-x")
	if err := os.MkdirAll(gitdirPath, 0o755); err != nil {
		t.Fatal(err)
	}

	gitFileContent := fmt.Sprintf("gitdir: %s\n", gitdirPath)
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte(gitFileContent), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := project.DetectWorktreeMainRepo(worktree)
	if err != nil {
		t.Fatalf("DetectWorktreeMainRepo failed: %v", err)
	}

	// NormalizePath resolves symlinks, so we need to normalize the expected path too
	expected := project.NormalizePath(mainRepo)
	if result != expected {
		t.Errorf("DetectWorktreeMainRepo = %q, want %q", result, expected)
	}
}

func TestDetectWorktreeMainRepo_NotWorktree(t *testing.T) {
	tmpDir := t.TempDir()

	// .git is a directory (normal repo, not a worktree)
	if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := project.DetectWorktreeMainRepo(tmpDir)
	if err != nil {
		t.Fatalf("DetectWorktreeMainRepo failed: %v", err)
	}
	if result != "" {
		t.Errorf("DetectWorktreeMainRepo = %q, want empty string", result)
	}
}

func TestDetectWorktreeMainRepo_NoGit(t *testing.T) {
	tmpDir := t.TempDir()

	result, err := project.DetectWorktreeMainRepo(tmpDir)
	if err != nil {
		t.Fatalf("DetectWorktreeMainRepo failed: %v", err)
	}
	if result != "" {
		t.Errorf("DetectWorktreeMainRepo = %q, want empty string", result)
	}
}

func TestCleanupWorkspaceConfigsNoMatch(t *testing.T) {
	arcHome := t.TempDir()

	// Create a config for a different workspace
	cfg := &project.Config{WorkspaceID: "ws-keep", WorkspaceName: "keep", ProjectRoot: "/home/user/keep"}
	if err := project.WriteConfig(arcHome, "/home/user/keep", cfg); err != nil {
		t.Fatal(err)
	}

	removed, err := project.CleanupWorkspaceConfigs(arcHome, "ws-nonexistent")
	if err != nil {
		t.Fatalf("CleanupWorkspaceConfigs failed: %v", err)
	}

	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}

	// Original config should still exist
	if _, err := project.LoadConfig(arcHome, "/home/user/keep"); err != nil {
		t.Fatalf("config should still exist: %v", err)
	}
}
