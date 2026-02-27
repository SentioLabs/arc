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

func TestMigrateLegacyConfig(t *testing.T) {
	tmpDir := t.TempDir()
	arcHome := t.TempDir()

	// Create a legacy .arc.json
	legacyContent := `{"workspace_id": "ws-old123", "workspace_name": "legacy-project"}`
	legacyPath := filepath.Join(tmpDir, ".arc.json")
	if err := os.WriteFile(legacyPath, []byte(legacyContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := MigrateLegacyConfig(tmpDir, arcHome)
	if err != nil {
		t.Fatalf("MigrateLegacyConfig failed: %v", err)
	}

	if cfg.WorkspaceID != "ws-old123" {
		t.Errorf("WorkspaceID = %q, want %q", cfg.WorkspaceID, "ws-old123")
	}
	if cfg.WorkspaceName != "legacy-project" {
		t.Errorf("WorkspaceName = %q, want %q", cfg.WorkspaceName, "legacy-project")
	}

	// Verify the new config was written
	loaded, err := LoadConfig(arcHome, tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig after migration failed: %v", err)
	}
	if loaded.WorkspaceID != "ws-old123" {
		t.Errorf("Migrated config WorkspaceID = %q, want %q", loaded.WorkspaceID, "ws-old123")
	}
}

func TestFindLegacyConfigWalksUp(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .arc.json in the root
	legacyContent := `{"workspace_id": "ws-walk", "workspace_name": "walk-test"}`
	if err := os.WriteFile(filepath.Join(tmpDir, ".arc.json"), []byte(legacyContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Search from nested dir
	nested := filepath.Join(tmpDir, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	path, err := FindLegacyConfig(nested)
	if err != nil {
		t.Fatalf("FindLegacyConfig failed: %v", err)
	}

	expected := filepath.Join(tmpDir, ".arc.json")
	if path != expected {
		t.Errorf("FindLegacyConfig = %q, want %q", path, expected)
	}
}

func TestCleanupWorkspaceConfigs(t *testing.T) {
	arcHome := t.TempDir()

	// Create three project configs: two for workspace "ws-target", one for "ws-other"
	cfg1 := &Config{WorkspaceID: "ws-target", WorkspaceName: "proj-a", ProjectRoot: "/home/user/proj-a"}
	cfg2 := &Config{WorkspaceID: "ws-target", WorkspaceName: "proj-b", ProjectRoot: "/home/user/proj-b"}
	cfg3 := &Config{WorkspaceID: "ws-other", WorkspaceName: "proj-c", ProjectRoot: "/home/user/proj-c"}

	if err := WriteConfig(arcHome, "/home/user/proj-a", cfg1); err != nil {
		t.Fatal(err)
	}
	if err := WriteConfig(arcHome, "/home/user/proj-b", cfg2); err != nil {
		t.Fatal(err)
	}
	if err := WriteConfig(arcHome, "/home/user/proj-c", cfg3); err != nil {
		t.Fatal(err)
	}

	// Clean up configs for ws-target
	removed, err := CleanupWorkspaceConfigs(arcHome, "ws-target")
	if err != nil {
		t.Fatalf("CleanupWorkspaceConfigs failed: %v", err)
	}

	if removed != 2 {
		t.Errorf("removed = %d, want 2", removed)
	}

	// ws-target configs should be gone
	if _, err := LoadConfig(arcHome, "/home/user/proj-a"); err == nil {
		t.Error("proj-a config should have been removed")
	}
	if _, err := LoadConfig(arcHome, "/home/user/proj-b"); err == nil {
		t.Error("proj-b config should have been removed")
	}

	// ws-other config should still exist
	loaded, err := LoadConfig(arcHome, "/home/user/proj-c")
	if err != nil {
		t.Fatalf("proj-c config should still exist: %v", err)
	}
	if loaded.WorkspaceID != "ws-other" {
		t.Errorf("proj-c WorkspaceID = %q, want %q", loaded.WorkspaceID, "ws-other")
	}
}

func TestCleanupWorkspaceConfigsNoMatch(t *testing.T) {
	arcHome := t.TempDir()

	// Create a config for a different workspace
	cfg := &Config{WorkspaceID: "ws-keep", WorkspaceName: "keep", ProjectRoot: "/home/user/keep"}
	if err := WriteConfig(arcHome, "/home/user/keep", cfg); err != nil {
		t.Fatal(err)
	}

	removed, err := CleanupWorkspaceConfigs(arcHome, "ws-nonexistent")
	if err != nil {
		t.Fatalf("CleanupWorkspaceConfigs failed: %v", err)
	}

	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}

	// Original config should still exist
	if _, err := LoadConfig(arcHome, "/home/user/keep"); err != nil {
		t.Fatalf("config should still exist: %v", err)
	}
}
