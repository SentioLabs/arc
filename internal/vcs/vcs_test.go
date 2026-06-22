package vcs_test

import (
	"os"
	"os/exec"
	"path/filepath"
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

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", full...)
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestDetectRemote_PlainGit(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "remote", "add", "origin", "git@example.com:org/repo.git")
	if got := vcs.DetectRemote(dir); got != "git@example.com:org/repo.git" {
		t.Fatalf("DetectRemote = %q, want the origin URL", got)
	}
}

func TestDetectRemote_NativeJJBackend(t *testing.T) {
	// Simulate a native jj repo by hand: .jj/repo/store/git_target -> "git",
	// where "git" is a real bare git repo holding the origin remote.
	root := t.TempDir()
	store := filepath.Join(root, ".jj", "repo", "store")
	backend := filepath.Join(store, "git")
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, t.TempDir(), "init") // warm-up no-op to fail fast if git missing
	if err := os.MkdirAll(backend, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, backend, "init", "--bare")
	runGit(t, backend, "remote", "add", "origin", "git@example.com:org/native.git")
	if err := os.WriteFile(filepath.Join(store, "git_target"), []byte("git"), 0o644); err != nil {
		t.Fatal(err)
	}
	// No .git at root, so the git-first attempt fails and the jj backend wins.
	if got := vcs.DetectRemote(root); got != "git@example.com:org/native.git" {
		t.Fatalf("DetectRemote(native jj) = %q, want the backend origin URL", got)
	}
}

func TestDetectRemote_None(t *testing.T) {
	if got := vcs.DetectRemote(t.TempDir()); got != "" {
		t.Fatalf("DetectRemote(no repo) = %q, want \"\"", got)
	}
}

func TestDetectMainRepo_NotInWorktree(t *testing.T) {
	if got := vcs.DetectMainRepo(t.TempDir()); got != "" {
		t.Fatalf("DetectMainRepo(plain dir) = %q, want \"\"", got)
	}
}
