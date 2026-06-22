package vcs_test

import (
	"testing"

	"github.com/sentiolabs/arc/internal/vcs"
)

// --- Contract assertions ---
// These verify the design spec. Do NOT modify without updating the approved plan.

var (
	_ func(string) string = vcs.DetectMainRepo
	_ func(string) string = vcs.DetectRemote
)

func TestVCSContract(t *testing.T) {
	if vcs.DetectMainRepo("/nonexistent/path/xyz") != "" {
		t.Fatal("DetectMainRepo on a nonexistent path should return \"\"")
	}
}

// --- Behavior tests (added by the vcs implementation task) ---
