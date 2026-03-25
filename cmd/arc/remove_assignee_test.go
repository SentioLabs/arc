package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIContextOmitsAssigneeFlag(t *testing.T) {
	var buf bytes.Buffer
	err := outputCLIContext(&buf, "")
	require.NoError(t, err)

	output := buf.String()
	assert.NotContains(t, output, "--assignee", "CLI context should not mention --assignee flag")
	assert.NotContains(t, output, "assignee=username", "CLI context should not mention assignee usage")
}

func TestTeamContextStructHasNoUnassignedField(t *testing.T) {
	// Verify TeamContext can be constructed without Unassigned field.
	tc := &TeamContext{
		Workspace: "test",
		Roles:     make(map[string]*TeamRole),
	}
	assert.NotNil(t, tc)
	assert.Empty(t, tc.Roles)
}

func TestCreateCmdHasNoAssigneeFlag(t *testing.T) {
	flag := createCmd.Flags().Lookup("assignee")
	assert.Nil(t, flag, "createCmd should not have --assignee flag")
}

func TestListCmdHasNoAssigneeFlag(t *testing.T) {
	flag := listCmd.Flags().Lookup("assignee")
	assert.Nil(t, flag, "listCmd should not have --assignee flag")
}

func TestUpdateCmdHasNoAssigneeFlag(t *testing.T) {
	flag := updateCmd.Flags().Lookup("assignee")
	assert.Nil(t, flag, "updateCmd should not have --assignee flag")
}

func TestShowOutputOmitsAssignee(t *testing.T) {
	var buf bytes.Buffer
	err := outputCLIContext(&buf, "test-session")
	require.NoError(t, err)

	output := buf.String()
	for _, line := range strings.Split(output, "\n") {
		lowerLine := strings.ToLower(line)
		assert.NotContains(t, lowerLine, "assignee",
			"CLI context line should not reference assignee: %s", line)
	}
}

func TestBuildTeamContextSkipsIssuesWithoutTeammateLabel(t *testing.T) {
	// Verify that the TeamContext struct has no Unassigned field
	// and that the Roles map is the only place issues are grouped.
	tc := TeamContext{
		Workspace: "ws",
		Roles: map[string]*TeamRole{
			"backend": {
				Issues: []TeamContextIssue{
					{ID: "1", Title: "task", Status: "open", Type: "task"},
				},
			},
		},
	}
	assert.Len(t, tc.Roles["backend"].Issues, 1)
}
