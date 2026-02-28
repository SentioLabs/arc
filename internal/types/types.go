// Package types defines core data structures for the arc issue tracker.
package types

import (
	"errors"
	"fmt"
	"time"
)

// Validation limits for data fields.
const (
	maxWorkspaceNameLength = 100 // maximum characters for workspace name
	maxPrefixLength        = 10  // maximum characters for workspace prefix
	maxTitleLength         = 500 // maximum characters for issue title
	maxPlanTitleLength     = 200 // maximum characters for plan title
)

// Workspace represents a project or workspace that contains issues.
// Replaces the per-repo concept from beads with explicit workspace management.
type Workspace struct {
	ID          string    `json:"id"`             // Short hash ID (e.g., "ws-a1b2")
	Name        string    `json:"name"`           // Display name
	Path        string    `json:"path,omitempty"` // Optional: associated directory path
	Description string    `json:"description,omitempty"`
	Prefix      string    `json:"prefix"` // Issue ID prefix (e.g., "bd")
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Validate checks if the workspace has valid field values.
func (w *Workspace) Validate() error {
	if w.Name == "" {
		return errors.New("workspace name is required")
	}
	if len(w.Name) > maxWorkspaceNameLength {
		return fmt.Errorf("workspace name must be %d characters or less", maxWorkspaceNameLength)
	}
	if w.Prefix == "" {
		return errors.New("workspace prefix is required")
	}
	if len(w.Prefix) > maxPrefixLength {
		return fmt.Errorf("workspace prefix must be %d characters or less", maxPrefixLength)
	}
	return nil
}

// Issue represents a trackable work item.
type Issue struct {
	// Core Identification
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	ParentID    string `json:"parent_id,omitempty"` // For hierarchical child IDs (e.g., parent-id.1)

	// Issue Content
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`

	// Status & Workflow
	Status    Status    `json:"status"`
	Priority  int       `json:"priority"` // 0 (critical) - 4 (backlog)
	Rank      int       `json:"rank"`     // 0 = unranked (sorts last), 1+ = lower rank = work on first
	IssueType IssueType `json:"issue_type"`

	// Assignment
	Assignee string `json:"assignee,omitempty"`

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
	if i.WorkspaceID == "" {
		return errors.New("workspace_id is required")
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

	// SortPolicyPriority always sorts by priority → rank → created_at.
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

// Label represents a tag that can be applied to issues.
type Label struct {
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	Color       string `json:"color,omitempty"`
	Description string `json:"description,omitempty"`
}

// CommentType distinguishes between regular comments and inline plans.
type CommentType string

const (
	CommentTypeComment CommentType = "comment"
	CommentTypePlan    CommentType = "plan"
)

// IsValid checks if the comment type value is valid.
func (c CommentType) IsValid() bool {
	switch c {
	case CommentTypeComment, CommentTypePlan:
		return true
	}
	return false
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
)

// IssueFilter is used to filter issue queries.
type IssueFilter struct {
	WorkspaceID string     // Required: filter by workspace
	Status      *Status    // Filter by status
	Priority    *int       // Filter by priority
	IssueType   *IssueType // Filter by issue type
	Assignee    *string    // Filter by assignee
	Labels      []string   // AND semantics: issue must have ALL these labels
	Query       string     // Full-text search in title/description
	IDs         []string   // Filter by specific issue IDs
	Limit       int        // Maximum results to return
	Offset      int        // Pagination offset
}

// WorkFilter is used to filter ready work queries.
type WorkFilter struct {
	WorkspaceID string     // Required: filter by workspace
	Status      *Status    // Filter by status
	IssueType   *IssueType // Filter by issue type
	Priority    *int       // Filter by priority
	Assignee    *string    // Filter by assignee
	Unassigned  bool       // Filter for unassigned issues
	Labels      []string   // AND semantics
	SortPolicy  SortPolicy // Sort policy: hybrid (default), priority, oldest
	Limit       int        // Maximum results
}

// Statistics provides aggregate metrics for a workspace.
type Statistics struct {
	WorkspaceID      string  `json:"workspace_id"`
	TotalIssues      int     `json:"total_issues"`
	OpenIssues       int     `json:"open_issues"`
	InProgressIssues int     `json:"in_progress_issues"`
	ClosedIssues     int     `json:"closed_issues"`
	BlockedIssues    int     `json:"blocked_issues"`
	DeferredIssues   int     `json:"deferred_issues"`
	ReadyIssues      int     `json:"ready_issues"`
	AvgLeadTimeHours float64 `json:"avg_lead_time_hours,omitempty"`
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
	PlanContext  *PlanContext  `json:"plan_context,omitempty"`
}

// Plan represents a shared plan that can be linked to multiple issues.
type Plan struct {
	ID          string    `json:"id"` // plan.xxxxx format
	WorkspaceID string    `json:"workspace_id"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	// LinkedIssues contains issue IDs linked to this plan (populated on detail views)
	LinkedIssues []string `json:"linked_issues,omitempty"`
}

// Validate checks if the plan has valid field values.
func (p *Plan) Validate() error {
	if p.Title == "" {
		return errors.New("plan title is required")
	}
	if len(p.Title) > maxPlanTitleLength {
		return fmt.Errorf("plan title must be %d characters or less", maxPlanTitleLength)
	}
	if p.WorkspaceID == "" {
		return errors.New("workspace_id is required")
	}
	return nil
}

// PlanContext aggregates all plans relevant to an issue.
// It supports three patterns:
// 1. InlinePlan: A plan comment directly on the issue
// 2. ParentPlan: A plan inherited from a parent issue (via parent-child dependency)
// 3. SharedPlans: Standalone plans linked to this issue
type PlanContext struct {
	// InlinePlan is a plan comment directly on this issue
	InlinePlan *Comment `json:"inline_plan,omitempty"`
	// ParentPlan is a plan inherited from a parent issue
	ParentPlan *Comment `json:"parent_plan,omitempty"`
	// ParentIssueID is the ID of the parent issue if ParentPlan is set
	ParentIssueID string `json:"parent_issue_id,omitempty"`
	// SharedPlans are standalone plans linked to this issue
	SharedPlans []*Plan `json:"shared_plans,omitempty"`
}

// HasPlan returns true if any plan is available in this context.
func (pc *PlanContext) HasPlan() bool {
	if pc == nil {
		return false
	}
	return pc.InlinePlan != nil || pc.ParentPlan != nil || len(pc.SharedPlans) > 0
}
