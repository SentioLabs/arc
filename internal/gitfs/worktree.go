// Package gitfs provides pure-Go helpers for inspecting on-disk git
// repositories. It does NOT shell out to the git binary and does NOT
// depend on any third-party git library. It only handles the narrow
// problems arc needs: locating .git entries and following linked
// worktree pointers.
package gitfs

import (
	"os"
	"path/filepath"
	"strings"
)

// FindGitEntry walks up from dir to locate a .git entry (file or
// directory). Returns the absolute path to the .git entry, or "" if
// none is found before reaching the filesystem root.
func FindGitEntry(dir string) string {
	dir = filepath.Clean(dir)
	for {
		candidate := filepath.Join(dir, ".git")
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

// DetectMainRepo returns the canonical path of the main repository if dir
// is inside a linked git worktree. Returns "" if dir is in the main
// worktree, has no reachable .git entry, or the .git pointer is malformed.
//
// Two layouts are supported:
//
//  1. Worktree of a normal repo: .git is a file like
//     "gitdir: /abs/path/main/.git/worktrees/<name>". This function returns
//     the main repo's working directory (parent of the .git directory).
//
//  2. Worktree of a bare repo: .git is a file like
//     "gitdir: /abs/path/repo.git/worktrees/<name>". A bare repo has no
//     working directory; this function returns the bare repo path itself
//     (validated by checking for HEAD and objects/).
func DetectMainRepo(dir string) string {
	gitPath := FindGitEntry(dir)
	if gitPath == "" {
		return ""
	}

	info, err := os.Lstat(gitPath)
	if err != nil {
		return ""
	}
	if info.IsDir() {
		return ""
	}

	data, err := os.ReadFile(gitPath)
	if err != nil {
		return ""
	}
	firstLine, _, _ := strings.Cut(string(data), "\n")
	line := strings.TrimSpace(firstLine)
	const prefix = "gitdir:"
	if !strings.HasPrefix(line, prefix) {
		return ""
	}
	gitdir := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	if !filepath.IsAbs(gitdir) {
		return ""
	}

	worktreesDir := filepath.Dir(gitdir)
	if filepath.Base(worktreesDir) != "worktrees" {
		return ""
	}

	mainGitDir := filepath.Dir(worktreesDir)

	// Normal repo case: gitdir is .../<main-worktree>/.git/worktrees/<name>.
	// The parent of mainGitDir is the main worktree's working directory.
	if filepath.Base(mainGitDir) == ".git" {
		return filepath.Dir(mainGitDir)
	}

	// Bare repo case: gitdir is .../repo.git/worktrees/<name>. There is no
	// working directory; the bare repo itself (mainGitDir) is the canonical
	// path users register. Validate it looks like a real bare repo by
	// checking for the standard bare-repo files HEAD and objects.
	if isBareRepo(mainGitDir) {
		return mainGitDir
	}

	return ""
}

// isBareRepo returns true if dir contains the typical structural files
// of a bare git repository (HEAD file and objects directory).
func isBareRepo(dir string) bool {
	if info, err := os.Stat(filepath.Join(dir, "HEAD")); err != nil || info.IsDir() {
		return false
	}
	if info, err := os.Stat(filepath.Join(dir, "objects")); err != nil || !info.IsDir() {
		return false
	}
	return true
}
