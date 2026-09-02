package main

import (
	"testing"

	"github.com/sentiolabs/arc/internal/client"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

// resetListFlags restores listCmd's flags to defaults so slice state set by
// one test's Set calls doesn't leak into the next.
func resetListFlags(t *testing.T) {
	t.Helper()
	f := listCmd.Flags().Lookup("priority")
	require.NotNil(t, f, "listCmd should have a --priority flag")
	require.NoError(t, f.Value.(pflag.SliceValue).Replace(nil))
	f.Changed = false
}

// runList executes listCmd.RunE after applying opts to its flags and returns
// stdout and the RunE error.
func runList(t *testing.T, opts ...func()) (string, error) {
	t.Helper()
	resetListFlags(t)
	for _, opt := range opts {
		opt()
	}
	var runErr error
	out := captureStdout(t, func() {
		runErr = listCmd.RunE(listCmd, nil)
	})
	return out, runErr
}

func withPriority(p string) func() {
	return func() {
		_ = listCmd.Flags().Set("priority", p)
	}
}

// seedPriorityIssues creates one task each at P0, P1, and P4 and returns
// their titles keyed by priority.
func seedPriorityIssues(t *testing.T, c *client.Client, wsID string) {
	t.Helper()
	for p, title := range map[int]string{0: "P0 task", 1: "P1 task", 4: "P4 task"} {
		_, err := c.CreateIssue(wsID, client.CreateIssueRequest{
			Title: title, IssueType: "task", Priority: &p,
		})
		require.NoError(t, err)
	}
}

func TestListPriorityFilterSingle(t *testing.T) {
	c, wsID, _ := setupSequencingTest(t)
	seedPriorityIssues(t, c, wsID)

	out, err := runList(t, withPriority("0"))
	require.NoError(t, err)

	require.Contains(t, out, "P0 task")
	require.NotContains(t, out, "P1 task")
	require.NotContains(t, out, "P4 task")
}

func TestListPriorityFilterRepeatedFlagsORCombine(t *testing.T) {
	c, wsID, _ := setupSequencingTest(t)
	seedPriorityIssues(t, c, wsID)

	out, err := runList(t, withPriority("0"), withPriority("4"))
	require.NoError(t, err)

	require.Contains(t, out, "P0 task")
	require.Contains(t, out, "P4 task")
	require.NotContains(t, out, "P1 task")
}

func TestListNoPriorityFlagUnchanged(t *testing.T) {
	c, wsID, _ := setupSequencingTest(t)
	seedPriorityIssues(t, c, wsID)

	out, err := runList(t)
	require.NoError(t, err)

	for _, title := range []string{"P0 task", "P1 task", "P4 task"} {
		require.Contains(t, out, title, "expected %q in unfiltered list", title)
	}
}
