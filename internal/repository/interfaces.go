// Package repository defines domain-focused repository interfaces.
// These replace the monolithic Storage interface with focused contracts.
package repository

import (
	"context"

	"github.com/sentiolabs/arc/internal/types"
)

// WorkspaceRepository handles workspace persistence.
type WorkspaceRepository interface {
	Create(ctx context.Context, workspace *types.Workspace) error
	Get(ctx context.Context, id string) (*types.Workspace, error)
	GetByName(ctx context.Context, name string) (*types.Workspace, error)
	List(ctx context.Context) ([]*types.Workspace, error)
	Update(ctx context.Context, workspace *types.Workspace) error
	Delete(ctx context.Context, id string) error
	GetStatistics(ctx context.Context, workspaceID string) (*types.Statistics, error)
}

// IssueRepository handles issue persistence.
type IssueRepository interface {
	// CRUD
	Create(ctx context.Context, issue *types.Issue, actor string) error
	Get(ctx context.Context, id string) (*types.Issue, error)
	GetByExternalRef(ctx context.Context, externalRef string) (*types.Issue, error)
	List(ctx context.Context, filter types.IssueFilter) ([]*types.Issue, error)
	Update(ctx context.Context, id string, updates map[string]any, actor string) error
	Delete(ctx context.Context, id string) error

	// Lifecycle
	Close(ctx context.Context, id string, reason string, actor string) error
	Reopen(ctx context.Context, id string, actor string) error

	// Details (aggregates related data)
	GetDetails(ctx context.Context, id string) (*types.IssueDetails, error)

	// Ready work & blocking
	GetReadyWork(ctx context.Context, filter types.WorkFilter) ([]*types.Issue, error)
	GetBlockedIssues(ctx context.Context, filter types.WorkFilter) ([]*types.BlockedIssue, error)
	IsBlocked(ctx context.Context, issueID string) (bool, []string, error)
}

// DependencyRepository handles issue dependencies.
type DependencyRepository interface {
	Add(ctx context.Context, dep *types.Dependency, actor string) error
	Remove(ctx context.Context, issueID, dependsOnID string, actor string) error
	GetForIssue(ctx context.Context, issueID string) ([]*types.Dependency, error)
	GetDependents(ctx context.Context, issueID string) ([]*types.Dependency, error)
}

// LabelRepository handles labels and issue-label associations.
type LabelRepository interface {
	// Label CRUD
	Create(ctx context.Context, label *types.Label) error
	Get(ctx context.Context, workspaceID, name string) (*types.Label, error)
	List(ctx context.Context, workspaceID string) ([]*types.Label, error)
	Update(ctx context.Context, label *types.Label) error
	Delete(ctx context.Context, workspaceID, name string) error

	// Issue-label associations
	AddToIssue(ctx context.Context, issueID, label, actor string) error
	RemoveFromIssue(ctx context.Context, issueID, label, actor string) error
	GetIssueLabels(ctx context.Context, issueID string) ([]string, error)
}

// CommentRepository handles comments on issues.
type CommentRepository interface {
	Add(ctx context.Context, issueID, author, text string) (*types.Comment, error)
	Get(ctx context.Context, commentID int64) (*types.Comment, error)
	List(ctx context.Context, issueID string) ([]*types.Comment, error)
	Update(ctx context.Context, commentID int64, text string) error
	Delete(ctx context.Context, commentID int64) error
}

// EventRepository handles audit trail events.
type EventRepository interface {
	List(ctx context.Context, issueID string, limit int) ([]*types.Event, error)
	Record(ctx context.Context, issueID string, eventType types.EventType, actor string, oldValue, newValue *string) error
}

// Repositories is a container for all repository interfaces.
// This can be injected into services as a single dependency.
type Repositories struct {
	Workspaces   WorkspaceRepository
	Issues       IssueRepository
	Dependencies DependencyRepository
	Labels       LabelRepository
	Comments     CommentRepository
	Events       EventRepository
}
