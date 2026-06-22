package jjfs_test

import (
	"os"
	"path/filepath"
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

// writeFile writes content to path, creating parent dirs.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFindJJEntry_FromSubdir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".jj", "repo"), 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	got := jjfs.FindJJEntry(sub)
	want := filepath.Join(root, ".jj")
	if got != want {
		t.Fatalf("FindJJEntry(%q) = %q, want %q", sub, got, want)
	}
}

func TestDetectMainRepo_MainWorkspaceReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".jj", "repo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := jjfs.DetectMainRepo(root); got != "" {
		t.Fatalf("DetectMainRepo(main) = %q, want \"\"", got)
	}
}

func TestDetectMainRepo_SecondaryWorkspace(t *testing.T) {
	root := t.TempDir()
	// main workspace
	if err := os.MkdirAll(filepath.Join(root, "main", ".jj", "repo"), 0o755); err != nil {
		t.Fatal(err)
	}
	// secondary workspace: .jj/repo is a FILE pointing to ../../main/.jj/repo
	wsJJ := filepath.Join(root, "ws", ".jj")
	writeFile(t, filepath.Join(wsJJ, "repo"), "../../main/.jj/repo")
	got := jjfs.DetectMainRepo(filepath.Join(root, "ws"))
	want := filepath.Join(root, "main")
	if got != want {
		t.Fatalf("DetectMainRepo(secondary) = %q, want %q", got, want)
	}
}

func TestDetectGitBackend_NativeRelative(t *testing.T) {
	root := t.TempDir()
	// native store: git_target points to the internal git dir "git"
	store := filepath.Join(root, ".jj", "repo", "store")
	if err := os.MkdirAll(filepath.Join(store, "git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(store, "git_target"), "git")
	got := jjfs.DetectGitBackend(root)
	want := filepath.Join(store, "git")
	if got != want {
		t.Fatalf("DetectGitBackend = %q, want %q", got, want)
	}
}

func TestDetectGitBackend_NotJJ(t *testing.T) {
	if got := jjfs.DetectGitBackend(t.TempDir()); got != "" {
		t.Fatalf("DetectGitBackend(non-jj) = %q, want \"\"", got)
	}
}
