package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sentiolabs/arc/internal/types"
)

func TestDeriveTitle_H1Heading(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "my-spec.md")
	content := "# My Spec Title\n\nSome body text here.\n"
	if err := os.WriteFile(f, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	got := deriveTitle(f)
	want := "My Spec Title"
	if got != want {
		t.Errorf("deriveTitle H1 case: got %q, want %q", got, want)
	}
}

func TestDeriveTitle_FilenameFallback(t *testing.T) {
	dir := t.TempDir()
	// File with a YYYY-MM-DD- date prefix, no H1 heading
	f := filepath.Join(dir, "2024-01-15-my-design-spec.md")
	content := "Some content without a heading.\n"
	if err := os.WriteFile(f, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	got := deriveTitle(f)
	want := "my-design-spec"
	if got != want {
		t.Errorf("deriveTitle filename fallback: got %q, want %q", got, want)
	}
}

func TestDeriveTitle_NonExistentPath(t *testing.T) {
	dir := t.TempDir()
	// Path that does not exist — should fall back to filename base (sans date prefix / .md)
	f := filepath.Join(dir, "2024-03-01-missing-plan.md")

	got := deriveTitle(f)
	want := "missing-plan"
	if got != want {
		t.Errorf("deriveTitle non-existent path: got %q, want %q", got, want)
	}
}

func TestDeriveTitle_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	// Empty file — no heading, should fall back to filename base
	f := filepath.Join(dir, "2024-05-10-empty-spec.md")
	if err := os.WriteFile(f, []byte(""), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	got := deriveTitle(f)
	want := "empty-spec"
	if got != want {
		t.Errorf("deriveTitle empty file: got %q, want %q", got, want)
	}
}

func TestDeriveTitle_H2OnlyHeading(t *testing.T) {
	dir := t.TempDir()
	// File whose only heading is ## (H2) — should NOT match, fall back to filename
	f := filepath.Join(dir, "2024-06-01-h2-only.md")
	content := "## Not An H1\n\nBody text.\n"
	if err := os.WriteFile(f, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	got := deriveTitle(f)
	want := "h2-only"
	if got != want {
		t.Errorf("deriveTitle H2-only heading: got %q, want %q", got, want)
	}
}

func TestTruncateQuoteShort(t *testing.T) {
	got := truncateQuote("short quote", 60)
	want := "short quote"
	if got != want {
		t.Errorf("truncateQuote short: got %q, want %q", got, want)
	}
}

func TestTruncateQuoteCollapsesWhitespace(t *testing.T) {
	got := truncateQuote("line one\n  line   two", 60)
	want := "line one line two"
	if got != want {
		t.Errorf("truncateQuote whitespace: got %q, want %q", got, want)
	}
}

func TestTruncateQuoteTruncatesWithEllipsis(t *testing.T) {
	got := truncateQuote("this is a very long quoted string that exceeds the max", 10)
	want := "this is a …"
	if got != want {
		t.Errorf("truncateQuote long: got %q, want %q", got, want)
	}
}

func TestPrintPlanCommentsEmpty(t *testing.T) {
	out := captureStdout(t, func() {
		printPlanComments(nil)
	})
	if out != "No comments\n" {
		t.Errorf("printPlanComments empty: got %q, want %q", out, "No comments\n")
	}
}

func TestPrintPlanCommentsLegacyLineShape(t *testing.T) {
	line := 12
	comments := []*types.PlanComment{
		{ID: "c1", LineNumber: &line, Content: "fix this"},
	}
	out := captureStdout(t, func() {
		printPlanComments(comments)
	})
	want := "[L12] fix this\n"
	if out != want {
		t.Errorf("printPlanComments legacy line: got %q, want %q", out, want)
	}
}

func TestPrintPlanCommentsOverallShape(t *testing.T) {
	comments := []*types.PlanComment{
		{ID: "c1", Content: "overall note"},
	}
	out := captureStdout(t, func() {
		printPlanComments(comments)
	})
	want := "[overall] overall note\n"
	if out != want {
		t.Errorf("printPlanComments overall: got %q, want %q", out, want)
	}
}

func TestPrintPlanCommentsAnchoredSingleLine(t *testing.T) {
	comments := []*types.PlanComment{
		{
			ID:      "c1",
			Content: "needs work",
			Anchor: &types.PlanCommentAnchor{
				LineStart:  12,
				LineEnd:    12,
				QuotedText: "some quoted text",
			},
		},
	}
	out := captureStdout(t, func() {
		printPlanComments(comments)
	})
	want := `[L12] "some quoted text" needs work` + "\n"
	if out != want {
		t.Errorf("printPlanComments anchored single line: got %q, want %q", out, want)
	}
}

func TestPrintPlanCommentsAnchoredRange(t *testing.T) {
	comments := []*types.PlanComment{
		{
			ID:      "c1",
			Content: "needs work",
			Anchor: &types.PlanCommentAnchor{
				LineStart:  12,
				LineEnd:    15,
				QuotedText: "quote",
			},
		},
	}
	out := captureStdout(t, func() {
		printPlanComments(comments)
	})
	want := `[L12-L15] "quote" needs work` + "\n"
	if out != want {
		t.Errorf("printPlanComments anchored range: got %q, want %q", out, want)
	}
}

func TestPrintPlanCommentsResolvedPrefix(t *testing.T) {
	now := time.Now()
	line := 3
	comments := []*types.PlanComment{
		{ID: "c1", LineNumber: &line, Content: "done", ResolvedAt: &now},
	}
	out := captureStdout(t, func() {
		printPlanComments(comments)
	})
	want := "✓ [L3] done\n"
	if out != want {
		t.Errorf("printPlanComments resolved: got %q, want %q", out, want)
	}
}

func TestPlanWaitCmdRegistered(t *testing.T) {
	found := false
	for _, cmd := range planCmd.Commands() {
		if cmd.Name() == "wait" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected planCmd to have a 'wait' subcommand")
	}
	if planWaitCmd.Flags().Lookup("timeout") == nil {
		t.Error("expected planWaitCmd to have a --timeout flag")
	}
}

func runPlanTest(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newPlanCommand()
	cmd.SetArgs(args)
	var err error
	out := captureStdout(t, func() { err = cmd.Execute() })
	return out, err
}

func TestDurablePlanCLIUploadExportAndReview(t *testing.T) {
	c, p := setupSessionTest(t)
	outputJSON = true
	path := filepath.Join(t.TempDir(), "draft.md")
	exact := "---\ncustom: keep\ntags: [mine]\n---\n# Remote\r\nλ\n"
	require.NoError(t, os.WriteFile(path, []byte(exact), 0o600))
	out, err := runPlanTest(t, "create", path, "--idempotency-key", "logical-upload")
	require.NoError(t, err)
	var created storage.PlanWriteResult
	require.NoError(t, json.Unmarshal([]byte(out), &created))
	require.Equal(t, exact, created.Revision.Content)
	local, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(local), "custom: keep")
	require.Contains(t, string(local), "mine")
	require.Contains(t, string(local), "revision: 1")
	require.Contains(t, string(local), p)
	require.NoError(t, os.Remove(path))
	out, err = runPlanTest(t, "show", created.Plan.ID, "--revision", "1")
	require.NoError(t, err)
	require.Contains(t, out, "Remote")
	destination := filepath.Join(t.TempDir(), "export.md")
	_, err = runPlanTest(t, "export", created.Plan.ID, "--revision", "1", "--output", destination)
	require.NoError(t, err)
	b, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, exact, string(b))
	_, err = runPlanTest(t, "export", created.Plan.ID, "--revision", "1", "--output", destination)
	require.Error(t, err)
	b, err = os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, exact, string(b))
	_, err = runPlanTest(
		t,
		"export",
		created.Plan.ID,
		"--revision",
		"1",
		"--output",
		destination,
		"--force",
		"--with-frontmatter",
	)
	require.NoError(t, err)
	b, err = os.ReadFile(destination)
	require.NoError(t, err)
	require.Contains(t, string(b), "durable-export")
	require.Contains(t, string(b), "custom: keep")
	_, err = runPlanTest(t, "approve", created.Plan.ID, "--revision", "1")
	require.ErrorContains(t, err, "expected-head")
	_, err = runPlanTest(
		t,
		"submit",
		created.Plan.ID,
		"--revision",
		"1",
		"--expected-head",
		"1",
		"--expected-review-version",
		"0",
		"--expected-feedback-version",
		"0",
	)
	require.NoError(t, err)
	context := filepath.Join(t.TempDir(), "review.json")
	_, err = runPlanTest(
		t,
		"show",
		created.Plan.ID,
		"--revision",
		"1",
		"--review-context-output",
		context,
	)
	require.NoError(t, err)
	_, err = runPlanTest(
		t,
		"approve",
		created.Plan.ID,
		"--revision",
		"1",
		"--review-context",
		context,
	)
	require.NoError(t, err)
	_, err = runPlanTest(
		t,
		"approve",
		created.Plan.ID,
		"--revision",
		"1",
		"--review-context",
		context,
	)
	require.Error(t, err)
	meta, err := c.GetPlan(p, created.Plan.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, meta.HeadRevision)
}

func TestDurablePlanCLIWaitHistoricalSupersededAndTimeout(t *testing.T) {
	c, p := setupSessionTest(t)
	outputJSON = true
	plan, err := c.CreatePlan(p, "wait", storage.PlanUpload{Content: "old"})
	require.NoError(t, err)
	_, err = runPlanTest(t, "wait", plan.Plan.ID, "--revision", "1", "--timeout", "1ms")
	require.ErrorContains(t, err, "timed out")
	_, err = c.SavePlanRevision(
		p,
		plan.Plan.ID,
		"save",
		storage.PlanSave{Content: "new", ExpectedRevision: 1},
	)
	require.NoError(t, err)
	out, err := runPlanTest(t, "wait", plan.Plan.ID, "--revision", "1")
	require.ErrorContains(t, err, "superseded")
	require.Contains(t, out, `"head_revision": 2`)
	_, err = c.DecidePlanRevision(
		p,
		plan.Plan.ID,
		2,
		types.PlanReviewRequest{Status: "in_review", ExpectedHead: 2},
	)
	require.NoError(t, err)
	_, err = c.DecidePlanRevision(
		p,
		plan.Plan.ID,
		2,
		types.PlanReviewRequest{Status: "approved", ExpectedHead: 2, ExpectedReviewVersion: 1},
	)
	require.NoError(t, err)
	_, err = c.SavePlanRevision(
		p,
		plan.Plan.ID,
		"third",
		storage.PlanSave{Content: "third", ExpectedRevision: 2},
	)
	require.NoError(t, err)
	out, err = runPlanTest(t, "wait", plan.Plan.ID, "--revision", "2")
	require.NoError(t, err)
	require.Contains(t, out, "approved")
}

func TestDurableExportAtomicNoClobber(t *testing.T) {
	const exportDirectory = "directory"
	for _, kind := range []string{"file", exportDirectory, pathTypeSymlink, "dangling"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			dst := filepath.Join(dir, "out")
			target := filepath.Join(dir, "target")
			switch kind {
			case "file":
				require.NoError(t, os.WriteFile(dst, []byte("old"), 0o600))
			case exportDirectory:
				require.NoError(t, os.Mkdir(dst, 0o700))
			case pathTypeSymlink:
				require.NoError(t, os.WriteFile(target, []byte("target"), 0o600))
				require.NoError(t, os.Symlink(target, dst))
			case "dangling":
				require.NoError(t, os.Symlink(target, dst))
			}
			require.Error(t, writePlanExport(dst, []byte("new"), false))
			if kind != exportDirectory {
				require.NoError(t, writePlanExport(dst, []byte("new"), true))
				b, err := os.ReadFile(dst)
				require.NoError(t, err)
				require.Equal(t, "new", string(b))
			}
			if kind == pathTypeSymlink {
				b, err := os.ReadFile(target)
				require.NoError(t, err)
				require.Equal(t, "target", string(b))
			}
		})
	}
}

func TestDurablePlanCommandsRegistered(t *testing.T) {
	cmd := newPlanCommand()
	for _, name := range []string{
		"create",
		"list",
		"show",
		"history",
		"update",
		"submit",
		"approve",
		"reject",
		"comments",
		"comment",
		"feedback",
		"dispositions",
		"archive",
		"restore",
		"export",
		"adopt",
		"resolve",
		"wait",
	} {
		child, _, err := cmd.Find([]string{name})
		require.NoError(t, err)
		require.NotEqual(t, cmd, child)
	}
}

//nolint:revive // One CLI lifecycle carries captured review and adoption state across operations.
func TestDurablePlanCLICommentsVersionsAndAdoption(t *testing.T) {
	c, p := setupSessionTest(t)
	outputJSON = true
	first, err := c.CreatePlan(
		p,
		"cli-commands",
		storage.PlanUpload{Title: "plan", Content: "# old"},
	)
	require.NoError(t, err)
	id := first.Plan.ID
	out, err := runPlanTest(t, "comment", "create", id, "--revision", "1", "--content", "fix this")
	require.NoError(t, err)
	var comment types.PlanComment
	require.NoError(t, json.Unmarshal([]byte(out), &comment))
	_, err = runPlanTest(
		t,
		"comment",
		"update",
		id,
		comment.ID,
		"--revision",
		"1",
		"--content",
		"clarify",
		"--expected-comment-version",
		"1",
	)
	require.NoError(t, err)
	_, err = runPlanTest(
		t,
		"comment",
		"update",
		id,
		comment.ID,
		"--revision",
		"1",
		"--content",
		"stale",
		"--expected-comment-version",
		"1",
	)
	require.Error(t, err)
	meta, err := c.GetPlan(p, id)
	require.NoError(t, err)
	_, err = runPlanTest(
		t,
		"feedback",
		id,
		"--revision",
		"1",
		"--comment",
		comment.ID,
		"--expected-comment-version",
		"2",
		"--expected-feedback-version",
		strconv.FormatInt(meta.FeedbackVersion, 10),
		"--disposition",
		"addressed",
		"--reason",
		"implemented",
	)
	require.NoError(t, err)
	for _, name := range []string{"comments", "dispositions"} {
		out, err = runPlanTest(t, name, id, "--revision", "1")
		require.NoError(t, err)
		require.Contains(t, out, comment.ID)
	}
	meta, err = c.GetPlan(p, id)
	require.NoError(t, err)
	_, err = runPlanTest(
		t,
		"submit",
		id,
		"--revision",
		"1",
		"--expected-head",
		"1",
		"--expected-review-version",
		"0",
		"--expected-feedback-version",
		strconv.FormatInt(meta.FeedbackVersion, 10),
	)
	require.NoError(t, err)
	_, err = runPlanTest(
		t,
		"approve",
		id,
		"--revision",
		"1",
		"--expected-head",
		"1",
		"--expected-review-version",
		"1",
		"--expected-feedback-version",
		strconv.FormatInt(meta.FeedbackVersion, 10),
	)
	require.NoError(t, err)
	epic, err := c.CreateIssue(p, client.CreateIssueRequest{Title: "epic", IssueType: "epic"})
	require.NoError(t, err)
	out, err = runPlanTest(t, "adopt", epic.ID, id, "--revision", "1", "--dry-run")
	require.NoError(t, err)
	var proposal storage.PlanAdoptionResult
	require.NoError(t, json.Unmarshal([]byte(out), &proposal))
	require.Empty(t, proposal.Errors)
	manifest := filepath.Join(t.TempDir(), "reconciliation.json")
	b, err := json.Marshal(proposal.Request)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(manifest, b, 0o600))
	_, err = runPlanTest(
		t,
		"adopt",
		epic.ID,
		id,
		"--revision",
		"1",
		"--reconciliation",
		manifest,
		"--idempotency-key",
		"adopt",
	)
	require.NoError(t, err)
	out, err = runPlanTest(t, "resolve", epic.ID)
	require.NoError(t, err)
	require.Contains(t, out, id)
	file := filepath.Join(t.TempDir(), "new.md")
	require.NoError(t, os.WriteFile(file, []byte("# new"), 0o600))
	_, err = runPlanTest(
		t,
		"update",
		id,
		file,
		"--expected-revision",
		"1",
		"--idempotency-key",
		"new",
	)
	require.NoError(t, err)
	_, err = runPlanTest(
		t,
		"update",
		id,
		file,
		"--expected-revision",
		"1",
		"--idempotency-key",
		"stale",
	)
	require.Error(t, err)
	out, err = runPlanTest(t, "history", id)
	require.NoError(t, err)
	require.Contains(t, out, `"revision": 2`)
	meta, err = c.GetPlan(p, id)
	require.NoError(t, err)
	_, err = runPlanTest(
		t,
		"archive",
		id,
		"--expected-version",
		strconv.FormatInt(meta.Version, 10),
	)
	require.NoError(t, err)
	out, err = runPlanTest(t, "list", "--archived")
	require.NoError(t, err)
	require.Contains(t, out, id)
	meta, err = c.GetPlan(p, id)
	require.NoError(t, err)
	_, err = runPlanTest(
		t,
		"restore",
		id,
		"--expected-version",
		strconv.FormatInt(meta.Version, 10),
	)
	require.NoError(t, err)
	_, err = runPlanTest(
		t,
		"comment",
		"delete",
		id,
		comment.ID,
		"--revision",
		"1",
		"--expected-comment-version",
		"2",
	)
	require.NoError(t, err)
}

func TestDurableWaitDecisionDuringSaveWins(t *testing.T) {
	calls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/comments") {
			_, _ = fmt.Fprint(w, `[]`)
			return
		}
		if strings.Contains(r.URL.Path, "/revisions/") {
			calls++
			status := "in_review"
			if calls > 1 {
				status = "approved"
			}
			_, _ = fmt.Fprintf(w, `{"plan_id":"plan","revision":1,"review_status":%q}`, status)
			return
		}
		_, _ = fmt.Fprint(w, `{"id":"plan","head_revision":2}`)
	}))
	defer ts.Close()
	result, err := pollPlanWait(client.New(ts.URL), "project", "plan", 1)
	require.NoError(t, err)
	require.Equal(t, "approved", result.Status)
}

func TestCreateWarnsAfterConfirmedUploadWithoutRetry(t *testing.T) {
	c, p := setupSessionTest(t)
	outputJSON = true
	path := filepath.Join(t.TempDir(), "bad-frontmatter.md")
	original := "---\nbroken: [\n---\n# Exact upload\n"
	require.NoError(t, os.WriteFile(path, []byte(original), 0o600))
	var warning bytes.Buffer
	command := newPlanCommand()
	command.SetErr(&warning)
	command.SetArgs([]string{"create", path, "--idempotency-key", "once"})
	var commandErr error
	out := captureStdout(t, func() { commandErr = command.Execute() })
	require.NoError(t, commandErr)
	var result storage.PlanWriteResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	require.Contains(t, warning.String(), result.Plan.ID)
	require.Contains(t, warning.String(), "revision 1")
	require.Contains(t, warning.String(), p)
	plans, err := c.ListPlans(p, false, 50, 0)
	require.NoError(t, err)
	require.Len(t, plans, 1)
	retained, err := c.ReadPlanRevision(p, result.Plan.ID, 1)
	require.NoError(t, err)
	require.Equal(t, original, retained.Content)
	unchanged, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, original, string(unchanged))
}

const testWaitPlanID = "wait-plan"

func TestWaitChangesRequestedUsesCurrentHeadOnly(t *testing.T) {
	for _, scenario := range []struct {
		name, initial, reread string
		head                  int64
		want                  string
	}{
		{"current", "changes_requested", "changes_requested", 1, "changes_requested"},
		{"older", "changes_requested", "changes_requested", 2, "superseded"},
		{"reread nonterminal", "in_review", "changes_requested", 2, "superseded"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			var reads atomic.Int32
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.Contains(r.URL.Path, "/comments"):
					_, _ = w.Write([]byte(`[]`))
				case strings.Contains(r.URL.Path, "/revisions/"):
					status := scenario.initial
					if reads.Add(1) > 1 {
						status = scenario.reread
					}
					_, _ = fmt.Fprintf(w, `{"plan_id":"wait-plan","revision":1,"review_status":%q}`, status)
				default:
					_, _ = fmt.Fprintf(w, `{"id":"wait-plan","head_revision":%d}`, scenario.head)
				}
			}))
			defer ts.Close()
			result, err := pollPlanWait(client.New(ts.URL), cmdProject, testWaitPlanID, 1)
			require.NoError(t, err)
			require.Equal(t, scenario.want, result.Status)
		})
	}
}

func TestWaitDeadlineAndCancellationReachEveryRead(t *testing.T) {
	for _, endpoint := range []string{cmdProject, "revision", "metadata", "comments"} {
		for _, mode := range []string{"timeout", "cancel"} {
			t.Run(endpoint+"/"+mode, func(t *testing.T) {
				runWaitCancellationCase(t, endpoint, mode)
			})
		}
	}
}

func TestCreateEmitsCanonicalRevisionURL(t *testing.T) {
	c, p := setupSessionTest(t)
	path := filepath.Join(t.TempDir(), "draft.md")
	require.NoError(t, os.WriteFile(path, []byte("# Plan"), 0o600))
	out, err := runPlanTest(t, "create", path, "--no-frontmatter")
	require.NoError(t, err)
	plans, err := c.ListPlans(p, false, 50, 0)
	require.NoError(t, err)
	require.Len(t, plans, 1)
	require.Contains(t, out, "Review at: "+c.BaseURL()+"/"+p+"/plans/"+plans[0].ID+"/1\n")
}

func TestWaitRejectsDecisionCancelledBeforeRendering(t *testing.T) {
	_, _ = setupSessionTest(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/comments") {
			cancel()
			_, _ = w.Write([]byte(`[]`))
			return
		}
		_, _ = w.Write([]byte(`{"plan_id":"wait-plan","revision":1,"review_status":"approved"}`))
	}))
	defer ts.Close()
	serverURL = ts.URL
	originalProject := projectID
	projectID = cmdProject
	t.Cleanup(func() { projectID = originalProject })
	cmd := newPlanWaitCommand()
	cmd.SetContext(ctx)
	require.NoError(t, cmd.Flags().Set("revision", "1"))
	var commandErr error
	out := captureStdout(t, func() { commandErr = cmd.RunE(cmd, []string{testWaitPlanID}) })
	require.ErrorIs(t, commandErr, context.Canceled)
	require.Empty(t, out)
}

func blockedWaitHandler(endpoint string, entered, release chan struct{}, writes *atomic.Int32) http.Handler {
	var once sync.Once
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writes.Add(1)
		}
		path := "metadata"
		switch {
		case strings.HasSuffix(r.URL.Path, "/projects/resolve"):
			path = cmdProject
		case strings.Contains(r.URL.Path, "/comments"):
			path = "comments"
		case strings.Contains(r.URL.Path, "/revisions/"):
			path = "revision"
		}
		if path == endpoint {
			once.Do(func() { close(entered) })
			select {
			case <-release:
			case <-r.Context().Done():
			}
			return
		}
		switch path {
		case "metadata":
			_, _ = w.Write([]byte(`{"id":"wait-plan","head_revision":1}`))
		case "comments":
			_, _ = w.Write([]byte(`[]`))
		default:
			status := "in_review"
			if endpoint == "comments" {
				status = "approved"
			}
			_, _ = fmt.Fprintf(w, `{"plan_id":"wait-plan","revision":1,"review_status":%q}`, status)
		}
	})
}

func runWaitCancellationCase(t *testing.T, endpoint, mode string) {
	t.Helper()
	_, _ = setupSessionTest(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	var writes atomic.Int32
	ts := httptest.NewServer(blockedWaitHandler(endpoint, entered, release, &writes))
	defer ts.Close()
	defer close(release)
	originalProject := projectID
	projectID = cmdProject
	if endpoint == cmdProject {
		projectID = ""
	}
	t.Cleanup(func() { projectID = originalProject })
	serverURL = ts.URL
	command := newPlanWaitCommand()
	require.NoError(t, command.Flags().Set("revision", "1"))
	require.NoError(t, command.Flags().Set("timeout", "100ms"))
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	command.SetContext(ctx)
	done := make(chan error, 1)
	go func() { done <- command.RunE(command, []string{testWaitPlanID}) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("request did not begin")
	}
	if mode == "cancel" {
		cancel()
	}
	select {
	case err := <-done:
		require.Error(t, err)
		if mode == "cancel" {
			require.ErrorIs(t, err, context.Canceled)
		} else {
			require.ErrorIs(t, err, context.DeadlineExceeded)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("wait did not stop its in-flight HTTP request")
	}
	require.Zero(t, writes.Load())
}

// These command tests exercise the real client decoding and default CLI output.
// Complete JSON equality checks semantic values at every nested pointer boundary.
func TestDurableDefaultCollectionAndCommentOutput(t *testing.T) {
	prior, current := defaultOutputComments()
	deleted := *current
	deletedAt := time.Date(2026, 9, 17, 12, 34, 56, 0, time.UTC)
	deleted.DeletedAt = &deletedAt
	cases := []struct {
		name   string
		args   []string
		result any
	}{
		{
			"plan collection",
			[]string{"list"},
			[]*types.Plan{
				{
					ID:           "visible-plan",
					Title:        "Retained design",
					HeadRevision: 7,
					Lifecycle:    "active",
					Version:      9,
				},
			},
		},
		{
			"history",
			[]string{"history", testWaitPlanID},
			[]*types.PlanRevision{
				{
					PlanID:        testWaitPlanID,
					Revision:      7,
					ReviewStatus:  "changes_requested",
					ReviewVersion: 3,
				},
			},
		},
		{
			"comments",
			[]string{"comments", testWaitPlanID, "--revision", "7"},
			[]*types.PlanComment{prior, current},
		},
		{
			"dispositions",
			[]string{"dispositions", testWaitPlanID, "--revision", "7"},
			[]*types.PlanFeedbackDisposition{
				{
					ID:             "addressed-record",
					PlanID:         testWaitPlanID,
					TargetRevision: 7,
					CommentID:      prior.ID,
					CommentVersion: prior.Version,
					Disposition:    "addressed",
					Reason:         "Recovery now preserves the original contract",
				},
				{
					ID:             "deferred-record",
					PlanID:         testWaitPlanID,
					TargetRevision: 7,
					CommentID:      current.ID,
					CommentVersion: current.Version,
					Disposition:    "deferred",
					Reason:         "Deferred with explicit follow-up",
				},
			},
		},
		{"empty plans", []string{"list"}, []*types.Plan{}},
		{"empty history", []string{"history", testWaitPlanID}, []*types.PlanRevision{}},
		{
			"empty comments",
			[]string{"comments", testWaitPlanID, "--revision", "7"},
			[]*types.PlanComment{},
		},
		{
			"empty dispositions",
			[]string{"dispositions", testWaitPlanID, "--revision", "7"},
			[]*types.PlanFeedbackDisposition{},
		},
		{
			"create comment",
			[]string{
				"comment",
				"create",
				testWaitPlanID,
				"--revision",
				"7",
				"--content",
				current.Content,
			},
			current,
		},
		{
			"update comment",
			[]string{
				"comment",
				"update",
				testWaitPlanID,
				current.ID,
				"--revision",
				"7",
				"--expected-comment-version",
				"4",
				"--content",
				current.Content,
			},
			current,
		},
		{
			"delete comment",
			[]string{
				"comment",
				planDeleteName,
				testWaitPlanID,
				current.ID,
				"--revision",
				"7",
				"--expected-comment-version",
				"5",
			},
			&deleted,
		},
		{
			"legacy unanchored comment",
			[]string{"comments", testWaitPlanID, "--revision", "7"},
			[]*types.PlanComment{
				{
					ID:      "legacy-comment",
					PlanID:  testWaitPlanID,
					Content: "Unknown original revision",
					Version: 1,
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response, err := json.Marshal(tc.result)
			require.NoError(t, err)
			setupDefaultOutputServer(
				t,
				func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(response) },
			)
			out, err := runPlanTest(t, tc.args...)
			require.NoError(t, err)
			require.JSONEq(t, string(response), out)
		})
	}
}

func defaultOutputComments() (prior, current *types.PlanComment) {
	priorRevision, currentRevision, line := int64(3), int64(7), 42
	prior = &types.PlanComment{
		ID: "prior-feedback", PlanID: testWaitPlanID, Revision: &priorRevision, Version: 2,
		Content: "Clarify recovery behavior", LineNumber: &line,
		Anchor: &types.PlanCommentAnchor{
			LineStart: 42, LineEnd: 44, QuotedText: "original recovery text", Occurrence: 2,
			HeadingSlug: "recovery", ContextBefore: "before quoted span", ContextAfter: "after quoted span",
		},
	}
	current = new(types.PlanComment)
	*current = *prior
	current.ID, current.Revision = "current-feedback", &currentRevision
	current.Version, current.Content = 5, "Current revision feedback"
	return prior, current
}

func setupDefaultOutputServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	_, p := setupSessionTest(t)
	originalProject := projectID
	projectID = p
	t.Cleanup(func() { projectID = originalProject })
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	serverURL = ts.URL
	require.False(t, outputJSON)
}

func TestDurableDefaultWaitFeedbackOutput(t *testing.T) {
	prior, current := defaultOutputComments()
	for _, comments := range [][]*types.PlanComment{{prior, current}, {}} {
		t.Run(fmt.Sprintf("comments=%d", len(comments)), func(t *testing.T) {
			setupDefaultOutputServer(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/comments"):
					_ = json.NewEncoder(w).Encode(comments)
				case strings.Contains(r.URL.Path, "/revisions/"):
					_ = json.NewEncoder(w).
						Encode(types.PlanRevisionWithContent{PlanRevision: types.PlanRevision{
							PlanID: testWaitPlanID, Revision: 7, ReviewStatus: "changes_requested",
						}})
				default:
					_ = json.NewEncoder(w).Encode(types.Plan{ID: testWaitPlanID, HeadRevision: 7})
				}
			})
			out, err := runPlanTest(t, "wait", testWaitPlanID, "--revision", "7")
			require.NoError(t, err)
			expected, err := json.Marshal(
				planWaitResult{
					Status:       "changes_requested",
					Revision:     7,
					HeadRevision: 7,
					Comments:     comments,
				},
			)
			require.NoError(t, err)
			require.JSONEq(t, string(expected), out)
		})
	}
}

func TestDurableDefaultAdoptionOutput(t *testing.T) {
	for _, linked := range []bool{true, false} {
		for _, dryRun := range []bool{true, false} {
			t.Run(fmt.Sprintf("linked=%t/dry=%t", linked, dryRun), func(t *testing.T) {
				runDefaultAdoptionOutputCase(t, linked, dryRun)
			})
		}
	}
}

func defaultOutputAdoption(linked bool) storage.PlanAdoptionResult {
	title, description := "Staged title value", "Staged description value"
	oldRoot := &types.PlanReference{PlanID: "old-root-plan", Revision: 2}
	newRoot := &types.PlanReference{PlanID: "new-root-plan", Revision: 8}
	oldChild := &types.PlanReference{PlanID: "old-child-plan", Revision: 4}
	newChild := &types.PlanReference{PlanID: "new-child-plan", Revision: 6}
	before := types.ExpectedGovernance{ContractVersion: 11}
	after := types.ExpectedGovernance{ContractVersion: 12}
	if linked {
		before.Governing = &types.GoverningPlan{
			ContainerID: "tactical-container", ContainerType: types.TypeEpic, Reference: *oldChild,
			Context: []types.GoverningPlanContext{
				{
					ContainerID:   "root-container",
					ContainerType: types.TypeMilestone,
					Reference:     *oldRoot,
				},
			},
		}
		after.Governing = &types.GoverningPlan{
			ContainerID: "tactical-container", ContainerType: types.TypeEpic, Reference: *newChild,
			Context: []types.GoverningPlanContext{
				{
					ContainerID:   "root-container",
					ContainerType: types.TypeMilestone,
					Reference:     *newRoot,
				},
			},
		}
	} else {
		oldRoot, newRoot, oldChild, newChild = nil, nil, nil, nil
	}
	pin := types.ReconciledContainerPin{
		ContainerID: "tactical-container", ExpectedContainerVersion: 13, ExpectedPin: oldChild, TargetPin: newChild,
		Disposition: "updated", Reason: "Tactical changes preserve higher-level requirements",
	}
	change := storage.ReconciliationChange{
		Before: storage.GovernanceSnapshot{
			IssueID:  "affected-task",
			Status:   types.StatusOpen,
			Expected: before,
		},
		After: storage.GovernanceSnapshot{
			IssueID:  "affected-task",
			Status:   types.StatusOpen,
			Expected: after,
		},
	}
	container := change
	container.Before.IssueID, container.After.IssueID = "tactical-container", "tactical-container"
	return storage.PlanAdoptionResult{
		ContainerID: "root-container", Before: change.Before, After: change.After,
		Request: types.PlanAdoptionRequest{
			ExpectedContainerVersion: 9, ExpectedGovernanceGeneration: 23, ExpectedPin: oldRoot, TargetPin: newRoot,
			Tasks: []types.ReconciledTask{
				{
					IssueID:      "affected-task",
					Expected:     before,
					Disposition:  "updated",
					Reason:       "Complete scope reconciliation",
					Title:        &title,
					Description:  &description,
					FollowUpKeys: []string{"follow-up"},
				},
			},
			ContainerPins: []types.ReconciledContainerPin{pin},
		},
		Containers: []storage.ReconciliationChange{
			container,
		}, Tasks: []storage.ReconciliationChange{change},
		ContainerPins: []types.ReconciledContainerPin{
			pin,
		}, FollowUpIDs: map[string]string{"follow-up": "created-task"},
		Errors: []storage.AdoptionCoverageError{
			{
				IssueID: "missing-task",
				Code:    "missing_coverage",
				Message: "A task still needs reconciliation",
			},
		},
	}
}

func runDefaultAdoptionOutputCase(t *testing.T, linked, dryRun bool) {
	t.Helper()
	result := defaultOutputAdoption(linked)
	result.DryRun, result.Request.DryRun = dryRun, dryRun
	setupDefaultOutputServer(t, func(w http.ResponseWriter, r *http.Request) {
		var actual types.PlanAdoptionRequest
		if err := json.NewDecoder(r.Body).Decode(&actual); err != nil {
			t.Error(err)
			return
		}
		assert.Equal(t, result.Request, actual)
		_ = json.NewEncoder(w).Encode(result)
	})
	manifest := filepath.Join(t.TempDir(), "proposal.json")
	require.NoError(t, writeContextJSON(manifest, result.Request))
	target := "-"
	if linked {
		target = result.Request.TargetPin.PlanID
	}
	args := []string{
		"adopt",
		"root-container",
		target,
		"--revision",
		"8",
		"--reconciliation",
		manifest,
	}
	if dryRun {
		args = append(args, "--dry-run")
	}
	out, err := runPlanTest(t, args...)
	require.NoError(t, err)
	expected, err := json.Marshal(result)
	require.NoError(t, err)
	require.JSONEq(t, string(expected), out)
}
