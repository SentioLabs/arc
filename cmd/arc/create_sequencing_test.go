package main

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sentiolabs/arc/internal/api"
	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/storage/sqlite"
	"github.com/sentiolabs/arc/internal/types"
	"github.com/stretchr/testify/require"
)

// setupSequencingTest spins up a test server and client, creates a project
// and a release to parent milestones under, and wires the CLI's global
// serverURL/projectID/outputJSON state to point at it. All test globals are
// restored via t.Cleanup.
func setupSequencingTest(t *testing.T) (c *client.Client, wsID, releaseID string) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := sqlite.New(dbPath)
	require.NoError(t, err)

	server := api.New(api.ServerOptions{Address: ":0", Store: store})
	ts := httptest.NewServer(server.Echo())

	c = client.New(ts.URL)
	c.SetActor("test-user")

	proj, err := c.CreateProject("Sequencing Test", "seq", "")
	require.NoError(t, err)

	release, err := c.CreateIssue(proj.ID, client.CreateIssueRequest{
		Title:     "Release 1",
		IssueType: string(types.TypeRelease),
		Priority:  2,
	})
	require.NoError(t, err)

	origServerURL, origProjectID, origOutputJSON := serverURL, projectID, outputJSON
	serverURL = ts.URL
	projectID = proj.ID
	outputJSON = false

	t.Cleanup(func() {
		ts.Close()
		_ = store.Close()
		serverURL, projectID, outputJSON = origServerURL, origProjectID, origOutputJSON
		resetCreateFlags(t)
	})

	resetCreateFlags(t)

	return c, proj.ID, release.ID
}

// resetCreateFlags restores createCmd's flags used by sequencing to their
// defaults, so state set by one test's cmd.Flags().Set calls doesn't leak
// into the next.
func resetCreateFlags(t *testing.T) {
	t.Helper()
	require.NoError(t, createCmd.Flags().Set("type", "task"))
	require.NoError(t, createCmd.Flags().Set("parent", ""))
	require.NoError(t, createCmd.Flags().Set("after", ""))
	require.NoError(t, createCmd.Flags().Set("parallel", "false"))
}

// createMilestone runs createCmd for a milestone under parentID with the
// given options applied to its flags first, returning the new issue's ID
// (parsed from the "Created: <id>" line) and any RunE error.
func createMilestone(t *testing.T, title, parentID string, opts ...func()) (string, error) {
	t.Helper()

	resetCreateFlags(t)
	require.NoError(t, createCmd.Flags().Set("type", string(types.TypeMilestone)))
	require.NoError(t, createCmd.Flags().Set("parent", parentID))
	for _, opt := range opts {
		opt()
	}

	var runErr error
	out := captureStdout(t, func() {
		runErr = createCmd.RunE(createCmd, []string{title})
	})
	if runErr != nil {
		return "", runErr
	}

	for _, line := range strings.Split(out, "\n") {
		if id, ok := strings.CutPrefix(line, "Created: "); ok {
			return strings.TrimSpace(id), nil
		}
	}
	t.Fatalf("no Created: line in output: %q", out)
	return "", nil
}

func withAfter(id string) func() {
	return func() {
		_ = createCmd.Flags().Set("after", id)
	}
}

func withParallel() func() {
	return func() {
		_ = createCmd.Flags().Set("parallel", "true")
	}
}

// blocksDepTarget returns the depends_on_id of id's "blocks" dependency, or
// "" if it has none.
func blocksDepTarget(t *testing.T, c *client.Client, wsID, id string) string {
	t.Helper()
	details, err := c.GetIssueDetails(wsID, id)
	require.NoError(t, err)
	for _, dep := range details.Dependencies {
		if dep.Type == types.DepBlocks {
			return dep.DependsOnID
		}
	}
	return ""
}

func TestSequencingFirstMilestoneNoDep(t *testing.T) {
	c, wsID, releaseID := setupSequencingTest(t)

	m1ID, err := createMilestone(t, "Milestone 1", releaseID)
	require.NoError(t, err)

	require.Empty(t, blocksDepTarget(t, c, wsID, m1ID), "first milestone should have no blocks dependency")
}

func TestSequencingSecondMilestoneBlocksOnFirst(t *testing.T) {
	c, wsID, releaseID := setupSequencingTest(t)

	m1ID, err := createMilestone(t, "Milestone 1", releaseID)
	require.NoError(t, err)
	m2ID, err := createMilestone(t, "Milestone 2", releaseID)
	require.NoError(t, err)

	require.Equal(t, m1ID, blocksDepTarget(t, c, wsID, m2ID))
}

func TestSequencingThirdMilestoneChainsNotFansIn(t *testing.T) {
	c, wsID, releaseID := setupSequencingTest(t)

	m1ID, err := createMilestone(t, "Milestone 1", releaseID)
	require.NoError(t, err)
	m2ID, err := createMilestone(t, "Milestone 2", releaseID)
	require.NoError(t, err)
	m3ID, err := createMilestone(t, "Milestone 3", releaseID)
	require.NoError(t, err)

	require.Equal(t, m2ID, blocksDepTarget(t, c, wsID, m3ID),
		"third milestone should chain onto the second, not fan in onto the first")
	require.NotEqual(t, m1ID, blocksDepTarget(t, c, wsID, m3ID))
}

func TestSequencingParallelNoDepAndLabeled(t *testing.T) {
	c, wsID, releaseID := setupSequencingTest(t)

	mID, err := createMilestone(t, "Parallel Milestone", releaseID, withParallel())
	require.NoError(t, err)

	details, err := c.GetIssueDetails(wsID, mID)
	require.NoError(t, err)
	require.Empty(t, blocksDepTarget(t, c, wsID, mID), "parallel milestone should have no sequencing dependency")
	require.Contains(t, details.Labels, "parallel")
}

func TestSequencingIgnoresParallelSiblingForTail(t *testing.T) {
	c, wsID, releaseID := setupSequencingTest(t)

	m1ID, err := createMilestone(t, "Milestone 1", releaseID)
	require.NoError(t, err)
	_, err = createMilestone(t, "Parallel Milestone", releaseID, withParallel())
	require.NoError(t, err)
	m3ID, err := createMilestone(t, "Milestone 3", releaseID)
	require.NoError(t, err)

	require.Equal(t, m1ID, blocksDepTarget(t, c, wsID, m3ID),
		"tail computation should skip the parallel sibling")
}

func TestSequencingAfterFlagTargetsNamedSibling(t *testing.T) {
	c, wsID, releaseID := setupSequencingTest(t)

	m1ID, err := createMilestone(t, "Milestone 1", releaseID)
	require.NoError(t, err)
	_, err = createMilestone(t, "Milestone 2", releaseID)
	require.NoError(t, err)
	m3ID, err := createMilestone(t, "Milestone 3", releaseID, withAfter(m1ID))
	require.NoError(t, err)

	require.Equal(t, m1ID, blocksDepTarget(t, c, wsID, m3ID))
}

func TestSequencingAfterAndParallelMutuallyExclusive(t *testing.T) {
	_, _, releaseID := setupSequencingTest(t)

	_, err := createMilestone(t, "Bad Milestone", releaseID, withAfter("some-id"), withParallel())
	require.Error(t, err)
	require.Contains(t, err.Error(), "--after and --parallel are mutually exclusive")
}

func TestSequencingFlagsOnlyApplyToMilestone(t *testing.T) {
	_, _, releaseID := setupSequencingTest(t)

	require.NoError(t, createCmd.Flags().Set("type", "task"))
	require.NoError(t, createCmd.Flags().Set("parent", releaseID))
	require.NoError(t, createCmd.Flags().Set("after", "some-id"))

	var runErr error
	captureStdout(t, func() {
		runErr = createCmd.RunE(createCmd, []string{"Bad Task"})
	})

	require.Error(t, runErr)
	require.Contains(t, runErr.Error(), "--after/--parallel only apply to --type=milestone")
}
