package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorkspaceSourceConstants(t *testing.T) {
	// After refactoring, WorkspaceSourceProject should be removed.
	// Only WorkspaceSourceFlag and WorkspaceSourceServer should exist.
	assert.Equal(t, WorkspaceSource(0), WorkspaceSourceFlag)
	assert.Equal(t, WorkspaceSource(1), WorkspaceSourceServer)
}

func TestWorkspaceSourceString(t *testing.T) {
	assert.Equal(t, "command line flag (-w)", WorkspaceSourceFlag.String())
	assert.Equal(t, "server path match", WorkspaceSourceServer.String())
}

func TestResolveWorkspaceWithFlag(t *testing.T) {
	// When -w flag is set, it should always win
	origWsID := workspaceID
	workspaceID = "test-workspace-id"
	t.Cleanup(func() { workspaceID = origWsID })

	wsID, source, warning, err := resolveWorkspace()
	assert.NoError(t, err)
	assert.Equal(t, "test-workspace-id", wsID)
	assert.Equal(t, WorkspaceSourceFlag, source)
	assert.Empty(t, warning)
}
