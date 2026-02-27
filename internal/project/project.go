package project

import (
	"path/filepath"
	"strings"
)

// PathToProjectDir converts an absolute filesystem path to a project directory name.
// Replaces "/" with "-", matching the Claude Code ~/.claude/projects/ convention.
// Example: "/home/user/my-repo" → "-home-user-my-repo"
func PathToProjectDir(absPath string) string {
	cleaned := filepath.Clean(absPath)
	return strings.ReplaceAll(cleaned, string(filepath.Separator), "-")
}
