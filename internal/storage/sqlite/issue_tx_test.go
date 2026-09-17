package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/sentiolabs/arc/internal/storage"

	"github.com/sentiolabs/arc/internal/types"
)

func TestIssueMutationRollback(t *testing.T) {
	for _, stage := range []string{"counter", "parent", "event", "index"} {
		t.Run(stage, func(t *testing.T) {
			assertCreationRollback(t, stage)
		})
	}
}

func TestContractVersionsAndGeneration(t *testing.T) {
	s, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()
	p := setupTestProject(t, s)
	i := setupTestIssue(t, s, p, "versioned")
	got, err := s.GetIssue(ctx, i.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ContractVersion != 1 {
		t.Fatalf("initial version=%d, want 1", got.ContractVersion)
	}
	p, err = s.GetProject(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if p.GovernanceGeneration != 1 {
		t.Fatalf("initial generation=%d", p.GovernanceGeneration)
	}
	for n, u := range []map[string]any{
		{"title": "changed"},
		{"description": "scope"},
		{"status": "in_progress", "ai_session_id": "worker"},
		{"issue_type": "bug"},
	} {
		before, _ := s.GetIssue(ctx, i.ID)
		if err := s.UpdateIssue(ctx, i.ID, u, "actor"); err != nil {
			t.Fatal(err)
		}
		after, _ := s.GetIssue(ctx, i.ID)
		if after.ContractVersion <= before.ContractVersion {
			t.Fatalf("update %d did not increment contract", n)
		}
	}
	before, _ := s.GetIssue(ctx, i.ID)
	if err := s.UpdateIssue(ctx, i.ID, map[string]any{"priority": 1}, "actor"); err != nil {
		t.Fatal(err)
	}
	after, _ := s.GetIssue(ctx, i.ID)
	if before.ContractVersion != after.ContractVersion {
		t.Fatal("display metadata invalidated contract")
	}
	for n, apply := range []func() error{
		func() error { return s.CloseIssue(ctx, i.ID, "done", false, "actor") },
		func() error { return s.ReopenIssue(ctx, i.ID, "actor") },
	} {
		before, _ = s.GetIssue(ctx, i.ID)
		if err := apply(); err != nil {
			t.Fatal(err)
		}
		after, _ = s.GetIssue(ctx, i.ID)
		if after.ContractVersion <= before.ContractVersion {
			t.Fatalf("transition %d did not increment", n)
		}
	}
}

func TestMergeAtomicAuditAndAllIssues(t *testing.T) {
	s, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()
	source := setupTestProject(t, s)
	target := &types.Project{ID: "target", Name: "target", Prefix: "tar"}
	if err := s.CreateProject(ctx, target); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 105; n++ {
		setupTestIssue(t, s, source, fmt.Sprintf("source item %d", n))
	}
	if _, err := s.DB().Exec(`
CREATE TRIGGER fail_merge BEFORE INSERT ON events WHEN NEW.event_type='merged' BEGIN
SELECT RAISE(ABORT,'merge audit failure'); END
`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MergeProjects(ctx, target.ID, []string{source.ID}, "merger"); err == nil {
		t.Fatal("merge succeeded with lost audit")
	}
	if _, err := s.GetProject(ctx, source.ID); err != nil {
		t.Fatal("partial source deletion", err)
	}
	if _, err := s.DB().Exec(`DROP TRIGGER fail_merge`); err != nil {
		t.Fatal(err)
	}
	result, err := s.MergeProjects(ctx, target.ID, []string{source.ID}, "merger")
	if err != nil {
		t.Fatal(err)
	}
	if result.IssuesMoved != 105 {
		t.Fatalf("moved=%d", result.IssuesMoved)
	}
	var count int
	if err := s.DB().QueryRow(
		`SELECT count(*) FROM events WHERE event_type='merged' AND actor='merger'`,
	).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 105 {
		t.Fatalf("merge audit count=%d", count)
	}
}

func TestUpdateClosePreservesChildCheck(t *testing.T) {
	s, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()
	p := setupTestProject(t, s)
	parent := setupTestIssue(t, s, p, "parent")
	child := &types.Issue{ProjectID: p.ID, Title: "child", ParentID: parent.ID}
	if err := s.CreateIssue(ctx, child, "actor"); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateIssue(ctx, parent.ID, map[string]any{"status": "closed"}, "actor"); err == nil {
		t.Fatal("status update bypassed open-child check")
	}
}

func TestAllMutationAuditRollback(t *testing.T) {
	for _, operation := range []string{
		"update",
		"close",
		"reopen",
		"add dependency",
		"remove dependency",
		"delete",
	} {
		t.Run(operation, func(t *testing.T) {
			assertMutationRollback(t, operation)
		})
	}
}

func assertCreationRollback(t *testing.T, stage string) {
	t.Helper()
	s, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()
	p := setupTestProject(t, s)
	parent := setupTestIssue(t, s, p, "parent")
	var inject string
	switch stage {
	case "counter":
		inject = `
CREATE TRIGGER fail_counter BEFORE INSERT ON child_counters BEGIN SELECT
RAISE(ABORT,'counter failure'); END
`
	case "parent":
		inject = `
CREATE TRIGGER fail_parent BEFORE INSERT ON dependencies BEGIN SELECT RAISE(ABORT,'parent
failure'); END
`
	case "event":
		inject = `
CREATE TRIGGER fail_event BEFORE INSERT ON events BEGIN SELECT RAISE(ABORT,'event
failure'); END
`
	case "index":
		inject = `DROP TABLE issues_fts`
	}
	if _, err := s.DB().Exec(inject); err != nil {
		t.Fatal(err)
	}
	child := &types.Issue{
		ProjectID: p.ID,
		ParentID:  parent.ID,
		Title:     "rollback child",
		IssueType: types.TypeTask,
	}
	if err := s.CreateIssue(ctx, child, "actor"); err == nil {
		t.Fatal("mutation unexpectedly succeeded")
	}
	for _, query := range []string{
		`SELECT COUNT(*) FROM issues WHERE title='rollback child'`,
		`SELECT COUNT(*) FROM child_counters WHERE parent_id=?`,
		`SELECT COUNT(*) FROM dependencies WHERE depends_on_id=?`,
	} {
		var n int
		var args []any
		if query != `SELECT COUNT(*) FROM issues WHERE title='rollback child'` {
			args = []any{parent.ID}
		}
		if err := s.DB().QueryRow(query, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("partial mutation: %s = %d", query, n)
		}
	}
}

func assertMutationRollback(t *testing.T, operation string) {
	t.Helper()
	s, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()
	p := setupTestProject(t, s)
	i := setupTestIssue(t, s, p, "audited")
	other := setupTestIssue(t, s, p, "other")
	if operation == "reopen" {
		if err := s.CloseIssue(ctx, i.ID, "done", false, "actor"); err != nil {
			t.Fatal(err)
		}
	}
	if operation == "remove dependency" {
		if err := s.AddDependency(ctx,
			&types.Dependency{IssueID: i.ID, DependsOnID: other.ID, Type: types.DepParentChild},
			"actor"); err != nil {
			t.Fatal(err)
		}
	}
	before, err := s.GetIssue(ctx, i.ID)
	if err != nil {
		t.Fatal(err)
	}
	generation, err := s.GetProject(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	trigger := `
CREATE TRIGGER fail_audit BEFORE INSERT ON events BEGIN SELECT RAISE(ABORT,'audit
failure'); END
`
	if operation == "delete" {
		trigger = `
CREATE TRIGGER fail_delete BEFORE DELETE ON issues BEGIN SELECT RAISE(ABORT,'delete
failure'); END
`
	}
	if _, err := s.DB().Exec(trigger); err != nil {
		t.Fatal(err)
	}
	switch operation {
	case "update":
		err = s.UpdateIssue(ctx,
			i.ID,
			map[string]any{"title": "new scope", "description": "new description"},
			"actor")
	case "close":
		err = s.CloseIssue(ctx, i.ID, "done", false, "actor")
	case "reopen":
		err = s.ReopenIssue(ctx, i.ID, "actor")
	case "add dependency":
		err = s.AddDependency(ctx,
			&types.Dependency{IssueID: i.ID, DependsOnID: other.ID, Type: types.DepParentChild},
			"actor")
	case "remove dependency":
		err = s.RemoveDependency(ctx, i.ID, other.ID, "actor")
	case "delete":
		err = s.DeleteIssue(ctx, i.ID)
	}
	if err == nil {
		t.Fatal("faulted mutation succeeded")
	}
	after, err := s.GetIssue(ctx, i.ID)
	if err != nil {
		t.Fatal(err)
	}
	next, err := s.GetProject(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if before.ContractVersion != after.ContractVersion ||
		before.Title != after.Title ||
		before.Status != after.Status ||
		generation.GovernanceGeneration != next.GovernanceGeneration {
		t.Fatal("failed mutation partially committed")
	}
	results, err := s.ListIssues(ctx, types.IssueFilter{ProjectID: p.ID, Query: "audited"})
	if err != nil || len(results) != 1 {
		t.Fatalf("index lost on rollback: %v %v", results, err)
	}
}

func TestCascadeRejectsLegacyCycle(t *testing.T) {
	s, cleanup := setupTestStore(t)
	defer cleanup()
	p := setupTestProject(t, s)
	a := setupTestIssue(t, s, p, "cycle a")
	b := setupTestIssue(t, s, p, "cycle b")
	for _, edge := range [][2]string{{a.ID, b.ID}, {b.ID, a.ID}} {
		if _, err := s.DB().Exec(
			`INSERT INTO dependencies(issue_id,depends_on_id,type) VALUES (?,?,'parent-child')`, edge[0], edge[1],
		); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := s.CloseIssue(ctx, a.ID, "done", true, "actor"); !errors.Is(err, storage.ErrGovernanceCycle) {
		t.Fatalf("cascade error=%v", err)
	}
}

func TestVersionsOnReadSurfacesAndDisplayChanges(t *testing.T) {
	s, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()
	p := setupTestProject(t, s)
	i := setupTestIssue(t, s, p, "SearchableVersion")
	if err := s.UpdateIssue(ctx, i.ID, map[string]any{"title": "SearchableVersion updated"}, "actor"); err != nil {
		t.Fatal(err)
	}
	before, err := s.GetIssue(ctx, i.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddLabelToIssue(ctx, i.ID, "display-only", "actor"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddComment(ctx, i.ID, "actor", "discussion only"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`UPDATE issues SET rank=3 WHERE id=?`, i.ID); err != nil {
		t.Fatal(err)
	}
	after, err := s.GetIssue(ctx, i.ID)
	if err != nil {
		t.Fatal(err)
	}
	if before.ContractVersion != after.ContractVersion {
		t.Fatal("display changes invalidated contract")
	}
	for _, query := range []string{"", "SearchableVersion"} {
		rows, err := s.ListIssues(ctx, types.IssueFilter{ProjectID: p.ID, Query: query})
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || rows[0].ContractVersion != before.ContractVersion {
			t.Fatalf("list/search omitted persisted version: %v", rows)
		}
	}
	ready, err := s.GetReadyWork(ctx, types.WorkFilter{ProjectID: p.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(ready) != 1 || ready[0].ContractVersion != before.ContractVersion {
		t.Fatal("ready omitted persisted version")
	}
}

func TestConcurrentChildrenAreAtomic(t *testing.T) {
	s, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()
	p := setupTestProject(t, s)
	parent := setupTestIssue(t, s, p, "concurrent parent")
	const writers = 4
	results := make(chan *types.Issue, writers)
	failures := make(chan error, writers)
	for range writers {
		go func() {
			child := &types.Issue{ProjectID: p.ID, ParentID: parent.ID, Title: "concurrent child"}
			if err := s.CreateIssue(ctx, child, "worker"); err != nil {
				failures <- err
				return
			}
			results <- child
		}()
	}
	ids := map[string]bool{}
	for range writers {
		select {
		case err := <-failures:
			t.Fatal(err)
		case child := <-results:
			if ids[child.ID] {
				t.Fatal("counter reused")
			}
			ids[child.ID] = true
			deps, err := s.GetDependencies(ctx, child.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(deps) != 1 || deps[0].DependsOnID != parent.ID || child.ContractVersion < 1 {
				t.Fatal("partial child creation")
			}
		}
	}
}
