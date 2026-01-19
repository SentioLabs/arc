// Package storage defines the interface for issue storage backends.
package storage

import (
	"context"

	"github.com/sentiolabs/arc/internal/types"
)

// Storage defines the interface for issue storage backends.
type Storage interface {
	// Workspaces
	CreateWorkspace(ctx context.Context, workspace *types.Workspace) error
	GetWorkspace(ctx context.Context, id string) (*types.Workspace, error)
	GetWorkspaceByName(ctx context.Context, name string) (*types.Workspace, error)
	GetWorkspaceByPath(ctx context.Context, path string) (*types.Workspace, error)
	ListWorkspaces(ctx context.Context) ([]*types.Workspace, error)
	UpdateWorkspace(ctx context.Context, workspace *types.Workspace) error
	DeleteWorkspace(ctx context.Context, id string) error

	// Issues
	CreateIssue(ctx context.Context, issue *types.Issue, actor string) error
	GetIssue(ctx context.Context, id string) (*types.Issue, error)
	GetIssueByExternalRef(ctx context.Context, externalRef string) (*types.Issue, error)
	ListIssues(ctx context.Context, filter types.IssueFilter) ([]*types.Issue, error)
	UpdateIssue(ctx context.Context, id string, updates map[string]interface{}, actor string) error
	CloseIssue(ctx context.Context, id string, reason string, actor string) error
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

	// Labels
	CreateLabel(ctx context.Context, label *types.Label) error
	GetLabel(ctx context.Context, workspaceID, name string) (*types.Label, error)
	ListLabels(ctx context.Context, workspaceID string) ([]*types.Label, error)
	UpdateLabel(ctx context.Context, label *types.Label) error
	DeleteLabel(ctx context.Context, workspaceID, name string) error
	AddLabelToIssue(ctx context.Context, issueID, label, actor string) error
	RemoveLabelFromIssue(ctx context.Context, issueID, label, actor string) error
	GetIssueLabels(ctx context.Context, issueID string) ([]string, error)
	GetLabelsForIssues(ctx context.Context, issueIDs []string) (map[string][]string, error)

	// Comments
	AddComment(ctx context.Context, issueID, author, text string) (*types.Comment, error)
	GetComments(ctx context.Context, issueID string) ([]*types.Comment, error)
	UpdateComment(ctx context.Context, commentID int64, text string) error
	DeleteComment(ctx context.Context, commentID int64) error

	// Events (audit trail)
	GetEvents(ctx context.Context, issueID string, limit int) ([]*types.Event, error)

	// Statistics
	GetStatistics(ctx context.Context, workspaceID string) (*types.Statistics, error)

	// Lifecycle
	Close() error
	Path() string
}
