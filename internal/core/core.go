// Package core provides shared utilities used across the arc codebase.
package core

import (
	"os"
	"path/filepath"
)

// DefaultArcHome returns the default arc home directory (~/.arc).
func DefaultArcHome() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".arc")
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

// NormalizePathPair returns both the absolute path (what the user typed / cwd reports)
// and the symlink-resolved path. When they differ, callers should store both variants
// so that lookups work regardless of which form is used to access the directory.
func NormalizePathPair(path string) (absPath, resolvedPath string) {
	abs, _ := filepath.Abs(path)
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return abs, abs
	}
	return abs, resolved
}
