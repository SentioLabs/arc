package main

import (
	"os"
	"path/filepath"
	"strings"
)

// gitWorktreeMainRepo detects if the given directory is a git worktree and
// returns the path to the main repository. Returns empty string if the
// directory is not a worktree or detection fails.
//
// Git worktrees have a `.git` file (not directory) containing:
//
//	gitdir: /path/to/main-repo/.git/worktrees/<name>
//
// We follow that back to the main repo's root.
func gitWorktreeMainRepo(dir string) string {
	dotGit := filepath.Join(dir, ".git")

	info, err := os.Lstat(dotGit)
	if err != nil || info.IsDir() {
		// Not a worktree — .git is either missing or is a regular directory
		return ""
	}

	// .git is a file — read it to find the gitdir pointer
	data, err := os.ReadFile(dotGit)
	if err != nil {
		return ""
	}

	content := strings.TrimSpace(string(data))
	if !strings.HasPrefix(content, "gitdir: ") {
		return ""
	}

	gitdir := strings.TrimPrefix(content, "gitdir: ")

	// Make relative paths absolute
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(dir, gitdir)
	}

	// gitdir points to .git/worktrees/<name> — go up 3 levels to reach the main repo
	// e.g., /path/to/repo/.git/worktrees/wt-branch -> /path/to/repo
	mainGitDir := filepath.Dir(filepath.Dir(filepath.Dir(gitdir)))

	// Verify this looks like a valid repo (has a .git directory)
	if info, err := os.Stat(filepath.Join(mainGitDir, ".git")); err != nil || !info.IsDir() {
		return ""
	}

	return mainGitDir
}

// uniquePaths returns a deduplicated slice of non-empty paths.
func uniquePaths(paths ...string) []string {
	seen := make(map[string]bool, len(paths))
	var result []string
	for _, p := range paths {
		if p != "" && !seen[p] {
			seen[p] = true
			result = append(result, p)
		}
	}
	return result
}
