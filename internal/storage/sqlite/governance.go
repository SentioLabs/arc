package sqlite

import (
	"context"
	"fmt"
	"reflect"

	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/types"
)

// governanceGraph contains the project snapshot used for both resolution and
// mutation comparisons. Parent references outside issues remain visible so
// corrupted cross-project ancestry is rejected, never treated as an unlinked root.
type governanceGraph struct {
	issues  map[string]*types.Issue
	parents map[string][]string
}

// governanceGraph reads both node and edge sets from one transaction snapshot.
// Exhaust the first cursor before opening the second on SQLite's sole connection.
func (s *issueMutationTx) governanceGraph(ctx context.Context, projectID string) (*governanceGraph, error) {
	rows, err := s.tx.QueryContext(ctx,
		`SELECT id,issue_type,governing_plan_id,governing_plan_revision FROM issues WHERE project_id=?`, projectID,
	)
	if err != nil {
		return nil, err
	}
	g := &governanceGraph{issues: map[string]*types.Issue{}, parents: map[string][]string{}}
	for rows.Next() {
		var id, typ string
		var planID *string
		var revision *int64
		if err := rows.Scan(&id, &typ, &planID, &revision); err != nil {
			_ = rows.Close()
			return nil, err
		}
		i := &types.Issue{ID: id, ProjectID: projectID, IssueType: types.IssueType(typ)}
		if planID != nil && revision != nil {
			i.GoverningPlan = &types.PlanReference{PlanID: *planID, Revision: *revision}
		}
		g.issues[id] = i
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	rows, err = s.tx.QueryContext(ctx, `
 SELECT d.issue_id,d.depends_on_id FROM dependencies d
 JOIN issues i ON i.id=d.issue_id WHERE i.project_id=? AND d.type='parent-child'`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var child, parent string
		if err := rows.Scan(&child, &parent); err != nil {
			return nil, err
		}
		g.parents[child] = append(g.parents[child], parent)
	}
	return g, rows.Err()
}

// resolve traverses all ancestry before selecting pins. A shortcut or repeated
// path cannot hide a cycle or turn a comparable chain into an ambiguity.
func (g *governanceGraph) resolve(id string) (*types.GoverningPlan, error) {
	if g.issues[id] == nil {
		return nil, fmt.Errorf("%w: %s", storage.ErrIssueNotFound, id)
	}
	ancestry := &governanceAncestry{state: map[string]int{}, ancestors: map[string]map[string]bool{}}
	if err := g.visitAncestry(id, ancestry); err != nil {
		return nil, err
	}
	ancestors, order := ancestry.ancestors, ancestry.order

	var chain []types.GoverningPlanContext
	for _, node := range order {
		issue := g.issues[node]
		if issue.GoverningPlan == nil {
			continue
		}
		if issue.IssueType != types.TypeEpic && issue.IssueType != types.TypeMilestone {
			return nil, fmt.Errorf("%w: unsupported pinned container %s", storage.ErrAmbiguousGovernance, node)
		}
		for _, c := range chain {
			if !ancestors[node][c.ContainerID] && !ancestors[c.ContainerID][node] {
				return nil, storage.ErrAmbiguousGovernance
			}
		}
		chain = append(chain, types.GoverningPlanContext{
			ContainerID: node, ContainerType: issue.IssueType, Reference: *issue.GoverningPlan,
		})
	}
	if len(chain) == 0 {
		return nil, nil //nolint:nilnil // The shared resolver contract uses nil for valid unlinked work.
	}
	// DFS postorder puts ancestors first. Comparability above makes that
	// topological order the single permitted outermost-to-primary chain.
	primary := chain[len(chain)-1]
	return &types.GoverningPlan{
		ContainerID: primary.ContainerID, ContainerType: primary.ContainerType,
		Reference: primary.Reference, Context: chain[:len(chain)-1],
	}, nil
}

// resolveGoverningPlan reuses the current mutation's snapshot without pool reentry.
func (s *issueMutationTx) resolveGoverningPlan(
	ctx context.Context, projectID, issueID string,
) (*types.GoverningPlan, error) {
	g, err := s.governanceGraph(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return g.resolve(issueID)
}

// ResolveGoverningPlan returns pinned ancestry, never the mutable plan head.
func (s *Store) ResolveGoverningPlan(ctx context.Context, projectID, issueID string) (*types.GoverningPlan, error) {
	var result *types.GoverningPlan
	err := s.withIssueMutation(ctx, func(m *issueMutationTx) error {
		var err error
		result, err = m.resolveGoverningPlan(ctx, projectID, issueID)
		return err
	})
	return result, err
}

// descendants includes the root and deduplicates diamond paths. The visited set
// makes affected-set collection terminate even for malformed legacy cycles.
func (g *governanceGraph) descendants(root string) map[string]bool {
	result := map[string]bool{root: true}
	changed := true
	for changed {
		changed = false
		for child, parents := range g.parents {
			if result[child] {
				continue
			}
			for _, parent := range parents {
				if result[parent] {
					result[child] = true
					changed = true
					break
				}
			}
		}
	}
	return result
}

// guardGovernanceMutation compares the affected subgraph in the same write
// transaction. Adoption composes the low-level helpers after its own validation.
func (s *issueMutationTx) guardGovernanceMutation(
	ctx context.Context, projectID, root string, apply func() error,
) error {
	before, err := s.governanceGraph(ctx, projectID)
	if err != nil {
		return err
	}
	if err := apply(); err != nil {
		return err
	}
	after, err := s.governanceGraph(ctx, projectID)
	if err != nil {
		return err
	}
	// A node can enter or leave the affected set when edges are added or removed.
	// Comparing the union prevents reparenting through an unpinned intermediary
	// from silently removing a higher-level required design.
	affected := before.descendants(root)
	for id := range after.descendants(root) {
		affected[id] = true
	}
	for id := range affected {
		if before.issues[id] == nil || after.issues[id] == nil {
			continue
		}
		old, err := before.resolve(id)
		if err != nil {
			return fmt.Errorf("%w: %w", storage.ErrGovernanceReconciliation, err)
		}
		next, err := after.resolve(id)
		if err != nil {
			return fmt.Errorf("%w: %w", storage.ErrGovernanceReconciliation, err)
		}
		if !reflect.DeepEqual(old, next) {
			return fmt.Errorf("%w: %s", storage.ErrGovernanceReconciliation, id)
		}
	}
	return nil
}

// governanceAncestry stores DFS colors and transitive reachability. Visiting a
// gray node means a cycle; black nodes safely share already-computed ancestry.
// Reachability, rather than distance or ID prefixes, establishes comparability.
type governanceAncestry struct {
	state     map[string]int
	ancestors map[string]map[string]bool
	order     []string
}

const (
	governanceVisiting = 1
	governanceVisited  = 2
)

// visitAncestry walks past pinned nodes too: a nearby pin cannot mask a cycle
// or additional required context farther up the graph. Each node is visited once.
func (g *governanceGraph) visitAncestry(node string, walk *governanceAncestry) error {
	switch walk.state[node] {
	case governanceVisiting:
		return storage.ErrGovernanceCycle
	case governanceVisited:
		return nil
	}
	if g.issues[node] == nil {
		return fmt.Errorf("%w: cross-project ancestor %s", storage.ErrIssueNotFound, node)
	}
	walk.state[node] = governanceVisiting
	walk.ancestors[node] = map[string]bool{}
	for _, parent := range g.parents[node] {
		if err := g.visitAncestry(parent, walk); err != nil {
			return err
		}
		walk.ancestors[node][parent] = true
		for ancestor := range walk.ancestors[parent] {
			walk.ancestors[node][ancestor] = true
		}
	}
	walk.state[node] = governanceVisited
	walk.order = append(walk.order, node)
	return nil
}
