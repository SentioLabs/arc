package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndLoadConfig(t *testing.T) {
	// Use a temp dir as the arc home
	tmpDir := t.TempDir()

	cfg := &Config{
		WorkspaceID:   "ws-abc123",
		WorkspaceName: "my-project",
		ProjectRoot:   "/home/user/my-project",
	}

	err := WriteConfig(tmpDir, "/home/user/my-project", cfg)
	if err != nil {
		t.Fatalf("WriteConfig failed: %v", err)
	}

	loaded, err := LoadConfig(tmpDir, "/home/user/my-project")
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

	_, err := LoadConfig(tmpDir, "/nonexistent/path")
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
			result := PathToProjectDir(tc.path)
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

	root, err := FindProjectRoot(nested)
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
	cfg := &Config{
		WorkspaceID:   "ws-test",
		WorkspaceName: "test",
		ProjectRoot:   tmpDir,
	}
	if err := WriteConfig(arcHome, tmpDir, cfg); err != nil {
		t.Fatal(err)
	}

	// Create nested dir
	nested := filepath.Join(tmpDir, "sub", "deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	root, err := FindProjectRootWithArcHome(nested, arcHome)
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

	_, err := FindProjectRootWithArcHome(tmpDir, arcHome)
	if err == nil {
		t.Fatal("FindProjectRootWithArcHome should fail when no project found")
	}
}
