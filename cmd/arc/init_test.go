package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectGitRemote_InGitRepo(t *testing.T) {
	// We are running inside a git repo, so this should return a non-empty URL
	url := detectGitRemote(".")
	// The test repo should have an origin remote
	assert.NotEmpty(t, url, "expected git remote URL in a git repo")
	assert.Contains(t, url, "arc", "expected the arc repo remote URL")
}

func TestDetectGitRemote_ReturnsNoNewline(t *testing.T) {
	url := detectGitRemote(".")
	assert.NotContains(t, url, "\n", "git remote URL should not contain newlines")
}
