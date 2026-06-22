// Package jjfs provides pure-Go helpers for inspecting on-disk Jujutsu (jj)
// repositories. Like gitfs, it does NOT shell out to the jj binary. It handles
// the narrow problems arc needs: locating .jj entries, mapping a secondary jj
// workspace back to its main repo, and locating the backing git directory.
package jjfs

import (
	"os"
	"path/filepath"
	"strings"
)

// FindJJEntry walks up from dir to locate a .jj entry. Returns the absolute
// path to the .jj directory, or "" if none is found before the filesystem root.
func FindJJEntry(dir string) string {
	dir = filepath.Clean(dir)
	for {
		candidate := filepath.Join(dir, ".jj")
		if _, err := os.Lstat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// repoDir resolves the actual .jj/repo directory for the given .jj entry.
// Main workspace: .jj/repo is a directory -> (repoPath, false).
// Secondary workspace: .jj/repo is a file holding a path to the shared repo's
// .jj/repo -> (resolved, true). Returns ("", false) if unresolvable.
func repoDir(jjEntry string) (dir string, secondary bool) {
	repoPath := filepath.Join(jjEntry, "repo")
	info, err := os.Lstat(repoPath)
	if err != nil {
		return "", false
	}
	if info.IsDir() {
		return repoPath, false
	}
	data, err := os.ReadFile(repoPath)
	if err != nil {
		return "", false
	}
	pointer := strings.TrimSpace(string(data))
	if pointer == "" {
		return "", false
	}
	if !filepath.IsAbs(pointer) {
		pointer = filepath.Join(jjEntry, pointer)
	}
	resolved := filepath.Clean(pointer)
	if fi, statErr := os.Stat(resolved); statErr != nil || !fi.IsDir() {
		return "", false
	}
	return resolved, true
}

// DetectMainRepo returns the main repository's working directory if dir is
// inside a SECONDARY jj workspace. Returns "" if dir is in the main workspace,
// has no reachable .jj entry, or the pointer is malformed. A secondary
// workspace's .jj/repo points to <main>/.jj/repo; the main working dir is two
// levels up from there.
func DetectMainRepo(dir string) string {
	jjEntry := FindJJEntry(dir)
	if jjEntry == "" {
		return ""
	}
	repo, secondary := repoDir(jjEntry)
	if !secondary {
		return ""
	}
	mainJJ := filepath.Dir(repo)     // <main>/.jj
	mainWork := filepath.Dir(mainJJ) // <main>
	if fi, err := os.Stat(mainWork); err != nil || !fi.IsDir() {
		return ""
	}
	return mainWork
}

// DetectGitBackend returns the absolute path of the backing git directory for
// the jj repo containing dir, resolved via .jj/repo/store/git_target. Returns
// "" if dir is not in a jj repo or the store cannot be resolved. For a
// secondary workspace, the shared repo is followed first.
func DetectGitBackend(dir string) string {
	jjEntry := FindJJEntry(dir)
	if jjEntry == "" {
		return ""
	}
	repo, _ := repoDir(jjEntry)
	if repo == "" {
		return ""
	}
	storeDir := filepath.Join(repo, "store")
	data, err := os.ReadFile(filepath.Join(storeDir, "git_target"))
	if err != nil {
		return ""
	}
	pointer := strings.TrimSpace(string(data))
	if pointer == "" {
		return ""
	}
	if !filepath.IsAbs(pointer) {
		pointer = filepath.Join(storeDir, pointer)
	}
	backend := filepath.Clean(pointer)
	if fi, statErr := os.Stat(backend); statErr != nil || !fi.IsDir() {
		return ""
	}
	return backend
}
