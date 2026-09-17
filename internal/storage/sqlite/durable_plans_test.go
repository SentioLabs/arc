package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"

	"github.com/sentiolabs/arc/internal/planfiles"
	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/storage/sqlite"
	"github.com/sentiolabs/arc/internal/types"
)

func durableStore(t *testing.T) (*sqlite.Store, string) {
	t.Helper()
	s, cleanup := setupTestStore(t)
	t.Cleanup(cleanup)
	p, err := planfiles.New(filepath.Join(t.TempDir(), "server"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	s.SetPlanPublisher(p)
	return s, setupTestProject(t, s).ID
}

func TestDurableSaveReplayAndProjectRetention(t *testing.T) {
	s, p := durableStore(t)
	ctx := context.Background()
	req := storage.PlanUpload{
		Title:      "Design",
		Content:    "# exact\r\n",
		SourceName: "/unavailable/client.md",
	}
	first, err := s.CreateDurablePlan(ctx, p, "create", req)
	if err != nil {
		t.Fatal(err)
	}
	if first.Replay || first.Revision.Revision != 1 || first.Revision.Content != req.Content {
		t.Fatalf("bad create: %+v", first)
	}
	var wg sync.WaitGroup
	results := make(chan *storage.PlanWriteResult, 2)
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := s.SavePlanRevision(
				ctx,
				p,
				first.Plan.ID,
				"save",
				storage.PlanSave{Content: "next", ExpectedRevision: 1},
			)
			results <- r
			errs <- e
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		require.NoError(t, e)
	}
	replayCount := 0
	for r := range results {
		if r.Revision.Revision != 2 {
			t.Fatal(r)
		}
		if r.Replay {
			replayCount++
		}
	}
	if replayCount != 1 {
		t.Fatal("identical saves did not replay")
	}
	meta, err := s.GetDurablePlan(ctx, p, first.Plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.UpdateDurablePlan(
		ctx,
		p,
		meta.ID,
		storage.PlanMetadataUpdate{ExpectedVersion: meta.Version, Lifecycle: "archived"},
	)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := s.CreateDurablePlan(ctx, p, "create", req)
	if err != nil || !replay.Replay || replay.Plan.HeadRevision != 1 {
		t.Fatalf("historical replay: %+v %v", replay, err)
	}
	_, err = s.CreateDurablePlan(ctx, p, "create", storage.PlanUpload{Content: "different"})
	if !errors.Is(err, storage.ErrPlanConflict) {
		t.Fatalf("payload reuse: %v", err)
	}
	if err = s.DeleteProject(ctx, p); err == nil {
		t.Fatal("deleted retained plan")
	}
	if _, err = s.GetDurablePlan(ctx, "other", meta.ID); !errors.Is(err, storage.ErrPlanNotFound) {
		t.Fatalf("ownership: %v", err)
	}
}

func TestDurableFeedbackReviewLifecycle(t *testing.T) {
	s, p := durableStore(t)
	ctx := context.Background()
	first, err := s.CreateDurablePlan(ctx, p, "create", storage.PlanUpload{Content: "design"})
	if err != nil {
		t.Fatal(err)
	}
	id := first.Plan.ID
	comment, err := s.CreateRevisionComment(
		ctx,
		p,
		id,
		1,
		storage.PlanCommentCreate{Content: "fix this"},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.DecidePlanRevision(
		ctx,
		p,
		id,
		1,
		types.PlanReviewRequest{
			Status:                  "in_review",
			ExpectedHead:            1,
			ExpectedReviewVersion:   0,
			ExpectedFeedbackVersion: 1,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	review := types.PlanReviewRequest{
		Status:                  "approved",
		ExpectedHead:            1,
		ExpectedReviewVersion:   1,
		ExpectedFeedbackVersion: 1,
	}
	if _, err = s.DecidePlanRevision(ctx, p, id, 1, review); !errors.Is(
		err,
		storage.ErrUnresolvedFeedback,
	) {
		t.Fatalf("approved outstanding feedback: %v", err)
	}
	_, err = s.AddPlanDisposition(
		ctx,
		p,
		id,
		1,
		storage.PlanDispositionRequest{
			CommentID:               comment.ID,
			ExpectedCommentVersion:  1,
			ExpectedFeedbackVersion: 1,
			Disposition:             "addressed",
			Reason:                  "implemented",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecidePlanRevision(ctx, p, id, 1, review); !errors.Is(
		err,
		storage.ErrPlanConflict,
	) {
		t.Fatalf("stale feedback: %v", err)
	}
	review.ExpectedFeedbackVersion = 2
	if _, err = s.DecidePlanRevision(ctx, p, id, 1, review); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SavePlanRevision(
		ctx,
		p,
		id,
		"save",
		storage.PlanSave{Content: "next", ExpectedRevision: 1},
	); err != nil {
		t.Fatal(err)
	}
	old, err := s.ReadPlanRevision(ctx, p, id, 1)
	if err != nil || old.ReviewStatus != "approved" || old.Content != "design" {
		t.Fatalf("lost history: %+v %v", old, err)
	}
	_, err = s.DecidePlanRevision(
		ctx,
		p,
		id,
		2,
		types.PlanReviewRequest{
			Status:                  "in_review",
			ExpectedHead:            2,
			ExpectedReviewVersion:   0,
			ExpectedFeedbackVersion: 2,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.DecidePlanRevision(
		ctx,
		p,
		id,
		2,
		types.PlanReviewRequest{
			Status:                  "approved",
			ExpectedHead:            2,
			ExpectedReviewVersion:   1,
			ExpectedFeedbackVersion: 2,
		},
	)
	if err != nil {
		t.Fatalf("addressed failed to carry forward: %v", err)
	}
}

// A second Store catches transaction upgrade races hidden by a one-connection pool.
func TestDurableIndependentConnections(t *testing.T) {
	s, p := durableStore(t)
	other, err := sqlite.New(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = other.Close() })
	publisher, err := planfiles.New(filepath.Join(t.TempDir(), "second-root"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = publisher.Close() })
	other.SetPlanPublisher(publisher)
	s.SetPlanPublisher(publisher)
	first, err := s.CreateDurablePlan(t.Context(), p, "create", storage.PlanUpload{Content: "base"})
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errs := make(chan error, 2)
	for i, store := range []*sqlite.Store{s, other} {
		go func() {
			<-start
			_, err := store.SavePlanRevision(
				t.Context(),
				p,
				first.Plan.ID,
				fmt.Sprintf("save-%d", i),
				storage.PlanSave{Content: "next", ExpectedRevision: 1},
			)
			errs <- err
		}()
	}
	close(start)
	success, conflicts := 0, 0
	for range 2 {
		e := <-errs
		switch {
		case e == nil:
			success++
		case errors.Is(e, storage.ErrPlanConflict):
			conflicts++
		default:
			t.Fatalf("race error: %v", e)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("success=%d conflicts=%d", success, conflicts)
	}
}

func TestDurableFeedbackTombstonesDeferralAndProvenance(t *testing.T) {
	s, p := durableStore(t)
	ctx := storage.WithPlanProvenance(t.Context(), "reviewer supplied by client", "session-1")
	first, err := s.CreateDurablePlan(ctx, p, "create", storage.PlanUpload{Content: "base"})
	if err != nil {
		t.Fatal(err)
	}
	id := first.Plan.ID
	c, err := s.CreateRevisionComment(
		ctx,
		p,
		id,
		1,
		storage.PlanCommentCreate{
			Content: "feedback",
			Anchor:  &types.PlanCommentAnchor{LineStart: 1, LineEnd: 1, QuotedText: "base"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.AddPlanDisposition(
		ctx,
		p,
		id,
		1,
		storage.PlanDispositionRequest{
			CommentID:               c.ID,
			ExpectedCommentVersion:  1,
			ExpectedFeedbackVersion: 1,
			Disposition:             "deferred",
			Reason:                  "later",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.SavePlanRevision(
		ctx,
		p,
		id,
		"save",
		storage.PlanSave{Content: "next", ExpectedRevision: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.DecidePlanRevision(
		ctx,
		p,
		id,
		2,
		types.PlanReviewRequest{Status: "in_review", ExpectedHead: 2, ExpectedFeedbackVersion: 2},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.DecidePlanRevision(
		ctx,
		p,
		id,
		2,
		types.PlanReviewRequest{
			Status:                  "approved",
			ExpectedHead:            2,
			ExpectedReviewVersion:   1,
			ExpectedFeedbackVersion: 2,
		},
	)
	if !errors.Is(err, storage.ErrUnresolvedFeedback) {
		t.Fatalf("deferral carried: %v", err)
	}
	_, err = s.AddPlanDisposition(
		ctx,
		p,
		id,
		2,
		storage.PlanDispositionRequest{
			CommentID:               c.ID,
			ExpectedCommentVersion:  1,
			ExpectedFeedbackVersion: 2,
			Disposition:             "addressed",
			Reason:                  "fixed",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := s.UpdateRevisionComment(
		ctx,
		p,
		id,
		1,
		c.ID,
		storage.PlanCommentUpdate{ExpectedVersion: 1, Delete: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if changed.DeletedAt == nil || changed.Version != 2 || changed.Anchor.QuotedText != "base" {
		t.Fatalf("tombstone: %+v", changed)
	}
	_, err = s.DecidePlanRevision(
		ctx,
		p,
		id,
		2,
		types.PlanReviewRequest{
			Status:                  "approved",
			ExpectedHead:            2,
			ExpectedReviewVersion:   1,
			ExpectedFeedbackVersion: 4,
		},
	)
	if !errors.Is(err, storage.ErrUnresolvedFeedback) {
		t.Fatalf("tombstone erased obligation: %v", err)
	}
	var actor, session string
	if err = s.DB().QueryRow(
		"SELECT actor,session_id FROM plan_review_events WHERE plan_id=?",
		id,
	).Scan(&actor, &session); err != nil {
		t.Fatal(err)
	}
	if actor != "reviewer supplied by client" || session != "session-1" {
		t.Fatalf("provenance %s %s", actor, session)
	}
	var count int
	if err = s.DB().QueryRow(
		"SELECT COUNT(*) FROM plan_comment_events WHERE comment_id=?",
		c.ID,
	).Scan(&count); err != nil ||
		count != 2 {
		t.Fatalf("events %d %v", count, err)
	}
}

type failedPlanPublisher struct{}

func (failedPlanPublisher) Publish(
	context.Context,
	string,
	string,
	int64,
	[]byte,
) (planfiles.Blob, error) {
	return planfiles.Blob{}, errors.New("injected publication failure")
}

func (failedPlanPublisher) Read(context.Context, planfiles.Blob) ([]byte, error) {
	return nil, errors.New("unexpected read")
}

func TestDurableFailedPublicationAndTampering(t *testing.T) {
	s, p := durableStore(t)
	s.SetPlanPublisher(failedPlanPublisher{})
	if _, err := s.CreateDurablePlan(t.Context(), p, "retry", storage.PlanUpload{Content: "bytes"}); err == nil {
		t.Fatal("publication succeeded")
	}
	for _, table := range []string{"plans", "plan_revisions", "plan_idempotency"} {
		var count int
		if err := s.DB().QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil ||
			count != 0 {
			t.Fatalf("%s references failed publication: %d %v", table, count, err)
		}
	}
	root := t.TempDir()
	publisher, err := planfiles.New(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = publisher.Close() })
	s.SetPlanPublisher(publisher)
	first, err := s.CreateDurablePlan(t.Context(), p, "retry", storage.PlanUpload{Content: "bytes"})
	if err != nil {
		t.Fatalf("failed request reserved key: %v", err)
	}
	_, err = s.DecidePlanRevision(
		t.Context(),
		p,
		first.Plan.ID,
		1,
		types.PlanReviewRequest{Status: "in_review", ExpectedHead: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	var relative string
	if err = s.DB().QueryRow(
		"SELECT content_path FROM plan_revisions WHERE plan_id=?",
		first.Plan.ID,
	).Scan(&relative); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, relative), []byte("edited"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []func() error{func() error {
		_, e := s.ReadPlanRevision(t.Context(), p, first.Plan.ID, 1)
		return e
	}, func() error {
		_, e := s.DecidePlanRevision(
			t.Context(),
			p,
			first.Plan.ID,
			1,
			types.PlanReviewRequest{Status: "approved", ExpectedHead: 1, ExpectedReviewVersion: 1},
		)
		return e
	}} {
		var integrity *planfiles.IntegrityError
		if err := operation(); !errors.As(err, &integrity) {
			t.Fatalf("tampering error: %v", err)
		}
	}
}

func TestDurableLegacyPreservation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.UpTo(conn, "migrations", 19); err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(
		"INSERT INTO plans(id,file_path,status) VALUES('plan.legacy','/unreadable/unrelated/client.md','approved')",
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(
		"INSERT INTO plan_comments(id,plan_id,content,line_start,line_end,quoted_text,occurrence)" +
			" VALUES('pc.legacy','plan.legacy','old feedback',1,1,'original',0)",
	)
	if err != nil {
		t.Fatal(err)
	}
	if err = conn.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := sqlite.New(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	legacy, err := store.GetPlan(t.Context(), "plan.legacy")
	if err != nil || legacy.Status != "approved" ||
		legacy.FilePath != "/unreadable/unrelated/client.md" {
		t.Fatalf("legacy metadata: %+v %v", legacy, err)
	}
	comments, err := store.ListPlanComments(t.Context(), legacy.ID)
	if err != nil || len(comments) != 1 || comments[0].Revision != nil ||
		comments[0].Anchor.QuotedText != "original" {
		t.Fatalf("unknown provenance lost: %+v %v", comments, err)
	}
	var count int
	if err = store.DB().QueryRow("SELECT COUNT(*) FROM plans").Scan(&count); err != nil ||
		count != 0 {
		t.Fatalf("invented durable approval: %d %v", count, err)
	}
}

func TestDurableUnknownRevisionFeedbackBlocksApproval(t *testing.T) {
	s, p := durableStore(t)
	first, err := s.CreateDurablePlan(
		t.Context(),
		p,
		"create",
		storage.PlanUpload{Content: "imported"},
	)
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.CreateRevisionComment(
		t.Context(),
		p,
		first.Plan.ID,
		1,
		storage.PlanCommentCreate{Content: "legacy question"},
	)
	if err != nil {
		t.Fatal(err)
	}
	// T3 imports unknown provenance; its discussion must remain an obligation.
	if _, err = s.DB().Exec("UPDATE plan_comments SET revision=NULL WHERE id=?", c.ID); err != nil {
		t.Fatal(err)
	}
	_, err = s.DecidePlanRevision(
		t.Context(),
		p,
		first.Plan.ID,
		1,
		types.PlanReviewRequest{Status: "in_review", ExpectedHead: 1, ExpectedFeedbackVersion: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.DecidePlanRevision(
		t.Context(),
		p,
		first.Plan.ID,
		1,
		types.PlanReviewRequest{
			Status:                  "approved",
			ExpectedHead:            1,
			ExpectedReviewVersion:   1,
			ExpectedFeedbackVersion: 1,
		},
	)
	if !errors.Is(err, storage.ErrUnresolvedFeedback) {
		t.Fatalf("unknown feedback ignored: %v", err)
	}
	_, err = s.AddPlanDisposition(
		t.Context(),
		p,
		first.Plan.ID,
		1,
		storage.PlanDispositionRequest{
			CommentID:               c.ID,
			ExpectedCommentVersion:  1,
			ExpectedFeedbackVersion: 1,
			Disposition:             "addressed",
			Reason:                  "reviewed imported discussion",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDurableConcurrentDecisionMutations(t *testing.T) {
	for _, mutation := range []string{"comment", "save"} {
		t.Run(mutation, func(t *testing.T) {
			checkDecisionMutationRace(t, mutation)
		})
	}
}

func TestDurableTransactionRollbackAndProjectRace(t *testing.T) {
	s, p := durableStore(t)
	if _, err := s.DB().Exec("CREATE TRIGGER fail_revision BEFORE INSERT ON plan_revisions BEGIN SELECT" +
		" RAISE(ABORT,'injected DB failure'); END"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateDurablePlan(t.Context(), p, "retry", storage.PlanUpload{Content: "rollback"}); err == nil {
		t.Fatal("injected DB failure accepted")
	}
	for _, table := range []string{"plans", "plan_revisions", "plan_idempotency"} {
		var count int
		if err := s.DB().QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil ||
			count != 0 {
			t.Fatalf("rollback left %s rows: %d %v", table, count, err)
		}
	}
	if _, err := s.DB().Exec("DROP TRIGGER fail_revision"); err != nil {
		t.Fatal(err)
	}
	first, err := s.CreateDurablePlan(
		t.Context(),
		p,
		"retry",
		storage.PlanUpload{Content: "rollback"},
	)
	if err != nil {
		t.Fatal(err)
	}
	target := &types.Project{Name: "merge-target", Prefix: "target"}
	if err = s.CreateProject(t.Context(), target); err != nil {
		t.Fatal(err)
	}
	if _, err = s.MergeProjects(t.Context(), target.ID, []string{p}, "test"); err == nil ||
		!strings.Contains(err.Error(), first.Plan.ID) {
		t.Fatalf("ownership merge: %v", err)
	}
	empty := &types.Project{Name: "merge-empty", Prefix: "empty"}
	if err = s.CreateProject(t.Context(), empty); err != nil {
		t.Fatal(err)
	}
	if _, err = s.MergeProjects(t.Context(), p, []string{empty.ID}, "test"); err != nil {
		t.Fatalf("plan-free source cannot merge into plan owner: %v", err)
	}
}

type gatedPlanPublisher struct {
	planfiles.Publisher
	entered  chan struct{}
	release  chan struct{}
	revision int64
}

func (p gatedPlanPublisher) Publish(
	ctx context.Context,
	projectID, id string,
	revision int64,
	content []byte,
) (planfiles.Blob, error) {
	if revision == p.revision {
		p.entered <- struct{}{}
		<-p.release
	}
	return p.Publisher.Publish(ctx, projectID, id, revision, content)
}

func TestDurableIndependentIdenticalSavesReplay(t *testing.T) {
	s, p := durableStore(t)
	root, err := planfiles.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	s.SetPlanPublisher(root)
	first, err := s.CreateDurablePlan(t.Context(), p, "create", storage.PlanUpload{Content: "base"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := sqlite.New(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = other.Close() })
	gate := gatedPlanPublisher{
		Publisher: root,
		entered:   make(chan struct{}, 2),
		release:   make(chan struct{}),
		revision:  2,
	}
	s.SetPlanPublisher(gate)
	other.SetPlanPublisher(gate)
	type outcome struct {
		result *storage.PlanWriteResult
		err    error
	}
	out := make(chan outcome, 2)
	for _, store := range []*sqlite.Store{s, other} {
		go func() {
			r, e := store.SavePlanRevision(
				t.Context(),
				p,
				first.Plan.ID,
				"same",
				storage.PlanSave{Content: "next", ExpectedRevision: 1},
			)
			out <- outcome{r, e}
		}()
	}
	<-gate.entered
	<-gate.entered
	close(gate.release)
	one, two := <-out, <-out
	if one.err != nil || two.err != nil {
		t.Fatalf("identical race: %v %v", one.err, two.err)
	}
	if one.result.Replay == two.result.Replay || one.result.Revision.Revision != 2 ||
		two.result.Revision.Revision != 2 {
		t.Fatalf("identical results: %+v %+v", one.result, two.result)
	}
}

func TestDurableProjectMutationDuringPublication(t *testing.T) {
	for _, operation := range []string{"delete", "merge"} {
		t.Run(operation, func(t *testing.T) {
			checkProjectPublicationRace(t, operation)
		})
	}
}

func checkDecisionMutationRace(t *testing.T, mutation string) {
	t.Helper()

	s, p := durableStore(t)
	ctx := t.Context()
	first, err := s.CreateDurablePlan(ctx, p, "create", storage.PlanUpload{Content: "base"})
	if err != nil {
		t.Fatal(err)
	}
	id := first.Plan.ID
	if _, err = s.DecidePlanRevision(
		ctx,
		p,
		id,
		1,
		types.PlanReviewRequest{Status: "in_review", ExpectedHead: 1},
	); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	decision := make(chan error, 1)
	changed := make(chan error, 1)
	go func() {
		<-start
		_, e := s.DecidePlanRevision(
			ctx,
			p,
			id,
			1,
			types.PlanReviewRequest{
				Status:                "approved",
				ExpectedHead:          1,
				ExpectedReviewVersion: 1,
			},
		)
		decision <- e
	}()
	go func() {
		<-start
		var e error
		if mutation == "comment" {
			_, e = s.CreateRevisionComment(
				ctx,
				p,
				id,
				1,
				storage.PlanCommentCreate{Content: "racing feedback"},
			)
		} else {
			_, e = s.SavePlanRevision(ctx, p, id, "save", storage.PlanSave{Content: "new", ExpectedRevision: 1})
		}
		changed <- e
	}()
	close(start)
	require.NoError(t, <-changed)
	err = <-decision
	if err != nil && !errors.Is(err, storage.ErrPlanConflict) {
		t.Fatal(err)
	}
	old, e := s.ReadPlanRevision(ctx, p, id, 1)
	if e != nil {
		t.Fatal(e)
	}
	if (err == nil) != (old.ReviewStatus == "approved") {
		t.Fatalf("decision evidence mismatch: %v %+v", err, old)
	}
	if mutation == "save" {
		newer, e := s.ReadPlanRevision(ctx, p, id, 2)
		if e != nil || newer.ReviewStatus != "draft" {
			t.Fatalf("save inherited review: %+v %v", newer, e)
		}
	}
}

func checkProjectPublicationRace(t *testing.T, operation string) {
	t.Helper()

	s, p := durableStore(t)
	root, err := planfiles.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	other, err := sqlite.New(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = other.Close() })
	target := &types.Project{Name: "target", Prefix: "target"}
	if err = other.CreateProject(t.Context(), target); err != nil {
		t.Fatal(err)
	}
	gate := gatedPlanPublisher{
		Publisher: root,
		entered:   make(chan struct{}, 1),
		release:   make(chan struct{}),
		revision:  1,
	}
	s.SetPlanPublisher(gate)
	done := make(chan error, 1)
	go func() {
		_, e := s.CreateDurablePlan(
			t.Context(),
			p,
			"create",
			storage.PlanUpload{Content: "racing"},
		)
		done <- e
	}()
	<-gate.entered
	if operation == "delete" {
		err = other.DeleteProject(t.Context(), p)
	} else {
		_, err = other.MergeProjects(t.Context(), target.ID, []string{p}, "test")
	}
	close(gate.release)
	if err != nil {
		t.Fatal(err)
	}
	if err = <-done; !errors.Is(err, storage.ErrPlanNotFound) {
		t.Fatalf("published reference after source removal: %v", err)
	}
	var count int
	if err = other.DB().QueryRow("SELECT COUNT(*) FROM plans").Scan(&count); err != nil ||
		count != 0 {
		t.Fatalf("orphan DB references: %d %v", count, err)
	}
}

func TestDurableReviewTransitionsArchiveAndRestore(t *testing.T) {
	s, p := durableStore(t)
	ctx := t.Context()
	first, err := s.CreateDurablePlan(ctx, p, "create", storage.PlanUpload{Content: "base"})
	require.NoError(t, err)
	id := first.Plan.ID
	for _, step := range []struct {
		status  string
		version int64
		success bool
	}{
		{"approved", 0, false},
		{"in_review", 0, true},
		{"changes_requested", 1, true},
		{"in_review", 2, true},
		{"rejected", 3, true},
		{"in_review", 4, false},
		{"approved", 4, false},
	} {
		_, err = s.DecidePlanRevision(
			ctx,
			p,
			id,
			1,
			types.PlanReviewRequest{Status: step.status, ExpectedHead: 1, ExpectedReviewVersion: step.version},
		)
		if step.success {
			require.NoError(t, err)
		} else {
			require.ErrorIs(t, err, storage.ErrPlanConflict)
		}
	}
	meta, err := s.GetDurablePlan(ctx, p, id)
	require.NoError(t, err)
	archived, err := s.UpdateDurablePlan(
		ctx,
		p,
		id,
		storage.PlanMetadataUpdate{ExpectedVersion: meta.Version, Lifecycle: "archived"},
	)
	require.NoError(t, err)
	_, err = s.SavePlanRevision(ctx, p, id, "save", storage.PlanSave{Content: "next", ExpectedRevision: 1})
	require.ErrorIs(t, err, storage.ErrPlanConflict)
	_, err = s.CreateRevisionComment(ctx, p, id, 1, storage.PlanCommentCreate{Content: "feedback"})
	require.ErrorIs(t, err, storage.ErrPlanConflict)
	_, err = s.UpdateDurablePlan(
		ctx,
		p,
		id,
		storage.PlanMetadataUpdate{ExpectedVersion: meta.Version, Lifecycle: "active"},
	)
	require.ErrorIs(t, err, storage.ErrPlanConflict)
	_, err = s.UpdateDurablePlan(
		ctx,
		p,
		id,
		storage.PlanMetadataUpdate{ExpectedVersion: archived.Version, Lifecycle: "active"},
	)
	require.NoError(t, err)
	old, err := s.ReadPlanRevision(ctx, p, id, 1)
	require.NoError(t, err)
	require.Equal(t, "rejected", old.ReviewStatus)
	saved, err := s.SavePlanRevision(ctx, p, id, "save", storage.PlanSave{Content: "next", ExpectedRevision: 1})
	require.NoError(t, err)
	require.Equal(t, "draft", saved.Revision.ReviewStatus)
	var count int
	require.NoError(t, s.DB().QueryRow("SELECT COUNT(*) FROM plan_review_events WHERE plan_id=?", id).Scan(&count))
	require.Equal(t, 4, count)
}
