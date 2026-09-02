package main

import (
	"io"
	"os"
	"testing"

	"github.com/sentiolabs/arc/internal/client"
	"github.com/stretchr/testify/require"
)

// captureStderr runs fn with os.Stderr redirected to a pipe and returns
// everything written to it.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w
	defer func() { os.Stderr = old }()

	fn()

	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(out)
}

// runClose executes closeCmd.RunE against ids and returns stdout, stderr,
// and the RunE error.
func runClose(t *testing.T, ids []string) (stdout, stderr string, runErr error) {
	t.Helper()
	stdout = captureStdout(t, func() {
		stderr = captureStderr(t, func() {
			runErr = closeCmd.RunE(closeCmd, ids)
		})
	})
	return stdout, stderr, runErr
}

func TestCloseRefusalReturnsError(t *testing.T) {
	_, _, releaseID := setupSequencingTest(t)

	_, err := createMilestone(t, "Open Child", releaseID)
	require.NoError(t, err)

	_, stderr, runErr := runClose(t, []string{releaseID})

	require.Error(t, runErr, "close refused by the server must return an error")
	require.Contains(t, stderr, "cannot close")
}

func TestCloseSuccessReturnsNil(t *testing.T) {
	c, wsID, _ := setupSequencingTest(t)

	task, err := c.CreateIssue(wsID, client.CreateIssueRequest{Title: "Leaf Task", IssueType: "task"})
	require.NoError(t, err)

	stdout, _, runErr := runClose(t, []string{task.ID})

	require.NoError(t, runErr)
	require.Contains(t, stdout, "Closed: "+task.ID)
}

func TestCloseMixedMultiIDReturnsError(t *testing.T) {
	c, wsID, releaseID := setupSequencingTest(t)

	_, err := createMilestone(t, "Open Child", releaseID)
	require.NoError(t, err)
	task, err := c.CreateIssue(wsID, client.CreateIssueRequest{Title: "Leaf Task", IssueType: "task"})
	require.NoError(t, err)

	stdout, stderr, runErr := runClose(t, []string{task.ID, releaseID})

	require.Error(t, runErr, "a mixed multi-close with any failure must return an error")
	require.Contains(t, stdout, "Closed: "+task.ID, "the successful close should still be reported")
	require.Contains(t, stderr, "cannot close")
}

func TestCloseCmdSilencesUsageOnRuntimeError(t *testing.T) {
	require.True(t, closeCmd.SilenceUsage,
		"a refused close is a runtime error, not a usage error, so usage must not print")
}

func TestCloseErrorMessageCountsFailures(t *testing.T) {
	_, _, releaseID := setupSequencingTest(t)

	_, err := createMilestone(t, "Open Child", releaseID)
	require.NoError(t, err)

	_, _, runErr := runClose(t, []string{releaseID})

	require.Error(t, runErr)
	require.Contains(t, runErr.Error(), "1 of 1",
		"error should summarize the failure count")
}
