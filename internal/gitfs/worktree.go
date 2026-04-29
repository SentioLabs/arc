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

// DetectMainRepo returns the main repository's working directory if
// dir is inside a linked git worktree. Returns "" if dir is in the
// main worktree, has no reachable .git entry, or the .git pointer is
// malformed.
//
// Linked worktrees use a .git *file* containing one line of the form
// "gitdir: /abs/path/to/main/.git/worktrees/<name>". This function
// peels two directory levels off that path to get the main .git, and
// returns the parent of that as the main worktree.
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
	line := strings.TrimSpace(string(data))
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
	if filepath.Base(mainGitDir) != ".git" {
		return ""
	}
	return filepath.Dir(mainGitDir)
}
