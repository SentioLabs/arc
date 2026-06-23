// Package vcs provides version-control detection that works across git and
// Jujutsu (jj) repositories. It dispatches to the pure-Go gitfs and jjfs
// helpers, and shells out to git for remote-URL detection (jj's backend is a
// git repo, so no jj binary is required).
package vcs

import (
	"os/exec"
	"strings"

	"github.com/sentiolabs/arc/internal/core"
	"github.com/sentiolabs/arc/internal/gitfs"
	"github.com/sentiolabs/arc/internal/jjfs"
)

// Detect reports which version-control systems are present at or above dir,
// in stable order ("git" before "jj"). A colocated jj/git repo returns both;
// native jj returns only "jj"; a plain git repo returns only "git". Returns
// an empty (non-nil) slice when neither is found. Pure filesystem walk — no
// git or jj binary is invoked.
func Detect(dir string) []string {
	systems := []string{}
	if gitfs.FindGitEntry(dir) != "" {
		systems = append(systems, "git")
	}
	if jjfs.FindJJEntry(dir) != "" {
		systems = append(systems, "jj")
	}
	return systems
}

// DetectMainRepo returns the canonical main-repo working directory if dir is
// inside a linked git worktree or a secondary jj workspace; otherwise "".
// git is tried first, jj is the fallback. The result is canonicalized via
// core.NormalizePath so it matches registered (canonical) project paths.
func DetectMainRepo(dir string) string {
	if main := gitfs.DetectMainRepo(dir); main != "" {
		return core.NormalizePath(main)
	}
	if main := jjfs.DetectMainRepo(dir); main != "" {
		return core.NormalizePath(main)
	}
	return ""
}

// DetectRemote returns the origin remote URL for the repo containing dir, or
// "". It tries `git -C dir remote get-url origin` first (covers plain git and
// colocated jj). On failure it retries against the jj backing git directory
// (covers native jj). No jj binary is invoked.
func DetectRemote(dir string) string {
	if url := gitRemoteURL(dir); url != "" {
		return url
	}
	if backend := jjfs.DetectGitBackend(dir); backend != "" {
		if url := gitRemoteURL(backend); url != "" {
			return url
		}
	}
	return ""
}

// gitRemoteURL runs `git -C dir remote get-url origin` and returns the trimmed
// URL, or "" if git fails (not a repo, no origin, or git not installed).
func gitRemoteURL(dir string) string {
	cmd := exec.Command("git", "-C", dir, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
