package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/types"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const workStatusField = "status"

func TestWorkContextPresenceAndCompleteChain(t *testing.T) {
	for _, input := range []string{
		`null`,
		`{}`,
		`{"governing":null}`,
		`{"contract_version":1}`,
		`{"governing":null,"contract_version":null}`,
		`{"governing":{},"contract_version":1}`,
		`{"governing":{"container_id":"e","container_type":"epic",` +
			`"reference":{"plan_id":"p","revision":1}},"contract_version":1}`,
	} {
		path := filepath.Join(t.TempDir(), "context.json")
		require.NoError(t, os.WriteFile(path, []byte(input), 0o600))
		_, err := readWorkContext(path)
		require.Error(t, err, input)
	}
	expected := types.ExpectedGovernance{
		ContractVersion: 3,
		Governing: &types.GoverningPlan{
			ContainerID:   "epic",
			ContainerType: types.TypeEpic,
			Reference:     types.PlanReference{PlanID: "tactical", Revision: 2},
			Context: []types.GoverningPlanContext{
				{
					ContainerID:   "milestone",
					ContainerType: types.TypeMilestone,
					Reference:     types.PlanReference{PlanID: "architecture", Revision: 4},
				},
			},
		},
	}
	b, err := json.Marshal(expected)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "context.json")
	require.NoError(t, os.WriteFile(path, b, 0o600))
	got, err := readWorkContext(path)
	require.NoError(t, err)
	require.Equal(t, expected, *got)
}

func TestTakeCapturesPostClaimAndCompletionReusesContext(t *testing.T) {
	c, p := setupSessionTest(t)
	t.Setenv("ARC_SESSION_ID", "worker")
	issue, err := c.CreateIssue(p, client.CreateIssueRequest{Title: "task"})
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "work.json")
	cmd := takeTestCommand()
	cmd.Flags().String("context-output", path, "")
	require.NoError(t, updateCmd.RunE(cmd, []string{issue.ID}))
	captured, err := readWorkContext(path)
	require.NoError(t, err)
	require.Greater(t, captured.ContractVersion, issue.ContractVersion)
	require.Nil(t, captured.Governing)
	sessionTestStdin(t, "passed tests")
	evidence := newEvidenceCommand()
	evidence.SetArgs([]string{issue.ID, "--phase", "build", "--context", path, "--stdin"})
	require.NoError(t, evidence.Execute())
	_, err = c.UpdateIssueByID(issue.ID, map[string]any{"title": "changed scope"})
	require.NoError(t, err)
	closer := closeCmd
	require.NoError(t, closer.Flags().Set("context", path))
	t.Cleanup(func() { _ = closer.Flags().Set("context", "") })
	require.Error(t, closer.RunE(closer, []string{issue.ID}))
}

func TestTakeJSONIncludesExplicitUnlinkedContext(t *testing.T) {
	c, p := setupSessionTest(t)
	outputJSON = true
	t.Setenv("ARC_SESSION_ID", "worker")
	issue, err := c.CreateIssue(p, client.CreateIssueRequest{Title: "task"})
	require.NoError(t, err)
	var commandErr error
	out := captureStdout(
		t,
		func() { commandErr = updateCmd.RunE(takeTestCommand(), []string{issue.ID}) },
	)
	require.NoError(t, commandErr)
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(out), &raw))
	require.Contains(t, raw, "work_context")
	expected, err := decodeWorkContext(raw["work_context"])
	require.NoError(t, err)
	require.Nil(t, expected.Governing)
	require.Greater(t, expected.ContractVersion, issue.ContractVersion)
}

func workTestPlan(t *testing.T, c *client.Client, p, key string) string {
	t.Helper()
	result, err := c.CreatePlan(p, key, storage.PlanUpload{Title: key, Content: key})
	require.NoError(t, err)
	_, err = c.DecidePlanRevision(
		p,
		result.Plan.ID,
		1,
		types.PlanReviewRequest{Status: "in_review", ExpectedHead: 1},
	)
	require.NoError(t, err)
	_, err = c.DecidePlanRevision(
		p,
		result.Plan.ID,
		1,
		types.PlanReviewRequest{Status: "approved", ExpectedHead: 1, ExpectedReviewVersion: 1},
	)
	require.NoError(t, err)
	return result.Plan.ID
}

//nolint:revive // Fixture names project, container, task, request key, target, and tactical records explicitly.
func workTestAdopt(
	t *testing.T,
	c *client.Client,
	p, container, task, key string,
	target *types.PlanReference,
	additional ...types.ReconciledContainerPin,
) {
	t.Helper()
	root, err := c.GetIssueDetails(p, container)
	require.NoError(t, err)
	project, err := c.GetProject(p)
	require.NoError(t, err)
	work, err := c.GetIssueDetails(p, task)
	require.NoError(t, err)
	_, err = c.AdoptPlan(
		p,
		container,
		key,
		types.PlanAdoptionRequest{
			ExpectedContainerVersion:     root.ContractVersion,
			ExpectedGovernanceGeneration: project.GovernanceGeneration,
			ExpectedPin:                  root.GoverningPlan,
			TargetPin:                    target,
			Tasks: []types.ReconciledTask{
				{
					IssueID: task,
					Expected: types.ExpectedGovernance{
						ContractVersion: work.ContractVersion,
						Governing:       work.ResolvedGovernance,
					},
					Disposition: "unchanged",
					Reason:      "reviewed compatibility",
				},
			},
			ContainerPins: additional,
		},
	)
	require.NoError(t, err)
}

func TestCLICompletionRejectsUnlinkedAttachmentAndStaleHigherRevision(t *testing.T) {
	c, p := setupSessionTest(t)
	t.Setenv("ARC_SESSION_ID", "worker")
	milestone, err := c.CreateIssue(
		p,
		client.CreateIssueRequest{Title: "architecture", IssueType: "milestone"},
	)
	require.NoError(t, err)
	epic, err := c.CreateIssue(
		p,
		client.CreateIssueRequest{Title: "tactical", IssueType: "epic", ParentID: milestone.ID},
	)
	require.NoError(t, err)
	task, err := c.CreateIssue(p, client.CreateIssueRequest{Title: "work", ParentID: epic.ID})
	require.NoError(t, err)
	unlinkedPath := filepath.Join(t.TempDir(), "unlinked.json")
	require.NoError(
		t,
		writeContextJSON(
			unlinkedPath,
			types.ExpectedGovernance{ContractVersion: task.ContractVersion},
		),
	)
	high := workTestPlan(t, c, p, "high")
	low := workTestPlan(t, c, p, "low")
	workTestAdopt(
		t,
		c,
		p,
		milestone.ID,
		task.ID,
		"attach-high",
		&types.PlanReference{PlanID: high, Revision: 1},
	)
	workTestAdopt(
		t,
		c,
		p,
		epic.ID,
		task.ID,
		"attach-low",
		&types.PlanReference{PlanID: low, Revision: 1},
	)
	sessionTestStdin(t, "verify evidence")
	evidence := newEvidenceCommand()
	evidence.SetArgs([]string{task.ID, "--phase", "verify", "--stdin", "--context", unlinkedPath})
	require.Error(t, evidence.Execute())
	path := filepath.Join(t.TempDir(), "governed.json")
	claim := takeTestCommand()
	claim.Flags().String("context-output", path, "")
	require.NoError(t, updateCmd.RunE(claim, []string{task.ID}))
	expected, err := readWorkContext(path)
	require.NoError(t, err)
	require.Len(t, expected.Governing.Context, 1)
	require.Equal(t, high, expected.Governing.Context[0].Reference.PlanID)
	require.Equal(t, low, expected.Governing.Reference.PlanID)
	_, err = c.RecordExecutionEvidence(
		p,
		task.ID,
		types.ExecutionEvidenceRequest{
			Expected: expected,
			Phase:    "build",
			Evidence: "captured all sources",
		},
	)
	require.NoError(t, err)
	_, err = c.UpdateIssueByID(task.ID, map[string]any{workStatusField: "open"})
	require.NoError(t, err)
	_, err = c.SavePlanRevision(
		p,
		high,
		"high-next",
		storage.PlanSave{Content: "amended architecture", ExpectedRevision: 1},
	)
	require.NoError(t, err)
	_, err = c.DecidePlanRevision(
		p,
		high,
		2,
		types.PlanReviewRequest{Status: "in_review", ExpectedHead: 2},
	)
	require.NoError(t, err)
	_, err = c.DecidePlanRevision(
		p,
		high,
		2,
		types.PlanReviewRequest{Status: "approved", ExpectedHead: 2, ExpectedReviewVersion: 1},
	)
	require.NoError(t, err)
	epicDetails, err := c.GetIssueDetails(p, epic.ID)
	require.NoError(t, err)
	workTestAdopt(
		t,
		c,
		p,
		milestone.ID,
		task.ID,
		"advance-high",
		&types.PlanReference{PlanID: high, Revision: 2},
		types.ReconciledContainerPin{
			ContainerID:              epic.ID,
			ExpectedContainerVersion: epicDetails.ContractVersion,
			ExpectedPin:              epicDetails.GoverningPlan,
			TargetPin:                epicDetails.GoverningPlan,
			Disposition:              "compatible",
			Reason:                   "tactical scope remains consistent",
		},
	)
	update := &cobra.Command{}
	update.Flags().String(workStatusField, "closed", "")
	update.Flags().String("context", path, "")
	require.Error(t, updateCmd.RunE(update, []string{task.ID}))
	closer := &cobra.Command{}
	closer.Flags().String("context", path, "")
	require.Error(t, closeCmd.RunE(closer, []string{task.ID}))
	evidence = newEvidenceCommand()
	evidence.SetArgs([]string{task.ID, "--phase", "verify", "--stdin", "--context", path})
	require.Error(t, evidence.Execute())
	records, err := c.ListExecutionEvidence(p, task.ID, 50, 0)
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, *expected, records[0].Expected)
	original, err := readWorkContext(path)
	require.NoError(t, err)
	require.Equal(t, *expected, *original)
}

func TestEvidenceHumanOutputNamesEveryCapturedSource(t *testing.T) {
	_, _ = setupSessionTest(t)
	expected := types.ExpectedGovernance{
		ContractVersion: 6,
		Governing: &types.GoverningPlan{
			ContainerID:   "epic-source",
			ContainerType: types.TypeEpic,
			Reference:     types.PlanReference{PlanID: "tactical-plan", Revision: 3},
			Context: []types.GoverningPlanContext{
				{
					ContainerID:   "milestone-source",
					ContainerType: types.TypeMilestone,
					Reference:     types.PlanReference{PlanID: "architecture-plan", Revision: 2},
				},
			},
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request types.ExecutionEvidenceRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		assert.Equal(t, expected, *request.Expected)
		_ = json.NewEncoder(w).
			Encode(storage.ExecutionEvidence{ID: "evidence", Expected: expected, Phase: "review"})
	}))
	defer ts.Close()
	serverURL = ts.URL
	originalProject := projectID
	projectID = cmdProject
	t.Cleanup(func() { projectID = originalProject })
	path := filepath.Join(t.TempDir(), "work.json")
	require.NoError(t, writeContextJSON(path, expected))
	sessionTestStdin(t, "reviewed")
	cmd := newEvidenceCommand()
	cmd.SetArgs([]string{"task", "--phase", "review", "--context", path, "--stdin"})
	var commandErr error
	out := captureStdout(t, func() { commandErr = cmd.Execute() })
	require.NoError(t, commandErr)
	for _, want := range []string{
		"phase review", "contract version 6", "epic-source", "tactical-plan",
		"revision 3", "milestone-source", "architecture-plan", "revision 2",
	} {
		require.Contains(t, out, want)
	}
	require.NotContains(t, out, "0x")
}

func TestFailedCaptureOutputRetainsTransactionalContext(t *testing.T) {
	for _, take := range []bool{true, false} {
		for _, linked := range []bool{true, false} {
			for _, asJSON := range []bool{false, true} {
				t.Run(
					fmt.Sprintf("take=%t/linked=%t/json=%t", take, linked, asJSON),
					func(t *testing.T) {
						runFailedCaptureOutputCase(t, take, linked, asJSON)
					},
				)
			}
		}
	}
}

func runFailedCaptureOutputCase(t *testing.T, take, linked, asJSON bool) {
	t.Helper()
	_, p := setupSessionTest(t)
	outputJSON = asJSON
	t.Setenv("ARC_SESSION_ID", "capture-worker")
	expected := defaultOutputAdoption(linked).After.Expected
	var writes, afterClaimReads atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			writes.Add(1)
			_ = json.NewEncoder(w).Encode(types.IssueDetails{
				Issue: types.Issue{
					ID:              "capture-task",
					ProjectID:       p,
					ContractVersion: expected.ContractVersion,
				},
				ResolvedGovernance: expected.Governing,
			})
			return
		}
		if writes.Load() != 0 {
			afterClaimReads.Add(1)
			http.Error(w, "must preserve original claim response", http.StatusConflict)
			return
		}
		if strings.Contains(r.URL.Path, "/ai/sessions/") {
			_ = json.NewEncoder(w).Encode(types.AISession{ID: "capture-worker", ProjectID: p})
			return
		}
		_ = json.NewEncoder(w).
			Encode(types.Issue{ID: "capture-task", ProjectID: p, ContractVersion: 1})
	}))
	defer ts.Close()
	serverURL = ts.URL
	path := filepath.Join(t.TempDir(), "original-context.json")
	require.NoError(t, os.WriteFile(path, []byte("preserve existing artifact"), 0o600))
	command := takeTestCommand()
	command.Flags().String("context-output", path, "")
	if !take {
		require.NoError(t, command.Flags().Set("take", "false"))
		require.NoError(t, command.Flags().Set("status", "in_progress"))
	}
	var commandErr error
	out := captureStdout(
		t,
		func() { commandErr = updateCmd.RunE(command, []string{"capture-task"}) },
	)
	require.ErrorContains(
		t,
		commandErr,
		"issue capture-task updated; captured context could not be written",
	)
	require.EqualValues(t, 1, writes.Load())
	require.Zero(t, afterClaimReads.Load())
	retained, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "preserve existing artifact", string(retained))
	// Recovery output is itself the exact artifact; it can be saved and reused.
	captured, err := decodeWorkContext([]byte(out))
	require.NoError(t, err, out)
	require.Equal(t, expected, *captured)
}
