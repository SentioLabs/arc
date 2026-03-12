package project_test

import (
	"os"
	"testing"

	"github.com/sentiolabs/arc/internal/project"
)

func TestNormalizePath(t *testing.T) {
	// NormalizePath should resolve symlinks and return canonical absolute paths.
	// For a non-existent path, it should fall back to filepath.Abs.
	tmpDir := t.TempDir()

	result := project.NormalizePath(tmpDir)
	if result == "" {
		t.Fatal("NormalizePath returned empty string")
	}

	// The result should be an absolute path
	if result[0] != '/' {
		t.Errorf("NormalizePath(%q) = %q, expected absolute path", tmpDir, result)
	}
}

func TestNormalizePathPair(t *testing.T) {
	// Create a real directory and a symlink to it.
	realDir := t.TempDir()
	symlinkDir := t.TempDir() + "/link"
	if err := os.Symlink(realDir, symlinkDir); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}
	t.Cleanup(func() { os.Remove(symlinkDir) })

	absPath, resolvedPath := project.NormalizePathPair(symlinkDir)

	// absPath should be the symlink path (what user sees)
	if absPath != symlinkDir {
		t.Errorf("absPath = %q, want %q", absPath, symlinkDir)
	}

	// resolvedPath should be the real directory (symlink resolved)
	// On macOS, TempDir may itself be behind /private symlink
	realResolved := project.NormalizePath(realDir)
	if resolvedPath != realResolved {
		t.Errorf("resolvedPath = %q, want %q", resolvedPath, realResolved)
	}

	// When they differ, both should be non-empty
	if absPath == resolvedPath {
		t.Error("expected absPath and resolvedPath to differ for symlinked path")
	}
}

func TestNormalizePathPair_NoSymlink(t *testing.T) {
	// For a real (non-symlinked) directory, both should be the same.
	dir := t.TempDir()
	resolved := project.NormalizePath(dir) // canonical form

	absPath, resolvedPath := project.NormalizePathPair(resolved)
	if absPath != resolvedPath {
		t.Errorf("expected same paths for non-symlinked dir, got abs=%q resolved=%q", absPath, resolvedPath)
	}
}

func TestDefaultArcHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot get user home dir: %v", err)
	}

	result := project.DefaultArcHome()
	expected := home + "/.arc"
	if result != expected {
		t.Errorf("DefaultArcHome() = %q, want %q", result, expected)
	}
}
