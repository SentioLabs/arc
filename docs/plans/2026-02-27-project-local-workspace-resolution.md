# Project-Local Workspace Resolution Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Move workspace binding from in-repo `.arc.json` to per-machine `~/.arc/projects/<path>/config.json`, enabling multi-machine workflows without conflicts.

**Architecture:** Add a `project` package under `internal/` that handles path-to-directory-name conversion, project config read/write, and the three-tier resolution (git walk → prefix walk → legacy fallback). Update `arc init` to write to `~/.arc/projects/` instead of `.arc.json`. Update workspace resolution in `cmd/arc/main.go` to use the new project package.

**Tech Stack:** Go stdlib (`os`, `path/filepath`, `os/exec`, `encoding/json`)

---

### Task 1: Create the `internal/project` package — path conversion

**Files:**
- Create: `internal/project/project.go`
- Test: `internal/project/project_test.go`

This package handles converting absolute filesystem paths to project directory names (the `-home-user-repo` format) and back.

**Step 1: Write the failing tests**

```go
// internal/project/project_test.go
package project

import (
	"testing"
)

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
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/project/ -run TestPathToProjectDir -v`
Expected: FAIL — package doesn't exist yet

**Step 3: Write minimal implementation**

```go
// internal/project/project.go
package project

import (
	"path/filepath"
	"strings"
)

// PathToProjectDir converts an absolute filesystem path to a project directory name.
// Replaces "/" with "-", matching the Claude Code ~/.claude/projects/ convention.
// Example: "/home/user/my-repo" → "-home-user-my-repo"
func PathToProjectDir(absPath string) string {
	cleaned := filepath.Clean(absPath)
	return strings.ReplaceAll(cleaned, string(filepath.Separator), "-")
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/project/ -run TestPathToProjectDir -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/project/project.go internal/project/project_test.go
git commit -m "feat(project): add path-to-project-dir conversion"
```

---

### Task 2: Add project config read/write

**Files:**
- Modify: `internal/project/project.go`
- Test: `internal/project/project_test.go`

Add functions to read/write the per-project `config.json` under `~/.arc/projects/<dir>/`.

**Step 1: Write the failing tests**

Add to `internal/project/project_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/project/ -run TestWriteAndLoadConfig -v`
Expected: FAIL — `Config`, `WriteConfig`, `LoadConfig` not defined

**Step 3: Write minimal implementation**

Add to `internal/project/project.go`:

```go
import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the per-project workspace binding.
type Config struct {
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
	ProjectRoot   string `json:"project_root"`
}

// projectsDir returns the path to the projects directory within arcHome.
func projectsDir(arcHome string) string {
	return filepath.Join(arcHome, "projects")
}

// configPath returns the full path to a project's config.json.
func configPath(arcHome, absProjectPath string) string {
	dirName := PathToProjectDir(absProjectPath)
	return filepath.Join(projectsDir(arcHome), dirName, "config.json")
}

// WriteConfig writes a project config to ~/.arc/projects/<path>/config.json.
func WriteConfig(arcHome, absProjectPath string, cfg *Config) error {
	path := configPath(arcHome, absProjectPath)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create project dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// LoadConfig reads a project config from ~/.arc/projects/<path>/config.json.
func LoadConfig(arcHome, absProjectPath string) (*Config, error) {
	path := configPath(arcHome, absProjectPath)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse project config: %w", err)
	}

	return &cfg, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/project/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/project/project.go internal/project/project_test.go
git commit -m "feat(project): add config read/write for ~/.arc/projects/"
```

---

### Task 3: Add project root resolution (git walk + prefix walk)

**Files:**
- Modify: `internal/project/project.go`
- Test: `internal/project/project_test.go`

Add the three-tier resolution: find project root via git, then prefix walk, then legacy `.arc.json` fallback.

**Step 1: Write the failing tests**

Add to `internal/project/project_test.go`:

```go
import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

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
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/project/ -run TestFindProjectRoot -v`
Expected: FAIL — `FindProjectRoot`, `FindProjectRootWithArcHome` not defined

**Step 3: Write minimal implementation**

Add to `internal/project/project.go`:

```go
// DefaultArcHome returns the default arc home directory (~/.arc).
func DefaultArcHome() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".arc")
}

// FindProjectRoot resolves the project root for the given directory
// using the default arc home (~/.arc).
// Resolution order:
//  1. Git walk — walk up looking for .git/
//  2. Prefix walk — longest-to-shortest match in ~/.arc/projects/
//  3. Returns error if nothing found
func FindProjectRoot(dir string) (string, error) {
	return FindProjectRootWithArcHome(dir, DefaultArcHome())
}

// FindProjectRootWithArcHome resolves the project root using a custom arc home.
// This is the testable version of FindProjectRoot.
func FindProjectRootWithArcHome(dir string, arcHome string) (string, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}

	// Strategy 1: Walk up looking for .git/
	if root, err := findGitRoot(absDir); err == nil {
		return root, nil
	}

	// Strategy 2: Prefix walk (longest to shortest)
	if root, err := findByPrefixWalk(absDir, arcHome); err == nil {
		return root, nil
	}

	return "", fmt.Errorf("no project found for %s\n  Run 'arc init' to set up a workspace", absDir)
}

// findGitRoot walks up from dir looking for a .git directory.
func findGitRoot(dir string) (string, error) {
	current := dir
	for {
		gitPath := filepath.Join(current, ".git")
		if info, err := os.Stat(gitPath); err == nil && info.IsDir() {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("no .git found")
		}
		current = parent
	}
}

// findByPrefixWalk converts dir to the project dir format and strips trailing
// segments (longest to shortest) looking for a match in ~/.arc/projects/.
func findByPrefixWalk(dir string, arcHome string) (string, error) {
	projDir := projectsDir(arcHome)
	current := dir

	for {
		dirName := PathToProjectDir(current)
		candidate := filepath.Join(projDir, dirName, "config.json")
		if _, err := os.Stat(candidate); err == nil {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("no registered project found")
		}
		current = parent
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/project/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/project/project.go internal/project/project_test.go
git commit -m "feat(project): add project root resolution (git walk + prefix walk)"
```

---

### Task 4: Add legacy `.arc.json` migration

**Files:**
- Modify: `internal/project/project.go`
- Test: `internal/project/project_test.go`

Add a function that finds a legacy `.arc.json`, reads it, migrates to the new format, and returns the config.

**Step 1: Write the failing test**

Add to `internal/project/project_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/project/ -run TestMigrate -v`
Expected: FAIL — `MigrateLegacyConfig`, `FindLegacyConfig` not defined

**Step 3: Write minimal implementation**

Add to `internal/project/project.go`:

```go
// legacyConfig matches the old .arc.json structure.
type legacyConfig struct {
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
}

// FindLegacyConfig searches for .arc.json starting from dir and walking up.
func FindLegacyConfig(dir string) (string, error) {
	current, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}

	for {
		candidate := filepath.Join(current, ".arc.json")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", os.ErrNotExist
		}
		current = parent
	}
}

// MigrateLegacyConfig reads a .arc.json from projectRoot, creates the new
// config under arcHome, and returns the migrated config.
// Does NOT delete the original .arc.json.
func MigrateLegacyConfig(projectRoot, arcHome string) (*Config, error) {
	legacyPath := filepath.Join(projectRoot, ".arc.json")

	data, err := os.ReadFile(legacyPath)
	if err != nil {
		return nil, fmt.Errorf("read legacy config: %w", err)
	}

	var legacy legacyConfig
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("parse legacy config: %w", err)
	}

	cfg := &Config{
		WorkspaceID:   legacy.WorkspaceID,
		WorkspaceName: legacy.WorkspaceName,
		ProjectRoot:   projectRoot,
	}

	if err := WriteConfig(arcHome, projectRoot, cfg); err != nil {
		return nil, fmt.Errorf("write migrated config: %w", err)
	}

	return cfg, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/project/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/project/project.go internal/project/project_test.go
git commit -m "feat(project): add legacy .arc.json migration"
```

---

### Task 5: Update `resolveWorkspace` in `cmd/arc/main.go`

**Files:**
- Modify: `cmd/arc/main.go:114-229` (the `localConfig`, `loadLocalConfig`, `findProjectConfig`, `resolveWorkspace` functions and `WorkspaceSource` type)

Replace the `.arc.json`-based resolution with the new project package. Keep the `-w` flag as highest priority.

**Step 1: Update the code**

Update `WorkspaceSource` constants and `resolveWorkspace()`:

```go
// WorkspaceSource indicates how the workspace was resolved
type WorkspaceSource int

const (
	WorkspaceSourceFlag    WorkspaceSource = iota
	WorkspaceSourceProject                         // ~/.arc/projects/<path>/config.json
	WorkspaceSourceLegacy                          // .arc.json (migrated)
)

func (s WorkspaceSource) String() string {
	switch s {
	case WorkspaceSourceFlag:
		return "command line flag (-w)"
	case WorkspaceSourceProject:
		return "~/.arc/projects/ (local)"
	case WorkspaceSourceLegacy:
		return ".arc.json (legacy, migrated)"
	default:
		return "unknown"
	}
}
```

Replace `resolveWorkspace` body to use the new project package:

```go
func resolveWorkspace() (wsID string, source WorkspaceSource, warning string, err error) {
	// Priority 1: CLI flag (explicit override)
	if workspaceID != "" {
		return workspaceID, WorkspaceSourceFlag, "", nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", 0, "", fmt.Errorf("get current directory: %w", err)
	}

	arcHome := project.DefaultArcHome()

	// Priority 2: Find project root and load config from ~/.arc/projects/
	projectRoot, rootErr := project.FindProjectRootWithArcHome(cwd, arcHome)
	if rootErr == nil {
		cfg, cfgErr := project.LoadConfig(arcHome, projectRoot)
		if cfgErr == nil && cfg.WorkspaceID != "" {
			// Validate workspace exists on server
			c, clientErr := getClient()
			if clientErr == nil {
				_, wsErr := c.GetWorkspace(cfg.WorkspaceID)
				if wsErr != nil {
					return "", 0, "", fmt.Errorf("workspace '%s' (%s) not found on server\n  Run 'arc init' to reconfigure this directory",
						cfg.WorkspaceName, cfg.WorkspaceID)
				}
			}
			return cfg.WorkspaceID, WorkspaceSourceProject, "", nil
		}
	}

	// Priority 3: Legacy .arc.json fallback with migration
	legacyPath, legacyErr := project.FindLegacyConfig(cwd)
	if legacyErr == nil {
		legacyRoot := filepath.Dir(legacyPath)
		cfg, migrateErr := project.MigrateLegacyConfig(legacyRoot, arcHome)
		if migrateErr == nil && cfg.WorkspaceID != "" {
			fmt.Fprintf(os.Stderr, "Migrated .arc.json → ~/.arc/projects/%s/config.json\n",
				project.PathToProjectDir(legacyRoot))

			// Validate workspace exists on server
			c, clientErr := getClient()
			if clientErr == nil {
				_, wsErr := c.GetWorkspace(cfg.WorkspaceID)
				if wsErr != nil {
					return "", 0, "", fmt.Errorf("workspace '%s' (%s) not found on server\n  Run 'arc init' to reconfigure this directory",
						cfg.WorkspaceName, cfg.WorkspaceID)
				}
			}
			return cfg.WorkspaceID, WorkspaceSourceLegacy, "", nil
		}
	}

	return "", 0, "", fmt.Errorf("no workspace configured for this directory\n  Run 'arc init' to set up a workspace, or use '-w <workspace>' to specify one")
}
```

Remove the now-unused `loadLocalConfig` and `findProjectConfig` functions (lines 120-164 in current `main.go`). The `localConfig` struct is also no longer needed.

Add import: `"github.com/sentiolabs/arc/internal/project"`

**Step 2: Run tests**

Run: `make test`
Expected: PASS — existing tests should still work (none directly test `resolveWorkspace` in isolation)

**Step 3: Manual verification**

The CLI should still work:
- `arc list` from a dir with `.arc.json` → migrates and works
- `arc list -w <id>` → flag override still works
- `arc which` → shows source correctly

**Step 4: Commit**

```bash
git add cmd/arc/main.go
git commit -m "feat: update workspace resolution to use ~/.arc/projects/"
```

---

### Task 6: Update `arc init` to write to `~/.arc/projects/`

**Files:**
- Modify: `cmd/arc/init.go:144-148,176-196` (the `createProjectConfig` call and function)

**Step 1: Update `runInit` to use project package**

Replace the `createProjectConfig` call (line 144) with:

```go
	// Create project config in ~/.arc/projects/
	arcHome := project.DefaultArcHome()
	projectCfg := &project.Config{
		WorkspaceID:   ws.ID,
		WorkspaceName: ws.Name,
		ProjectRoot:   cwd,
	}
	if err := project.WriteConfig(arcHome, cwd, projectCfg); err != nil {
		if !quiet {
			fmt.Fprintf(os.Stderr, "Warning: failed to create project config: %v\n", err)
		}
	}
```

Update the warning message at line 146 from `.arc.json` to `project config`.

Remove the old `createProjectConfig` function (lines 176-196) — it's no longer needed.

Add import: `"github.com/sentiolabs/arc/internal/project"`

**Step 2: Run tests**

Run: `make test`
Expected: PASS

**Step 3: Build and manual test**

Run: `make build-quick`

Test in a temp directory:
```bash
mkdir /tmp/test-arc-init && cd /tmp/test-arc-init
git init
arc init
# Should NOT create .arc.json
# Should create ~/.arc/projects/-tmp-test-arc-init/config.json
cat ~/.arc/projects/-tmp-test-arc-init/config.json
```

**Step 4: Commit**

```bash
git add cmd/arc/init.go
git commit -m "feat: arc init writes to ~/.arc/projects/ instead of .arc.json"
```

---

### Task 7: Update `arc which` to show new source info

**Files:**
- Modify: `cmd/arc/main.go:298-356` (the `whichCmd`)

**Step 1: Update the which command**

The `WorkspaceSource.String()` method was already updated in Task 5. The `whichCmd` uses `source.String()` so it should already show the correct source. Verify this works correctly and add the project config path to the output:

```go
// In the human-readable output section of whichCmd:
if source == WorkspaceSourceProject || source == WorkspaceSourceLegacy {
	cwd, _ := os.Getwd()
	arcHome := project.DefaultArcHome()
	if root, err := project.FindProjectRootWithArcHome(cwd, arcHome); err == nil {
		fmt.Printf("Config: ~/.arc/projects/%s/config.json\n", project.PathToProjectDir(root))
	}
}
```

**Step 2: Build and test**

Run: `make build-quick`
Run: `./bin/arc which` from a project directory
Expected: Shows workspace, source as `~/.arc/projects/ (local)`, and config path

**Step 3: Commit**

```bash
git add cmd/arc/main.go
git commit -m "feat: arc which shows project config path"
```

---

### Task 8: Final integration testing and cleanup

**Files:**
- No new files — verification only

**Step 1: Run full test suite**

Run: `make test`
Expected: All tests pass

**Step 2: Build and verify end-to-end**

```bash
make build-quick

# Test 1: Fresh init in a new dir
mkdir /tmp/test-e2e && cd /tmp/test-e2e && git init
arc init
ls -la .arc.json  # Should NOT exist
cat ~/.arc/projects/-tmp-test-e2e/config.json  # Should exist
arc list  # Should work
arc which  # Should show source as ~/.arc/projects/

# Test 2: Legacy migration
cd /some/dir/with/existing/.arc.json
arc list  # Should migrate and work, print migration message

# Test 3: Nested directory resolution
cd /tmp/test-e2e/some/deep/subdir
mkdir -p /tmp/test-e2e/some/deep/subdir
arc list  # Should resolve to the parent project
```

**Step 3: Clean up temp test directories**

```bash
rm -rf /tmp/test-e2e /tmp/test-arc-init
```

**Step 4: Final commit (if any fixups needed)**

```bash
git add -A && git commit -m "fix: address integration test findings"
```

**Step 5: Push**

```bash
git push
```
