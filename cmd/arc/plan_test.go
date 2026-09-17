package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/storage"
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
