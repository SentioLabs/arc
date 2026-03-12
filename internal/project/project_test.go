package project_test

import (
	"os"
	"testing"

	"github.com/sentiolabs/arc/internal/core"
	"github.com/sentiolabs/arc/internal/project"
)

// TestWrappersDelegateToCore verifies the backward-compatibility wrappers
// in the project package delegate to core correctly.
func TestWrappersDelegateToCore(t *testing.T) {
	t.Run("DefaultArcHome delegates to core", func(t *testing.T) {
		if project.DefaultArcHome() != core.DefaultArcHome() {
			t.Error("project.DefaultArcHome() != core.DefaultArcHome()")
		}
	})

	t.Run("NormalizePath delegates to core", func(t *testing.T) {
		dir := t.TempDir()
		if project.NormalizePath(dir) != core.NormalizePath(dir) {
			t.Error("project.NormalizePath() != core.NormalizePath()")
		}
	})

	t.Run("NormalizePathPair delegates to core", func(t *testing.T) {
		realDir := t.TempDir()
		symlinkDir := t.TempDir() + "/link"
		if err := os.Symlink(realDir, symlinkDir); err != nil {
			t.Skipf("cannot create symlink: %v", err)
		}
		t.Cleanup(func() { os.Remove(symlinkDir) })

		pAbs, pRes := project.NormalizePathPair(symlinkDir)
		cAbs, cRes := core.NormalizePathPair(symlinkDir)
		if pAbs != cAbs || pRes != cRes {
			t.Error("project.NormalizePathPair() != core.NormalizePathPair()")
		}
	})
}
