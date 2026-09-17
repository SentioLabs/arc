package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/storage/sqlite/db"
	"github.com/sentiolabs/arc/internal/types"
)

const (
	adoptionUpdated    = "updated"
	adoptionCompatible = "compatible"
	adoptionFollowUp   = "follow_up"
	adoptionUnchanged  = "unchanged"
)

type adoptionProposal struct {
	before, after *governanceGraph
	result        *storage.PlanAdoptionResult
	affected      map[string]bool
	pins          map[string]types.ReconciledContainerPin
}

// AdoptPlan validates a complete staged graph under the same writer lock as
// claims and completion. Dry runs only build an in-memory proposal.
func (s *Store) AdoptPlan(
	ctx context.Context,
	projectID, rootID, key string,
	req types.PlanAdoptionRequest,
) (*storage.PlanAdoptionResult, error) {
	if !req.DryRun && strings.TrimSpace(key) == "" {
		return nil, storage.ErrPlanPrecondition
	}
	fingerprint, err := planFingerprint(req)
	if err != nil {
		return nil, err
	}
	var result *storage.PlanAdoptionResult
	err = s.withIssueMutation(ctx, func(m *issueMutationTx) error {
		if !req.DryRun {
			replay, err := m.lookupAdoption(ctx, projectID, rootID, key, fingerprint)
			if err != nil {
				return err
			}
			if replay != nil {
				result = replay
				return nil
			}
		}
		proposal, err := s.captureAdoption(ctx, m, projectID, rootID, req)
		if err != nil {
			return err
		}
		result = proposal.result
		if req.DryRun {
			return nil
		}
		if len(result.Errors) > 0 {
			return fmt.Errorf(
				"%w: %s (%s)",
				storage.ErrPlanConflict,
				result.Errors[0].Message,
				result.Errors[0].IssueID,
			)
		}
		if err := m.applyAdoption(ctx, req, proposal); err != nil {
			return err
		}
		return m.recordAdoption(ctx, req, result, key, fingerprint)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// lookupAdoption precedes all mutable versions, statuses and lifecycle checks.
// A matching committed fingerprint returns the original immutable response;
// payload reuse with a different fingerprint remains a conflict forever.
func (m *issueMutationTx) lookupAdoption(
	ctx context.Context,
	p, root, key, fingerprint string,
) (*storage.PlanAdoptionResult, error) {
	var stored, body string
	err := m.tx.QueryRowContext(ctx, `
SELECT k.fingerprint,a.result FROM adoption_idempotency k JOIN plan_adoptions a ON
a.id=k.adoption_id WHERE k.project_id=? AND k.container_id=? AND k.key=?
`, p, root, key).
		Scan(&stored, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // No committed replay exists.
	}
	if err != nil {
		return nil, err
	}
	if stored != fingerprint {
		return nil, storage.ErrPlanConflict
	}
	var result storage.PlanAdoptionResult
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		return nil, err
	}
	result.Replay = true
	return &result, nil
}

// adoptionGraph materializes a complete issue snapshot on the writer connection.
// Contract versions and statuses are read together with ancestry, preventing
// claims or ordinary edits from entering between validation and application.
func (m *issueMutationTx) adoptionGraph(ctx context.Context, p string) (*governanceGraph, error) {
	g, err := m.governanceGraph(ctx, p)
	if err != nil {
		return nil, err
	}
	for id := range g.issues {
		i, err := m.GetIssue(ctx, id)
		if err != nil {
			return nil, err
		}
		g.issues[id] = i
	}
	return g, nil
}

// cloneAdoptionGraph separates proposed node/edge changes from the captured graph.
// Reconciliation never writes temporary edits to live rows during dry-run.
// Pin references are replaced as values rather than mutated through pointers.
func cloneAdoptionGraph(g *governanceGraph) *governanceGraph {
	n := &governanceGraph{issues: map[string]*types.Issue{}, parents: map[string][]string{}}
	for id, i := range g.issues {
		copyIssue := *i
		n.issues[id] = &copyIssue
	}
	for id, parents := range g.parents {
		n.parents[id] = slices.Clone(parents)
	}
	return n
}

// snapshotGovernance retains the full ordered chain used at an audit boundary.
// A malformed chain is an error, never an implicit unlinked expectation.
// The issue status and version belong to the same transaction snapshot.
func snapshotGovernance(g *governanceGraph, id string) (storage.GovernanceSnapshot, error) {
	i := g.issues[id]
	if i == nil {
		return storage.GovernanceSnapshot{}, storage.ErrIssueNotFound
	}
	resolved, err := g.resolve(id)
	if err != nil {
		return storage.GovernanceSnapshot{}, err
	}
	return storage.GovernanceSnapshot{
		IssueID:  id,
		Status:   i.Status,
		Expected: types.ExpectedGovernance{Governing: resolved, ContractVersion: i.ContractVersion},
	}, nil
}

// coverage collects reviewable missing or stale preconditions for dry-run.
// Apply rejects the complete proposal if this collection is nonempty.
// Structural errors are returned separately because no valid graph exists.
func (p *adoptionProposal) coverage(id, code, message string) {
	p.result.Errors = append(
		p.result.Errors,
		storage.AdoptionCoverageError{IssueID: id, Code: code, Message: message},
	)
}

// captureAdoption binds the root, tactical pins and affected contracts before
// any counters or IDs are allocated. Every source and target pin is checked
// against durable approval metadata and the retained content publisher.
func (s *Store) captureAdoption(
	ctx context.Context,
	m *issueMutationTx,
	projectID, rootID string,
	req types.PlanAdoptionRequest,
) (*adoptionProposal, error) {
	before, err := m.adoptionGraph(ctx, projectID)
	if err != nil {
		return nil, err
	}
	root := before.issues[rootID]
	if root == nil {
		return nil, storage.ErrIssueNotFound
	}
	if root.IssueType != types.TypeEpic && root.IssueType != types.TypeMilestone {
		return nil, fmt.Errorf(
			"%w: adoption root must be epic or milestone",
			storage.ErrPlanInvalid,
		)
	}
	if req.ExpectedContainerVersion < 1 || req.ExpectedGovernanceGeneration < 0 {
		return nil, storage.ErrPlanPrecondition
	}
	project, err := m.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	p := &adoptionProposal{
		before:   before,
		after:    cloneAdoptionGraph(before),
		affected: map[string]bool{},
		pins:     map[string]types.ReconciledContainerPin{},
		result: &storage.PlanAdoptionResult{
			ProjectID:            projectID,
			ContainerID:          rootID,
			GovernanceGeneration: project.GovernanceGeneration,
			Tasks:                []storage.ReconciliationChange{},
			ContainerPins:        req.ContainerPins,
			FollowUpIDs:          map[string]string{},
			Errors:               []storage.AdoptionCoverageError{},
			DryRun:               req.DryRun,
		},
	}
	if root.ContractVersion != req.ExpectedContainerVersion ||
		project.GovernanceGeneration != req.ExpectedGovernanceGeneration ||
		!reflect.DeepEqual(root.GoverningPlan, req.ExpectedPin) {
		p.coverage(
			rootID,
			"stale_container",
			"container pin, version or governance generation changed",
		)
	}
	p.pins[rootID] = types.ReconciledContainerPin{
		ContainerID:              rootID,
		ExpectedContainerVersion: req.ExpectedContainerVersion,
		ExpectedPin:              req.ExpectedPin,
		TargetPin:                req.TargetPin,
		Disposition:              adoptionUpdated,
	}
	if err := p.stageContainerPins(req); err != nil {
		return nil, err
	}
	for id, pin := range p.pins {
		if err := s.validateAdoptionPin(ctx, m, projectID, pin.ExpectedPin, false); err != nil {
			return nil, err
		}
		if err := s.validateAdoptionPin(ctx, m, projectID, pin.TargetPin, true); err != nil {
			return nil, err
		}
		p.after.issues[id].GoverningPlan = pin.TargetPin
	}
	if err := p.stageAdoption(ctx, m, req); err != nil {
		return nil, err
	}
	if err := p.validateAdoptionCoverage(req); err != nil {
		return nil, err
	}
	if err := p.captureContainerSnapshots(req); err != nil {
		return nil, err
	}

	if err := s.validateProposalReferences(ctx, m, p); err != nil {
		return nil, err
	}
	return p, nil
}

// validateAdoptionPin never substitutes the current plan head for a requested pin.
// Historical approved source pins can be detached after archival, while any
// adopted target must remain active, approved, intact and project-owned.
func (s *Store) validateAdoptionPin(
	ctx context.Context,
	m *issueMutationTx,
	p string,
	pin *types.PlanReference,
	target bool,
) error {
	if pin == nil {
		return nil
	}
	if pin.PlanID == "" || pin.Revision < 1 {
		return storage.ErrPlanInvalid
	}
	plan, err := getDurablePlan(ctx, m.queries, p, pin.PlanID)
	if err != nil {
		return err
	}
	row, err := getRevision(ctx, m.queries, pin.PlanID, pin.Revision)
	if err != nil {
		return err
	}
	if row.ReviewStatus != types.PlanStatusApproved || (target && plan.Lifecycle != "active") {
		return fmt.Errorf("%w: pin must name an active approved revision", storage.ErrPlanConflict)
	}
	if s.planFiles == nil {
		return errors.New("plan publisher unavailable")
	}
	_, err = s.planFiles.Read(ctx, revisionBlob(row))
	return err
}

// stageAdoption builds the complete final graph before evaluating coverage.
// Types, follow-up identities and edges are staged in memory and then checked
// for both governance ambiguity and hierarchy/execution dependency cycles.
func (p *adoptionProposal) stageAdoption(
	ctx context.Context,
	m *issueMutationTx,
	req types.PlanAdoptionRequest,
) error {
	if err := p.stageTypes(req); err != nil {
		return err
	}
	if err := p.validateContainerEligibility(req); err != nil {
		return err
	}
	if err := p.stageFollowUps(req); err != nil {
		return err
	}
	edges, err := m.adoptionEdges(ctx, p.result.ProjectID)
	if err != nil {
		return err
	}
	for _, f := range req.FollowUps {
		edges[[2]string{"new:" + f.Key, f.ParentID}] = types.DepParentChild
	}
	if err := p.stageEdges(req, edges); err != nil {
		return err
	}
	return p.validateStagedCycles(edges)
}

// adoptionEdges reads existing same-project edges for exact staged validation.
// Legacy cross-project blocking dependencies are deliberately left untouched;
// a proposal referencing such a target is rejected by local node lookup.
func (m *issueMutationTx) adoptionEdges(
	ctx context.Context,
	p string,
) (map[[2]string]types.DependencyType, error) {
	rows, err := m.tx.QueryContext(
		ctx,
		`
SELECT d.issue_id,d.depends_on_id,d.type FROM dependencies d JOIN issues i ON i.id=d.issue_id
JOIN issues j ON j.id=d.depends_on_id WHERE i.project_id=? AND j.project_id=?
`,
		p,
		p,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[[2]string]types.DependencyType{}
	for rows.Next() {
		var child, parent, typ string
		if err := rows.Scan(&child, &parent, &typ); err != nil {
			return nil, err
		}
		result[[2]string{child, parent}] = types.DependencyType(typ)
	}
	return result, rows.Err()
}

// validateAdoptionCoverage compares all effective chains after the graph is staged.
// The full union includes tasks moved away from pins as well as newly inherited
// context. Supplied task records must match this set exactly, including versions.
func (p *adoptionProposal) validateAdoptionCoverage(req types.PlanAdoptionRequest) error {
	root := p.result.ContainerID
	scope, err := p.validateStagedScope(req)
	if err != nil {
		return err
	}
	tasks, err := p.validateTaskManifest(req)
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(p.before.issues))
	for id := range p.before.issues {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		old, err := snapshotGovernance(p.before, id)
		if err != nil {
			return err
		}
		next, err := snapshotGovernance(p.after, id)
		if err != nil {
			return err
		}
		p.checkTaskCoverage(id, root, old, next, tasks)
	}
	return p.validateSuppliedTasks(req, tasks, scope)
}

// applyAdoption composes validated mutations without entering public Store APIs.
// Temporary pin clearing permits type changes, then final edges and pins are
// installed before after snapshots and the immutable response are recorded.
func (m *issueMutationTx) applyAdoption(
	ctx context.Context,
	req types.PlanAdoptionRequest,
	p *adoptionProposal,
) error {
	actor := storage.PlanProvenanceFromContext(ctx).Actor
	if err := m.clearAdoptionPins(ctx, p); err != nil {
		return err
	}
	for _, e := range req.Edges {
		if e.Remove && !strings.HasPrefix(e.IssueID, "new:") &&
			!strings.HasPrefix(e.DependsOnID, "new:") {
			if err := m.deleteDependency(ctx, e.IssueID, e.DependsOnID, actor); err != nil {
				return err
			}
		}
	}
	if err := m.applyAdoptionFields(ctx, req, actor); err != nil {
		return err
	}
	if err := m.applyAdoptionFollowUps(ctx, req, p, actor); err != nil {
		return err
	}
	if err := m.writeAdoptionPins(ctx, p, actor); err != nil {
		return err
	}
	if err := m.invalidateAdoptionUnion(ctx, p); err != nil {
		return err
	}
	return m.refreshAdoptionResult(ctx, p)
}

// recordAdoption retains all rationales, source identities and actual versions.
// The idempotency key is inserted in this same transaction only on success.
// Audit or key failure rolls back every staged issue, counter, edge and pin.
func (m *issueMutationTx) recordAdoption(
	ctx context.Context,
	req types.PlanAdoptionRequest,
	result *storage.PlanAdoptionResult,
	key, fingerprint string,
) error {
	result.Actor = storage.PlanProvenanceFromContext(ctx).Actor
	result.SessionID = storage.PlanProvenanceFromContext(ctx).SessionID
	result.ID = rand.Text()
	result.CreatedAt = time.Now().UTC()
	request, err := json.Marshal(req)
	if err != nil {
		return err
	}
	body, err := json.Marshal(result)
	if err != nil {
		return err
	}
	provenance := storage.PlanProvenanceFromContext(ctx)
	_, err = m.tx.ExecContext(
		ctx,
		`
INSERT INTO
plan_adoptions(id,project_id,container_id,request,result,actor,session_id,created_at)
VALUES(?,?,?,?,?,?,?,?)
`,
		result.ID,
		result.ProjectID,
		result.ContainerID,
		string(request),
		string(body),
		provenance.Actor,
		provenance.SessionID,
		result.CreatedAt,
	)
	if err != nil {
		return err
	}
	_, err = m.tx.ExecContext(
		ctx,
		`
INSERT INTO adoption_idempotency(project_id,container_id,key,fingerprint,adoption_id)
VALUES(?,?,?,?,?)
`,
		result.ProjectID,
		result.ContainerID,
		key,
		fingerprint,
		result.ID,
	)
	return err
}

// ListPlanAdoptions returns the committed request, rationale and full before/after
// snapshots. Replays and later adoptions never rewrite these historical records.
func (s *Store) ListPlanAdoptions(
	ctx context.Context,
	p, id string,
	limit, offset int,
) ([]storage.PlanAdoptionResult, error) {
	return readGovernanceHistory[storage.PlanAdoptionResult](
		ctx,
		s,
		p,
		id,
		func(m *issueMutationTx) ([]string, error) {
			if limit < 1 || limit > 200 || offset < 0 {
				return nil, storage.ErrPlanInvalid
			}
			return m.queries.ListPlanAdoptionRecords(
				ctx,
				db.ListPlanAdoptionRecordsParams{
					ProjectID:   p,
					ContainerID: id,
					Limit:       int64(limit),
					Offset:      int64(offset),
				},
			)
		},
	)
}

// stageContainerPins binds every tactical assessment to its captured container.
// Compatible entries cannot silently change pins; updated entries are validated
// against approved durable revisions by the transaction owner.
func (p *adoptionProposal) stageContainerPins(req types.PlanAdoptionRequest) error {
	for _, pin := range req.ContainerPins {
		if _, ok := p.pins[pin.ContainerID]; ok {
			return fmt.Errorf("%w: duplicate container pin", storage.ErrPlanInvalid)
		}
		if strings.TrimSpace(pin.Reason) == "" ||
			(pin.Disposition != adoptionCompatible && pin.Disposition != adoptionUpdated) {
			return fmt.Errorf(
				"%w: tactical compatibility reason and disposition required",
				storage.ErrPlanInvalid,
			)
		}
		if pin.Disposition == adoptionCompatible &&
			!reflect.DeepEqual(pin.ExpectedPin, pin.TargetPin) {
			return fmt.Errorf("%w: compatible pins must be unchanged", storage.ErrPlanInvalid)
		}
		i := p.before.issues[pin.ContainerID]
		if i == nil {
			return storage.ErrIssueNotFound
		}
		if i.ContractVersion != pin.ExpectedContainerVersion ||
			!reflect.DeepEqual(i.GoverningPlan, pin.ExpectedPin) {
			p.coverage(i.ID, "stale_container", "tactical container version or pin changed")
		}
		p.pins[i.ID] = pin
	}

	return nil
}

// stageTypes validates exact versions before editing the in-memory graph.
// Clearing a pin and changing container eligibility is supported in one proposal.
// Closed contracts cannot be reclassified as part of reconciliation.
func (p *adoptionProposal) stageTypes(req types.PlanAdoptionRequest) error {
	touched := map[string]bool{}
	for _, change := range req.TypeChanges {
		i := p.after.issues[change.IssueID]
		if i == nil {
			return storage.ErrIssueNotFound
		}
		if touched[i.ID] || !change.IssueType.IsValid() || i.IssueType == change.IssueType {
			return fmt.Errorf(
				"%w: duplicate, invalid or redundant type change",
				storage.ErrPlanInvalid,
			)
		}
		if i.Status == types.StatusClosed {
			return fmt.Errorf("%w: closed contract type edit", storage.ErrPlanInvalid)
		}
		if i.ContractVersion != change.ExpectedContractVersion {
			p.coverage(i.ID, "stale_task", "type change contract version changed")
		}
		touched[i.ID] = true
		i.IssueType = change.IssueType
		p.affected[i.ID] = true
	}
	for id, i := range p.after.issues {
		if i.GoverningPlan != nil && i.IssueType != types.TypeEpic &&
			i.IssueType != types.TypeMilestone {
			return fmt.Errorf("%w: unsupported pinned container %s", storage.ErrPlanInvalid, id)
		}
	}

	return nil
}

// validateContainerEligibility runs after type staging so attach/detach can
// accompany eligibility changes. A tactical record must concern an epic or
// milestone in its source or destination state; null pins do not exempt it.
func (p *adoptionProposal) validateContainerEligibility(req types.PlanAdoptionRequest) error {
	for _, pin := range req.ContainerPins {
		before, after := p.before.issues[pin.ContainerID], p.after.issues[pin.ContainerID]
		sourceEligible := before.IssueType == types.TypeEpic ||
			before.IssueType == types.TypeMilestone
		targetEligible := after.IssueType == types.TypeEpic ||
			after.IssueType == types.TypeMilestone
		if (!sourceEligible && !targetEligible) || (pin.ExpectedPin != nil && !sourceEligible) ||
			(pin.TargetPin != nil && !targetEligible) {
			return fmt.Errorf(
				"%w: ineligible container pin %s",
				storage.ErrPlanInvalid,
				pin.ContainerID,
			)
		}
	}
	return nil
}

// stageFollowUps uses only proposal-local identities; it never reserves counters.
// The parent edge is part of the proposed graph from the outset, so governance
// and cycles are validated before CreateIssue allocates an actual ID.
func (p *adoptionProposal) stageFollowUps(req types.PlanAdoptionRequest) error {
	followKeys := map[string]bool{}
	for _, f := range req.FollowUps {
		if f.Key == "" || strings.HasPrefix(f.Key, "new:") || followKeys[f.Key] || f.Title == "" ||
			!f.IssueType.IsValid() ||
			f.IssueType.IsContainer() ||
			f.Priority < 0 ||
			f.Priority > 4 {
			return fmt.Errorf("%w: invalid or duplicate follow-up", storage.ErrPlanInvalid)
		}
		if strings.HasPrefix(f.ParentID, "new:") {
			return fmt.Errorf(
				"%w: follow-up parent must be a persisted issue",
				storage.ErrPlanInvalid,
			)
		}
		if p.before.issues[f.ParentID] == nil {
			return storage.ErrIssueNotFound
		}
		followKeys[f.Key] = true
		id := "new:" + f.Key
		issue := &types.Issue{
			ID:          id,
			ProjectID:   p.result.ProjectID,
			Title:       f.Title,
			Description: f.Description,
			Status:      types.StatusOpen,
			IssueType:   f.IssueType,
			Priority:    f.Priority,
		}
		if err := issue.Validate(); err != nil {
			return fmt.Errorf("%w: %w", storage.ErrPlanInvalid, err)
		}
		p.after.issues[id] = issue
		p.after.parents[id] = []string{f.ParentID}
	}

	return nil
}

// stageEdges rejects contradictory edits and validates both endpoints locally.
// Cross-project blocking edges remain valid for ordinary operations, but staged
// reconciliation never accepts an out-of-project target.
func (p *adoptionProposal) stageEdges(
	req types.PlanAdoptionRequest,
	edges map[[2]string]types.DependencyType,
) error {
	seen := map[[2]string]bool{}
	for _, e := range req.Edges {
		pair := [2]string{e.IssueID, e.DependsOnID}
		if seen[pair] || e.IssueID == e.DependsOnID || !e.Type.IsValid() {
			return fmt.Errorf("%w: contradictory or invalid edge", storage.ErrPlanInvalid)
		}
		seen[pair] = true
		if err := p.stageEdge(e, edges); err != nil {
			return err
		}
	}

	return nil
}

// validateStagedCycles examines the final hierarchy and execution dependencies.
// Blocking edges participate in cycle checks but never become governing ancestry.
// A valid proposal cannot rely on transient intermediate graph states.
func (p *adoptionProposal) validateStagedCycles(edges map[[2]string]types.DependencyType) error {
	// Parent and blocking dependency cycles are rejected across the staged graph.
	cycleGraph := &governanceGraph{issues: p.after.issues, parents: map[string][]string{}}
	for pair, typ := range edges {
		if typ == types.DepParentChild || typ == types.DepBlocks {
			cycleGraph.parents[pair[0]] = append(cycleGraph.parents[pair[0]], pair[1])
		}
	}
	for id := range p.after.issues {
		if _, err := p.after.resolve(id); err != nil {
			return err
		}
		walk := &governanceAncestry{
			state:     map[string]int{},
			ancestors: map[string]map[string]bool{},
		}
		if err := cycleGraph.visitAncestry(id, walk); err != nil {
			return err
		}
	}
	return nil
}

// validateTaskManifest checks dispositions, proposed prose and follow-up links.
// Semantic consistency remains a reviewer judgment recorded by the reason; the
// server enforces structural completeness and prohibits edits to closed work.
func (p *adoptionProposal) validateTaskManifest(
	req types.PlanAdoptionRequest,
) (map[string]types.ReconciledTask, error) {
	tasks := map[string]types.ReconciledTask{}
	usedKeys := map[string]bool{}
	for _, t := range req.Tasks {
		if _, exists := tasks[t.IssueID]; exists {
			return nil, fmt.Errorf("%w: duplicate task", storage.ErrPlanInvalid)
		}
		if strings.TrimSpace(t.Reason) == "" ||
			(t.Disposition != adoptionUnchanged &&
				t.Disposition != adoptionUpdated && t.Disposition != adoptionFollowUp) {
			return nil, fmt.Errorf(
				"%w: task disposition and reason required",
				storage.ErrPlanInvalid,
			)
		}
		i := p.before.issues[t.IssueID]
		if i == nil {
			return nil, storage.ErrIssueNotFound
		}
		if err := validateReconciledTask(t, i); err != nil {
			return nil, err
		}
		if err := p.validateFollowUpKeys(t, usedKeys); err != nil {
			return nil, err
		}
		tasks[t.IssueID] = t
		if t.Title != nil || t.Description != nil {
			p.affected[t.IssueID] = true
		}
	}
	for _, f := range req.FollowUps {
		if !usedKeys[f.Key] {
			return nil, fmt.Errorf("%w: unused follow-up key", storage.ErrPlanInvalid)
		}
	}

	return tasks, nil
}

// checkTaskCoverage compares every source identity, not merely the plan bytes.
// Higher-level changes therefore include work beneath retained tactical pins.
// An unchanged disposition never exempts an active worker from the pause rule.
func (p *adoptionProposal) checkTaskCoverage(
	id, root string,
	old, next storage.GovernanceSnapshot,
	tasks map[string]types.ReconciledTask,
) {
	i, j := p.before.issues[id], p.after.issues[id]
	changed := !sameGovernance(old.Expected.Governing, next.Expected.Governing)
	if id != root && (i.GoverningPlan != nil || j.GoverningPlan != nil) && changed {
		if _, exists := p.pins[id]; !exists {
			p.coverage(
				id,
				"missing_container_pin",
				"affected tactical pin requires compatibility or update record",
			)
		}
	}
	if changed {
		p.affected[id] = true
	}
	if i.IssueType.IsContainer() && j.IssueType.IsContainer() {
		return
	}
	if p.affected[id] {
		p.result.Tasks = append(
			p.result.Tasks,
			storage.ReconciliationChange{Before: old, After: next},
		)
		t, exists := tasks[id]
		if !exists {
			p.coverage(id, "missing_task", "affected task requires reconciliation")
		} else if !sameExpected(t.Expected, old.Expected) {
			p.coverage(id, "stale_task", "task governance or contract version changed")
		}
		if i.Status == types.StatusInProgress {
			p.coverage(id, "task_in_progress", "affected execution must pause before adoption")
		}
	}
}

// clearAdoptionPins allows a final type change to remove container eligibility.
// These transient pin removals are hidden by the owning transaction; failures
// roll back the old pins, descendant versions and retained pin markers together.
func (m *issueMutationTx) clearAdoptionPins(ctx context.Context, p *adoptionProposal) error {
	// Clear changed pins first so a reconciled type change can remove container eligibility.
	// The transaction is not externally visible until every final pin and audit commits.
	for id, pin := range p.pins {
		if pin.ExpectedPin != nil && !reflect.DeepEqual(pin.ExpectedPin, pin.TargetPin) {
			if _, err := m.tx.ExecContext(
				ctx, `UPDATE issues SET governing_plan_id=NULL,governing_plan_revision=NULL WHERE id=?`, id,
			); err != nil {
				return err
			}
		}
	}

	return nil
}

// applyAdoptionFields composes the same audit and index writes as ordinary edits.
// Manifest validation is complete before this stage; no public Store method is
// called while the mutation transaction owns SQLite's connection.
func (m *issueMutationTx) applyAdoptionFields(
	ctx context.Context,
	req types.PlanAdoptionRequest,
	actor string,
) error {
	for _, change := range req.TypeChanges {
		fields := map[string]any{"issue_type": string(change.IssueType)}
		if err := m.writeIssueFields(ctx, change.IssueID, fields, actor); err != nil {
			return err
		}
	}
	for _, t := range req.Tasks {
		fields := map[string]any{}
		if t.Title != nil {
			fields["title"] = *t.Title
		}
		if t.Description != nil {
			fields["description"] = *t.Description
		}
		if len(fields) > 0 {
			if err := m.writeIssueFields(ctx, t.IssueID, fields, actor); err != nil {
				return err
			}
		}
	}

	return nil
}

// applyAdoptionFollowUps allocates IDs only after the complete proposal passes.
// Counter allocation, parent edges, dependency audit and full-text indexing all
// use the same transaction and are discarded on any later failure.
func (m *issueMutationTx) applyAdoptionFollowUps(
	ctx context.Context,
	req types.PlanAdoptionRequest,
	p *adoptionProposal,
	actor string,
) error {
	for _, f := range req.FollowUps {
		i := &types.Issue{
			ProjectID:   p.result.ProjectID,
			ParentID:    f.ParentID,
			Title:       f.Title,
			Description: f.Description,
			IssueType:   f.IssueType,
			Priority:    f.Priority,
		}
		if err := m.createIssueRecord(ctx, i, actor); err != nil {
			return err
		}
		p.result.FollowUpIDs[f.Key] = i.ID
	}
	resolveID := func(id string) string {
		if strings.HasPrefix(id, "new:") {
			return p.result.FollowUpIDs[strings.TrimPrefix(id, "new:")]
		}
		return id
	}
	for _, e := range req.Edges {
		if e.Remove &&
			(strings.HasPrefix(e.IssueID, "new:") || strings.HasPrefix(e.DependsOnID, "new:")) {
			if err := m.deleteDependency(ctx, resolveID(e.IssueID), resolveID(e.DependsOnID), actor); err != nil {
				return err
			}
		}
	}
	for _, e := range req.Edges {
		if !e.Remove {
			dep := &types.Dependency{
				IssueID:     resolveID(e.IssueID),
				DependsOnID: resolveID(e.DependsOnID),
				Type:        e.Type,
			}
			if err := m.insertDependency(ctx, dep, actor); err != nil {
				return err
			}
		}
	}

	return nil
}

// writeAdoptionPins installs the validated final references. The existing pin
// triggers invalidate all current descendants, including tactical and diamond
// paths, while the union pass covers descendants moved away by staged edges.
func (m *issueMutationTx) writeAdoptionPins(
	ctx context.Context,
	p *adoptionProposal,
	actor string,
) error {
	for id, pin := range p.pins {
		if pin.TargetPin != nil && !reflect.DeepEqual(pin.ExpectedPin, pin.TargetPin) {
			if _, err := m.tx.ExecContext(
				ctx, `UPDATE issues SET governing_plan_id=?,governing_plan_revision=? WHERE id=?`,
				pin.TargetPin.PlanID, pin.TargetPin.Revision, id,
			); err != nil {
				return err
			}
		}
		if !reflect.DeepEqual(pin.ExpectedPin, pin.TargetPin) {
			if err := m.recordEvent(ctx, id, types.EventUpdated, actor, nil, nil); err != nil {
				return err
			}
		}
	}

	return nil
}

// invalidateAdoptionUnion accounts for effective changes caused by hierarchy
// and type edits. An already incremented contract needs no artificial second
// increment, but every affected pre-existing issue must have a new version.
func (m *issueMutationTx) invalidateAdoptionUnion(ctx context.Context, p *adoptionProposal) error {
	// Pin triggers cover descendants present during pin writes. The before/after
	// union also includes tasks moved away before those writes or type-only changes.
	for id := range p.affected {
		i, err := m.GetIssue(ctx, id)
		if err != nil {
			return err
		}
		if i.ContractVersion == p.before.issues[id].ContractVersion {
			if _, err := m.tx.ExecContext(
				ctx, `UPDATE issues SET contract_version=contract_version+1 WHERE id=?`, id,
			); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateReconciledTask separates proposed contract edits from compatibility
// assertions. Completed work remains immutable; differences use linked follow-ups.
func validateReconciledTask(t types.ReconciledTask, i *types.Issue) error {
	if (t.Title != nil || t.Description != nil) &&
		(t.Disposition != adoptionUpdated || i.Status == types.StatusClosed) {
		return fmt.Errorf("%w: inappropriate contract edit", storage.ErrPlanInvalid)
	}
	if t.Title != nil && strings.TrimSpace(*t.Title) == "" {
		return storage.ErrPlanInvalid
	}
	if t.Title != nil {
		staged := *i
		staged.Title = *t.Title
		if err := staged.Validate(); err != nil {
			return fmt.Errorf("%w: %w", storage.ErrPlanInvalid, err)
		}
	}
	if (len(t.FollowUpKeys) > 0) != (t.Disposition == adoptionFollowUp) {
		return fmt.Errorf(
			"%w: follow-up disposition must reference follow-ups",
			storage.ErrPlanInvalid,
		)
	}
	return nil
}

// captureContainerSnapshots records every staged tactical assessment alongside
// the root. Dry-run versions remain captured values; apply replaces the after
// versions with the values actually committed by triggers and union invalidation.
func (p *adoptionProposal) captureContainerSnapshots(req types.PlanAdoptionRequest) error {
	p.result.Request = req
	p.result.Containers = []storage.ReconciliationChange{}
	pinIDs := make([]string, 0, len(p.pins))
	for id := range p.pins {
		pinIDs = append(pinIDs, id)
	}
	slices.Sort(pinIDs)
	for _, id := range pinIDs {
		old, err := snapshotGovernance(p.before, id)
		if err != nil {
			return err
		}
		next, err := snapshotGovernance(p.after, id)
		if err != nil {
			return err
		}
		p.result.Containers = append(
			p.result.Containers,
			storage.ReconciliationChange{Before: old, After: next},
		)
	}
	var err error
	p.result.Before, err = snapshotGovernance(p.before, p.result.ContainerID)
	if err != nil {
		return err
	}
	p.result.After, err = snapshotGovernance(p.after, p.result.ContainerID)
	if err != nil {
		return err
	}
	return nil
}

// validateStagedScope includes descendants entering and leaving the root.
// This validates staged structural operations; the task manifest is checked
// against the same scope again after proposed prose edits are collected.
func (p *adoptionProposal) validateStagedScope(
	req types.PlanAdoptionRequest,
) (map[string]bool, error) {
	scope := p.before.descendants(p.result.ContainerID)
	for id := range p.after.descendants(p.result.ContainerID) {
		scope[id] = true
	}
	for id := range p.pins {
		if !scope[id] {
			return nil, fmt.Errorf("%w: unrelated container pin %s", storage.ErrPlanInvalid, id)
		}
	}
	for id := range p.affected {
		if !scope[id] {
			return nil, fmt.Errorf("%w: unrelated staged contract %s", storage.ErrPlanInvalid, id)
		}
	}
	for _, f := range req.FollowUps {
		if !scope["new:"+f.Key] {
			return nil, fmt.Errorf("%w: unrelated follow-up", storage.ErrPlanInvalid)
		}
	}

	return scope, nil
}

// verifyAppliedGovernance checks the completed persisted graph, including every
// allocated follow-up, before recording success. Individual inserts intentionally
// do not resolve transient topology while staged edge replacements are underway.
func (p *adoptionProposal) verifyAppliedGovernance(after *governanceGraph) error {
	for id := range p.after.issues {
		actualID := id
		if strings.HasPrefix(id, "new:") {
			actualID = p.result.FollowUpIDs[strings.TrimPrefix(id, "new:")]
		}
		proposed, err := p.after.resolve(id)
		if err != nil {
			return err
		}
		actual, err := after.resolve(actualID)
		if err != nil {
			return err
		}
		if !sameGovernance(proposed, actual) {
			return fmt.Errorf(
				"%w: applied governance differs from proposal for %s",
				storage.ErrPlanConflict,
				id,
			)
		}
	}
	return nil
}

// refreshAdoptionResult reads authoritative after versions before commit. Trigger
// increments may differ by the number of changed fields and pins; clients compare
// captured values rather than infer version arithmetic from the request.
func (m *issueMutationTx) refreshAdoptionResult(ctx context.Context, p *adoptionProposal) error {
	after, err := m.adoptionGraph(ctx, p.result.ProjectID)
	if err != nil {
		return err
	}
	if err := p.verifyAppliedGovernance(after); err != nil {
		return err
	}
	p.result.After, err = snapshotGovernance(after, p.result.ContainerID)
	if err != nil {
		return err
	}
	for n := range p.result.Tasks {
		p.result.Tasks[n].After, err = snapshotGovernance(after, p.result.Tasks[n].Before.IssueID)
		if err != nil {
			return err
		}
	}
	for n := range p.result.Containers {
		p.result.Containers[n].After, err = snapshotGovernance(
			after,
			p.result.Containers[n].Before.IssueID,
		)
		if err != nil {
			return err
		}
	}
	project, err := m.GetProject(ctx, p.result.ProjectID)
	if err != nil {
		return err
	}
	p.result.GovernanceGeneration = project.GovernanceGeneration
	return nil
}

// stageEdge requires an exact existing edge for removal and a missing edge for
// insertion. It mutates only the proposal's graph and invalidation set; actual
// dependency writes and their audit events occur later under the owner lock.
func (p *adoptionProposal) stageEdge(
	e types.ReconciliationEdge,
	edges map[[2]string]types.DependencyType,
) error {
	pair := [2]string{e.IssueID, e.DependsOnID}
	child, parent := p.after.issues[e.IssueID], p.after.issues[e.DependsOnID]
	if child == nil || parent == nil {
		return fmt.Errorf("%w: dangling or cross-project edge target", storage.ErrPlanInvalid)
	}
	if child.Status == types.StatusClosed {
		return fmt.Errorf("%w: closed contract dependency edit", storage.ErrPlanInvalid)
	}
	old, exists := edges[pair]
	if (e.Remove && (!exists || old != e.Type)) || (!e.Remove && exists) {
		return fmt.Errorf("%w: edge does not match staged operation", storage.ErrPlanInvalid)
	}
	if e.Type == types.DepParentChild {
		if e.Remove {
			p.after.parents[e.IssueID] = slices.DeleteFunc(
				p.after.parents[e.IssueID],
				func(v string) bool { return v == e.DependsOnID },
			)
		} else {
			p.after.parents[e.IssueID] = append(p.after.parents[e.IssueID], e.DependsOnID)
		}
	}
	if e.Remove {
		delete(edges, pair)
	} else {
		edges[pair] = e.Type
	}
	if p.before.issues[e.IssueID] != nil {
		p.affected[e.IssueID] = true
	}
	return nil
}

// validateFollowUpKeys ties each declared follow-up to at least one reconciled
// task. Duplicate references within one disposition are rejected; multiple
// completed tasks may legitimately share one follow-up issue.
func (p *adoptionProposal) validateFollowUpKeys(
	t types.ReconciledTask,
	usedKeys map[string]bool,
) error {
	for _, key := range t.FollowUpKeys {
		if usedKeys[t.IssueID+"\x00"+key] || p.after.issues["new:"+key] == nil {
			return fmt.Errorf(
				"%w: duplicate or dangling follow-up reference",
				storage.ErrPlanInvalid,
			)
		}
		usedKeys[t.IssueID+"\x00"+key] = true
		usedKeys[key] = true
	}
	return nil
}

// validateSuppliedTasks rejects unrelated entries after the full invalidation set
// is computed. Structural task edits need an updated disposition even when their
// resulting governing design happens to remain unchanged.
func (p *adoptionProposal) validateSuppliedTasks(
	req types.PlanAdoptionRequest, tasks map[string]types.ReconciledTask, scope map[string]bool,
) error {
	for id := range tasks {
		if !scope[id] || !p.affected[id] ||
			(p.before.issues[id].IssueType.IsContainer() && p.after.issues[id].IssueType.IsContainer()) {
			return fmt.Errorf("%w: unrelated task reconciliation %s", storage.ErrPlanInvalid, id)
		}
	}
	// Content edits must be updated, and staged edge/type edits cannot masquerade as unchanged.
	for _, e := range req.Edges {
		if t, ok := tasks[e.IssueID]; ok && t.Disposition != adoptionUpdated {
			return fmt.Errorf(
				"%w: dependency edit requires updated disposition",
				storage.ErrPlanInvalid,
			)
		}
	}
	for _, c := range req.TypeChanges {
		if t, ok := tasks[c.IssueID]; ok && t.Disposition != adoptionUpdated {
			return fmt.Errorf("%w: type edit requires updated disposition", storage.ErrPlanInvalid)
		}
	}
	return nil
}

// validateProposalReferences also covers inherited sources introduced by staged
// hierarchy edits. Such a source need not have an explicit pin-edit record, but
// cannot bypass approval, activity, project ownership or integrity validation.
func (s *Store) validateProposalReferences(
	ctx context.Context,
	m *issueMutationTx,
	p *adoptionProposal,
) error {
	before, after := map[types.PlanReference]bool{}, map[types.PlanReference]bool{}
	collect := func(g *types.GoverningPlan, refs map[types.PlanReference]bool) {
		if g == nil {
			return
		}
		refs[g.Reference] = true
		for _, ancestor := range g.Context {
			refs[ancestor.Reference] = true
		}
	}
	for _, changes := range [][]storage.ReconciliationChange{p.result.Tasks, p.result.Containers} {
		for _, change := range changes {
			collect(change.Before.Expected.Governing, before)
			collect(change.After.Expected.Governing, after)
		}
	}
	// Newly allocated work has no before snapshot but still inherits every final source.
	for _, followUp := range p.result.Request.FollowUps {
		governing, err := p.after.resolve("new:" + followUp.Key)
		if err != nil {
			return err
		}
		collect(governing, after)
	}
	for reference := range before {
		if err := s.validateAdoptionPin(ctx, m, p.result.ProjectID, &reference, false); err != nil {
			return err
		}
	}
	for reference := range after {
		if err := s.validateAdoptionPin(ctx, m, p.result.ProjectID, &reference, true); err != nil {
			return err
		}
	}
	return nil
}
