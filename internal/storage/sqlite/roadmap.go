package sqlite

import (
	"context"
	"fmt"
	"sort"

	"github.com/sentiolabs/arc/internal/storage/sqlite/db"
	"github.com/sentiolabs/arc/internal/types"
)

// GetRoadmap returns the container tree (releases, milestones, epics) for a
// project with progress counts and gating info. Tree assembly happens in Go
// from two plain-SQL passes rather than a recursive query, since the counting
// and gating rules are easier to express as a depth-capped DFS.
func (s *Store) GetRoadmap(ctx context.Context, projectID string) ([]*types.RoadmapNode, error) {
	issuesByID, err := s.loadRoadmapIssues(ctx, projectID)
	if err != nil {
		return nil, err
	}

	parentOf, childrenOf, blocksOf, err := s.loadRoadmapEdges(ctx, projectID, issuesByID)
	if err != nil {
		return nil, err
	}

	containerChildrenOf := map[string][]string{}
	var rootIDs []string
	for id, issue := range issuesByID {
		if !issue.IssueType.IsContainer() {
			continue
		}
		if parentID, ok := parentOf[id]; ok {
			if parent, ok := issuesByID[parentID]; ok && parent.IssueType.IsContainer() {
				containerChildrenOf[parentID] = append(containerChildrenOf[parentID], id)
				continue
			}
		}
		rootIDs = append(rootIDs, id)
	}
	sortRoadmapIDs(rootIDs, issuesByID)

	nodes := make([]*types.RoadmapNode, 0, len(rootIDs))
	for _, id := range rootIDs {
		nodes = append(nodes, buildRoadmapNode(id, issuesByID, childrenOf, containerChildrenOf, blocksOf))
	}
	return nodes, nil
}

// loadRoadmapIssues loads every issue in a project keyed by ID.
func (s *Store) loadRoadmapIssues(ctx context.Context, projectID string) (map[string]*types.Issue, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, project_id, title, description, status, priority, issue_type,
       ai_session_id, external_ref, rank, created_at, updated_at, closed_at, close_reason
FROM issues WHERE project_id = ?`, projectID)
	if err != nil {
		return nil, fmt.Errorf("load roadmap issues: %w", err)
	}
	defer rows.Close()

	issuesByID := map[string]*types.Issue{}
	for rows.Next() {
		var row db.Issue
		if err := rows.Scan(
			&row.ID, &row.ProjectID, &row.Title, &row.Description,
			&row.Status, &row.Priority, &row.IssueType,
			&row.AiSessionID, &row.ExternalRef, &row.Rank,
			&row.CreatedAt, &row.UpdatedAt, &row.ClosedAt, &row.CloseReason,
		); err != nil {
			return nil, fmt.Errorf("scan roadmap issue: %w", err)
		}
		issuesByID[row.ID] = dbIssueToType(&row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("roadmap issue rows: %w", err)
	}
	return issuesByID, nil
}

// loadRoadmapEdges loads a project's parent-child and blocks dependencies in
// one query, returning:
//   - parentOf: issue ID -> parent-child parent ID
//   - childrenOf: parent-child parent ID -> child issue IDs (any type)
//   - blocksOf: issue ID -> open (non-closed) blocker IDs
func (s *Store) loadRoadmapEdges(ctx context.Context, projectID string, issuesByID map[string]*types.Issue) (
	parentOf map[string]string, childrenOf, blocksOf map[string][]string, err error,
) {
	rows, err := s.db.QueryContext(ctx, `
SELECT d.issue_id, d.depends_on_id, d.type FROM dependencies d
JOIN issues i ON i.id = d.issue_id WHERE i.project_id = ?`, projectID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load roadmap dependencies: %w", err)
	}
	defer rows.Close()

	parentOf = map[string]string{}
	childrenOf = map[string][]string{}
	blocksOf = map[string][]string{}
	for rows.Next() {
		var issueID, dependsOnID, depType string
		if err := rows.Scan(&issueID, &dependsOnID, &depType); err != nil {
			return nil, nil, nil, fmt.Errorf("scan roadmap dependency: %w", err)
		}
		switch types.DependencyType(depType) {
		case types.DepParentChild:
			parentOf[issueID] = dependsOnID
			childrenOf[dependsOnID] = append(childrenOf[dependsOnID], issueID)
		case types.DepBlocks:
			if blocker, ok := issuesByID[dependsOnID]; ok && blocker.Status != types.StatusClosed {
				blocksOf[issueID] = append(blocksOf[issueID], dependsOnID)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, fmt.Errorf("roadmap dependency rows: %w", err)
	}
	return parentOf, childrenOf, blocksOf, nil
}

// buildRoadmapNode assembles one container node and, recursively, its
// container children.
func buildRoadmapNode(
	id string,
	issuesByID map[string]*types.Issue,
	childrenOf, containerChildrenOf map[string][]string,
	blocksOf map[string][]string,
) *types.RoadmapNode {
	total, closed := countRoadmapDescendants(id, childrenOf, issuesByID)

	childIDs := containerChildrenOf[id]
	sortRoadmapIDs(childIDs, issuesByID)

	node := &types.RoadmapNode{
		Issue:       *issuesByID[id],
		TotalCount:  total,
		ClosedCount: closed,
		GatedBy:     blocksOf[id],
	}
	for _, childID := range childIDs {
		child := buildRoadmapNode(childID, issuesByID, childrenOf, containerChildrenOf, blocksOf)
		node.Children = append(node.Children, child)
	}
	return node
}

// countRoadmapDescendants walks a container's full descendant tree (through
// containers and non-containers alike, since a container's counts roll up
// its sub-containers' descendants) and tallies the non-container ones.
// The walk is depth-capped and tracks visited IDs so cyclic edges terminate.
func countRoadmapDescendants(
	id string, childrenOf map[string][]string, issuesByID map[string]*types.Issue,
) (total, closed int) {
	visited := map[string]bool{}
	var walk func(nodeID string, depth int)
	walk = func(nodeID string, depth int) {
		if depth > maxAncestorDepth || visited[nodeID] {
			return
		}
		visited[nodeID] = true
		for _, childID := range childrenOf[nodeID] {
			child, ok := issuesByID[childID]
			if !ok {
				continue
			}
			if !child.IssueType.IsContainer() {
				total++
				if child.Status == types.StatusClosed {
					closed++
				}
			}
			walk(childID, depth+1)
		}
	}
	walk(id, 0)
	return total, closed
}

// sortRoadmapIDs orders container IDs for display: releases first, then
// priority ascending, then created_at ascending.
func sortRoadmapIDs(ids []string, issuesByID map[string]*types.Issue) {
	sort.SliceStable(ids, func(i, j int) bool {
		a, b := issuesByID[ids[i]], issuesByID[ids[j]]
		aRelease := a.IssueType == types.TypeRelease
		bRelease := b.IssueType == types.TypeRelease
		if aRelease != bRelease {
			return aRelease
		}
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		return a.CreatedAt.Before(b.CreatedAt)
	})
}
