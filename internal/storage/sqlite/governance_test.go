package sqlite_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/storage/sqlite"
	"github.com/sentiolabs/arc/internal/types"
)

func TestLayeredGovernanceGraphs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		edges   [][2]string
		pins    []string
		want    []string
		wantErr error
	}{
		{name: "unlinked", edges: [][2]string{{"task", "epic"}}},
		{
			name:  "milestone",
			edges: [][2]string{{"task", "epic"}, {"epic", "milestone"}},
			pins:  []string{"milestone"},
			want:  []string{"milestone"},
		},
		{
			name:  "tactical",
			edges: [][2]string{{"task", "epic"}, {"epic", "milestone"}},
			pins:  []string{"milestone", "epic"},
			want:  []string{"milestone", "epic"},
		},
		{
			name:  "unpinned intermediate",
			edges: [][2]string{{"task", "middle"}, {"middle", "epic"}, {"epic", "milestone"}},
			pins:  []string{"milestone", "epic"},
			want:  []string{"milestone", "epic"},
		},
		{
			name:  "diamond",
			edges: [][2]string{{"task", "epic"}, {"task", "middle"}, {"epic", "milestone"}, {"middle", "milestone"}},
			pins:  []string{"milestone"},
			want:  []string{"milestone"},
		},
		{
			name:  "shortcut",
			edges: [][2]string{{"task", "epic"}, {"epic", "milestone"}, {"task", "milestone"}},
			pins:  []string{"milestone", "epic"},
			want:  []string{"milestone", "epic"},
		},
		{
			name:    "incomparable identical pins",
			edges:   [][2]string{{"task", "epic"}, {"task", "middle"}},
			pins:    []string{"epic", "middle"},
			wantErr: storage.ErrAmbiguousGovernance,
		},
		{
			name: "cycle beyond pin",
			edges: [][2]string{
				{"task", "epic"}, {"epic", "middle"}, {"middle", "milestone"}, {"milestone", "middle"},
			},
			pins:    []string{"epic"},
			wantErr: storage.ErrGovernanceCycle,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, cleanup := setupTestStore(t)
			defer cleanup()
			ctx := context.Background()
			p := setupTestProject(t, s)
			ids := setupGovernanceGraph(t, s, p, tc.edges, tc.pins)
			got, err := s.ResolveGoverningPlan(ctx, p.ID, ids["task"])
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("error=%v want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			assertGovernanceChain(t, got, ids, tc.want)
			if _, err := s.ResolveGoverningPlan(ctx, p.ID, "missing"); !errors.Is(err, storage.ErrIssueNotFound) {
				t.Fatalf("missing error=%v", err)
			}
			if _, err := s.ResolveGoverningPlan(ctx,
				"other-project",
				ids["task"]); !errors.Is(err,
				storage.ErrIssueNotFound) {
				t.Fatalf("project error=%v", err)
			}
		})
	}
}

func TestGovernanceMutationGuardsAndRetention(t *testing.T) {
	s, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()
	p := setupTestProject(t, s)
	epic := &types.Issue{ProjectID: p.ID, Title: "pinned", IssueType: types.TypeEpic}
	if err := s.CreateIssue(ctx, epic, "actor"); err != nil {
		t.Fatal(err)
	}
	child := &types.Issue{ProjectID: p.ID, Title: "child", ParentID: epic.ID}
	if err := s.CreateIssue(ctx, child, "actor"); err != nil {
		t.Fatal(err)
	}
	outsider := setupTestIssue(t, s, p, "outsider")
	for _, query := range []string{
		`INSERT INTO plans(id,project_id,title,head_revision,created_at,updated_at)
 VALUES ('guard-plan','` + p.ID + `','Fixture',1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
		`
INSERT INTO
plan_revisions(plan_id,revision,content_path,content_sha256,content_bytes,review_status,created_at)
VALUES ('guard-plan',1,'guard-fixture','hash',0,'approved',CURRENT_TIMESTAMP)
`,
		`UPDATE issues SET governing_plan_id='guard-plan',governing_plan_revision=1 WHERE id='` + epic.ID + `'`,
	} {
		if _, err := s.DB().Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	for _, apply := range []func() error{
		func() error {
			return s.AddDependency(ctx,
				&types.Dependency{IssueID: outsider.ID, DependsOnID: epic.ID, Type: types.DepParentChild},
				"actor")
		},
		func() error { return s.RemoveDependency(ctx, child.ID, epic.ID, "actor") },
		func() error {
			return s.AddDependency(ctx,
				&types.Dependency{IssueID: child.ID, DependsOnID: epic.ID, Type: types.DepBlocks},
				"actor")
		},
		func() error { return s.UpdateIssue(ctx, epic.ID, map[string]any{"issue_type": "task"}, "actor") },
		func() error { return s.DeleteIssue(ctx, epic.ID) },
	} {
		before, _ := s.GetProject(ctx, p.ID)
		version, _ := s.GetIssue(ctx, child.ID)
		if err := apply(); err == nil {
			t.Fatal("governance bypass accepted")
		}
		after, _ := s.GetProject(ctx, p.ID)
		next, _ := s.GetIssue(ctx, child.ID)
		if before.GovernanceGeneration != after.GovernanceGeneration ||
			version.ContractVersion != next.ContractVersion {
			t.Fatal("failed mutation changed versions")
		}
	}
	got, err := s.ResolveGoverningPlan(ctx, p.ID, epic.ID)
	if err != nil || got == nil || got.ContainerID != epic.ID {
		t.Fatalf("container self pin: %v %v", got, err)
	}
	if _, err := s.DB().Exec(`
UPDATE issues SET governing_plan_id=NULL,governing_plan_revision=NULL WHERE id=?
`, epic.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteIssue(ctx, epic.ID); !errors.Is(err, storage.ErrGovernanceReconciliation) {
		t.Fatalf("detached history rejection: %v", err)
	}
	target := &types.Project{ID: "retention-target", Name: "retention-target", Prefix: "rt"}
	if err := s.CreateProject(ctx, target); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MergeProjects(ctx, target.ID, []string{p.ID}, "actor"); err == nil {
		t.Fatal("detached provenance moved")
	}
	if err := s.DeleteProject(ctx, p.ID); err == nil {
		t.Fatal("retained project deleted")
	}
}

func TestCrossProjectGovernanceAndMalformedInspection(t *testing.T) {
	s, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()
	p := setupTestProject(t, s)
	q := &types.Project{ID: "foreign", Name: "foreign", Prefix: "fr"}
	if err := s.CreateProject(ctx, q); err != nil {
		t.Fatal(err)
	}
	child := setupTestIssue(t, s, p, "child")
	parent := setupTestIssue(t, s, q, "parent")
	if err := s.AddDependency(ctx,
		&types.Dependency{IssueID: child.ID, DependsOnID: parent.ID, Type: types.DepParentChild},
		"actor"); err == nil {
		t.Fatal("cross-project parent accepted")
	}
	if _, err := s.DB().Exec(`
INSERT INTO dependencies(issue_id,depends_on_id,type) VALUES (?,?,'parent-child')
`, child.ID, parent.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ResolveGoverningPlan(ctx, p.ID, child.ID); !errors.Is(err, storage.ErrIssueNotFound) {
		t.Fatalf("cross-project resolution error=%v", err)
	}
	if _, err := s.GetIssue(ctx, child.ID); err != nil {
		t.Fatalf("legacy graph not inspectable: %v", err)
	}
	if _, err := s.GetDependencies(ctx, child.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetIssueDetails(ctx, child.ID); !errors.Is(err, storage.ErrIssueNotFound) {
		t.Fatalf("detail falsely resolved malformed graph as unlinked: %v", err)
	}
	if _, err := s.MergeProjects(ctx,
		q.ID,
		[]string{p.ID},
		"actor"); err == nil {
		t.Fatal("merge silently repaired cross-project ancestry")
	}
}

func setupGovernanceGraph(t *testing.T,
	s *sqlite.Store,
	p *types.Project,
	edges [][2]string,
	pins []string,
) map[string]string {
	t.Helper()
	ctx := context.Background()
	ids := map[string]string{}
	for _, name := range []string{"task", "epic", "milestone", "middle"} {
		typ := types.TypeEpic
		if name == "task" {
			typ = types.TypeTask
		}
		if name == "milestone" {
			typ = types.TypeMilestone
		}
		i := &types.Issue{ID: "nonhierarchical-" + name, ProjectID: p.ID, Title: name, IssueType: typ}
		if err := s.CreateIssue(ctx, i, "actor"); err != nil {
			t.Fatal(err)
		}
		ids[name] = i.ID
	}
	// Import legacy graph fixtures directly: ordinary mutations must reject governance bypasses.
	for _, edge := range edges {
		if _, err := s.DB().Exec(`
INSERT INTO dependencies(issue_id,depends_on_id,type) VALUES (?,?,'parent-child')
`, ids[edge[0]], ids[edge[1]]); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.DB().Exec(`
INSERT INTO plans(id,project_id,title,head_revision,created_at,updated_at) VALUES
('plan-fixture',?,'Fixture',2,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
`, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`
INSERT INTO
plan_revisions(plan_id,revision,content_path,content_sha256,content_bytes,review_status,created_at)
VALUES ('plan-fixture',1,'fixture.md','hash',0,'approved',CURRENT_TIMESTAMP)
`); err != nil {
		t.Fatal(err)
	}
	for _, name := range pins {
		if _, err := s.DB().Exec(`
UPDATE issues SET governing_plan_id='plan-fixture',governing_plan_revision=1 WHERE id=?
`, ids[name]); err != nil {
			t.Fatal(err)
		}
	}
	return ids
}

func assertGovernanceChain(t *testing.T, got *types.GoverningPlan, ids map[string]string, names []string) {
	t.Helper()
	var actual []string
	if got != nil {
		for _, c := range got.Context {
			actual = append(actual, c.ContainerID)
			if c.Reference.Revision != 1 {
				t.Fatal("substituted head")
			}
		}
		actual = append(actual, got.ContainerID)
		if got.Reference.Revision != 1 {
			t.Fatal("substituted head")
		}
	}
	want := make([]string, 0, len(names))
	for _, name := range names {
		want = append(want, ids[name])
	}
	if len(actual) != len(want) || (len(want) > 0 && !reflect.DeepEqual(actual, want)) {
		t.Fatalf("chain=%v want %v", actual, want)
	}
}
