// Package storage defines the interface for issue storage backends.
package storage

import (
	"context"
	"errors"
	"time"

	"github.com/sentiolabs/arc/internal/types"
)

//nolint:interfacebloat // Storage interface intentionally covers all operations as a single contract
type Storage interface {
	DurablePlans
	// Projects
	CreateProject(ctx context.Context, project *types.Project) error
	GetProject(ctx context.Context, id string) (*types.Project, error)
	GetProjectByName(ctx context.Context, name string) (*types.Project, error)
	ListProjects(ctx context.Context) ([]*types.Project, error)
	UpdateProject(ctx context.Context, project *types.Project) error
	DeleteProject(ctx context.Context, id string) error
	MergeProjects(
		ctx context.Context,
		targetID string,
		sourceIDs []string,
		actor string,
	) (*types.MergeResult, error)

	// Project config (per-project key/value settings)
	GetProjectConfig(ctx context.Context, projectID string) (map[string]string, error)
	SetProjectConfig(ctx context.Context, projectID, key, value string) error
	DeleteProjectConfig(ctx context.Context, projectID, key string) error

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
	// GetReadyWork returns unblocked, ungated, non-container issues with
	// effective priority and ancestry path populated.
	GetReadyWork(ctx context.Context, filter types.WorkFilter) ([]*types.ReadyIssue, error)
	GetBlockedIssues(ctx context.Context, filter types.WorkFilter) ([]*types.BlockedIssue, error)
	IsBlocked(ctx context.Context, issueID string) (bool, []string, error)

	// GetRoadmap returns the container tree (releases, milestones, epics) for a
	// project with progress counts and gating info.
	GetRoadmap(ctx context.Context, projectID string) ([]*types.RoadmapNode, error)

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

	// Plans
	ListLegacyPlans(context.Context, int, int) ([]LegacyPlanInventory, error)
	CreatePlan(ctx context.Context, plan *types.LegacyPlan) error
	GetPlan(ctx context.Context, id string) (*types.LegacyPlan, error)
	UpdatePlanStatus(ctx context.Context, id string, status string) error
	DeletePlan(ctx context.Context, id string) error

	// Plan Comments
	CreatePlanComment(ctx context.Context, comment *types.PlanComment) error
	ListPlanComments(ctx context.Context, planID string) ([]*types.PlanComment, error)
	GetPlanComment(ctx context.Context, id string) (*types.PlanComment, error)
	UpdatePlanComment(ctx context.Context, comment *types.PlanComment) error
	DeletePlanComment(ctx context.Context, id string) error

	// AI Sessions
	CreateAISession(ctx context.Context, session *types.AISession) error
	GetAISession(ctx context.Context, id string) (*types.AISession, error)
	ListAISessionsByProject(
		ctx context.Context,
		projectID string,
		limit, offset int,
	) ([]*types.AISession, error)
	CountAISessionsByProject(ctx context.Context, projectID string) (int64, error)
	DeleteAISession(ctx context.Context, id string) error
	CreateAIAgent(ctx context.Context, agent *types.AIAgent) error
	GetAIAgent(ctx context.Context, id string) (*types.AIAgent, error)
	ListAIAgents(ctx context.Context, sessionID string) ([]*types.AIAgent, error)
	GetAgentSummariesForSessions(
		ctx context.Context,
		sessionIDs []string,
	) (map[string]*types.AgentSummary, error)

	// Events (audit trail)
	GetEvents(ctx context.Context, issueID string, limit int) ([]*types.Event, error)

	// Statistics
	GetStatistics(ctx context.Context, projectID string) (*types.Statistics, error)

	// Lifecycle
	Close() error
	Path() string
}

// PlanUpload retains the exact UTF-8 content; SourceName is optional provenance.
type PlanUpload struct {
	Title      string `json:"title"`
	Content    string `json:"content"`
	SourceName string `json:"source_name,omitempty"`
}
type PlanSave struct {
	Content          string `json:"content"`
	SourceName       string `json:"source_name,omitempty"`
	ExpectedRevision int64  `json:"expected_revision"`
}
type PlanWriteResult struct {
	Plan     types.Plan                    `json:"plan"`
	Revision types.PlanRevisionWithContent `json:"revision"`
	Replay   bool                          `json:"replay"`
}
type PlanMetadataUpdate struct {
	ExpectedVersion int64   `json:"expected_version"`
	Title           *string `json:"title,omitempty"`
	Lifecycle       string  `json:"lifecycle,omitempty"`
}
type PlanCommentCreate struct {
	Content    string                   `json:"content"`
	LineNumber *int                     `json:"line_number,omitempty"`
	Anchor     *types.PlanCommentAnchor `json:"anchor,omitempty"`
}
type PlanCommentUpdate struct {
	ExpectedVersion int64                    `json:"expected_version"`
	Content         *string                  `json:"content,omitempty"`
	Anchor          *types.PlanCommentAnchor `json:"anchor,omitempty"`
	Reopen          bool                     `json:"reopen,omitempty"`
	Delete          bool                     `json:"-"`
}
type PlanDispositionRequest struct {
	CommentID               string `json:"comment_id"`
	ExpectedCommentVersion  int64  `json:"expected_comment_version"`
	ExpectedFeedbackVersion int64  `json:"expected_feedback_version"`
	Disposition             string `json:"disposition"`
	Reason                  string `json:"reason"`
}

// PlanReviewEvent exposes immutable decision evidence, including the exact
// dispositions retained by that event rather than the current feedback state.
type PlanReviewEvent struct {
	ID              string                           `json:"id"`
	PlanID          string                           `json:"plan_id"`
	Revision        int64                            `json:"revision"`
	Status          string                           `json:"status"`
	ReviewVersion   int64                            `json:"review_version"`
	FeedbackVersion int64                            `json:"feedback_version"`
	Dispositions    []*types.PlanFeedbackDisposition `json:"dispositions"`
	CreatedAt       time.Time                        `json:"created_at"`
	Actor           string                           `json:"actor"`
	SessionID       string                           `json:"session_id"`
}

// PlanCommentVersion exposes an original comment snapshot with event attribution.
// Its nested comment preserves the public anchor and unknown-revision contracts.
type PlanCommentVersion struct {
	Comment   types.PlanComment `json:"comment"`
	CreatedAt time.Time         `json:"created_at"`
	Actor     string            `json:"actor"`
	SessionID string            `json:"session_id"`
}

// DurablePlans is the project-scoped service contract for retained plan history.
//
//nolint:interfacebloat // One transactional plan service shared by API and storage consumers.
type DurablePlans interface {
	ListPlanReviewEvents(
		context.Context,
		string,
		string,
		int64,
		int,
		int,
	) ([]*PlanReviewEvent, error)
	ListPlanCommentVersions(
		context.Context,
		string,
		string,
		int64,
		string,
		int,
		int,
	) ([]*PlanCommentVersion, error)
	CreateDurablePlan(context.Context, string, string, PlanUpload) (*PlanWriteResult, error)
	SavePlanRevision(context.Context, string, string, string, PlanSave) (*PlanWriteResult, error)
	GetDurablePlan(context.Context, string, string) (*types.Plan, error)
	ListDurablePlans(context.Context, string, bool, int, int) ([]*types.Plan, error)
	ReadPlanRevision(context.Context, string, string, int64) (*types.PlanRevisionWithContent, error)
	ListPlanRevisions(context.Context, string, string, int, int) ([]*types.PlanRevision, error)
	UpdateDurablePlan(context.Context, string, string, PlanMetadataUpdate) (*types.Plan, error)
	DecidePlanRevision(
		context.Context,
		string,
		string,
		int64,
		types.PlanReviewRequest,
	) (*types.PlanRevision, error)
	CreateRevisionComment(
		context.Context,
		string,
		string,
		int64,
		PlanCommentCreate,
	) (*types.PlanComment, error)
	UpdateRevisionComment(
		context.Context,
		string,
		string,
		int64,
		string,
		PlanCommentUpdate,
	) (*types.PlanComment, error)
	ListRevisionComments(context.Context, string, string, int64, bool, int, int) ([]*types.PlanComment, error)
	AddPlanDisposition(
		context.Context,
		string,
		string,
		int64,
		PlanDispositionRequest,
	) (*types.PlanFeedbackDisposition, error)
	ListPlanDispositions(
		context.Context,
		string,
		string,
		int64,
		int,
		int,
	) ([]*types.PlanFeedbackDisposition, error)
}

var (
	ErrPlanNotFound       = errors.New("plan or revision not found in project")
	ErrPlanConflict       = errors.New("plan precondition conflict")
	ErrPlanPrecondition   = errors.New("plan precondition required")
	ErrPlanInvalid        = errors.New("invalid plan request")
	ErrUnresolvedFeedback = errors.New(
		"unresolved plan feedback requires addressed or deferred dispositions with reasons",
	)
)

// PlanProvenance retains caller-supplied attribution, not authenticated identity.
type PlanProvenance struct {
	Actor     string
	SessionID string
}
type planProvenanceKey struct{}

// WithPlanProvenance records available request attribution for append-only events.
func WithPlanProvenance(ctx context.Context, actor, sessionID string) context.Context {
	return context.WithValue(ctx, planProvenanceKey{}, PlanProvenance{Actor: actor, SessionID: sessionID})
}

// PlanProvenanceFromContext returns optional caller-supplied attribution.
func PlanProvenanceFromContext(ctx context.Context) PlanProvenance {
	value, _ := ctx.Value(planProvenanceKey{}).(PlanProvenance)
	return value
}

// LegacyPlanInventory is metadata only: paths are never dereferenced by HTTP.
// Comments have unknown content revision even when the preserved plan is imported.
type LegacyPlanInventory struct {
	types.LegacyPlan
	Comments          []*types.PlanComment `json:"comments"`
	ImportedProjectID string               `json:"imported_project_id,omitempty"`
}

// LegacyPlanImport is a local operator request, never an HTTP path request.
type LegacyPlanImport struct {
	LegacyID   string `json:"legacy_id"`
	ProjectID  string `json:"project_id"`
	SourceFile string `json:"source_file"`
	Content    string `json:"content"`
}
