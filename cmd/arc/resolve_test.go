package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProjectFlagRegistered(t *testing.T) {
	// The root command should have a --project flag (no shorthand — -p conflicts
	// with subcommand flags like --prefix, --priority, --port)
	flag := rootCmd.PersistentFlags().Lookup("project")
	assert.NotNil(t, flag, "--project flag should be registered on root command")
	assert.Empty(t, flag.Shorthand, "--project should not have a shorthand (conflicts with -p on subcommands)")

	// The old --workspace flag should not exist
	oldFlag := rootCmd.PersistentFlags().Lookup("workspace")
	assert.Nil(t, oldFlag, "--workspace flag should no longer exist")
}

func TestProjectSourceConstants(t *testing.T) {
	// Verify the renamed constants exist and have correct string representations
	assert.Equal(t, ProjectSourceFlag, ProjectSource(0))
	assert.Equal(t, ProjectSourceProject, ProjectSource(1))
	assert.Equal(t, ProjectSourceServer, ProjectSource(2))

	assert.Contains(t, ProjectSourceFlag.String(), "--project")
	assert.Contains(t, ProjectSourceProject.String(), "local")
	assert.Contains(t, ProjectSourceServer.String(), "server")
}

func TestProjectCmdRegistered(t *testing.T) {
	// The root command should have a "project" subcommand for managing issue containers
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "project" {
			found = true
			break
		}
	}
	assert.True(t, found, "root command should have a 'project' subcommand")

	// The old "workspace" command for managing issue containers should be gone
	// (Note: "workspace" may exist as a different command for directory paths later)
}

func TestGetProjectIDFunction(t *testing.T) {
	// The function getProjectID should exist (renamed from getWorkspaceID)
	// We can't easily test it without a server, but we verify it compiles
	_ = getProjectID
}

func TestResolveProjectFunction(t *testing.T) {
	// The function resolveProject should exist (renamed from resolveWorkspace)
	_ = resolveProject
}
