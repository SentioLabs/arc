package sqlite_test

import (
	"fmt"
	"testing"

	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/storage/sqlite"
	"github.com/sentiolabs/arc/internal/types"
	"github.com/stretchr/testify/require"
)

func TestPlanReviewEvidenceRemainsAccessible(t *testing.T) {
	s, p, comment, original := approvedHistoryFixture(t)
	id := comment.PlanID
	ctx := storage.WithPlanProvenance(t.Context(), "later-editor", "session-later")
	editedBody := "edited question"
	_, err := s.UpdateRevisionComment(
		ctx,
		p,
		id,
		1,
		comment.ID,
		storage.PlanCommentUpdate{
			ExpectedVersion: 1,
			Content:         &editedBody,
			Anchor: &types.PlanCommentAnchor{
				LineStart:  3,
				LineEnd:    3,
				QuotedText: "edited",
				Occurrence: 1,
			},
		},
	)
	require.NoError(t, err)
	_, err = s.SavePlanRevision(
		ctx,
		p,
		id,
		"save",
		storage.PlanSave{Content: "new design", ExpectedRevision: 1},
	)
	require.NoError(t, err)
	_, err = s.AddPlanDisposition(
		ctx,
		p,
		id,
		2,
		storage.PlanDispositionRequest{
			CommentID:               comment.ID,
			ExpectedCommentVersion:  2,
			ExpectedFeedbackVersion: 3,
			Disposition:             "addressed",
			Reason:                  "later answer",
		},
	)
	require.NoError(t, err)
	_, err = s.UpdateRevisionComment(
		ctx,
		p,
		id,
		1,
		comment.ID,
		storage.PlanCommentUpdate{ExpectedVersion: 2, Reopen: true},
	)
	require.NoError(t, err)
	_, err = s.UpdateRevisionComment(
		ctx,
		p,
		id,
		1,
		comment.ID,
		storage.PlanCommentUpdate{ExpectedVersion: 3, Delete: true},
	)
	require.NoError(t, err)
	meta, err := s.GetDurablePlan(ctx, p, id)
	require.NoError(t, err)
	_, err = s.UpdateDurablePlan(
		ctx,
		p,
		id,
		storage.PlanMetadataUpdate{ExpectedVersion: meta.Version, Lifecycle: "archived"},
	)
	require.NoError(t, err)
	retained, err := s.ListPlanReviewEvents(ctx, p, id, 1, 200, 0)
	require.NoError(t, err)
	require.Equal(
		t,
		original,
		retained,
		"later feedback/head/lifecycle must not rewrite approval evidence",
	)
	versions, err := s.ListPlanCommentVersions(ctx, p, id, 1, comment.ID, 200, 0)
	require.NoError(t, err)
	require.Len(t, versions, 4)
	require.Equal(t, *comment, versions[0].Comment)
	require.Equal(t, comment.Anchor, versions[0].Comment.Anchor)
	require.Equal(t, "reviewer", versions[0].Actor)
	require.Equal(t, "session-history", versions[0].SessionID)
	require.False(t, versions[0].CreatedAt.IsZero())
	require.Equal(t, editedBody, versions[1].Comment.Content)
	require.Equal(t, "later-editor", versions[1].Actor)
	require.Equal(t, "session-later", versions[1].SessionID)
	require.EqualValues(t, 3, versions[2].Comment.Version)
	require.NotNil(t, versions[3].Comment.DeletedAt)
	require.Nil(t, versions[0].Comment.DeletedAt)

	reviewPage, err := s.ListPlanReviewEvents(ctx, p, id, 1, 1, 1)
	require.NoError(t, err)
	require.Equal(t, original[1:], reviewPage)
	commentPage, err := s.ListPlanCommentVersions(ctx, p, id, 1, comment.ID, 1, 1)
	require.NoError(t, err)
	require.Equal(t, versions[1:2], commentPage)
	for _, offset := range []int{4, 200} {
		page, e := s.ListPlanCommentVersions(ctx, p, id, 1, comment.ID, 1, offset)
		require.NoError(t, e)
		require.Empty(t, page)
	}
	checkHistoryOwnership(t, s, p, comment)
}

// approvedHistoryFixture captures the exact approval before any mutable feedback changes.
func approvedHistoryFixture(t *testing.T) (
	store *sqlite.Store, project string, comment *types.PlanComment, events []*storage.PlanReviewEvent,
) {
	t.Helper()
	s, p := durableStore(t)
	ctx := storage.WithPlanProvenance(t.Context(), "reviewer", "session-history")
	first, err := s.CreateDurablePlan(
		ctx,
		p,
		"create",
		storage.PlanUpload{Content: "original design"},
	)
	require.NoError(t, err)
	id := first.Plan.ID
	anchor := &types.PlanCommentAnchor{
		LineStart:     1,
		LineEnd:       2,
		QuotedText:    "original design",
		Occurrence:    0,
		HeadingSlug:   "original",
		ContextBefore: "before",
		ContextAfter:  "after",
	}
	comment, err = s.CreateRevisionComment(
		ctx,
		p,
		id,
		1,
		storage.PlanCommentCreate{Content: "original question", Anchor: anchor},
	)
	require.NoError(t, err)
	disposition, err := s.AddPlanDisposition(
		ctx,
		p,
		id,
		1,
		storage.PlanDispositionRequest{
			CommentID:               comment.ID,
			ExpectedCommentVersion:  1,
			ExpectedFeedbackVersion: 1,
			Disposition:             "addressed",
			Reason:                  "original answer",
		},
	)
	require.NoError(t, err)
	_, err = s.DecidePlanRevision(
		ctx,
		p,
		id,
		1,
		types.PlanReviewRequest{Status: "in_review", ExpectedHead: 1, ExpectedFeedbackVersion: 2},
	)
	require.NoError(t, err)
	_, err = s.DecidePlanRevision(
		ctx,
		p,
		id,
		1,
		types.PlanReviewRequest{
			Status:                  "approved",
			ExpectedHead:            1,
			ExpectedReviewVersion:   1,
			ExpectedFeedbackVersion: 2,
		},
	)
	require.NoError(t, err)
	original, err := s.ListPlanReviewEvents(ctx, p, id, 1, 200, 0)
	require.NoError(t, err)
	require.Len(t, original, 2)
	require.Empty(t, original[0].Dispositions)
	approval := original[1]
	require.Equal(t, "approved", approval.Status)
	require.EqualValues(t, 2, approval.ReviewVersion)
	require.EqualValues(t, 2, approval.FeedbackVersion)
	require.Equal(t, "reviewer", approval.Actor)
	require.Equal(t, "session-history", approval.SessionID)
	require.False(t, approval.CreatedAt.IsZero())
	require.Equal(t, []*types.PlanFeedbackDisposition{disposition}, approval.Dispositions)
	require.NotEmpty(t, approval.ID)
	require.Equal(t, id, approval.PlanID)
	require.EqualValues(t, 1, approval.Revision)

	return s, p, comment, original
}

// checkHistoryOwnership distinguishes a real other parent from missing identifiers.
func checkHistoryOwnership(t *testing.T, s *sqlite.Store, p string, comment *types.PlanComment) {
	t.Helper()
	ctx := t.Context()
	id := comment.PlanID
	other, err := s.CreateDurablePlan(ctx, p, "other", storage.PlanUpload{Content: "other"})
	require.NoError(t, err)
	for _, scope := range []struct {
		project, plan string
		revision      int64
	}{
		{"another-project", id, 1}, {p, other.Plan.ID, 1}, {p, id, 2}, {p, id, 99}, {p, "missing", 1},
	} {
		t.Run(
			fmt.Sprintf("comment scope %s %s %d", scope.project, scope.plan, scope.revision),
			func(t *testing.T) {
				_, e := s.ListPlanCommentVersions(
					ctx,
					scope.project,
					scope.plan,
					scope.revision,
					comment.ID,
					50,
					0,
				)
				require.ErrorIs(t, e, storage.ErrPlanNotFound)
			},
		)
	}
	_, err = s.ListPlanCommentVersions(ctx, p, id, 1, "missing", 50, 0)
	require.ErrorIs(t, err, storage.ErrPlanNotFound)
	_, err = s.ListPlanReviewEvents(ctx, "another-project", id, 1, 50, 0)
	require.ErrorIs(t, err, storage.ErrPlanNotFound)
	_, err = s.ListPlanReviewEvents(ctx, p, id, 99, 50, 0)
	require.ErrorIs(t, err, storage.ErrPlanNotFound)
	for _, page := range []struct{ limit, offset int }{{0, 0}, {201, 0}, {50, -1}} {
		_, err = s.ListPlanReviewEvents(ctx, p, id, 1, page.limit, page.offset)
		require.ErrorIs(t, err, storage.ErrPlanInvalid)
		_, err = s.ListPlanCommentVersions(ctx, p, id, 1, comment.ID, page.limit, page.offset)
		require.ErrorIs(t, err, storage.ErrPlanInvalid)
	}
	for _, owned := range []struct {
		plan     string
		revision int64
	}{{other.Plan.ID, 1}, {id, 2}} {
		events, e := s.ListPlanReviewEvents(ctx, p, owned.plan, owned.revision, 50, 0)
		require.NoError(t, e)
		require.Empty(t, events)
	}
}
