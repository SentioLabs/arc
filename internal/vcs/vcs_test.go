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
	_ func(string) string   = vcs.DetectMainRepo
	_ func(string) string   = vcs.DetectRemote
	_ func(string) []string = vcs.Detect
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

func TestDetectRemote_ColocatedPrefersGit(t *testing.T) {
	// A colocated repo has both .git and .jj. DetectRemote must try git first
	// and return the .git origin without consulting the jj backend. The jj
	// backend is given a DIFFERENT origin so a wrong (jj-first) ordering would
	// return that URL instead and fail this test.
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "remote", "add", "origin", "git@example.com:org/colocated.git")

	store := filepath.Join(dir, ".jj", "repo", "store")
	backend := filepath.Join(store, "git")
	if err := os.MkdirAll(backend, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, backend, "init", "--bare")
	runGit(t, backend, "remote", "add", "origin", "git@example.com:org/jjbackend.git")
	if err := os.WriteFile(filepath.Join(store, "git_target"), []byte("git"), 0o600); err != nil {
		t.Fatal(err)
	}

	if got := vcs.DetectRemote(dir); got != "git@example.com:org/colocated.git" {
		t.Fatalf("DetectRemote(colocated) = %q, want the .git origin (git-first)", got)
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
	if err := os.WriteFile(filepath.Join(store, "git_target"), []byte("git"), 0o600); err != nil {
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

// --- vcs.Detect tests ---

func makeJJEntry(t *testing.T, dir string) {
	t.Helper()
	// Simulate a .jj directory with the minimal structure FindJJEntry looks for.
	jjDir := filepath.Join(dir, ".jj")
	if err := os.MkdirAll(jjDir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestDetect_PlainGit(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")

	got := vcs.Detect(dir)
	if len(got) != 1 || got[0] != "git" {
		t.Fatalf("Detect(plain git) = %v, want [git]", got)
	}
}

func TestDetect_NativeJJ(t *testing.T) {
	dir := t.TempDir()
	makeJJEntry(t, dir)
	// No .git directory — native jj only.

	got := vcs.Detect(dir)
	if len(got) != 1 || got[0] != "jj" {
		t.Fatalf("Detect(native jj) = %v, want [jj]", got)
	}
}

func TestDetect_Colocated(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	makeJJEntry(t, dir)

	got := vcs.Detect(dir)
	if len(got) != 2 || got[0] != "git" || got[1] != "jj" {
		t.Fatalf("Detect(colocated) = %v, want [git jj]", got)
	}
}

func TestDetect_Neither(t *testing.T) {
	dir := t.TempDir()

	got := vcs.Detect(dir)
	// Must be non-nil and empty so JSON marshals to [] not null.
	if got == nil {
		t.Fatal("Detect(no repo) returned nil, want empty non-nil slice")
	}
	if len(got) != 0 {
		t.Fatalf("Detect(no repo) = %v, want []", got)
	}
}
