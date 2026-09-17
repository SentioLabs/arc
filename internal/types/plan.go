package types

import "time"

type PlanReference struct {
	PlanID   string `json:"plan_id"`
	Revision int64  `json:"revision"`
}

type Plan struct {
	ID              string    `json:"id"`
	ProjectID       string    `json:"project_id"`
	Title           string    `json:"title"`
	Lifecycle       string    `json:"lifecycle"`
	HeadRevision    int64     `json:"head_revision"`
	Version         int64     `json:"version"`
	FeedbackVersion int64     `json:"feedback_version"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type PlanRevision struct {
	PlanID        string    `json:"plan_id"`
	Revision      int64     `json:"revision"`
	ContentSHA256 string    `json:"content_sha256"`
	ContentBytes  int64     `json:"content_bytes"`
	ReviewStatus  string    `json:"review_status"`
	ReviewVersion int64     `json:"review_version"`
	CreatedAt     time.Time `json:"created_at"`
}

type PlanRevisionWithContent struct {
	PlanRevision
	Content string `json:"content"`
}

type GoverningPlan struct {
	ContainerID   string                 `json:"container_id"`
	ContainerType IssueType              `json:"container_type"`
	Reference     PlanReference          `json:"reference"`
	Context       []GoverningPlanContext `json:"context"`
}

type GoverningPlanContext struct {
	ContainerID   string        `json:"container_id"`
	ContainerType IssueType     `json:"container_type"`
	Reference     PlanReference `json:"reference"`
}

type ExpectedGovernance struct {
	// The request must include this object. Explicit null means expected unlinked.
	Governing       *GoverningPlan `json:"governing"`
	ContractVersion int64          `json:"contract_version"`
}

type PlanReviewRequest struct {
	Status                  string `json:"status"`
	ExpectedHead            int64  `json:"expected_head"`
	ExpectedReviewVersion   int64  `json:"expected_review_version"`
	ExpectedFeedbackVersion int64  `json:"expected_feedback_version"`
}

type PlanFeedbackDisposition struct {
	ID             string    `json:"id"`
	PlanID         string    `json:"plan_id"`
	TargetRevision int64     `json:"target_revision"`
	CommentID      string    `json:"comment_id"`
	CommentVersion int64     `json:"comment_version"`
	Disposition    string    `json:"disposition"` // addressed or deferred
	Reason         string    `json:"reason"`
	CreatedAt      time.Time `json:"created_at"`
}

type ExecutionEvidenceRequest struct {
	Expected *ExpectedGovernance `json:"expected"`
	Phase    string              `json:"phase"` // build, review, verify
	Evidence string              `json:"evidence"`
}

type ReconciledTask struct {
	IssueID      string             `json:"issue_id"`
	Expected     ExpectedGovernance `json:"expected"`
	Disposition  string             `json:"disposition"` // unchanged, updated, follow_up
	Reason       string             `json:"reason"`
	Title        *string            `json:"title,omitempty"`
	Description  *string            `json:"description,omitempty"`
	FollowUpKeys []string           `json:"follow_up_keys"`
}

type ReconciliationFollowUp struct {
	Key         string    `json:"key"`
	ParentID    string    `json:"parent_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	IssueType   IssueType `json:"issue_type"`
	Priority    int       `json:"priority"`
}

type ReconciliationEdge struct {
	IssueID     string         `json:"issue_id"`
	DependsOnID string         `json:"depends_on_id"`
	Type        DependencyType `json:"type"`
	Remove      bool           `json:"remove"`
}

type ReconciliationTypeChange struct {
	IssueID                 string    `json:"issue_id"`
	ExpectedContractVersion int64     `json:"expected_contract_version"`
	IssueType               IssueType `json:"issue_type"`
}

type ReconciledContainerPin struct {
	ContainerID              string         `json:"container_id"`
	ExpectedContainerVersion int64          `json:"expected_container_version"`
	ExpectedPin              *PlanReference `json:"expected_pin"`
	TargetPin                *PlanReference `json:"target_pin"`
	Disposition              string         `json:"disposition"` // compatible or updated
	Reason                   string         `json:"reason"`
}

type PlanAdoptionRequest struct {
	ExpectedContainerVersion     int64                      `json:"expected_container_version"`
	ExpectedGovernanceGeneration int64                      `json:"expected_governance_generation"`
	ExpectedPin                  *PlanReference             `json:"expected_pin"`
	TargetPin                    *PlanReference             `json:"target_pin"`
	Tasks                        []ReconciledTask           `json:"tasks"`
	ContainerPins                []ReconciledContainerPin   `json:"container_pins"`
	FollowUps                    []ReconciliationFollowUp   `json:"follow_ups"`
	Edges                        []ReconciliationEdge       `json:"edges"`
	TypeChanges                  []ReconciliationTypeChange `json:"type_changes"`
	DryRun                       bool                       `json:"dry_run"`
}
