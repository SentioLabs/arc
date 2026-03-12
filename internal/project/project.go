package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// dirPermissions is the file mode for directories created by arc.
	dirPermissions = 0o755
	// filePermissions is the file mode for config files written by arc.
	filePermissions = 0o600
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

	if err := os.MkdirAll(filepath.Dir(path), dirPermissions); err != nil {
		return fmt.Errorf("create project dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(path, append(data, '\n'), filePermissions)
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
			return "", errors.New("no .git found")
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
			return "", errors.New("no registered project found")
		}
		current = parent
	}
}

// NormalizePath resolves symlinks and returns the canonical absolute path.
// Falls back to filepath.Abs if symlink resolution fails (e.g. path doesn't exist).
func NormalizePath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		abs, _ := filepath.Abs(path)
		return abs
	}
	return resolved
}

// DetectWorktreeMainRepo checks if dir is a git worktree and returns the main
// repository root path. Returns ("", nil) if dir is not a worktree or has no .git.
func DetectWorktreeMainRepo(dir string) (string, error) {
	gitPath := filepath.Join(dir, ".git")
	info, err := os.Lstat(gitPath)
	if err != nil {
		return "", nil //nolint:nilerr // no .git entry means not a worktree — not an error
	}
	if info.IsDir() {
		return "", nil
	}

	// .git is a file — read its content
	data, err := os.ReadFile(gitPath)
	if err != nil {
		return "", fmt.Errorf("read .git file: %w", err)
	}

	content := strings.TrimSpace(string(data))
	if !strings.HasPrefix(content, "gitdir: ") {
		return "", nil
	}

	gitdir := strings.TrimPrefix(content, "gitdir: ")

	// Find ".git/worktrees/" segment and strip it plus everything after
	marker := string(filepath.Separator) + ".git" +
		string(filepath.Separator) + "worktrees" + string(filepath.Separator)
	idx := strings.Index(gitdir, marker)
	if idx < 0 {
		return "", nil
	}

	mainRepo := filepath.Clean(gitdir[:idx])
	return NormalizePath(mainRepo), nil
}

// PathToProjectDir converts an absolute filesystem path to a project directory name.
// Replaces "/" with "-", matching the Claude Code ~/.claude/projects/ convention.
// Example: "/home/user/my-repo" → "-home-user-my-repo"
func PathToProjectDir(absPath string) string {
	cleaned := filepath.Clean(absPath)
	return strings.ReplaceAll(cleaned, string(filepath.Separator), "-")
}

// CleanupWorkspaceConfigs removes all project configs under arcHome that reference
// the given workspace ID. Returns the number of project directories removed.
func CleanupWorkspaceConfigs(arcHome, workspaceID string) (int, error) {
	projDir := projectsDir(arcHome)

	entries, err := os.ReadDir(projDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read projects dir: %w", err)
	}

	removed := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		cfgPath := filepath.Join(projDir, entry.Name(), "config.json")
		data, err := os.ReadFile(cfgPath)
		if err != nil {
			continue // skip dirs without config.json
		}

		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			continue // skip unparseable configs
		}

		if cfg.WorkspaceID == workspaceID {
			if err := os.RemoveAll(filepath.Join(projDir, entry.Name())); err != nil {
				return removed, fmt.Errorf("remove project dir %s: %w", entry.Name(), err)
			}
			removed++
		}
	}

	return removed, nil
}
