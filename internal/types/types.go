// Package types defines core data structures for the arc issue tracker.
package types

import (
	"fmt"
	"time"
)

// Workspace represents a project or workspace that contains issues.
// Replaces the per-repo concept from beads with explicit workspace management.
type Workspace struct {
	ID          string    `json:"id"`          // Short hash ID (e.g., "ws-a1b2")
	Name        string    `json:"name"`        // Display name
	Path        string    `json:"path,omitempty"` // Optional: associated directory path
	Description string    `json:"description,omitempty"`
	Prefix      string    `json:"prefix"`      // Issue ID prefix (e.g., "bd")
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Validate checks if the workspace has valid field values.
func (w *Workspace) Validate() error {
	if w.Name == "" {
		return fmt.Errorf("workspace name is required")
	}
	if len(w.Name) > 100 {
		return fmt.Errorf("workspace name must be 100 characters or less")
	}
	if w.Prefix == "" {
		return fmt.Errorf("workspace prefix is required")
	}
	if len(w.Prefix) > 10 {
		return fmt.Errorf("workspace prefix must be 10 characters or less")
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
		return fmt.Errorf("title is required")
	}
	if len(i.Title) > 500 {
		return fmt.Errorf("title must be 500 characters or less (got %d)", len(i.Title))
	}
	if i.WorkspaceID == "" {
		return fmt.Errorf("workspace_id is required")
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
		return fmt.Errorf("closed issues must have closed_at timestamp")
	}
	if i.Status != StatusClosed && i.ClosedAt != nil {
		return fmt.Errorf("non-closed issues cannot have closed_at timestamp")
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
	DepBlocks        DependencyType = "blocks"
	DepParentChild   DependencyType = "parent-child"
	DepRelated       DependencyType = "related"
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

// Comment represents a comment on an issue.
type Comment struct {
	ID        int64     `json:"id"`
	IssueID   string    `json:"issue_id"`
	Author    string    `json:"author"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// Event represents an audit trail entry.
type Event struct {
	ID        int64      `json:"id"`
	IssueID   string     `json:"issue_id"`
	EventType EventType  `json:"event_type"`
	Actor     string     `json:"actor"`
	OldValue  *string    `json:"old_value,omitempty"`
	NewValue  *string    `json:"new_value,omitempty"`
	Comment   *string    `json:"comment,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
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
}
