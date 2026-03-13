//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestListFilterByStatus creates issues with different statuses and verifies
// that the --status flag filters correctly.
func TestListFilterByStatus(t *testing.T) {
	home := setupHome(t)
	proj := fmt.Sprintf("filter-status-%d", uniqueCounter())

	arcCmdSuccess(t, home, "init", proj, "--server", serverURL)

	// Create 3 issues.
	out1 := arcCmdSuccess(t, home, "create", "Status open issue", "--type", "task", "--server", serverURL)
	id1, ok := extractID(out1)
	if !ok {
		t.Fatalf("could not extract ID from: %s", out1)
	}

	out2 := arcCmdSuccess(t, home, "create", "Status inprogress issue", "--type", "task", "--server", serverURL)
	id2, ok := extractID(out2)
	if !ok {
		t.Fatalf("could not extract ID from: %s", out2)
	}

	out3 := arcCmdSuccess(t, home, "create", "Status closed issue", "--type", "task", "--server", serverURL)
	id3, ok := extractID(out3)
	if !ok {
		t.Fatalf("could not extract ID from: %s", out3)
	}

	// Update one to in_progress.
	arcCmdSuccess(t, home, "update", id2, "--status", "in_progress", "--server", serverURL)

	// Close another.
	arcCmdSuccess(t, home, "close", id3, "--server", serverURL)

	// Filter by open — should show only the open one.
	listOpen := arcCmdSuccess(t, home, "list", "--status", "open", "--server", serverURL)
	if !strings.Contains(listOpen, "Status open issue") {
		t.Errorf("expected open issue in --status open output, got: %s", listOpen)
	}
	if strings.Contains(listOpen, "Status closed issue") {
		t.Errorf("closed issue should not appear in --status open output, got: %s", listOpen)
	}

	// Filter by closed — should show only the closed one.
	listClosed := arcCmdSuccess(t, home, "list", "--status", "closed", "--server", serverURL)
	if !strings.Contains(listClosed, "Status closed issue") {
		t.Errorf("expected closed issue in --status closed output, got: %s", listClosed)
	}
	if strings.Contains(listClosed, "Status open issue") {
		t.Errorf("open issue should not appear in --status closed output, got: %s", listClosed)
	}

	// Use id1 to avoid unused variable warning.
	_ = id1
}

// TestListFilterByType creates issues of different types and verifies
// that --type filters correctly.
func TestListFilterByType(t *testing.T) {
	home := setupHome(t)
	proj := fmt.Sprintf("filter-type-%d", uniqueCounter())

	arcCmdSuccess(t, home, "init", proj, "--server", serverURL)

	arcCmdSuccess(t, home, "create", "Filter type task issue", "--type", "task", "--server", serverURL)
	arcCmdSuccess(t, home, "create", "Filter type bug issue", "--type", "bug", "--server", serverURL)

	listBugs := arcCmdSuccess(t, home, "list", "--type", "bug", "--server", serverURL)
	if !strings.Contains(listBugs, "Filter type bug issue") {
		t.Errorf("expected bug issue in --type bug output, got: %s", listBugs)
	}
	if strings.Contains(listBugs, "Filter type task issue") {
		t.Errorf("task issue should not appear in --type bug output, got: %s", listBugs)
	}
}

// TestListFilterByAssignee creates issues and assigns one, then verifies
// that --assignee filters correctly.
func TestListFilterByAssignee(t *testing.T) {
	home := setupHome(t)
	proj := fmt.Sprintf("filter-assignee-%d", uniqueCounter())

	arcCmdSuccess(t, home, "init", proj, "--server", serverURL)

	out1 := arcCmdSuccess(t, home, "create", "Assignee alice issue", "--type", "task", "--server", serverURL)
	id1, ok := extractID(out1)
	if !ok {
		t.Fatalf("could not extract ID from: %s", out1)
	}

	arcCmdSuccess(t, home, "create", "Assignee unassigned issue", "--type", "task", "--server", serverURL)

	// Assign the first issue to alice.
	arcCmdSuccess(t, home, "update", id1, "--assignee", "alice", "--server", serverURL)

	listAlice := arcCmdSuccess(t, home, "list", "--assignee", "alice", "--server", serverURL)
	if !strings.Contains(listAlice, "Assignee alice issue") {
		t.Errorf("expected alice's issue in --assignee alice output, got: %s", listAlice)
	}
	if strings.Contains(listAlice, "Assignee unassigned issue") {
		t.Errorf("unassigned issue should not appear in --assignee alice output, got: %s", listAlice)
	}
}

// TestListFilterByQuery creates issues with distinct titles and verifies
// that --query performs FTS search.
func TestListFilterByQuery(t *testing.T) {
	home := setupHome(t)
	proj := fmt.Sprintf("filter-query-%d", uniqueCounter())

	arcCmdSuccess(t, home, "init", proj, "--server", serverURL)

	arcCmdSuccess(t, home, "create", "Database migration task", "--type", "task", "--server", serverURL)
	arcCmdSuccess(t, home, "create", "Frontend styling task", "--type", "task", "--server", serverURL)

	listDB := arcCmdSuccess(t, home, "list", "--query", "database", "--server", serverURL)
	if !strings.Contains(listDB, "Database migration task") {
		t.Errorf("expected database issue in --query database output, got: %s", listDB)
	}
	if strings.Contains(listDB, "Frontend styling task") {
		t.Errorf("frontend issue should not appear in --query database output, got: %s", listDB)
	}
}

// TestListLimit creates multiple issues and verifies that --limit restricts
// the number of results.
func TestListLimit(t *testing.T) {
	home := setupHome(t)
	proj := fmt.Sprintf("filter-limit-%d", uniqueCounter())

	arcCmdSuccess(t, home, "init", proj, "--server", serverURL)

	for i := 0; i < 5; i++ {
		arcCmdSuccess(t, home, "create", fmt.Sprintf("Limit test issue %d", i), "--type", "task", "--server", serverURL)
	}

	listOut := arcCmdSuccess(t, home, "list", "--limit", "2", "--server", serverURL)

	// Count how many issue IDs appear in the output.
	lines := strings.Split(strings.TrimSpace(listOut), "\n")
	issueCount := 0
	for _, line := range lines {
		if _, ok := extractID(line); ok {
			issueCount++
		}
	}
	if issueCount > 2 {
		t.Errorf("expected at most 2 issues with --limit 2, got %d in output:\n%s", issueCount, listOut)
	}
}

// TestListFilterByParent creates an epic with child tasks and verifies
// that --parent filters to only children of that epic. Uses a temp dir for
// CWD isolation so project resolution is correct.
func TestListFilterByParent(t *testing.T) {
	home := setupHome(t)
	proj := fmt.Sprintf("filter-parent-%d", uniqueCounter())
	dir := t.TempDir()

	arcCmdInDirSuccess(t, home, dir, "init", proj, "--server", serverURL)

	// Create an epic.
	epicOut := arcCmdInDirSuccess(t, home, dir, "create", "Parent epic issue", "--type", "epic", "--server", serverURL)
	epicID, ok := extractID(epicOut)
	if !ok {
		t.Fatalf("could not extract epic ID from: %s", epicOut)
	}

	// Create child tasks using --parent flag.
	arcCmdInDirSuccess(t, home, dir, "create", "Child task one", "--type", "task", "--parent", epicID, "--server", serverURL)
	arcCmdInDirSuccess(t, home, dir, "create", "Child task two", "--type", "task", "--parent", epicID, "--server", serverURL)

	// Create a standalone task.
	arcCmdInDirSuccess(t, home, dir, "create", "Standalone task", "--type", "task", "--server", serverURL)

	// Filter by parent.
	listChildren := arcCmdInDirSuccess(t, home, dir, "list", "--parent", epicID, "--server", serverURL)
	if !strings.Contains(listChildren, "Child task one") {
		t.Errorf("expected child task one in --parent output, got: %s", listChildren)
	}
	if !strings.Contains(listChildren, "Child task two") {
		t.Errorf("expected child task two in --parent output, got: %s", listChildren)
	}
	if strings.Contains(listChildren, "Standalone task") {
		t.Errorf("standalone task should not appear in --parent output, got: %s", listChildren)
	}
}

// TestListJsonOutput verifies that `arc list --json` outputs valid JSON
// containing the expected issue data. Uses temp dir for CWD isolation.
func TestListJsonOutput(t *testing.T) {
	home := setupHome(t)
	proj := fmt.Sprintf("filter-json-list-%d", uniqueCounter())
	dir := t.TempDir()

	arcCmdInDirSuccess(t, home, dir, "init", proj, "--server", serverURL)

	arcCmdInDirSuccess(t, home, dir, "create", "JSON list issue one", "--type", "task", "--server", serverURL)
	arcCmdInDirSuccess(t, home, dir, "create", "JSON list issue two", "--type", "bug", "--server", serverURL)

	listJSON := arcCmdInDirSuccess(t, home, dir, "list", "--json", "--server", serverURL)

	var issues []map[string]interface{}
	if err := json.Unmarshal([]byte(listJSON), &issues); err != nil {
		t.Fatalf("failed to parse JSON list output: %v\nraw output: %s", err, listJSON)
	}

	if len(issues) != 2 {
		t.Errorf("expected 2 issues in JSON output, got %d", len(issues))
	}

	for i, issue := range issues {
		if _, ok := issue["id"]; !ok {
			t.Errorf("issue %d missing 'id' field", i)
		}
		if _, ok := issue["title"]; !ok {
			t.Errorf("issue %d missing 'title' field", i)
		}
		if _, ok := issue["status"]; !ok {
			t.Errorf("issue %d missing 'status' field", i)
		}
	}
}

// TestShowJsonOutput verifies that `arc show <id> --json` outputs valid JSON
// with the correct issue details.
func TestShowJsonOutput(t *testing.T) {
	home := setupHome(t)
	proj := fmt.Sprintf("filter-json-show-%d", uniqueCounter())

	arcCmdSuccess(t, home, "init", proj, "--server", serverURL)

	createOut := arcCmdSuccess(t, home, "create", "JSON test issue", "--type", "task", "--server", serverURL)
	id, ok := extractID(createOut)
	if !ok {
		t.Fatalf("could not extract ID from: %s", createOut)
	}

	showJSON := arcCmdSuccess(t, home, "show", id, "--json", "--server", serverURL)

	var issue map[string]interface{}
	if err := json.Unmarshal([]byte(showJSON), &issue); err != nil {
		t.Fatalf("failed to parse JSON show output: %v\nraw output: %s", err, showJSON)
	}

	if issue["id"] == nil {
		t.Error("show JSON missing 'id' field")
	}
	title, _ := issue["title"].(string)
	if title != "JSON test issue" {
		t.Errorf("expected title 'JSON test issue', got '%s'", title)
	}
	if issue["status"] == nil {
		t.Error("show JSON missing 'status' field")
	}
}

// TestReadyJsonOutput verifies that `arc ready --json` outputs a valid JSON array.
func TestReadyJsonOutput(t *testing.T) {
	home := setupHome(t)
	proj := fmt.Sprintf("filter-json-ready-%d", uniqueCounter())

	arcCmdSuccess(t, home, "init", proj, "--server", serverURL)

	arcCmdSuccess(t, home, "create", "Ready JSON issue", "--type", "task", "--server", serverURL)

	readyJSON := arcCmdSuccess(t, home, "ready", "--json", "--server", serverURL)

	var issues []map[string]interface{}
	if err := json.Unmarshal([]byte(readyJSON), &issues); err != nil {
		t.Fatalf("failed to parse JSON ready output: %v\nraw output: %s", err, readyJSON)
	}

	if len(issues) < 1 {
		t.Errorf("expected at least 1 issue in ready JSON output, got %d", len(issues))
	}
}

// _filterCounter is an atomic counter combined with a timestamp base to
// generate project names that are unique across test runs.
var _filterCounter int64

func uniqueCounter() int64 {
	base := time.Now().UnixMilli()
	seq := atomic.AddInt64(&_filterCounter, 1)
	return base + seq
}
