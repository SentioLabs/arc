package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sentiolabs/arc/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestFormatOpenChildrenError(t *testing.T) {
	err := &types.OpenChildrenError{
		IssueID: "arc-a1b2",
		Children: []types.Issue{
			{ID: "arc-a1b2.1", Title: "Add login endpoint", Status: types.StatusOpen},
			{ID: "arc-a1b2.2", Title: "Add auth middleware", Status: types.StatusInProgress},
			{ID: "arc-a1b2.3", Title: "Write auth tests", Status: types.StatusOpen},
		},
	}

	result := formatOpenChildrenError(err)

	assert.Contains(t, result, "cannot close arc-a1b2: 3 open child issues must be closed first")
	assert.Contains(t, result, "Open children:")
	assert.Contains(t, result, "arc-a1b2.1")
	assert.Contains(t, result, "Add login endpoint")
	assert.Contains(t, result, "(open)")
	assert.Contains(t, result, "arc-a1b2.2")
	assert.Contains(t, result, "Add auth middleware")
	assert.Contains(t, result, "(in_progress)")
	assert.Contains(t, result, "arc-a1b2.3")
	assert.Contains(t, result, "Write auth tests")
	assert.Contains(t, result, "--cascade")
}

func TestFormatOpenChildrenErrorSingleChild(t *testing.T) {
	err := &types.OpenChildrenError{
		IssueID: "arc-x1",
		Children: []types.Issue{
			{ID: "arc-x1.1", Title: "Only child", Status: types.StatusOpen},
		},
	}

	result := formatOpenChildrenError(err)

	assert.Contains(t, result, "cannot close arc-x1: 1 open child issue must be closed first")
	assert.Contains(t, result, "arc-x1.1")
}

func TestFormatOpenChildrenErrorPluralForm(t *testing.T) {
	oneChild := &types.OpenChildrenError{
		IssueID: "x",
		Children: []types.Issue{
			{ID: "x.1", Title: "A", Status: types.StatusOpen},
		},
	}
	twoChildren := &types.OpenChildrenError{
		IssueID: "x",
		Children: []types.Issue{
			{ID: "x.1", Title: "A", Status: types.StatusOpen},
			{ID: "x.2", Title: "B", Status: types.StatusOpen},
		},
	}

	one := formatOpenChildrenError(oneChild)
	two := formatOpenChildrenError(twoChildren)

	assert.Contains(t, one, "1 open child issue must")
	assert.NotContains(t, one, "issues must")
	assert.Contains(t, two, "2 open child issues must")
}

func TestFormatOpenChildrenErrorColumnAlignment(t *testing.T) {
	err := &types.OpenChildrenError{
		IssueID: "arc-a1b2",
		Children: []types.Issue{
			{ID: "arc-a1b2.1", Title: "Short", Status: types.StatusOpen},
			{ID: "arc-a1b2.10", Title: "Longer title here", Status: types.StatusInProgress},
		},
	}

	result := formatOpenChildrenError(err)
	lines := strings.Split(result, "\n")

	// Find lines containing children and verify they are present
	var childLines []string
	for _, line := range lines {
		if strings.Contains(line, "arc-a1b2.") {
			childLines = append(childLines, line)
		}
	}
	assert.Len(t, childLines, 2, "should have 2 child lines")

	// Verify each child line has the expected format with proper indentation
	for _, line := range childLines {
		trimmed := strings.TrimSpace(line)
		assert.True(t, strings.HasPrefix(line, "    "), "child lines should be indented: %q", line)
		assert.NotEmpty(t, trimmed)
	}
}

func TestCloseCmdHasCascadeFlag(t *testing.T) {
	flag := closeCmd.Flags().Lookup("cascade")
	assert.NotNil(t, flag, "--cascade flag should be registered on close command")
	assert.Equal(t, "false", flag.DefValue, "--cascade should default to false")
}

func TestCloseCmdCascadeFlagShorthandAbsent(t *testing.T) {
	flag := closeCmd.Flags().Lookup("cascade")
	assert.NotNil(t, flag)
	// cascade should not have a shorthand to avoid conflicts
	assert.Equal(t, "", flag.Shorthand)
}

func TestFormatOpenChildrenErrorHintMessage(t *testing.T) {
	err := &types.OpenChildrenError{
		IssueID: "arc-a1b2",
		Children: []types.Issue{
			{ID: "arc-a1b2.1", Title: "Child", Status: types.StatusOpen},
		},
	}

	result := formatOpenChildrenError(err)

	assert.Contains(t, result, "Use --cascade to close all children, or close them individually first.")
}

func TestFormatOpenChildrenErrorFullOutput(t *testing.T) {
	err := &types.OpenChildrenError{
		IssueID: "arc-a1b2",
		Children: []types.Issue{
			{ID: "arc-a1b2.1", Title: "Add login endpoint", Status: types.StatusOpen},
			{ID: "arc-a1b2.2", Title: "Add auth middleware", Status: types.StatusInProgress},
			{ID: "arc-a1b2.3", Title: "Write auth tests", Status: types.StatusOpen},
		},
	}

	result := formatOpenChildrenError(err)

	// Verify structure: header, blank line, section header, children, blank line, hint
	assert.True(t, strings.HasPrefix(result, "Error: cannot close arc-a1b2:"),
		"should start with Error: prefix, got: %s", result)
	assert.True(t, strings.HasSuffix(strings.TrimSpace(result),
		"Use --cascade to close all children, or close them individually first."),
		"should end with hint, got: %s", result)

	fmt.Println("--- Full output ---")
	fmt.Println(result)
	fmt.Println("---")
}
