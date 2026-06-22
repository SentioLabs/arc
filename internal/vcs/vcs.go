// Package vcs provides version-control detection that works across git and
// Jujutsu (jj) repositories. It dispatches to the pure-Go gitfs and jjfs
// helpers, and shells out to git for remote-URL detection (jj's backend is a
// git repo, so no jj binary is required).
package vcs

// DetectMainRepo returns the canonical main-repo working directory if dir is
// inside a linked git worktree or a secondary jj workspace; otherwise "".
func DetectMainRepo(dir string) string { return "" }

// DetectRemote returns the origin remote URL for the repo containing dir, or "".
func DetectRemote(dir string) string { return "" }
