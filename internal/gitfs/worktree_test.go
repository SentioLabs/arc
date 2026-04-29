package gitfs_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/sentiolabs/arc/internal/gitfs"
)

// --- Contract assertions ---

var _ func(string) string = gitfs.FindGitEntry
var _ func(string) string = gitfs.DetectMainRepo

// --- Behavior tests ---

func initRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	run := func(name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
	}
	run("git", "init", "-q")
	run("git", "commit", "--allow-empty", "-m", "init", "-q")
}

func addWorktree(t *testing.T, mainDir, worktreeDir, branch string) {
	t.Helper()
	cmd := exec.Command("git", "worktree", "add", "-q", "-b", branch, worktreeDir)
	cmd.Dir = mainDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("worktree add: %v\n%s", err, out)
	}
	t.Cleanup(func() {
		_ = exec.Command("git", "worktree", "remove", "--force", worktreeDir).Run()
	})
}

func TestFindGitEntry_MainWorktree(t *testing.T) {
	root := t.TempDir()
	main := filepath.Join(root, "main")
	initRepo(t, main)

	got := gitfs.FindGitEntry(main)
	want := filepath.Join(main, ".git")
	if got != want {
		t.Fatalf("FindGitEntry(%q) = %q, want %q", main, got, want)
	}
}

func TestFindGitEntry_FromSubdirectory(t *testing.T) {
	root := t.TempDir()
	main := filepath.Join(root, "main")
	sub := filepath.Join(main, "deep", "nested", "dir")
	initRepo(t, main)
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}

	got := gitfs.FindGitEntry(sub)
	want := filepath.Join(main, ".git")
	if got != want {
		t.Fatalf("FindGitEntry(%q) = %q, want %q", sub, got, want)
	}
}

func TestFindGitEntry_NoRepo(t *testing.T) {
	root := t.TempDir()
	got := gitfs.FindGitEntry(root)
	if got != "" {
		t.Fatalf("FindGitEntry(%q) = %q, want empty", root, got)
	}
}

func TestDetectMainRepo_MainWorktreeReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	main := filepath.Join(root, "main")
	initRepo(t, main)

	if got := gitfs.DetectMainRepo(main); got != "" {
		t.Fatalf("DetectMainRepo(main) = %q, want empty", got)
	}
}

func TestDetectMainRepo_LinkedWorktreeRoot(t *testing.T) {
	root := t.TempDir()
	main := filepath.Join(root, "main")
	wt := filepath.Join(root, "feature-x")
	initRepo(t, main)
	addWorktree(t, main, wt, "feature-x")

	got := gitfs.DetectMainRepo(wt)
	if got != main {
		t.Fatalf("DetectMainRepo(%q) = %q, want %q", wt, got, main)
	}
}

func TestDetectMainRepo_LinkedWorktreeSubdir(t *testing.T) {
	root := t.TempDir()
	main := filepath.Join(root, "main")
	wt := filepath.Join(root, "feature-x")
	initRepo(t, main)
	addWorktree(t, main, wt, "feature-x")

	sub := filepath.Join(wt, "internal", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}

	got := gitfs.DetectMainRepo(sub)
	if got != main {
		t.Fatalf("DetectMainRepo(%q) = %q, want %q", sub, got, main)
	}
}

func TestDetectMainRepo_NoRepo(t *testing.T) {
	root := t.TempDir()
	if got := gitfs.DetectMainRepo(root); got != "" {
		t.Fatalf("DetectMainRepo(no-repo) = %q, want empty", got)
	}
}

func TestDetectMainRepo_MalformedGitFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("not a gitdir pointer\n"), 0o644); err != nil {
		t.Fatalf("write .git: %v", err)
	}
	if got := gitfs.DetectMainRepo(root); got != "" {
		t.Fatalf("DetectMainRepo(malformed) = %q, want empty", got)
	}
}

func TestDetectMainRepo_RelativeGitdir(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: ../relative/path\n"), 0o644); err != nil {
		t.Fatalf("write .git: %v", err)
	}
	if got := gitfs.DetectMainRepo(root); got != "" {
		t.Fatalf("DetectMainRepo(relative) = %q, want empty", got)
	}
}

func mustRunGit(t *testing.T, workdir string, args ...string) {
	t.Helper()
	if workdir != "" {
		if err := os.MkdirAll(workdir, 0o755); err != nil {
			t.Fatalf("mkdir %q: %v", workdir, err)
		}
	}
	cmd := exec.Command("git", args...)
	if workdir != "" {
		cmd.Dir = workdir
	}
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=/dev/null",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestDetectMainRepo_BareRepoWorktree(t *testing.T) {
	root := t.TempDir()
	bare := filepath.Join(root, "repo.git")
	wt := filepath.Join(root, "feature-x")

	// Create a bare repo with one commit so worktree add has a ref to branch from.
	mustRunGit(t, "", "init", "--bare", "-q", bare)

	// `git worktree add` on a bare repo needs an existing branch. Easiest path:
	// init a temporary normal repo, push to bare, then bare repo can serve worktrees.
	src := filepath.Join(root, "src")
	mustRunGit(t, src, "init", "-q")
	mustRunGit(t, src, "commit", "--allow-empty", "-m", "init", "-q")
	mustRunGit(t, src, "remote", "add", "origin", bare)
	mustRunGit(t, src, "push", "-q", "origin", "HEAD:refs/heads/main")

	// Now add a worktree from the bare repo.
	mustRunGit(t, bare, "worktree", "add", "-q", "-b", "feature-x", wt, "main")
	t.Cleanup(func() {
		_ = exec.Command("git", "worktree", "remove", "--force", wt).Run()
	})

	got := gitfs.DetectMainRepo(wt)
	if got != bare {
		t.Fatalf("DetectMainRepo(bare-repo-worktree) = %q, want %q", got, bare)
	}
}

func TestDetectMainRepo_PointerToNonRepo(t *testing.T) {
	// .git file points at a directory that isn't a git repo (no HEAD, no objects).
	root := t.TempDir()
	fakeGitdir := filepath.Join(root, "not-a-repo", "worktrees", "x")
	if err := os.MkdirAll(fakeGitdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: "+fakeGitdir+"\n"), 0o644); err != nil {
		t.Fatalf("write .git: %v", err)
	}

	if got := gitfs.DetectMainRepo(root); got != "" {
		t.Fatalf("DetectMainRepo(pointer-to-non-repo) = %q, want empty", got)
	}
}
