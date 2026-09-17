package sqlite_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/storage/sqlite"
	"github.com/sentiolabs/arc/internal/types"
	"github.com/stretchr/testify/require"
)

func approvedAdoptionPin(t *testing.T, s *sqlite.Store, p, key string) *types.PlanReference {
	t.Helper()
	ctx := context.Background()
	r, err := s.CreateDurablePlan(ctx, p, key, storage.PlanUpload{Title: key, Content: "# " + key})
	require.NoError(t, err)
	_, err = s.DecidePlanRevision(
		ctx,
		p,
		r.Plan.ID,
		1,
		types.PlanReviewRequest{
			Status:                  "in_review",
			ExpectedHead:            1,
			ExpectedReviewVersion:   0,
			ExpectedFeedbackVersion: 0,
		},
	)
	require.NoError(t, err)
	_, err = s.DecidePlanRevision(
		ctx,
		p,
		r.Plan.ID,
		1,
		types.PlanReviewRequest{
			Status:                  "approved",
			ExpectedHead:            1,
			ExpectedReviewVersion:   1,
			ExpectedFeedbackVersion: 0,
		},
	)
	require.NoError(t, err)
	return &types.PlanReference{PlanID: r.Plan.ID, Revision: 1}
}

//
//nolint:revive // Fixture needs project, title, parent and issue type.
func adoptionIssue(
	t *testing.T,
	s *sqlite.Store,
	p, title, parent string,
	typ types.IssueType,
) *types.Issue {
	t.Helper()
	i := &types.Issue{ProjectID: p, Title: title, ParentID: parent, IssueType: typ}
	require.NoError(t, s.CreateIssue(context.Background(), i, "fixture"))
	return i
}

func captured(t *testing.T, s *sqlite.Store, p, id string) types.ExpectedGovernance {
	t.Helper()
	i, e := s.GetIssue(context.Background(), id)
	require.NoError(t, e)
	g, e := s.ResolveGoverningPlan(context.Background(), p, id)
	require.NoError(t, e)
	return types.ExpectedGovernance{Governing: g, ContractVersion: i.ContractVersion}
}

func adoptionRequest(
	t *testing.T,
	s *sqlite.Store,
	p, root string,
	pin *types.PlanReference,
) types.PlanAdoptionRequest {
	t.Helper()
	ctx := context.Background()
	i, e := s.GetIssue(ctx, root)
	require.NoError(t, e)
	pr, e := s.GetProject(ctx, p)
	require.NoError(t, e)
	req := types.PlanAdoptionRequest{
		ExpectedContainerVersion:     i.ContractVersion,
		ExpectedGovernanceGeneration: pr.GovernanceGeneration,
		ExpectedPin:                  i.GoverningPlan,
		TargetPin:                    pin,
		DryRun:                       true,
	}
	proposal, e := s.AdoptPlan(ctx, p, root, "", req)
	require.NoError(t, e)
	for _, change := range proposal.Tasks {
		req.Tasks = append(
			req.Tasks,
			types.ReconciledTask{
				IssueID:     change.Before.IssueID,
				Expected:    change.Before.Expected,
				Disposition: "unchanged",
				Reason:      "consistent with target design",
			},
		)
	}
	req.DryRun = false
	return req
}

func TestAtomicAdoptionAttachDryRunAndReplay(t *testing.T) {
	s, p := durableStore(t)
	ctx := context.Background()
	root := adoptionIssue(t, s, p, "epic", "", types.TypeEpic)
	task := adoptionIssue(t, s, p, "task", root.ID, types.TypeTask)
	pin := approvedAdoptionPin(t, s, p, "design")
	req := adoptionRequest(t, s, p, root.ID, pin)
	before := captured(t, s, p, task.ID)
	req.FollowUps = []types.ReconciliationFollowUp{
		{Key: "repair", ParentID: root.ID, Title: "repair", IssueType: types.TypeTask, Priority: 2},
	}
	req.Tasks[0].Disposition = "follow_up"
	req.Tasks[0].FollowUpKeys = []string{"repair"}
	req.DryRun = true
	drySnapshot := adoptionDBSnapshot(t, s)
	proposal, err := s.AdoptPlan(ctx, p, root.ID, "key", req)
	require.NoError(t, err)
	require.Empty(t, proposal.Errors)
	require.Empty(t, proposal.FollowUpIDs)
	require.Equal(t, drySnapshot, adoptionDBSnapshot(t, s))
	require.Equal(t, before, captured(t, s, p, task.ID))
	req.DryRun = false
	result, err := s.AdoptPlan(ctx, p, root.ID, "key", req)
	require.NoError(t, err)
	require.NotEmpty(t, result.ID)
	require.Len(t, result.FollowUpIDs, 1)
	replay, err := s.AdoptPlan(ctx, p, root.ID, "key", req)
	require.NoError(t, err)
	require.True(t, replay.Replay)
	require.Equal(t, result.ID, replay.ID)
	require.Equal(t, result.FollowUpIDs, replay.FollowUpIDs)
	require.Greater(t, captured(t, s, p, task.ID).ContractVersion, before.ContractVersion)
	history, err := s.ListPlanAdoptions(ctx, p, root.ID, 50, 0)
	require.NoError(t, err)
	require.Len(t, history, 1)
}

func TestAdoptionLayeredPinsPauseAndClosedEvidence(t *testing.T) {
	s, p := durableStore(t)
	ctx := context.Background()
	m := adoptionIssue(t, s, p, "milestone", "", types.TypeMilestone)
	e := adoptionIssue(t, s, p, "epic", m.ID, types.TypeEpic)
	task := adoptionIssue(t, s, p, "task", e.ID, types.TypeTask)
	a := approvedAdoptionPin(t, s, p, "architecture")
	b := approvedAdoptionPin(t, s, p, "tactical")
	next := approvedAdoptionPin(t, s, p, "new architecture")
	newTactical := approvedAdoptionPin(t, s, p, "new tactical")
	_, err := s.AdoptPlan(ctx, p, m.ID, "attach", adoptionRequest(t, s, p, m.ID, a))
	require.NoError(t, err)
	_, err = s.AdoptPlan(ctx, p, e.ID, "tactical", adoptionRequest(t, s, p, e.ID, b))
	require.NoError(t, err)
	before := captured(t, s, p, task.ID)
	evidence, err := s.RecordExecutionEvidence(
		ctx,
		p,
		task.ID,
		types.ExecutionEvidenceRequest{
			Expected: &before,
			Phase:    "verify",
			Evidence: "original test run",
		},
	)
	require.NoError(t, err)
	req := adoptionRequest(t, s, p, m.ID, next)
	_, err = s.AdoptPlan(ctx, p, m.ID, "missing-tactical", req)
	require.Error(t, err)
	ei, err := s.GetIssue(ctx, e.ID)
	require.NoError(t, err)
	req.ContainerPins = []types.ReconciledContainerPin{
		{
			ContainerID:              e.ID,
			ExpectedContainerVersion: ei.ContractVersion,
			ExpectedPin:              b,
			TargetPin:                newTactical,
			Disposition:              "updated",
			Reason:                   "reconciles architecture change",
		},
	}
	require.NoError(
		t,
		s.UpdateIssue(ctx, task.ID, map[string]any{"status": "in_progress"}, "worker"),
	)
	req.Tasks[0].Expected = captured(t, s, p, task.ID)
	_, err = s.AdoptPlan(ctx, p, m.ID, "running", req)
	require.Error(t, err)
	require.NoError(t, s.UpdateIssue(ctx, task.ID, map[string]any{"status": "open"}, "worker"))
	req.Tasks[0].Expected = captured(t, s, p, task.ID)
	result, err := s.AdoptPlan(ctx, p, m.ID, "adopt-both", req)
	require.NoError(t, err)
	require.Len(t, result.Tasks, 1)
	require.Equal(t, req, result.Request)
	require.Len(t, result.Containers, 2)
	for _, c := range result.Containers {
		require.Greater(t, c.After.Expected.ContractVersion, c.Before.Expected.ContractVersion)
	}
	now := captured(t, s, p, task.ID)
	require.Equal(t, *newTactical, now.Governing.Reference)
	require.Equal(t, *next, now.Governing.Context[0].Reference)
	require.Error(t, s.CloseIssueWithExpected(ctx, task.ID, "stale", false, "worker", &before))
	records, err := s.ListExecutionEvidence(ctx, p, task.ID, 50, 0)
	require.NoError(t, err)
	require.Equal(t, evidence.Expected, records[0].Expected)
	require.NoError(t, s.CloseIssueWithExpected(ctx, task.ID, "done", false, "worker", &now))
	detach := adoptionRequest(t, s, p, m.ID, nil)
	ei, err = s.GetIssue(ctx, e.ID)
	require.NoError(t, err)
	detach.ContainerPins = []types.ReconciledContainerPin{
		{
			ContainerID:              e.ID,
			ExpectedContainerVersion: ei.ContractVersion,
			ExpectedPin:              newTactical,
			TargetPin:                newTactical,
			Disposition:              "compatible",
			Reason:                   "completed work remains valid",
		},
	}
	_, err = s.AdoptPlan(ctx, p, m.ID, "detach", detach)
	require.NoError(t, err)
	records, err = s.ListExecutionEvidence(ctx, p, task.ID, 50, 0)
	require.NoError(t, err)
	require.Equal(t, evidence.Expected, records[0].Expected)
	// Historical replay survives a subsequent adoption and plan archive.
	meta, err := s.GetDurablePlan(ctx, p, next.PlanID)
	require.NoError(t, err)
	_, err = s.UpdateDurablePlan(
		ctx,
		p,
		next.PlanID,
		storage.PlanMetadataUpdate{ExpectedVersion: meta.Version, Lifecycle: "archived"},
	)
	require.NoError(t, err)
	replay, err := s.AdoptPlan(ctx, p, m.ID, "adopt-both", req)
	require.NoError(t, err)
	require.True(t, replay.Replay)
	require.Equal(t, result.ID, replay.ID)
}

func TestAdoptionStagedSourceIdentityAndVersionUnion(t *testing.T) {
	s, p := durableStore(t)
	ctx := context.Background()
	root := adoptionIssue(t, s, p, "root", "", types.TypeMilestone)
	a := adoptionIssue(t, s, p, "a", root.ID, types.TypeEpic)
	b := adoptionIssue(t, s, p, "b", root.ID, types.TypeEpic)
	middle := adoptionIssue(t, s, p, "middle", a.ID, types.TypeEpic)
	task := adoptionIssue(t, s, p, "task", middle.ID, types.TypeTask)
	pin := approvedAdoptionPin(t, s, p, "same bytes")
	_, err := s.AdoptPlan(ctx, p, a.ID, "a", adoptionRequest(t, s, p, a.ID, pin))
	require.NoError(t, err)
	_, err = s.AdoptPlan(ctx, p, b.ID, "b", adoptionRequest(t, s, p, b.ID, pin))
	require.NoError(t, err)
	before := captured(t, s, p, task.ID)
	req := adoptionRequest(t, s, p, root.ID, nil)
	req.Edges = []types.ReconciliationEdge{
		{IssueID: middle.ID, DependsOnID: a.ID, Type: types.DepParentChild, Remove: true},
		{IssueID: middle.ID, DependsOnID: b.ID, Type: types.DepParentChild},
	}
	req.DryRun = true
	preview, err := s.AdoptPlan(ctx, p, root.ID, "", req)
	require.NoError(t, err)
	require.Len(t, preview.Tasks, 1)
	require.NotEmpty(t, preview.Errors)
	req.DryRun = false
	req.Tasks = []types.ReconciledTask{
		{
			IssueID:     task.ID,
			Expected:    before,
			Disposition: "unchanged",
			Reason:      "same approved plan, changed source",
		},
	}
	_, err = s.AdoptPlan(ctx, p, root.ID, "move", req)
	require.NoError(t, err)
	after := captured(t, s, p, task.ID)
	require.Greater(t, after.ContractVersion, before.ContractVersion)
	require.Equal(t, b.ID, after.Governing.ContainerID)
	require.Equal(t, before.Governing.Reference, after.Governing.Reference)
}

func TestAdoptionRollbackAtAuditIndexCounterAndHistory(t *testing.T) {
	for _, failure := range []string{"event", "index", "counter", "history"} {
		t.Run(failure, func(t *testing.T) {
			s, p := durableStore(t)
			ctx := context.Background()
			root := adoptionIssue(t, s, p, "epic", "", types.TypeEpic)
			task := adoptionIssue(t, s, p, "task", root.ID, types.TypeTask)
			pin := approvedAdoptionPin(t, s, p, "design")
			req := adoptionRequest(t, s, p, root.ID, pin)
			title := "changed"
			req.Tasks[0].Disposition = "updated"
			req.Tasks[0].Title = &title
			req.FollowUps = []types.ReconciliationFollowUp{
				{
					Key:       "new",
					ParentID:  root.ID,
					Title:     "follow",
					IssueType: types.TypeTask,
					Priority:  2,
				},
			}
			// A second affected task links the follow-up while the first edits its contract.
			second := adoptionIssue(t, s, p, "second", root.ID, types.TypeTask)
			req = adoptionRequest(t, s, p, root.ID, pin)
			for n := range req.Tasks {
				if req.Tasks[n].IssueID == task.ID {
					req.Tasks[n].Disposition = "updated"
					req.Tasks[n].Title = &title
				} else {
					req.Tasks[n].Disposition = "follow_up"
					req.Tasks[n].FollowUpKeys = []string{"new"}
				}
			}
			req.FollowUps = []types.ReconciliationFollowUp{
				{
					Key:       "new",
					ParentID:  root.ID,
					Title:     "follow",
					IssueType: types.TypeTask,
					Priority:  2,
				},
			}
			_ = second
			switch failure {
			case "event":
				_, err := s.DB().
					Exec(`
CREATE TRIGGER fail_adoption BEFORE INSERT ON events BEGIN SELECT RAISE(ABORT,'audit failure');
END
`)
				require.NoError(t, err)
			case "index":
				_, err := s.DB().Exec(`DROP TABLE issues_fts`)
				require.NoError(t, err)
			case "counter":
				_, err := s.DB().
					Exec(`
CREATE TRIGGER fail_adoption BEFORE UPDATE ON child_counters BEGIN SELECT RAISE(ABORT,'counter
failure'); END
`)
				require.NoError(t, err)
			case "history":
				_, err := s.DB().
					Exec(`
CREATE TRIGGER fail_adoption BEFORE INSERT ON plan_adoptions BEGIN SELECT RAISE(ABORT,'history
failure'); END
`)
				require.NoError(t, err)
			}
			before := adoptionDBSnapshot(t, s)
			_, err := s.AdoptPlan(ctx, p, root.ID, "failure", req)
			require.Error(t, err)
			require.Equal(t, before, adoptionDBSnapshot(t, s))
		})
	}
}

func adoptionDBSnapshot(t *testing.T, s *sqlite.Store) map[string][]string {
	t.Helper()
	snapshot := map[string][]string{}
	for _, table := range []string{
		"issues",
		"dependencies",
		"events",
		"child_counters",
		"projects",
		"issue_governance_history",
		"plan_adoptions",
		"adoption_idempotency",
		"execution_evidence",
		"issues_fts",
	} {
		rows, err := s.DB().Query("SELECT * FROM " + table + " ORDER BY 1")
		if err != nil {
			snapshot[table] = []string{err.Error()}
			continue
		}
		columns, err := rows.Columns()
		require.NoError(t, err)
		for rows.Next() {
			values := make([]any, len(columns))
			dest := make([]any, len(columns))
			for n := range values {
				dest[n] = &values[n]
			}
			require.NoError(t, rows.Scan(dest...))
			body, err := json.Marshal(values)
			require.NoError(t, err)
			snapshot[table] = append(snapshot[table], string(body))
		}
		require.NoError(t, rows.Err())
		require.NoError(t, rows.Close())
	}
	return snapshot
}

func TestAdoptionRejectsUnrelatedContentEdit(t *testing.T) {
	s, p := durableStore(t)
	ctx := context.Background()
	root := adoptionIssue(t, s, p, "root", "", types.TypeEpic)
	unrelated := adoptionIssue(t, s, p, "unrelated", "", types.TypeTask)
	pin := approvedAdoptionPin(t, s, p, "design")
	req := adoptionRequest(t, s, p, root.ID, pin)
	title := "unrelated change"
	req.Tasks = []types.ReconciledTask{
		{
			IssueID:     unrelated.ID,
			Expected:    captured(t, s, p, unrelated.ID),
			Disposition: "updated",
			Title:       &title,
			Reason:      "unrelated",
		},
	}
	_, err := s.AdoptPlan(ctx, p, root.ID, "unrelated", req)
	require.Error(t, err)
}

//nolint:revive // Parallel table scenarios keep each race and its assertions together.
func TestAdoptionConcurrentReplayAndWorkerRaces(t *testing.T) {
	for _, race := range []string{"identical", "claim", "completion"} {
		t.Run(race, func(t *testing.T) {
			s, p := durableStore(t)
			ctx := context.Background()
			root := adoptionIssue(t, s, p, "root", "", types.TypeEpic)
			task := adoptionIssue(t, s, p, "task", root.ID, types.TypeTask)
			pin := approvedAdoptionPin(t, s, p, "design")
			req := adoptionRequest(t, s, p, root.ID, pin)
			expected := captured(t, s, p, task.ID)
			start := make(chan struct{})
			results := make(chan error, 2)
			replays := make(chan bool, 2)
			go func() {
				<-start
				r, err := s.AdoptPlan(ctx, p, root.ID, "race", req)
				if r != nil {
					replays <- r.Replay
				}
				results <- err
			}()
			go func() {
				<-start
				switch race {
				case "identical":
					r, err := s.AdoptPlan(ctx, p, root.ID, "race", req)
					if r != nil {
						replays <- r.Replay
					}
					results <- err
				case "claim":
					results <- s.UpdateIssue(
						ctx, task.ID, map[string]any{"status": "in_progress", "ai_session_id": "worker"}, "worker",
					)
				case "completion":
					results <- s.CloseIssueWithExpected(ctx, task.ID, "done", false, "worker", &expected)
				}
			}()
			close(start)
			first, second := <-results, <-results
			switch race {
			case "identical":
				require.NoError(t, first)
				require.NoError(t, second)
				firstReplay, secondReplay := <-replays, <-replays
				require.NotEqual(t, firstReplay, secondReplay)
			case "completion":
				require.NotEqual(
					t,
					first == nil,
					second == nil,
					"only one competing contract write can succeed",
				)
			case "claim":
				issue, err := s.GetIssue(ctx, task.ID)
				require.NoError(t, err)
				require.Equal(t, types.StatusInProgress, issue.Status)
				require.True(t, first == nil || second == nil)
			}
		})
	}
}

func TestAdoptionValidationMatrix(t *testing.T) {
	for _, kind := range []string{
		"duplicate_task",
		"missing_reason",
		"closed_edit",
		"cycle",
		"dangling_new",
		"duplicate_new",
		"unused_new",
		"cross_project",
		"stale_version",
		"stale_generation",
		"draft",
		"tampered",
		"unchanged_running",
		"closed_type",
		"closed_edge",
	} {
		t.Run(kind, func(t *testing.T) {
			s, p := durableStore(t)
			ctx := context.Background()
			root := adoptionIssue(t, s, p, "root", "", types.TypeEpic)
			task := adoptionIssue(t, s, p, "task", root.ID, types.TypeTask)
			pin := approvedAdoptionPin(t, s, p, "design")
			req := adoptionRequest(t, s, p, root.ID, pin)
			title := "changed"
			switch kind {
			case "duplicate_task":
				req.Tasks = append(req.Tasks, req.Tasks[0])
			case "missing_reason":
				req.Tasks[0].Reason = " "
			case "closed_edit":
				require.NoError(t, s.CloseIssue(ctx, task.ID, "done", false, "actor"))
				req.Tasks[0].Expected = captured(t, s, p, task.ID)
				req.Tasks[0].Disposition = "updated"
				req.Tasks[0].Title = &title
			case "closed_type":
				require.NoError(t, s.CloseIssue(ctx, task.ID, "done", false, "actor"))
				req.TypeChanges = []types.ReconciliationTypeChange{
					{
						IssueID:                 task.ID,
						ExpectedContractVersion: captured(t, s, p, task.ID).ContractVersion,
						IssueType:               types.TypeBug,
					},
				}
			case "closed_edge":
				require.NoError(t, s.CloseIssue(ctx, task.ID, "done", false, "actor"))
				req.Edges = []types.ReconciliationEdge{
					{
						IssueID:     task.ID,
						DependsOnID: root.ID,
						Type:        types.DepParentChild,
						Remove:      true,
					},
				}
			case "cycle":
				req.Edges = []types.ReconciliationEdge{
					{IssueID: root.ID, DependsOnID: task.ID, Type: types.DepParentChild},
				}
			case "dangling_new":
				req.Edges = []types.ReconciliationEdge{
					{IssueID: "new:missing", DependsOnID: task.ID, Type: types.DepBlocks},
				}
			case "duplicate_new", "unused_new":
				f := types.ReconciliationFollowUp{
					Key:       "a",
					ParentID:  root.ID,
					Title:     "a",
					IssueType: types.TypeTask,
					Priority:  2,
				}
				req.FollowUps = []types.ReconciliationFollowUp{f}
				if kind == "duplicate_new" {
					req.FollowUps = append(req.FollowUps, f)
				}
			case "cross_project":
				other := &types.Project{Name: "other", Prefix: "other"}
				require.NoError(t, s.CreateProject(ctx, other))
				foreign := adoptionIssue(t, s, other.ID, "foreign", "", types.TypeTask)
				req.Edges = []types.ReconciliationEdge{
					{IssueID: task.ID, DependsOnID: foreign.ID, Type: types.DepBlocks},
				}
			case "stale_version":
				req.Tasks[0].Expected.ContractVersion++
			case "stale_generation":
				req.ExpectedGovernanceGeneration++
			case "draft":
				r, err := s.CreateDurablePlan(ctx, p, "draft", storage.PlanUpload{Content: "draft"})
				require.NoError(t, err)
				req.TargetPin = &types.PlanReference{PlanID: r.Plan.ID, Revision: 1}
			case "tampered":
				_, err := s.DB().
					Exec(`UPDATE plan_revisions SET content_sha256='broken' WHERE plan_id=?`, pin.PlanID)
				require.NoError(t, err)
			case "unchanged_running":
				require.NoError(
					t,
					s.UpdateIssue(ctx, task.ID, map[string]any{"status": "in_progress"}, "actor"),
				)
				req.Tasks[0].Expected = captured(t, s, p, task.ID)
			}
			before := adoptionDBSnapshot(t, s)
			_, err := s.AdoptPlan(ctx, p, root.ID, "invalid", req)
			require.Error(t, err)
			require.Equal(t, before, adoptionDBSnapshot(t, s))
		})
	}
}

func TestAdoptionResolvesNewEdgeEndpointsDuringApply(t *testing.T) {
	s, p := durableStore(t)
	ctx := context.Background()
	root := adoptionIssue(t, s, p, "root", "", types.TypeEpic)
	task := adoptionIssue(t, s, p, "task", root.ID, types.TypeTask)
	pin := approvedAdoptionPin(t, s, p, "design")
	req := adoptionRequest(t, s, p, root.ID, pin)
	req.Tasks[0].Disposition = "follow_up"
	req.Tasks[0].FollowUpKeys = []string{"child"}
	req.FollowUps = []types.ReconciliationFollowUp{
		{Key: "child", ParentID: root.ID, Title: "follow", IssueType: types.TypeTask, Priority: 2},
	}
	req.Edges = []types.ReconciliationEdge{
		{IssueID: "new:child", DependsOnID: root.ID, Type: types.DepParentChild, Remove: true},
		{IssueID: "new:child", DependsOnID: task.ID, Type: types.DepParentChild},
	}
	result, err := s.AdoptPlan(ctx, p, root.ID, "new-edges", req)
	require.NoError(t, err)
	edges, err := s.GetDependencies(ctx, result.FollowUpIDs["child"])
	require.NoError(t, err)
	require.Len(t, edges, 1)
	require.Equal(t, task.ID, edges[0].DependsOnID)
}

func TestAdoptionCommitsStagedContractsAndContainerType(t *testing.T) {
	s, p := durableStore(t)
	ctx := context.Background()
	root := adoptionIssue(t, s, p, "root", "", types.TypeEpic)
	task := adoptionIssue(t, s, p, "task", root.ID, types.TypeTask)
	pin := approvedAdoptionPin(t, s, p, "design")
	req := adoptionRequest(t, s, p, root.ID, pin)
	title, description := "reconciled title", "reconciled contract"
	req.Tasks[0].Disposition = "updated"
	req.Tasks[0].Title = &title
	req.Tasks[0].Description = &description
	req.TypeChanges = []types.ReconciliationTypeChange{
		{
			IssueID:                 task.ID,
			ExpectedContractVersion: req.Tasks[0].Expected.ContractVersion,
			IssueType:               types.TypeBug,
		},
	}
	result, err := s.AdoptPlan(ctx, p, root.ID, "contracts", req)
	require.NoError(t, err)
	i, err := s.GetIssue(ctx, task.ID)
	require.NoError(t, err)
	require.Equal(t, title, i.Title)
	require.Equal(t, description, i.Description)
	require.Equal(t, types.TypeBug, i.IssueType)
	require.Equal(t, i.ContractVersion, result.Tasks[0].After.Expected.ContractVersion)
	var indexed string
	require.NoError(
		t,
		s.DB().QueryRow(`SELECT title FROM issues_fts WHERE id=?`, task.ID).Scan(&indexed),
	)
	require.Equal(t, title, indexed)
	req = adoptionRequest(t, s, p, root.ID, nil)
	req.TypeChanges = []types.ReconciliationTypeChange{
		{
			IssueID:                 root.ID,
			ExpectedContractVersion: req.ExpectedContainerVersion,
			IssueType:               types.TypeFeature,
		},
	}
	req.DryRun = true
	req.Tasks = nil
	proposal, err := s.AdoptPlan(ctx, p, root.ID, "", req)
	require.NoError(t, err)
	require.Len(t, proposal.Tasks, 2)
	for _, change := range proposal.Tasks {
		disposition := "unchanged"
		if change.Before.IssueID == root.ID {
			disposition = "updated"
		}
		req.Tasks = append(
			req.Tasks,
			types.ReconciledTask{
				IssueID:     change.Before.IssueID,
				Expected:    change.Before.Expected,
				Disposition: disposition,
				Reason:      "remove container design",
			},
		)
	}
	req.DryRun = false
	_, err = s.AdoptPlan(ctx, p, root.ID, "convert", req)
	require.NoError(t, err)
	i, err = s.GetIssue(ctx, root.ID)
	require.NoError(t, err)
	require.Equal(t, types.TypeFeature, i.IssueType)
	require.Nil(t, i.GoverningPlan)
}

func TestAdoptionValidatesInheritedStagedTargets(t *testing.T) {
	for _, failure := range []string{"archived", "tampered"} {
		t.Run(failure, func(t *testing.T) {
			s, p := durableStore(t)
			ctx := context.Background()
			root := adoptionIssue(t, s, p, "root", "", types.TypeMilestone)
			a := adoptionIssue(t, s, p, "a", root.ID, types.TypeEpic)
			b := adoptionIssue(t, s, p, "b", root.ID, types.TypeEpic)
			task := adoptionIssue(t, s, p, "task", a.ID, types.TypeTask)
			pin := approvedAdoptionPin(t, s, p, "target")
			_, err := s.AdoptPlan(ctx, p, b.ID, "attach", adoptionRequest(t, s, p, b.ID, pin))
			require.NoError(t, err)
			req := adoptionRequest(t, s, p, root.ID, nil)
			req.Edges = []types.ReconciliationEdge{
				{IssueID: task.ID, DependsOnID: a.ID, Type: types.DepParentChild, Remove: true},
				{IssueID: task.ID, DependsOnID: b.ID, Type: types.DepParentChild},
			}
			req.Tasks = []types.ReconciledTask{
				{
					IssueID:     task.ID,
					Expected:    captured(t, s, p, task.ID),
					Disposition: "updated",
					Reason:      "move into approved context",
				},
			}
			if failure == "archived" {
				meta, err := s.GetDurablePlan(ctx, p, pin.PlanID)
				require.NoError(t, err)
				_, err = s.UpdateDurablePlan(
					ctx,
					p,
					pin.PlanID,
					storage.PlanMetadataUpdate{
						ExpectedVersion: meta.Version,
						Lifecycle:       "archived",
					},
				)
				require.NoError(t, err)
			} else {
				_, err = s.DB().Exec(`UPDATE plan_revisions SET content_sha256='tampered' WHERE plan_id=?`, pin.PlanID)
				require.NoError(t, err)
			}
			snapshot := adoptionDBSnapshot(t, s)
			_, err = s.AdoptPlan(ctx, p, root.ID, "move", req)
			require.Error(t, err)
			require.Equal(t, snapshot, adoptionDBSnapshot(t, s))
		})
	}
}

func TestAdoptionValidatesNewFollowUpGovernance(t *testing.T) {
	s, p := durableStore(t)
	ctx := context.Background()
	root := adoptionIssue(t, s, p, "root", "", types.TypeMilestone)
	a := adoptionIssue(t, s, p, "a", root.ID, types.TypeEpic)
	b := adoptionIssue(t, s, p, "b", root.ID, types.TypeEpic)
	middle := adoptionIssue(t, s, p, "middle", a.ID, types.TypeEpic)
	task := adoptionIssue(t, s, p, "task", middle.ID, types.TypeTask)
	source := approvedAdoptionPin(t, s, p, "source")
	target := approvedAdoptionPin(t, s, p, "target")
	_, err := s.AdoptPlan(ctx, p, a.ID, "a", adoptionRequest(t, s, p, a.ID, source))
	require.NoError(t, err)
	_, err = s.AdoptPlan(ctx, p, b.ID, "b", adoptionRequest(t, s, p, b.ID, target))
	require.NoError(t, err)
	req := adoptionRequest(t, s, p, root.ID, nil)
	req.Edges = []types.ReconciliationEdge{
		{IssueID: middle.ID, DependsOnID: a.ID, Type: types.DepParentChild, Remove: true},
		{IssueID: middle.ID, DependsOnID: root.ID, Type: types.DepParentChild},
	}
	req.Tasks = []types.ReconciledTask{
		{
			IssueID:      task.ID,
			Expected:     captured(t, s, p, task.ID),
			Disposition:  "follow_up",
			Reason:       "move remaining work",
			FollowUpKeys: []string{"new"},
		},
	}
	req.FollowUps = []types.ReconciliationFollowUp{
		{Key: "new", ParentID: b.ID, Title: "follow-up", IssueType: types.TypeTask, Priority: 2},
	}
	meta, err := s.GetDurablePlan(ctx, p, target.PlanID)
	require.NoError(t, err)
	_, err = s.UpdateDurablePlan(
		ctx,
		p,
		target.PlanID,
		storage.PlanMetadataUpdate{ExpectedVersion: meta.Version, Lifecycle: "archived"},
	)
	require.NoError(t, err)
	snapshot := adoptionDBSnapshot(t, s)
	_, err = s.AdoptPlan(ctx, p, root.ID, "follow", req)
	require.Error(t, err)
	require.Equal(t, snapshot, adoptionDBSnapshot(t, s))
}
