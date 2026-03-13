// Package storage defines the interface for issue storage backends.
package storage

import (
	"context"

	"github.com/sentiolabs/arc/internal/types"
)

//nolint:interfacebloat // Storage interface intentionally covers all operations as a single contract
type Storage interface {
	// Projects
	CreateProject(ctx context.Context, project *types.Project) error
	GetProject(ctx context.Context, id string) (*types.Project, error)
	GetProjectByName(ctx context.Context, name string) (*types.Project, error)
	ListProjects(ctx context.Context) ([]*types.Project, error)
	UpdateProject(ctx context.Context, project *types.Project) error
	DeleteProject(ctx context.Context, id string) error
	MergeProjects(ctx context.Context, targetID string, sourceIDs []string, actor string) (*types.MergeResult, error)

	// Workspaces
	CreateWorkspace(ctx context.Context, ws *types.Workspace) error
	GetWorkspace(ctx context.Context, id string) (*types.Workspace, error)
	ListWorkspaces(ctx context.Context, projectID string) ([]*types.Workspace, error)
	UpdateWorkspace(ctx context.Context, ws *types.Workspace) error
	DeleteWorkspace(ctx context.Context, id string) error
	ResolveProjectByPath(ctx context.Context, path string) (*types.Workspace, error)
	UpdateWorkspaceLastAccessed(ctx context.Context, id string) error

	// Issues
	CreateIssue(ctx context.Context, issue *types.Issue, actor string) error
	GetIssue(ctx context.Context, id string) (*types.Issue, error)
	GetIssueByExternalRef(ctx context.Context, externalRef string) (*types.Issue, error)
	ListIssues(ctx context.Context, filter types.IssueFilter) ([]*types.Issue, error)
	UpdateIssue(ctx context.Context, id string, updates map[string]any, actor string) error
	CloseIssue(ctx context.Context, id string, reason string, cascade bool, actor string) error
	ReopenIssue(ctx context.Context, id string, actor string) error
	DeleteIssue(ctx context.Context, id string) error
	GetIssueDetails(ctx context.Context, id string) (*types.IssueDetails, error)

	// Ready Work & Blocking
	GetReadyWork(ctx context.Context, filter types.WorkFilter) ([]*types.Issue, error)
	GetBlockedIssues(ctx context.Context, filter types.WorkFilter) ([]*types.BlockedIssue, error)
	IsBlocked(ctx context.Context, issueID string) (bool, []string, error)

	// Dependencies
	AddDependency(ctx context.Context, dep *types.Dependency, actor string) error
	RemoveDependency(ctx context.Context, issueID, dependsOnID string, actor string) error
	GetDependencies(ctx context.Context, issueID string) ([]*types.Dependency, error)
	GetDependents(ctx context.Context, issueID string) ([]*types.Dependency, error)

	// Labels (global)
	CreateLabel(ctx context.Context, label *types.Label) error
	GetLabel(ctx context.Context, name string) (*types.Label, error)
	ListLabels(ctx context.Context) ([]*types.Label, error)
	UpdateLabel(ctx context.Context, label *types.Label) error
	DeleteLabel(ctx context.Context, name string) error
	AddLabelToIssue(ctx context.Context, issueID, label, actor string) error
	RemoveLabelFromIssue(ctx context.Context, issueID, label, actor string) error
	GetIssueLabels(ctx context.Context, issueID string) ([]string, error)
	GetLabelsForIssues(ctx context.Context, issueIDs []string) (map[string][]string, error)

	// Comments
	AddComment(ctx context.Context, issueID, author, text string) (*types.Comment, error)
	GetComments(ctx context.Context, issueID string) ([]*types.Comment, error)
	UpdateComment(ctx context.Context, commentID int64, text string) error
	DeleteComment(ctx context.Context, commentID int64) error

	// Plans (shared plans)
	CreatePlan(ctx context.Context, plan *types.Plan) error
	GetPlan(ctx context.Context, id string) (*types.Plan, error)
	ListPlans(ctx context.Context, projectID string) ([]*types.Plan, error)
	UpdatePlan(ctx context.Context, id, title, content string) error
	DeletePlan(ctx context.Context, id string) error
	LinkIssueToPlan(ctx context.Context, issueID, planID string) error
	UnlinkIssueFromPlan(ctx context.Context, issueID, planID string) error
	GetLinkedPlans(ctx context.Context, issueID string) ([]*types.Plan, error)
	GetLinkedIssues(ctx context.Context, planID string) ([]string, error)

	// Inline Plans (comment-based plans on issues)
	SetInlinePlan(ctx context.Context, issueID, author, text string) (*types.Comment, error)
	GetInlinePlan(ctx context.Context, issueID string) (*types.Comment, error)
	GetPlanHistory(ctx context.Context, issueID string) ([]*types.Comment, error)
	GetPlanContext(ctx context.Context, issueID string) (*types.PlanContext, error)

	// AI Sessions
	CreateAISession(ctx context.Context, session *types.AISession) error
	GetAISession(ctx context.Context, id string) (*types.AISession, error)
	ListAISessions(ctx context.Context, limit, offset int) ([]*types.AISession, error)
	DeleteAISession(ctx context.Context, id string) error
	CreateAIAgent(ctx context.Context, agent *types.AIAgent) error
	GetAIAgent(ctx context.Context, id string) (*types.AIAgent, error)
	ListAIAgents(ctx context.Context, sessionID string) ([]*types.AIAgent, error)

	// Events (audit trail)
	GetEvents(ctx context.Context, issueID string, limit int) ([]*types.Event, error)

	// Statistics
	GetStatistics(ctx context.Context, projectID string) (*types.Statistics, error)

	// Lifecycle
	Close() error
	Path() string
}
