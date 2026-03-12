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
