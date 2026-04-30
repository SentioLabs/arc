// Package gittest provides hardened git-command helpers for tests that
// need to set up real git repositories or worktrees in t.TempDir().
//
// All commands run with system and global git config disabled
// (GIT_CONFIG_NOSYSTEM=1, GIT_CONFIG_GLOBAL=/dev/null) and with deterministic
// author/committer identity, so tests behave the same way on every host.
package gittest

import (
	"os"
	"os/exec"
	"testing"
)

// dirPerm is the permission bits used when creating test repository
// directories. 0o755 matches the default umask-aware mode for `git init`.
const dirPerm = 0o755

// Run executes `git <args...>` with cmd.Dir set to workdir and the standard
// hardened test environment. workdir must already exist; if it doesn't,
// Run fails the test. Pass "" for workdir to use the current process's CWD.
func Run(t *testing.T, workdir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	if workdir != "" {
		cmd.Dir = workdir
	}
	cmd.Env = hardenedEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// InitRepo initializes a normal (non-bare) git repository at dir, creating
// the directory if needed, and makes one empty commit so subsequent
// `git worktree add` calls have a ref to branch from.
func InitRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		t.Fatalf("mkdir %q: %v", dir, err)
	}
	Run(t, dir, "init", "-q")
	Run(t, dir, "commit", "--allow-empty", "-m", "init", "-q")
}

// AddWorktree creates a linked git worktree at worktreeDir, branching from
// HEAD of mainDir to a new branch named branch. Registers a t.Cleanup that
// runs `git worktree remove --force` so t.TempDir() teardown can succeed
// without lock-file flakes.
func AddWorktree(t *testing.T, mainDir, worktreeDir, branch string) {
	t.Helper()
	Run(t, mainDir, "worktree", "add", "-q", "-b", branch, worktreeDir)
	t.Cleanup(func() {
		_ = exec.Command("git", "worktree", "remove", "--force", worktreeDir).Run()
	})
}

func hardenedEnv() []string {
	return append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=/dev/null",
	)
}
