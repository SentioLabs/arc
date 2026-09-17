package sqlite_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/sentiolabs/arc/internal/planfiles"
	"github.com/sentiolabs/arc/internal/storage/sqlite"

	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/types"
	"github.com/stretchr/testify/require"
)

func TestLegacyImportPreservesProvenanceAndReplays(t *testing.T) {
	s, p := durableStore(t)
	ctx := context.Background()
	legacy := &types.LegacyPlan{ID: "plan.legacy", FilePath: "/unavailable/old.md", Status: "approved"}
	require.NoError(t, s.CreatePlan(ctx, legacy))
	comment := &types.PlanComment{
		ID:      "comment.legacy",
		PlanID:  legacy.ID,
		Content: "exact\r\n旧",
		Anchor:  &types.PlanCommentAnchor{LineStart: 1, LineEnd: 1, QuotedText: "prior", Occurrence: 1},
	}
	require.NoError(t, s.CreatePlanComment(ctx, comment))
	req := storage.LegacyPlanImport{
		LegacyID:   legacy.ID,
		ProjectID:  p,
		SourceFile: "/explicit/new.md",
		Content:    "edited after approval\r\n",
	}
	result, err := s.ImportLegacyPlan(ctx, req)
	require.NoError(t, err)
	require.Equal(t, legacy.ID, result.Plan.ID)
	require.Equal(t, legacy.CreatedAt.Unix(), result.Plan.CreatedAt.Unix())
	require.Equal(t, "draft", result.Revision.ReviewStatus)
	require.Equal(t, req.Content, result.Revision.Content)
	comments, err := s.ListRevisionComments(ctx, p, legacy.ID, 1, true, 50, 0)
	require.NoError(t, err)
	require.Len(t, comments, 1)
	require.Nil(t, comments[0].Revision)
	require.Equal(t, int64(1), comments[0].Version)
	require.Equal(t, comment.Content, comments[0].Content)
	require.Equal(t, comment.Anchor, comments[0].Anchor)
	var originalStatus, body string
	require.NoError(
		t,
		s.DB().QueryRow("SELECT legacy_status_unverified FROM plans WHERE id=?", legacy.ID).Scan(&originalStatus),
	)
	require.Equal(t, "approved", originalStatus)
	require.NoError(
		t,
		s.DB().QueryRow("SELECT body FROM plan_comment_events WHERE comment_id=?", comment.ID).Scan(&body),
	)
	var recorded types.PlanComment
	require.NoError(t, json.Unmarshal([]byte(body), &recorded))
	require.Equal(t, *comments[0], recorded)
	replay, err := s.ImportLegacyPlan(ctx, req)
	require.NoError(t, err)
	require.True(t, replay.Replay)
	req.SourceFile = "/different.md"
	_, err = s.ImportLegacyPlan(ctx, req)
	require.ErrorIs(t, err, storage.ErrPlanConflict)
	inventory, err := s.ListLegacyPlans(ctx, 50, 0)
	require.NoError(t, err)
	require.Len(t, inventory, 1)
	require.Equal(t, p, inventory[0].ImportedProjectID)
	require.Equal(t, legacy.FilePath, inventory[0].FilePath)
	require.Equal(t, "approved", inventory[0].Status)
	require.Equal(t, comment.Content, inventory[0].Comments[0].Content)
}

// importInterruptedPublisher simulates death after durable publication, before
// SQLite can store its reference, and can hold that dangerous window open.
type importInterruptedPublisher struct {
	planfiles.Publisher
	published chan struct{}
	release   chan struct{}
}

func (p *importInterruptedPublisher) Publish(
	ctx context.Context,
	project, id string,
	revision int64,
	content []byte,
) (planfiles.Blob, error) {
	blob, err := p.Publisher.Publish(ctx, project, id, revision, content)
	if err != nil {
		return blob, err
	}
	if p.published != nil {
		close(p.published)
		<-p.release
	}
	return planfiles.Blob{}, errors.New("interrupted after publication")
}

func TestLegacyImportPublicationInterruptionAndCommitReplay(t *testing.T) {
	s, cleanup := setupTestStore(t)
	defer cleanup()
	project := setupTestProject(t, s).ID
	root := filepath.Join(t.TempDir(), "plans")
	publisher, err := planfiles.New(root)
	require.NoError(t, err)
	ctx := t.Context()
	require.NoError(
		t,
		s.CreatePlan(ctx, &types.LegacyPlan{ID: "plan.interrupted", FilePath: "/prior", Status: "approved"}),
	)
	require.NoError(
		t,
		s.CreatePlanComment(
			ctx,
			&types.PlanComment{ID: "comment.interrupted", PlanID: "plan.interrupted", Content: "retained"},
		),
	)
	interrupted := &importInterruptedPublisher{
		Publisher: publisher,
		published: make(chan struct{}),
		release:   make(chan struct{}),
	}
	s.SetPlanPublisher(interrupted)
	req := storage.LegacyPlanImport{
		LegacyID:   "plan.interrupted",
		ProjectID:  project,
		SourceFile: "/explicit/source",
		Content:    "bytes",
	}
	done := make(chan error, 1)
	go func() { _, err := s.ImportLegacyPlan(ctx, req); done <- err }()
	<-interrupted.published
	_, err = planfiles.OpenMaintenance(root)
	require.ErrorIs(t, err, planfiles.ErrBusy)
	close(interrupted.release)
	require.Error(t, <-done)
	require.NoError(t, publisher.Close())
	m, err := planfiles.OpenMaintenance(root)
	require.NoError(t, err)
	references, err := s.PlanBlobs(ctx)
	require.NoError(t, err)
	require.Empty(t, references)
	reports, err := m.Inspect(ctx, references)
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.Equal(t, "orphan", reports[0].Status)
	require.NoError(t, m.Close())
	publisher, err = planfiles.New(root)
	require.NoError(t, err)
	defer publisher.Close()
	s.SetPlanPublisher(publisher)
	// Fail the DB transaction after plan/comment insertion but before its marker.
	_, err = s.DB().Exec(`CREATE TRIGGER fail_import BEFORE INSERT ON plan_idempotency
 BEGIN SELECT RAISE(ABORT,'interrupted commit'); END`)
	require.NoError(t, err)
	_, err = s.ImportLegacyPlan(ctx, req)
	require.Error(t, err)
	for _, table := range []string{
		"plans",
		"plan_revisions",
		"plan_comments",
		"plan_comment_events",
		"plan_idempotency",
	} {
		var count int
		require.NoError(t, s.DB().QueryRow("SELECT count(*) FROM "+table).Scan(&count))
		require.Zero(t, count)
	}
	_, err = s.DB().Exec("DROP TRIGGER fail_import")
	require.NoError(t, err)
	_, err = s.ImportLegacyPlan(ctx, req)
	require.NoError(t, err)
	// Discard the success response, reopen the DB, then retry the same manifest.
	path := s.Path()
	require.NoError(t, s.Close())
	reopened, err := sqlite.OpenPlanOperator(path, true)
	require.NoError(t, err)
	defer reopened.Close()
	reopened.SetPlanPublisher(publisher)
	result, err := reopened.ImportLegacyPlan(ctx, req)
	require.NoError(t, err)
	require.True(t, result.Replay)
	var count int
	require.NoError(t, reopened.DB().QueryRow("SELECT count(*) FROM plan_revisions").Scan(&count))
	require.Equal(t, 1, count)
}
