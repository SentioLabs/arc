package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/types"
	"github.com/stretchr/testify/require"
)

func TestGovernedLegacyCompletionCannotBypassExpectation(t *testing.T) {
	for _, route := range []string{"close", "update", "update_and_get"} {
		t.Run(route, func(t *testing.T) {
			s, cleanup := setupTestStore(t)
			defer cleanup()
			p := setupTestProject(t, s)
			ids := setupGovernanceGraph(t, s, p, [][2]string{{"task", "epic"}}, []string{"epic"})
			ctx := context.Background()
			var err error
			switch route {
			case "close":
				err = s.CloseIssue(ctx, ids["task"], "done", false, "worker")
			case "update":
				err = s.UpdateIssue(ctx, ids["task"], map[string]any{"status": "closed"}, "worker")
			default:
				_, err = s.UpdateIssueAndGet(
					ctx,
					ids["task"],
					map[string]any{"status": "closed"},
					"worker",
				)
			}
			require.Error(t, err, "governed completion must require captured expectations")
			issue, err := s.GetIssue(ctx, ids["task"])
			require.NoError(t, err)
			require.NotEqual(t, types.StatusClosed, issue.Status)
		})
	}
}

func TestExecutionEvidenceAndGuardedCompletion(t *testing.T) {
	s, cleanup := setupTestStore(t)
	defer cleanup()
	p := setupTestProject(t, s)
	ids := setupGovernanceGraph(t, s, p, [][2]string{{"task", "epic"}}, []string{"epic"})
	ctx := storage.WithPlanProvenance(context.Background(), "builder", "session-original")
	i, err := s.GetIssue(ctx, ids["task"])
	require.NoError(t, err)
	g, err := s.ResolveGoverningPlan(ctx, p.ID, i.ID)
	require.NoError(t, err)
	expected := &types.ExpectedGovernance{Governing: g, ContractVersion: i.ContractVersion}
	e, err := s.RecordExecutionEvidence(
		ctx,
		p.ID,
		i.ID,
		types.ExecutionEvidenceRequest{
			Expected: expected,
			Phase:    "build",
			Evidence: "tests passed",
		},
	)
	require.NoError(t, err)
	require.Equal(t, *expected, e.Expected)
	require.Equal(t, "builder", e.Actor)
	require.Equal(t, "session-original", e.SessionID)
	after, err := s.GetIssue(ctx, i.ID)
	require.NoError(t, err)
	require.Equal(t, i.ContractVersion, after.ContractVersion)
	require.NoError(t, s.CloseIssueWithExpected(ctx, i.ID, "done", false, "worker", expected))
	evidence, err := s.ListExecutionEvidence(ctx, p.ID, i.ID, 50, 0)
	require.NoError(t, err)
	require.Len(t, evidence, 1)
	require.Equal(t, *expected, evidence[0].Expected)
}

func TestUnlinkedEvidenceMergeAndImmutableHistory(t *testing.T) {
	s, p := durableStore(t)
	ctx := context.Background()
	task := adoptionIssue(t, s, p, "legacy", "", types.TypeTask)
	expected := captured(t, s, p, task.ID)
	e, err := s.RecordExecutionEvidence(
		ctx,
		p,
		task.ID,
		types.ExecutionEvidenceRequest{
			Expected: &expected,
			Phase:    "build",
			Evidence: "legacy evidence",
		},
	)
	require.NoError(t, err)
	_, err = s.DB().Exec(`UPDATE execution_evidence SET result='{}' WHERE id=?`, e.ID)
	require.Error(t, err, "phase evidence must remain immutable")
	target := &types.Project{Name: "target", Prefix: "target"}
	require.NoError(t, s.CreateProject(ctx, target))
	_, err = s.MergeProjects(ctx, target.ID, []string{p}, "actor")
	require.NoError(t, err)
	records, err := s.ListExecutionEvidence(ctx, target.ID, task.ID, 50, 0)
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, e.Expected, records[0].Expected)
}

func TestGovernedClosedCreationRollsBack(t *testing.T) {
	s, p := durableStore(t)
	ctx := context.Background()
	root := adoptionIssue(t, s, p, "root", "", types.TypeEpic)
	pin := approvedAdoptionPin(t, s, p, "design")
	_, err := s.AdoptPlan(ctx, p, root.ID, "attach", adoptionRequest(t, s, p, root.ID, pin))
	require.NoError(t, err)
	before := adoptionDBSnapshot(t, s)
	now := time.Now()
	issue := &types.Issue{
		ProjectID: p,
		Title:     "already complete",
		ParentID:  root.ID,
		Status:    types.StatusClosed,
		ClosedAt:  &now,
	}
	err = s.CreateIssue(ctx, issue, "worker")
	require.Error(t, err)
	require.Equal(t, before, adoptionDBSnapshot(t, s))
	require.Empty(t, issue.ID)
}
