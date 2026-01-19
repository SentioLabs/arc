package service

import (
	"github.com/sentiolabs/arc/internal/repository"
)

// Services is a container for all service instances.
// This can be injected into handlers as a single dependency.
type Services struct {
	Workspaces   *WorkspaceService
	Issues       *IssueService
	Dependencies *DependencyService
	Labels       *LabelService
	Comments     *CommentService
}

// NewServices creates all services from the given repositories.
func NewServices(repos *repository.Repositories) *Services {
	return &Services{
		Workspaces:   NewWorkspaceService(repos.Workspaces),
		Issues:       NewIssueService(repos.Issues),
		Dependencies: NewDependencyService(repos.Dependencies),
		Labels:       NewLabelService(repos.Labels),
		Comments:     NewCommentService(repos.Comments, repos.Events),
	}
}
