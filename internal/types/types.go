// Package types defines core data structures for the arc issue tracker.
package types

import (
	"errors"
	"fmt"
	"time"
)

// Validation limits for data fields.
const (
	maxProjectNameLength = 100 // maximum characters for project name
	maxTitleLength       = 500 // maximum characters for issue title
)

// MaxPrefixLength is the maximum allowed project prefix length.
// Must match project.MaxPrefixLength (kept separate to avoid circular imports).
const MaxPrefixLength = 15

// Project represents a project that contains issues.
// Previously named Workspace; renamed to clarify that this is the issue container.
type Project struct {
	ID          string    `json:"id"`   // Short hash ID (e.g., "proj-a1b2")
	Name        string    `json:"name"` // Display name
	Description string    `json:"description,omitempty"`
	Prefix      string    `json:"prefix"` // Issue ID prefix (e.g., "bd")
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Validate checks if the project has valid field values.
func (p *Project) Validate() error {
	if p.Name == "" {
		return errors.New("project name is required")
	}
	if len(p.Name) > maxProjectNameLength {
		return fmt.Errorf("project name must be %d characters or less", maxProjectNameLength)
	}
	if p.Prefix == "" {
		return errors.New("project prefix is required")
	}
	if len(p.Prefix) > MaxPrefixLength {
		return fmt.Errorf("project prefix must be %d characters or less", MaxPrefixLength)
	}
	return nil
}

// Issue represents a trackable work item.
type Issue struct {
	// Core Identification
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	ParentID  string `json:"parent_id,omitempty"` // For hierarchical child IDs (e.g., parent-id.1)

	// Issue Content
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`

	// Status & Workflow
	Status    Status    `json:"status"`
	Priority  int       `json:"priority"` // 0 (critical) - 4 (backlog)
	Rank      int       `json:"rank"`     // 0 = unranked (sorts last), 1+ = lower rank = work on first
	IssueType IssueType `json:"issue_type"`

	// AI Session Tracking
	AISessionID string `json:"ai_session_id,omitempty"` // Claude Code session UUID

	// Timestamps
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ClosedAt    *time.Time `json:"closed_at,omitempty"`
	CloseReason string     `json:"close_reason,omitempty"`

	// External Integration
	ExternalRef string `json:"external_ref,omitempty"` // e.g., "gh-9", "jira-ABC"

	// Relational Data (populated for detail views)
	Labels       []string      `json:"labels,omitempty"`
	Dependencies []*Dependency `json:"dependencies,omitempty"`
	Comments     []*Comment    `json:"comments,omitempty"`
}

// Validate checks if the issue has valid field values.
func (i *Issue) Validate() error {
	if i.Title == "" {
		return errors.New("title is required")
	}
	if len(i.Title) > maxTitleLength {
		return fmt.Errorf("title must be %d characters or less (got %d)", maxTitleLength, len(i.Title))
	}
	if i.ProjectID == "" {
		return errors.New("project_id is required")
	}
	if i.Priority < 0 || i.Priority > 4 {
		return fmt.Errorf("priority must be between 0 and 4 (got %d)", i.Priority)
	}
	if !i.Status.IsValid() {
		return fmt.Errorf("invalid status: %s", i.Status)
	}
	if !i.IssueType.IsValid() {
		return fmt.Errorf("invalid issue type: %s", i.IssueType)
	}
	// Enforce closed_at invariant
	if i.Status == StatusClosed && i.ClosedAt == nil {
		return errors.New("closed issues must have closed_at timestamp")
	}
	if i.Status != StatusClosed && i.ClosedAt != nil {
		return errors.New("non-closed issues cannot have closed_at timestamp")
	}
	return nil
}

// SetDefaults applies default values for missing fields.
func (i *Issue) SetDefaults() {
	if i.Status == "" {
		i.Status = StatusOpen
	}
	if i.IssueType == "" {
		i.IssueType = TypeTask
	}
	if i.Priority == 0 {
		i.Priority = 2 // Default priority
	}
}

// Status represents the current state of an issue.
type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusBlocked    Status = "blocked"
	StatusDeferred   Status = "deferred"
	StatusClosed     Status = "closed"
)

// IsValid checks if the status value is valid.
func (s Status) IsValid() bool {
	switch s {
	case StatusOpen, StatusInProgress, StatusBlocked, StatusDeferred, StatusClosed:
		return true
	}
	return false
}

// AllStatuses returns all valid status values.
func AllStatuses() []Status {
	return []Status{StatusOpen, StatusInProgress, StatusBlocked, StatusDeferred, StatusClosed}
}

// IssueType categorizes the kind of work.
type IssueType string

const (
	TypeBug     IssueType = "bug"
	TypeFeature IssueType = "feature"
	TypeTask    IssueType = "task"
	TypeEpic    IssueType = "epic"
	TypeChore   IssueType = "chore"
)

// IsValid checks if the issue type is valid.
func (t IssueType) IsValid() bool {
	switch t {
	case TypeBug, TypeFeature, TypeTask, TypeEpic, TypeChore:
		return true
	}
	return false
}

// AllIssueTypes returns all valid issue type values.
func AllIssueTypes() []IssueType {
	return []IssueType{TypeBug, TypeFeature, TypeTask, TypeEpic, TypeChore}
}

// SortPolicy defines how ready work should be sorted.
type SortPolicy string

const (
	// SortPolicyHybrid sorts recent issues (<48h) by priority/rank, older issues by age.
	// This prevents backlog starvation while keeping high-priority recent work visible.
	SortPolicyHybrid SortPolicy = "hybrid"

	// SortPolicyPriority always sorts by priority -> rank -> created_at.
	SortPolicyPriority SortPolicy = "priority"

	// SortPolicyOldest always sorts by created_at (oldest first) for backlog clearing.
	SortPolicyOldest SortPolicy = "oldest"
)

// IsValid checks if the sort policy is valid.
func (s SortPolicy) IsValid() bool {
	switch s {
	case SortPolicyHybrid, SortPolicyPriority, SortPolicyOldest:
		return true
	}
	return false
}

// AllSortPolicies returns all valid sort policy values.
func AllSortPolicies() []SortPolicy {
	return []SortPolicy{SortPolicyHybrid, SortPolicyPriority, SortPolicyOldest}
}

// Dependency represents a relationship between issues.
type Dependency struct {
	IssueID     string         `json:"issue_id"`
	DependsOnID string         `json:"depends_on_id"`
	Type        DependencyType `json:"type"`
	CreatedAt   time.Time      `json:"created_at"`
	CreatedBy   string         `json:"created_by,omitempty"`
}

// DependencyType categorizes the relationship.
type DependencyType string

const (
	DepBlocks         DependencyType = "blocks"
	DepParentChild    DependencyType = "parent-child"
	DepRelated        DependencyType = "related"
	DepDiscoveredFrom DependencyType = "discovered-from"
)

// IsValid checks if the dependency type value is valid.
func (d DependencyType) IsValid() bool {
	switch d {
	case DepBlocks, DepParentChild, DepRelated, DepDiscoveredFrom:
		return true
	}
	return false
}

// AffectsReadyWork returns true if this dependency type blocks work.
func (d DependencyType) AffectsReadyWork() bool {
	return d == DepBlocks || d == DepParentChild
}

// AllDependencyTypes returns all valid dependency type values.
func AllDependencyTypes() []DependencyType {
	return []DependencyType{DepBlocks, DepParentChild, DepRelated, DepDiscoveredFrom}
}

// Label represents a global tag that can be applied to issues.
type Label struct {
	Name        string `json:"name"`
	Color       string `json:"color,omitempty"`
	Description string `json:"description,omitempty"`
}

// CommentType distinguishes between different comment types.
type CommentType string

const (
	CommentTypeComment CommentType = "comment"
)

// IsValid checks if the comment type value is valid.
func (c CommentType) IsValid() bool {
	return c == CommentTypeComment
}

// Comment represents a comment on an issue.
type Comment struct {
	ID          int64       `json:"id"`
	IssueID     string      `json:"issue_id"`
	Author      string      `json:"author"`
	Text        string      `json:"text"`
	CommentType CommentType `json:"comment_type"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// Event represents an audit trail entry.
type Event struct {
	ID        int64     `json:"id"`
	IssueID   string    `json:"issue_id"`
	EventType EventType `json:"event_type"`
	Actor     string    `json:"actor"`
	OldValue  *string   `json:"old_value,omitempty"`
	NewValue  *string   `json:"new_value,omitempty"`
	Comment   *string   `json:"comment,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// EventType categorizes audit trail events.
type EventType string

const (
	EventCreated           EventType = "created"
	EventUpdated           EventType = "updated"
	EventStatusChanged     EventType = "status_changed"
	EventCommented         EventType = "commented"
	EventClosed            EventType = "closed"
	EventReopened          EventType = "reopened"
	EventDependencyAdded   EventType = "dependency_added"
	EventDependencyRemoved EventType = "dependency_removed"
	EventLabelAdded        EventType = "label_added"
	EventLabelRemoved      EventType = "label_removed"
	EventMerged            EventType = "merged"
)

// IssueFilter is used to filter issue queries.
type IssueFilter struct {
	ProjectID   string      // Required: filter by project
	Statuses    []Status    // Filter by statuses (multi-select, empty means all)
	Priorities  []int       // Filter by priorities (multi-select, empty means all)
	IssueTypes  []IssueType // Filter by issue types (multi-select, empty means all)
	AISessionID *string     // Filter by AI session ID
	Labels      []string    // AND semantics: issue must have ALL these labels
	ParentID    string      // Filter by parent issue (via parent-child dependency)
	Query       string      // Full-text search in title/description
	IDs         []string    // Filter by specific issue IDs
	Limit       int         // Maximum results to return
	Offset      int         // Pagination offset
}

// WorkFilter is used to filter ready work queries.
type WorkFilter struct {
	ProjectID  string     // Required: filter by project
	Status     *Status    // Filter by status
	IssueType  *IssueType // Filter by issue type
	Priority   *int       // Filter by priority
	Labels     []string   // AND semantics
	SortPolicy SortPolicy // Sort policy: hybrid (default), priority, oldest
	Limit      int        // Maximum results
}

// Statistics provides aggregate metrics for a project.
type Statistics struct {
	ProjectID        string  `json:"project_id"`
	TotalIssues      int     `json:"total_issues"`
	OpenIssues       int     `json:"open_issues"`
	InProgressIssues int     `json:"in_progress_issues"`
	ClosedIssues     int     `json:"closed_issues"`
	BlockedIssues    int     `json:"blocked_issues"`
	DeferredIssues   int     `json:"deferred_issues"`
	ReadyIssues      int     `json:"ready_issues"`
	AvgLeadTimeHours float64 `json:"avg_lead_time_hours,omitempty"`
}

// MergeResult contains the outcome of merging one or more source projects into a target.
type MergeResult struct {
	TargetProject  *Project `json:"target_project"`
	IssuesMoved    int      `json:"issues_moved"`
	SourcesDeleted []string `json:"sources_deleted"`
}

// ShareKind distinguishes local-only shares from hosted (published) shares.
type ShareKind string

const (
	ShareKindLocal  ShareKind = "local"
	ShareKindShared ShareKind = "shared"
)

// IsValid checks if the share kind value is valid.
func (k ShareKind) IsValid() bool {
	return k == ShareKindLocal || k == ShareKindShared
}

// AllShareKinds returns all valid share kind values.
func AllShareKinds() []ShareKind {
	return []ShareKind{ShareKindLocal, ShareKindShared}
}

// Share represents an entry in the author-side keyring of paste shares created on this machine.
type Share struct {
	ID        string    `json:"id"`
	Kind      ShareKind `json:"kind"`
	URL       string    `json:"url"`
	KeyB64Url string    `json:"key_b64url"`
	EditToken string    `json:"edit_token"`
	PlanFile  string    `json:"plan_file,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Validate checks if the share has valid field values.
func (s *Share) Validate() error {
	if s.ID == "" {
		return errors.New("share: id is required")
	}
	if !s.Kind.IsValid() {
		return fmt.Errorf("share: invalid kind %q", s.Kind)
	}
	if s.URL == "" {
		return errors.New("share: url is required")
	}
	if s.KeyB64Url == "" {
		return errors.New("share: key_b64url is required")
	}
	if s.EditToken == "" {
		return errors.New("share: edit_token is required")
	}
	return nil
}

// Workspace represents a directory path associated with a project.
// Multiple workspaces can be linked to a single project to support multi-directory projects.
// Previously named WorkspacePath; renamed because this IS the workspace (a directory where work happens).
type Workspace struct {
	ID             string     `json:"id"`
	ProjectID      string     `json:"project_id"`
	Path           string     `json:"path"`
	Label          string     `json:"label,omitempty"`
	Hostname       string     `json:"hostname,omitempty"`
	GitRemote      string     `json:"git_remote,omitempty"`
	PathType       string     `json:"path_type,omitempty"`
	LastAccessedAt *time.Time `json:"last_accessed_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// Validate checks if the workspace has valid field values.
func (w *Workspace) Validate() error {
	if w.Path == "" {
		return errors.New("path is required")
	}
	if w.ProjectID == "" {
		return errors.New("project_id is required")
	}
	return nil
}

// OpenChildrenError is returned when attempting to close an issue that has open child issues.
type OpenChildrenError struct {
	IssueID  string  // The issue that cannot be closed
	Children []Issue // The open child issues
}

// Error implements the error interface.
func (e *OpenChildrenError) Error() string {
	return fmt.Sprintf("cannot close issue %s: %d open child issue(s)", e.IssueID, len(e.Children))
}

// BlockedIssue extends Issue with blocking information.
type BlockedIssue struct {
	Issue
	BlockedByCount int      `json:"blocked_by_count"`
	BlockedBy      []string `json:"blocked_by"`
}

// IssueDetails extends Issue with full relational data.
type IssueDetails struct {
	Issue
	Labels       []string      `json:"labels,omitempty"`
	Dependencies []*Dependency `json:"dependencies,omitempty"`
	Dependents   []*Dependency `json:"dependents,omitempty"`
	Comments     []*Comment    `json:"comments,omitempty"`
}

// Plan status constants.
const (
	PlanStatusDraft    = "draft"
	PlanStatusInReview = "in_review"
	PlanStatusApproved = "approved"
	PlanStatusRejected = "rejected"
)

// Plan represents an ephemeral review artifact backed by a filesystem markdown file.
type Plan struct {
	ID        string    `json:"id"`
	FilePath  string    `json:"file_path"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PlanComment is a review comment on a plan, optionally anchored to a line number.
type PlanComment struct {
	ID         string    `json:"id"`
	PlanID     string    `json:"plan_id"`
	LineNumber *int      `json:"line_number,omitempty"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

// PlanWithContent combines plan metadata with the file content read from disk.
type PlanWithContent struct {
	Plan
	Content string `json:"content"`
}

// AISession represents an AI coding session (e.g., a Claude Code conversation).
type AISession struct {
	ID             string    `json:"id"`
	ProjectID      string    `json:"project_id"`
	TranscriptPath string    `json:"transcript_path"`
	CWD            string    `json:"cwd,omitempty"`
	StartedAt      time.Time `json:"started_at"`
}

// AIAgent represents a sub-agent spawned within an AI session.
type AIAgent struct {
	ID           string    `json:"id"`
	SessionID    string    `json:"session_id"`
	Description  string    `json:"description,omitempty"`
	Prompt       string    `json:"prompt,omitempty"`
	AgentType    string    `json:"agent_type,omitempty"`
	Model        string    `json:"model,omitempty"`
	Status       string    `json:"status"`
	DurationMs   *int      `json:"duration_ms,omitempty"`
	TotalTokens  *int      `json:"total_tokens,omitempty"`
	ToolUseCount *int      `json:"tool_use_count,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// AgentSummary contains aggregated agent status counts for a session.
type AgentSummary struct {
	AgentCount     int `json:"agent_count"`
	RunningCount   int `json:"running_count"`
	CompletedCount int `json:"completed_count"`
	ErrorCount     int `json:"error_count"`
}

// ProjectResolution contains the result of resolving a project by path.
type ProjectResolution struct {
	ProjectID   string `json:"project_id"`
	ProjectName string `json:"project_name"`
	PathID      string `json:"path_id"`
}
