package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sentiolabs/arc/internal/api"
	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/storage/sqlite"
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

func TestPlanWaitTimesOutWhenNoDecision(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	server := api.New(api.ServerOptions{Address: ":0", Store: store})
	ts := httptest.NewServer(server.Echo())
	defer ts.Close()

	c := client.New(ts.URL)
	c.SetActor("test-user")

	filePath := filepath.Join(t.TempDir(), "plan.md")
	if err := os.WriteFile(filePath, []byte("# Plan\n"), 0o600); err != nil {
		t.Fatalf("write plan file: %v", err)
	}
	plan, err := c.CreatePlan(filePath)
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}

	origServerURL := serverURL
	origTimeout := planWaitTimeout
	serverURL = ts.URL
	planWaitTimeout = 100 * time.Millisecond
	defer func() {
		serverURL = origServerURL
		planWaitTimeout = origTimeout
	}()

	err = planWaitCmd.RunE(planWaitCmd, []string{plan.ID})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if want := "timed out"; !strings.Contains(err.Error(), want) {
		t.Errorf("expected error to contain %q, got %q", want, err.Error())
	}
}

func TestPlanWaitReturnsDecisionAndComments(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	server := api.New(api.ServerOptions{Address: ":0", Store: store})
	ts := httptest.NewServer(server.Echo())
	defer ts.Close()

	c := client.New(ts.URL)
	c.SetActor("test-user")

	filePath := filepath.Join(t.TempDir(), "plan.md")
	if err := os.WriteFile(filePath, []byte("# Plan\n"), 0o600); err != nil {
		t.Fatalf("write plan file: %v", err)
	}
	plan, err := c.CreatePlan(filePath)
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if _, err := c.CreatePlanComment(plan.ID, nil, "looks good overall"); err != nil {
		t.Fatalf("create plan comment: %v", err)
	}
	if err := c.UpdatePlanStatus(plan.ID, "approved"); err != nil {
		t.Fatalf("update plan status: %v", err)
	}

	origServerURL := serverURL
	origTimeout := planWaitTimeout
	origOutputJSON := outputJSON
	serverURL = ts.URL
	planWaitTimeout = time.Minute
	outputJSON = false
	defer func() {
		serverURL = origServerURL
		planWaitTimeout = origTimeout
		outputJSON = origOutputJSON
	}()

	out := captureStdout(t, func() {
		if err := planWaitCmd.RunE(planWaitCmd, []string{plan.ID}); err != nil {
			t.Fatalf("planWaitCmd.RunE: %v", err)
		}
	})

	want := "Decision: approved\n[overall] looks good overall\n"
	if out != want {
		t.Errorf("planWaitCmd output: got %q, want %q", out, want)
	}
}
