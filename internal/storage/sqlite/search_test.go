package sqlite_test

import (
	"context"
	"testing"

	"github.com/sentiolabs/arc/internal/storage/sqlite"
	"github.com/sentiolabs/arc/internal/types"
)

func TestPrepareSearchQuery(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty string", "", ""},
		{"single term", "auth", "auth*"},
		{"multiple terms", "fix bug", "fix* bug*"},
		{"quoted phrase", `"fix bug"`, `"fix bug"`},
		{"mixed terms and quotes", `auth "fix bug" deploy`, `auth* "fix bug" deploy*`},
		{"extra whitespace", "  auth   deploy  ", "auth* deploy*"},
		{"unclosed quote", `"fix bug`, `"fix bug`},
		{"single character", "a", "a*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sqlite.PrepareSearchQuery(tt.input)
			if got != tt.want {
				t.Errorf("PrepareSearchQuery(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFTSSearchBasic(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestProject(t, store)

	// Create issues with distinctive content
	issues := []struct {
		title       string
		description string
	}{
		{"Fix authentication bug", "The login system fails when using OAuth tokens"},
		{"Add deployment pipeline", "Set up CI/CD for automated deployments"},
		{"Database migration tool", "Build a tool to manage database schema migrations"},
		{"Update user documentation", "Refresh the authentication section of the docs"},
	}

	for _, iss := range issues {
		issue := &types.Issue{
			ProjectID: ws.ID,
			Title:       iss.title,
			Description: iss.description,
			Status:      types.StatusOpen,
			Priority:    2,
			IssueType:   types.TypeTask,
		}
		if err := store.CreateIssue(ctx, issue, "test-actor"); err != nil {
			t.Fatalf("failed to create issue %q: %v", iss.title, err)
		}
	}

	// Search for "auth" — should match issues with authentication
	results, err := store.ListIssues(ctx, types.IssueFilter{
		ProjectID: ws.ID,
		Query:       "auth",
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) < 2 {
		t.Errorf("expected at least 2 results for 'auth', got %d", len(results))
	}

	// The title match ("Fix authentication bug") should rank higher than the
	// description-only match ("Update user documentation" with auth in description)
	if len(results) >= 2 {
		foundTitleMatch := false
		for _, r := range results {
			if r.Title == "Fix authentication bug" {
				foundTitleMatch = true
				break
			}
		}
		if !foundTitleMatch {
			t.Error("expected 'Fix authentication bug' in results")
		}
	}
}

func TestFTSSearchTitleRanksHigher(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestProject(t, store)

	// Issue with "deploy" in title
	titleIssue := &types.Issue{
		ProjectID: ws.ID,
		Title:       "Deploy the application",
		Description: "Standard deployment task",
		Status:      types.StatusOpen,
		Priority:    2,
		IssueType:   types.TypeTask,
	}
	if err := store.CreateIssue(ctx, titleIssue, "test-actor"); err != nil {
		t.Fatalf("failed to create title issue: %v", err)
	}

	// Issue with "deploy" only in description
	descIssue := &types.Issue{
		ProjectID: ws.ID,
		Title:       "Update infrastructure",
		Description: "Need to deploy new version of the service",
		Status:      types.StatusOpen,
		Priority:    2,
		IssueType:   types.TypeTask,
	}
	if err := store.CreateIssue(ctx, descIssue, "test-actor"); err != nil {
		t.Fatalf("failed to create desc issue: %v", err)
	}

	results, err := store.ListIssues(ctx, types.IssueFilter{
		ProjectID: ws.ID,
		Query:       "deploy",
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Title match should come first (BM25 weights title at 10x vs description at 5x)
	if results[0].Title != "Deploy the application" {
		t.Errorf("expected title match first, got %q", results[0].Title)
	}
}

func TestFTSPrefixSearch(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestProject(t, store)

	setupTestIssue(t, store, ws, "Handle dependencies correctly")
	setupTestIssue(t, store, ws, "Deployment automation")
	setupTestIssue(t, store, ws, "Something unrelated")

	// "dep" should prefix-match both "dependencies" and "deployment"
	results, err := store.ListIssues(ctx, types.IssueFilter{
		ProjectID: ws.ID,
		Query:       "dep",
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results for prefix 'dep', got %d", len(results))
		for _, r := range results {
			t.Logf("  - %s", r.Title)
		}
	}
}

func TestFTSStemming(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestProject(t, store)

	setupTestIssue(t, store, ws, "Running the test suite")
	setupTestIssue(t, store, ws, "Something else entirely")

	// "run" should match "running" via Porter stemming
	results, err := store.ListIssues(ctx, types.IssueFilter{
		ProjectID: ws.ID,
		Query:       "run",
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result for stemmed 'run', got %d", len(results))
	}
	if len(results) > 0 && results[0].Title != "Running the test suite" {
		t.Errorf("expected 'Running the test suite', got %q", results[0].Title)
	}
}

func TestFTSCommentSearchNotIndexed(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestProject(t, store)

	setupTestIssue(t, store, ws, "Generic issue title")

	// Add a comment with unique searchable text
	issue := setupTestIssue(t, store, ws, "Another issue")
	_, err := store.AddComment(ctx, issue.ID, "author", "The frobnicator module needs refactoring")
	if err != nil {
		t.Fatalf("failed to add comment: %v", err)
	}

	// FTS only indexes title and description (simplified in migration 010).
	// Comments are not indexed, so searching for comment-only text returns no results.
	results, err := store.ListIssues(ctx, types.IssueFilter{
		ProjectID: ws.ID,
		Query:     "frobnicator",
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for comment-only text (not indexed), got %d", len(results))
	}
}

func TestFTSLabelSearchNotIndexed(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestProject(t, store)

	issue := setupTestIssue(t, store, ws, "Generic task")

	// Create label and add to issue
	label := &types.Label{Name: "frontend", Description: "Frontend work"}
	if err := store.CreateLabel(ctx, label); err != nil {
		t.Fatalf("failed to create label: %v", err)
	}
	if err := store.AddLabelToIssue(ctx, issue.ID, "frontend", "test-actor"); err != nil {
		t.Fatalf("failed to add label: %v", err)
	}

	// FTS only indexes title and description (simplified in migration 010).
	// Labels are not indexed, so searching for label-only text returns no results.
	results, err := store.ListIssues(ctx, types.IssueFilter{
		ProjectID: ws.ID,
		Query:     "frontend",
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for label-only text (not indexed), got %d", len(results))
	}
}

func TestFTSPlanSearchNotIndexed(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestProject(t, store)

	issue := setupTestIssue(t, store, ws, "Issue with plan")

	// Set an inline plan with unique text
	_, err := store.SetInlinePlan(ctx, issue.ID, "author", "Implement the zigzag algorithm for sorting")
	if err != nil {
		t.Fatalf("failed to set inline plan: %v", err)
	}

	// FTS only indexes title and description (simplified in migration 010).
	// Plan text is not indexed, so searching for plan-only text returns no results.
	results, err := store.ListIssues(ctx, types.IssueFilter{
		ProjectID: ws.ID,
		Query:     "zigzag",
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for plan-only text (not indexed), got %d", len(results))
	}
}

func TestFTSNoResults(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestProject(t, store)

	setupTestIssue(t, store, ws, "Some issue")

	results, err := store.ListIssues(ctx, types.IssueFilter{
		ProjectID: ws.ID,
		Query:       "xyznonexistent",
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestFTSDeletedIssueNotSearchable(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestProject(t, store)

	issue := setupTestIssue(t, store, ws, "Deletable task with uniqueword")

	// Verify it's searchable
	results, err := store.ListIssues(ctx, types.IssueFilter{
		ProjectID: ws.ID,
		Query:       "uniqueword",
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result before delete, got %d", len(results))
	}

	// Delete the issue
	if err := store.DeleteIssue(ctx, issue.ID); err != nil {
		t.Fatalf("failed to delete issue: %v", err)
	}

	// Verify it's no longer searchable
	results, err = store.ListIssues(ctx, types.IssueFilter{
		ProjectID: ws.ID,
		Query:       "uniqueword",
	})
	if err != nil {
		t.Fatalf("search failed after delete: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results after delete, got %d", len(results))
	}
}
