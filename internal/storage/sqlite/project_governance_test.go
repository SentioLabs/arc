package sqlite_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/storage/sqlite"
	"github.com/sentiolabs/arc/internal/types"
)

func TestMergeRejectsIncidentCrossProjectAncestry(t *testing.T) {
	for _, edge := range [][2]string{
		{"source", "target"},
		{"target", "source"},
		{"third", "source"},
		{"source", "third"},
		{"target", "third"},
		{"third", "target"},
	} {
		t.Run(edge[0]+" child to "+edge[1]+" parent", func(t *testing.T) {
			s, cleanup := setupTestStore(t)
			defer cleanup()
			ctx := context.Background()
			projects, issues := setupMergeGovernanceFixture(t, s)
			child, parent := issues[edge[0]], issues[edge[1]]
			// Legacy malformed ancestry is seeded directly; ordinary dependency writes reject it.
			if _, err := s.DB().Exec(`INSERT INTO dependencies(issue_id,depends_on_id,type)
 VALUES (?,?,'parent-child')`, child.ID, parent.ID); err != nil {
				t.Fatal(err)
			}
			before := governanceDatabaseSnapshot(t, s.DB())
			_, err := s.MergeProjects(ctx,
				projects["target"].ID,
				[]string{projects["clean"].ID, projects["source"].ID},
				"merger")
			after := governanceDatabaseSnapshot(t, s.DB())
			t.Logf("before: %s\nafter: %s", before, after)
			if !errors.Is(err, storage.ErrGovernanceReconciliation) {
				t.Errorf("merge must reject unsafe ancestry: %v", err)
			}
			if before != after {
				t.Error("rejected merge changed ownership, generation, versions, audit, counters or index")
			}
		})
	}
}

func TestMergePreservesCrossProjectBlocksAndUnrelatedAncestry(t *testing.T) {
	s, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()
	projects, issues := setupMergeGovernanceFixture(t, s)
	for _, edge := range [][2]string{{"target", "source"}, {"third", "source"}, {"source", "third"}} {
		if err := s.AddDependency(ctx, &types.Dependency{
			IssueID: issues[edge[0]].ID, DependsOnID: issues[edge[1]].ID, Type: types.DepBlocks,
		}, "actor"); err != nil {
			t.Fatal(err)
		}
	}
	outsider := &types.Project{ID: "unrelated", Name: "unrelated", Prefix: "unr"}
	if err := s.CreateProject(ctx, outsider); err != nil {
		t.Fatal(err)
	}
	other := setupTestIssue(t, s, outsider, "unrelated child")
	if _, err := s.DB().Exec(`INSERT INTO dependencies(issue_id,depends_on_id,type)
 VALUES (?,?,'parent-child')`, other.ID, issues["third"].ID); err != nil {
		t.Fatal(err)
	}
	result, err := s.MergeProjects(ctx, projects["target"].ID, []string{projects["source"].ID}, "merger")
	if err != nil {
		t.Fatal(err)
	}
	if result.IssuesMoved != 1 {
		t.Fatalf("moved=%d", result.IssuesMoved)
	}
	moved, err := s.GetIssue(ctx, issues["source"].ID)
	if err != nil {
		t.Fatal(err)
	}
	if moved.ProjectID != projects["target"].ID {
		t.Fatal("ownership did not move")
	}
	deps, err := s.GetDependencies(ctx, issues["third"].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 1 || deps[0].Type != types.DepBlocks {
		t.Fatal("cross-project blocker lost")
	}
}

func setupMergeGovernanceFixture(t *testing.T,
	s *sqlite.Store) (map[string]*types.Project,
	map[string]*types.Issue,
) {
	t.Helper()
	ctx := context.Background()
	projects := map[string]*types.Project{}
	issues := map[string]*types.Issue{}
	for _, name := range []string{"source", "target", "third", "clean"} {
		p := &types.Project{ID: name, Name: name, Prefix: name}
		if err := s.CreateProject(ctx, p); err != nil {
			t.Fatal(err)
		}
		projects[name] = p
		issues[name] = setupTestIssue(t, s, p, name+" issue")
	}
	// A target-owned pin makes silent repair especially dangerous: the target task
	// can go from malformed ancestry to apparently valid governed work after merge.
	epic := &types.Issue{
		ID:        "target-epic",
		ProjectID: projects["target"].ID,
		Title:     "Target design",
		IssueType: types.TypeEpic,
	}
	if err := s.CreateIssue(ctx, epic, "actor"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddDependency(ctx, &types.Dependency{
		IssueID: issues["target"].ID, DependsOnID: epic.ID, Type: types.DepParentChild,
	}, "actor"); err != nil {
		t.Fatal(err)
	}
	seedGovernanceRevisions(t, s.DB(), projects["target"].ID)
	if _, err := s.DB().Exec(`UPDATE issues SET governing_plan_id='version-plan',governing_plan_revision=1 WHERE id=?`,
		epic.ID); err != nil {
		t.Fatal(err)
	}
	return projects, issues
}

// governanceDatabaseSnapshot captures observable state, including logical FTS
// rows. It deliberately omits SQLite's internal page/index representation.
func governanceDatabaseSnapshot(t *testing.T, database *sql.DB) string {
	t.Helper()
	snapshot := map[string][][]any{}
	for table, query := range map[string]string{
		"projects":                 "SELECT * FROM projects ORDER BY 1,2",
		"issues":                   "SELECT * FROM issues ORDER BY 1,2",
		"dependencies":             "SELECT * FROM dependencies ORDER BY 1,2",
		"events":                   "SELECT * FROM events ORDER BY 1,2",
		"child_counters":           "SELECT * FROM child_counters ORDER BY 1,2",
		"issues_fts":               "SELECT * FROM issues_fts ORDER BY 1,2",
		"issue_governance_history": "SELECT * FROM issue_governance_history ORDER BY 1,2",
	} {
		snapshot[table] = snapshotGovernanceRows(t, database, query)
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func snapshotGovernanceRows(t *testing.T, database *sql.DB, query string) [][]any {
	t.Helper()
	// Queries come from the fixed snapshot list above.
	rows, err := database.Query(query)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var result [][]any
	for rows.Next() {
		values := make([]any, len(columns))
		dest := make([]any, len(columns))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			t.Fatal(err)
		}
		result = append(result, values)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func seedGovernanceRevisions(t *testing.T, database *sql.DB, projectID string) {
	t.Helper()
	if _, err := database.Exec(`INSERT INTO plans(id,project_id,title,head_revision,created_at,updated_at)
 VALUES ('version-plan',?,'Version fixture',2,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`, projectID); err != nil {
		t.Fatal(err)
	}
	for revision := 1; revision <= 2; revision++ {
		if _, err := database.Exec(`INSERT INTO plan_revisions
 (plan_id,revision,content_path,content_sha256,content_bytes,review_status,created_at)
 VALUES ('version-plan',
?,
?,
 'fixture',
0,
'approved',
CURRENT_TIMESTAMP)`, revision, fmt.Sprintf("revision-%d", revision)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSuccessfulGovernanceGenerationChanges(t *testing.T) {
	s, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()
	p := setupTestProject(t, s)
	a, b := &types.Issue{ProjectID: p.ID, Title: "a"}, &types.Issue{ProjectID: p.ID, Title: "b"}
	changes := []struct {
		name  string
		apply func() error
	}{
		{"create a", func() error { return s.CreateIssue(ctx, a, "actor") }},
		{"create b", func() error { return s.CreateIssue(ctx, b, "actor") }},
		{"parent add", func() error {
			return s.AddDependency(ctx,
				&types.Dependency{IssueID: a.ID, DependsOnID: b.ID, Type: types.DepParentChild},
				"actor")
		}},
		{"parent remove", func() error { return s.RemoveDependency(ctx, a.ID, b.ID, "actor") }},
		{"parent readd", func() error {
			return s.AddDependency(ctx,
				&types.Dependency{IssueID: a.ID, DependsOnID: b.ID, Type: types.DepParentChild},
				"actor")
		}},
		{"edge conversion", func() error {
			return s.AddDependency(ctx,
				&types.Dependency{IssueID: a.ID, DependsOnID: b.ID, Type: types.DepBlocks},
				"actor")
		}},
		{
			"issue type",
			func() error { return s.UpdateIssue(ctx, a.ID, map[string]any{"issue_type": "epic"}, "actor") },
		},
		{"delete", func() error { return s.DeleteIssue(ctx, a.ID) }},
	}
	for _, change := range changes {
		t.Run(change.name, func(t *testing.T) {
			before, err := s.GetProject(ctx, p.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err := change.apply(); err != nil {
				t.Fatal(err)
			}
			after, err := s.GetProject(ctx, p.ID)
			if err != nil {
				t.Fatal(err)
			}
			if after.GovernanceGeneration <= before.GovernanceGeneration {
				t.Fatalf("generation did not advance: %d -> %d",
					before.GovernanceGeneration,
					after.GovernanceGeneration)
			}
		})
	}
}

func TestPinChangesInvalidateLayeredDiamondDescendants(t *testing.T) {
	s, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()
	p := setupTestProject(t, s)
	ids := setupGovernanceGraph(t, s, p, [][2]string{
		{"task", "epic"}, {"task", "middle"}, {"epic", "milestone"}, {"middle", "milestone"},
	}, []string{"epic"})
	unrelated := setupTestIssue(t, s, p, "unrelated")
	seedGovernanceRevisions(t, s.DB(), p.ID)
	for _, revision := range []any{int64(1), int64(2), nil} {
		before := governanceVersions(t, s, p.ID)
		var planID any = "version-plan"
		if revision == nil {
			planID = nil
		}
		if _, err := s.DB().Exec(`UPDATE issues SET governing_plan_id=?,governing_plan_revision=? WHERE id=?`,
			planID,
			revision,
			ids["milestone"]); err != nil {
			t.Fatal(err)
		}
		after := governanceVersions(t, s, p.ID)
		assertPinInvalidation(t, before, after, ids, unrelated.ID)
	}
	before := governanceVersions(t, s, p.ID)
	if err := s.AddLabelToIssue(ctx, ids["task"], "display", "actor"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddComment(ctx, ids["task"], "actor", "discussion"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`UPDATE issues SET rank=4 WHERE id=?`, ids["task"]); err != nil {
		t.Fatal(err)
	}
	if after := governanceVersions(t, s, p.ID); !reflect.DeepEqual(before, after) {
		t.Fatalf("display invalidated governance: %+v -> %+v", before, after)
	}
	// A failed pin transaction must also undo descendant versions, generation and history.
	beforeDB := governanceDatabaseSnapshot(t, s.DB())
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback() //nolint:errcheck // Also release the transaction if an assertion aborts the test.
	if _, err := tx.Exec(`UPDATE issues SET governing_plan_id='version-plan',governing_plan_revision=2 WHERE id=?`,
		ids["milestone"]); err != nil {
		t.Fatal(err)
	}
	var stagedVersion int64
	if err := tx.QueryRow(`SELECT contract_version FROM issues WHERE id=?`,
		ids["task"]).Scan(&stagedVersion); err != nil {
		t.Fatal(err)
	}
	if stagedVersion != before.versions[ids["task"]]+1 {
		t.Fatal("pin did not invalidate inside transaction")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if afterDB := governanceDatabaseSnapshot(t, s.DB()); beforeDB != afterDB {
		t.Fatal("rolled-back pin left invalidation or history")
	}
}

type governanceVersionSnapshot struct {
	generation int64
	versions   map[string]int64
}

func governanceVersions(t *testing.T, s *sqlite.Store, projectID string) governanceVersionSnapshot {
	t.Helper()
	ctx := context.Background()
	p, err := s.GetProject(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	issues, err := s.ListIssues(ctx, types.IssueFilter{ProjectID: projectID})
	if err != nil {
		t.Fatal(err)
	}
	result := governanceVersionSnapshot{generation: p.GovernanceGeneration, versions: map[string]int64{}}
	for _, issue := range issues {
		result.versions[issue.ID] = issue.ContractVersion
	}
	return result
}

func assertPinInvalidation(t *testing.T,
	before,
	after governanceVersionSnapshot,
	ids map[string]string,
	unrelatedID string,
) {
	t.Helper()
	if after.generation != before.generation+1 {
		t.Fatalf("pin generation: %d -> %d", before.generation, after.generation)
	}
	for _, id := range ids {
		if after.versions[id] != before.versions[id]+1 {
			t.Fatalf("pin did not invalidate %s exactly once: %v -> %v", id, before.versions, after.versions)
		}
	}
	if after.versions[unrelatedID] != before.versions[unrelatedID] {
		t.Fatal("unrelated contract invalidated")
	}
}
