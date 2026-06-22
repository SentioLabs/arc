// Package jjfs provides pure-Go helpers for inspecting on-disk Jujutsu (jj)
// repositories. Like gitfs, it does NOT shell out to the jj binary. It handles
// the narrow problems arc needs: locating .jj entries, mapping a secondary jj
// workspace back to its main repo, and locating the backing git directory.
package jjfs

// FindJJEntry walks up from dir to locate a .jj entry (directory). Returns the
// absolute path to the .jj entry, or "" if none is found before the root.
func FindJJEntry(dir string) string { return "" }

// DetectMainRepo returns the main repository's working directory if dir is
// inside a SECONDARY jj workspace; otherwise "".
func DetectMainRepo(dir string) string { return "" }

// DetectGitBackend returns the absolute path of the backing git directory for
// the jj repo containing dir (resolved via .jj/repo/store/git_target), or "".
func DetectGitBackend(dir string) string { return "" }
