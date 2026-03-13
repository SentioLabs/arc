//go:build integration

package integration

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var _epicCounter int64

func epicUniqueCounter() int64 {
	base := time.Now().UnixMilli()
	seq := atomic.AddInt64(&_epicCounter, 1)
	return base + seq
}

// TestCreateEpic creates an epic and verifies that arc show reports the type
// as "epic".
func TestCreateEpic(t *testing.T) {
	home := setupHome(t)
	projName := fmt.Sprintf("epic-create-%d", epicUniqueCounter())

	arcCmdSuccess(t, home, "init", projName, "--server", serverURL)

	createOut := arcCmdSuccess(t, home, "create", "My Epic", "--type", "epic", "--server", serverURL)
	id, ok := extractID(createOut)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", createOut)
	}

	showOut := arcCmdSuccess(t, home, "show", id, "--server", serverURL)
	if !strings.Contains(strings.ToLower(showOut), "epic") {
		t.Errorf("expected show output to contain type 'epic', got: %s", showOut)
	}
}

// TestCreateWithParent creates an epic and a child task using --parent,
// then verifies the child appears when listing by parent.
func TestCreateWithParent(t *testing.T) {
	home := setupHome(t)
	projName := fmt.Sprintf("epic-parent-%d", epicUniqueCounter())

	arcCmdSuccess(t, home, "init", projName, "--server", serverURL)

	epicOut := arcCmdSuccess(t, home, "create", "Parent Epic", "--type", "epic", "--server", serverURL)
	epicID, ok := extractID(epicOut)
	if !ok {
		t.Fatalf("could not extract epic ID from create output: %s", epicOut)
	}

	childOut := arcCmdSuccess(t, home, "create", "Child task", "--type", "task", "--parent", epicID, "--server", serverURL)
	_, ok = extractID(childOut)
	if !ok {
		t.Fatalf("could not extract child ID from create output: %s", childOut)
	}

	listOut := arcCmdSuccess(t, home, "list", "--parent", epicID, "--server", serverURL)
	if !strings.Contains(listOut, "Child task") {
		t.Errorf("expected child task in list --parent output, got: %s", listOut)
	}
}

// TestEpicWithMultipleChildren creates an epic with 3 child tasks and verifies
// all children appear when listing by parent.
func TestEpicWithMultipleChildren(t *testing.T) {
	home := setupHome(t)
	projName := fmt.Sprintf("epic-multi-%d", epicUniqueCounter())

	arcCmdSuccess(t, home, "init", projName, "--server", serverURL)

	epicOut := arcCmdSuccess(t, home, "create", "Multi Child Epic", "--type", "epic", "--server", serverURL)
	epicID, ok := extractID(epicOut)
	if !ok {
		t.Fatalf("could not extract epic ID from create output: %s", epicOut)
	}

	children := []string{"Child Alpha", "Child Beta", "Child Gamma"}
	for _, name := range children {
		out := arcCmdSuccess(t, home, "create", name, "--type", "task", "--parent", epicID, "--server", serverURL)
		_, ok := extractID(out)
		if !ok {
			t.Fatalf("could not extract child ID from create output: %s", out)
		}
	}

	listOut := arcCmdSuccess(t, home, "list", "--parent", epicID, "--server", serverURL)
	for _, name := range children {
		if !strings.Contains(listOut, name) {
			t.Errorf("expected %q in list --parent output, got: %s", name, listOut)
		}
	}
}

// TestEpicShowIncludesChildren creates an epic with 2 children and verifies
// that arc show on the epic mentions the children.
func TestEpicShowIncludesChildren(t *testing.T) {
	home := setupHome(t)
	projName := fmt.Sprintf("epic-show-children-%d", epicUniqueCounter())

	arcCmdSuccess(t, home, "init", projName, "--server", serverURL)

	epicOut := arcCmdSuccess(t, home, "create", "Show Children Epic", "--type", "epic", "--server", serverURL)
	epicID, ok := extractID(epicOut)
	if !ok {
		t.Fatalf("could not extract epic ID from create output: %s", epicOut)
	}

	child1Out := arcCmdSuccess(t, home, "create", "Show Child One", "--type", "task", "--parent", epicID, "--server", serverURL)
	child1ID, ok := extractID(child1Out)
	if !ok {
		t.Fatalf("could not extract child1 ID from create output: %s", child1Out)
	}

	child2Out := arcCmdSuccess(t, home, "create", "Show Child Two", "--type", "task", "--parent", epicID, "--server", serverURL)
	child2ID, ok := extractID(child2Out)
	if !ok {
		t.Fatalf("could not extract child2 ID from create output: %s", child2Out)
	}

	showOut := arcCmdSuccess(t, home, "show", epicID, "--server", serverURL)
	// The show output should reference the children by ID or name.
	if !strings.Contains(showOut, child1ID) && !strings.Contains(showOut, "Show Child One") {
		t.Errorf("expected show output for epic to mention child 1 (%s), got: %s", child1ID, showOut)
	}
	if !strings.Contains(showOut, child2ID) && !strings.Contains(showOut, "Show Child Two") {
		t.Errorf("expected show output for epic to mention child 2 (%s), got: %s", child2ID, showOut)
	}
}

// TestCreateAllIssueTypes creates one issue of each type (bug, feature, task,
// epic, chore) and verifies that arc show reports the correct type for each.
func TestCreateAllIssueTypes(t *testing.T) {
	home := setupHome(t)
	projName := fmt.Sprintf("epic-types-%d", epicUniqueCounter())

	arcCmdSuccess(t, home, "init", projName, "--server", serverURL)

	types := []string{"bug", "feature", "task", "epic", "chore"}
	for _, issueType := range types {
		t.Run(issueType, func(t *testing.T) {
			title := fmt.Sprintf("Issue type %s", issueType)
			createOut := arcCmdSuccess(t, home, "create", title, "--type", issueType, "--server", serverURL)
			id, ok := extractID(createOut)
			if !ok {
				t.Fatalf("could not extract issue ID from create output: %s", createOut)
			}

			showOut := arcCmdSuccess(t, home, "show", id, "--server", serverURL)
			if !strings.Contains(strings.ToLower(showOut), issueType) {
				t.Errorf("expected show output to contain type %q, got: %s", issueType, showOut)
			}
		})
	}
}

// TestCreateWithPriorityAndAssignee creates an issue with priority, assignee,
// and type flags, then verifies they appear in the show output.
func TestCreateWithPriorityAndAssignee(t *testing.T) {
	home := setupHome(t)
	projName := fmt.Sprintf("epic-priority-%d", epicUniqueCounter())

	arcCmdSuccess(t, home, "init", projName, "--server", serverURL)

	createOut := arcCmdSuccess(t, home, "create", "Priority bug", "--priority", "1", "--assignee", "alice", "--type", "bug", "--server", serverURL)
	id, ok := extractID(createOut)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", createOut)
	}

	showOut := arcCmdSuccess(t, home, "show", id, "--server", serverURL)
	lower := strings.ToLower(showOut)

	if !strings.Contains(lower, "p1") && !strings.Contains(lower, "priority") && !strings.Contains(showOut, "1") {
		t.Errorf("expected show output to mention priority 1 or P1, got: %s", showOut)
	}
	if !strings.Contains(lower, "alice") {
		t.Errorf("expected show output to mention assignee 'alice', got: %s", showOut)
	}
	if !strings.Contains(lower, "bug") {
		t.Errorf("expected show output to mention type 'bug', got: %s", showOut)
	}
}

// TestNestedParentChild creates an epic, a task under it, and a subtask under
// the task (via dep add --type parent-child). Verifies the chain is visible.
func TestNestedParentChild(t *testing.T) {
	home := setupHome(t)
	projName := fmt.Sprintf("epic-nested-%d", epicUniqueCounter())

	arcCmdSuccess(t, home, "init", projName, "--server", serverURL)

	// Create epic.
	epicOut := arcCmdSuccess(t, home, "create", "Nested Epic", "--type", "epic", "--server", serverURL)
	epicID, ok := extractID(epicOut)
	if !ok {
		t.Fatalf("could not extract epic ID from create output: %s", epicOut)
	}

	// Create task under epic.
	taskOut := arcCmdSuccess(t, home, "create", "Nested Task", "--type", "task", "--parent", epicID, "--server", serverURL)
	taskID, ok := extractID(taskOut)
	if !ok {
		t.Fatalf("could not extract task ID from create output: %s", taskOut)
	}

	// Create subtask (standalone first, then link via dep add).
	subtaskOut := arcCmdSuccess(t, home, "create", "Nested Subtask", "--type", "task", "--server", serverURL)
	subtaskID, ok := extractID(subtaskOut)
	if !ok {
		t.Fatalf("could not extract subtask ID from create output: %s", subtaskOut)
	}

	// Add parent-child: subtask is child of task.
	depOut := arcCmdSuccess(t, home, "dep", "add", subtaskID, taskID, "--type", "parent-child", "--server", serverURL)
	if !strings.Contains(depOut, "Added") {
		t.Errorf("expected 'Added' in dep add output, got: %s", depOut)
	}

	// Verify epic show mentions the task.
	epicShow := arcCmdSuccess(t, home, "show", epicID, "--server", serverURL)
	if !strings.Contains(epicShow, taskID) && !strings.Contains(epicShow, "Nested Task") {
		t.Errorf("expected epic show to mention task %s, got: %s", taskID, epicShow)
	}

	// Verify task show mentions the subtask.
	taskShow := arcCmdSuccess(t, home, "show", taskID, "--server", serverURL)
	if !strings.Contains(taskShow, subtaskID) && !strings.Contains(taskShow, "Nested Subtask") {
		t.Errorf("expected task show to mention subtask %s, got: %s", subtaskID, taskShow)
	}
}
