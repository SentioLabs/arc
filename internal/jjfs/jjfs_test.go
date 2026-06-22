package jjfs_test

import (
	"testing"

	"github.com/sentiolabs/arc/internal/jjfs"
)

// --- Contract assertions ---
// These verify the design spec. Do NOT modify without updating the approved plan.

var (
	_ func(string) string = jjfs.FindJJEntry
	_ func(string) string = jjfs.DetectMainRepo
	_ func(string) string = jjfs.DetectGitBackend
)

func TestJJFSContract(t *testing.T) {
	// Compile-time contract is asserted by the vars above; this test exists so
	// the package has a runnable test target.
	if jjfs.FindJJEntry("/nonexistent/path/xyz") != "" {
		t.Fatal("FindJJEntry on a nonexistent path should return \"\"")
	}
}

// --- Behavior tests (added by the jjfs implementation task) ---
