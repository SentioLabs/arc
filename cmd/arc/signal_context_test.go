package main

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sentiolabs/arc/internal/api"
	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/storage/sqlite"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// cancelReturnTimeout bounds how long a cancelled command may take to return.
	cancelReturnTimeout = 2 * time.Second
	// followSettle is long enough for tailFollow to reach EOF and start
	// waiting on its context.
	followSettle = 250 * time.Millisecond
)

// awaitCommand reports the error run produced, failing the test if run has
// not returned within cancelReturnTimeout.
func awaitCommand(t *testing.T, run func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- run() }()
	select {
	case err := <-done:
		return err
	case <-time.After(cancelReturnTimeout):
		t.Fatal("command did not return after its context was cancelled")
		return nil
	}
}

func TestCmdContextFallsBackToBackground(t *testing.T) {
	cmd := &cobra.Command{Use: "probe"}
	require.Nil(t, cmd.Context(), "an unexecuted command has no context")
	ctx := cmdContext(cmd)
	require.NotNil(t, ctx)
	assert.NoError(t, ctx.Err(), "the fallback context must not start cancelled")
}

func TestSleepOrCancelReportsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	assert.False(t, sleepOrCancel(ctx, time.Hour), "a cancelled context must not wait")
	assert.True(t, sleepOrCancel(t.Context(), time.Millisecond), "a live context must wait out the delay")
}

// writeLogFile returns the path to a one-line log file for tailFollow.
func writeLogFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "server.log")
	require.NoError(t, os.WriteFile(path, []byte("first line\n"), 0o600))
	return path
}

// TestTailFollowStopsOnCancelledContext covers `server logs --follow` handed
// a context that is already cancelled.
func TestTailFollowStopsOnCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	logPath := writeLogFile(t)
	assert.NoError(t, awaitCommand(t, func() error { return tailFollow(ctx, logPath, 1) }))
}

// TestTailFollowStopsWhileFollowing cancels after tailFollow has reached EOF,
// so it covers the wait that would otherwise poll forever.
func TestTailFollowStopsWhileFollowing(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	logPath := writeLogFile(t)
	go func() {
		time.Sleep(followSettle)
		cancel()
	}()
	assert.NoError(t, awaitCommand(t, func() error { return tailFollow(ctx, logPath, 1) }))
}

// TestPlanWaitStopsOnCancel covers `arc plan wait`, whose default poll runs
// for 30 minutes. It must give up as soon as its context is cancelled.
func TestPlanWaitStopsOnCancel(t *testing.T) {
	store, err := sqlite.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer store.Close()
	ts := httptest.NewServer(api.New(api.ServerOptions{Address: ":0", Store: store}).Echo())
	defer ts.Close()

	c := client.New(ts.URL)
	c.SetActor("test-user")
	planPath := filepath.Join(t.TempDir(), "plan.md")
	require.NoError(t, os.WriteFile(planPath, []byte("# Plan\n"), 0o600))
	plan, err := c.CreatePlan(planPath)
	require.NoError(t, err)

	origServerURL, origTimeout := serverURL, planWaitTimeout
	serverURL, planWaitTimeout = ts.URL, time.Hour
	t.Cleanup(func() { serverURL, planWaitTimeout = origServerURL, origTimeout })

	// A separate command carries the context so the shared planWaitCmd, which
	// other tests invoke with no context at all, keeps its nil one.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	probe := &cobra.Command{Use: "wait"}
	probe.SetContext(ctx)

	err = awaitCommand(t, func() error { return planWaitCmd.RunE(probe, []string{plan.ID}) })
	require.ErrorIs(t, err, context.Canceled)
	assert.Contains(t, err.Error(), "stopped waiting for a decision")
}
