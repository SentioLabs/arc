package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetGitRemoteURL_InGitRepo(t *testing.T) {
	// We are running inside a git repo, so this should return a non-empty URL
	url := getGitRemoteURL()
	// The test repo should have an origin remote
	assert.NotEmpty(t, url, "expected git remote URL in a git repo")
	assert.Contains(t, url, "arc", "expected the arc repo remote URL")
}

func TestGetGitRemoteURL_ReturnsNoNewline(t *testing.T) {
	url := getGitRemoteURL()
	assert.NotContains(t, url, "\n", "git remote URL should not contain newlines")
}
