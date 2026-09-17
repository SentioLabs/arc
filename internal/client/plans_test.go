package client_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDurableClientUploadReplayAndConflicts(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()
	p := createTestProjectClient(t, c)
	first, err := c.CreatePlan(
		p.ID,
		"create-1",
		storage.PlanUpload{Title: "Remote", Content: "# Exact\r\n", SourceName: "client.md"},
	)
	require.NoError(t, err)
	replay, err := c.CreatePlan(
		p.ID,
		"create-1",
		storage.PlanUpload{Title: "Remote", Content: "# Exact\r\n", SourceName: "client.md"},
	)
	require.NoError(t, err)
	require.True(t, replay.Replay)
	require.Equal(t, first.Plan.ID, replay.Plan.ID)
	saved, err := c.SavePlanRevision(
		p.ID,
		first.Plan.ID,
		"save-1",
		storage.PlanSave{Content: "next", ExpectedRevision: 1},
	)
	require.NoError(t, err)
	require.EqualValues(t, 2, saved.Revision.Revision)
	_, err = c.SavePlanRevision(
		p.ID,
		first.Plan.ID,
		"save-2",
		storage.PlanSave{Content: "stale", ExpectedRevision: 1},
	)
	var apiErr *client.APIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, 409, apiErr.StatusCode)
	old, err := c.ReadPlanRevision(p.ID, first.Plan.ID, 1)
	require.NoError(t, err)
	require.Equal(t, "# Exact\r\n", old.Content)
	_, err = c.GetPlan("wrong-project", first.Plan.ID)
	require.Error(t, err)
	history, err := c.ListPlanRevisions(p.ID, first.Plan.ID, 50, 0)
	require.NoError(t, err)
	require.Len(t, history, 2)
	plans, err := c.ListPlans(p.ID, false, 50, 0)
	require.NoError(t, err)
	require.Len(t, plans, 1)
}

func TestDurableClientReviewFeedbackLifecycle(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()
	p := createTestProjectClient(t, c)
	write, err := c.CreatePlan(p.ID, "upload", storage.PlanUpload{Content: "# Plan"})
	require.NoError(t, err)
	id := write.Plan.ID
	rev, err := c.DecidePlanRevision(
		p.ID,
		id,
		1,
		types.PlanReviewRequest{Status: "in_review", ExpectedHead: 1},
	)
	require.NoError(t, err)
	comment, err := c.CreatePlanComment(p.ID, id, 1, storage.PlanCommentCreate{Content: "explain"})
	require.NoError(t, err)
	_, err = c.DecidePlanRevision(
		p.ID,
		id,
		1,
		types.PlanReviewRequest{
			Status:                "approved",
			ExpectedHead:          1,
			ExpectedReviewVersion: rev.ReviewVersion,
		},
	)
	require.Error(t, err)
	body := "clarify"
	comment, err = c.UpdatePlanComment(
		p.ID,
		id,
		1,
		comment.ID,
		storage.PlanCommentUpdate{Content: &body, ExpectedVersion: comment.Version},
	)
	require.NoError(t, err)
	meta, err := c.GetPlan(p.ID, id)
	require.NoError(t, err)
	_, err = c.AddPlanDisposition(
		p.ID,
		id,
		1,
		storage.PlanDispositionRequest{
			CommentID:               comment.ID,
			ExpectedCommentVersion:  comment.Version,
			ExpectedFeedbackVersion: meta.FeedbackVersion,
			Disposition:             "deferred",
			Reason:                  "follow up",
		},
	)
	require.NoError(t, err)
	dispositions, err := c.ListPlanDispositions(p.ID, id, 1, 50, 0)
	require.NoError(t, err)
	require.Len(t, dispositions, 1)
	meta, err = c.GetPlan(p.ID, id)
	require.NoError(t, err)
	_, err = c.DecidePlanRevision(
		p.ID,
		id,
		1,
		types.PlanReviewRequest{
			Status:                  "approved",
			ExpectedHead:            1,
			ExpectedReviewVersion:   rev.ReviewVersion,
			ExpectedFeedbackVersion: meta.FeedbackVersion,
		},
	)
	require.NoError(t, err)
	comments, err := c.ListPlanComments(p.ID, id, 1, true, 50, 0)
	require.NoError(t, err)
	require.Len(t, comments, 1)
	meta, err = c.GetPlan(p.ID, id)
	require.NoError(t, err)
	archived, err := c.UpdatePlan(
		p.ID,
		id,
		storage.PlanMetadataUpdate{ExpectedVersion: meta.Version, Lifecycle: "archived"},
	)
	require.NoError(t, err)
	_, err = c.UpdatePlan(
		p.ID,
		id,
		storage.PlanMetadataUpdate{ExpectedVersion: archived.Version, Lifecycle: "active"},
	)
	require.NoError(t, err)
	_, err = c.DeletePlanComment(p.ID, id, 1, comment.ID, comment.Version)
	require.NoError(t, err)
}

func TestDurableClientRetriesOneLogicalKeyAndPreservesError(t *testing.T) {
	calls := 0
	key := ""
	var payload []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		assert.Equal(t, "/api/v1/projects/proj/plans", r.URL.Path)
		if key == "" {
			key = r.Header.Get("Idempotency-Key")
		}
		assert.NotEmpty(t, key)
		assert.Equal(t, key, r.Header.Get("Idempotency-Key"))
		var raw json.RawMessage
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&raw))
		if payload == nil {
			payload = raw
		}
		assert.JSONEq(t, string(payload), string(raw))
		if calls == 1 {
			conn, _, err := w.(http.Hijacker).Hijack()
			assert.NoError(t, err)
			assert.NoError(t, conn.Close())
			return
		}
		_, _ = w.Write(
			[]byte(`{"plan":{"id":"plan.remote"},"revision":{"revision":1},"replay":true}`),
		)
	}))
	defer ts.Close()
	result, err := client.New(ts.URL).CreatePlan("proj", "", storage.PlanUpload{Content: "exact"})
	require.NoError(t, err)
	require.True(t, result.Replay)
	require.Equal(t, 2, calls)
	conflict := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(409)
		_, _ = w.Write(
			[]byte(
				`{"error":"stale","code":"execution_conflict","current":{"contract_version":9}}`,
			),
		)
	}))
	defer conflict.Close()
	_, err = client.New(conflict.URL).GetPlan("proj", "plan")
	var detail *client.APIError
	require.ErrorAs(t, err, &detail)
	require.Equal(t, "execution_conflict", detail.Code)
	require.Contains(t, string(detail.Body), "contract_version")
}

func TestDurableClientCapturedWork(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()
	p := createTestProjectClient(t, c)
	task := createTestIssueClient(t, c, p.ID, "work")
	claimed, err := c.UpdateIssueWithContext(task.ID, map[string]any{"status": "in_progress"})
	require.NoError(t, err)
	expected := &types.ExpectedGovernance{
		ContractVersion: claimed.ContractVersion,
		Governing:       claimed.ResolvedGovernance,
	}
	record, err := c.RecordExecutionEvidence(
		p.ID,
		task.ID,
		types.ExecutionEvidenceRequest{Expected: expected, Phase: "build", Evidence: "tested"},
	)
	require.NoError(t, err)
	require.Equal(t, *expected, record.Expected)
	records, err := c.ListExecutionEvidence(p.ID, task.ID, 50, 0)
	require.NoError(t, err)
	require.Len(t, records, 1)
	governing, err := c.ResolveGoverningPlan(p.ID, task.ID)
	require.NoError(t, err)
	require.Nil(t, governing)
	_, err = c.CloseIssueWithContext(task.ID, "done", false, expected)
	require.NoError(t, err)
	_, err = c.RecordExecutionEvidence(
		p.ID,
		task.ID,
		types.ExecutionEvidenceRequest{Expected: expected, Phase: "verify", Evidence: "stale"},
	)
	require.Error(t, err)
	epic, err := c.CreateIssue(
		p.ID,
		client.CreateIssueRequest{Title: "container", IssueType: "epic"},
	)
	require.NoError(t, err)
	p, err = c.GetProject(p.ID)
	require.NoError(t, err)
	adoption, err := c.AdoptPlan(
		p.ID,
		epic.ID,
		"",
		types.PlanAdoptionRequest{
			ExpectedContainerVersion:     epic.ContractVersion,
			ExpectedGovernanceGeneration: p.GovernanceGeneration,
			DryRun:                       true,
		},
	)
	require.NoError(t, err)
	require.True(t, adoption.DryRun)
}

func TestInterruptedUploadReportsSameRetryKey(t *testing.T) {
	keys := make(chan string, 2)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keys <- r.Header.Get("Idempotency-Key")
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = conn.Close()
	}))
	defer ts.Close()
	_, err := client.New(ts.URL).CreatePlan("project", "retry-original", storage.PlanUpload{Content: "exact"})
	require.ErrorContains(t, err, "--idempotency-key retry-original")
	require.Equal(t, []string{"retry-original", "retry-original"}, []string{<-keys, <-keys})
}
