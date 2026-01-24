package types

import (
	"testing"
	"time"
)

func TestPlanValidate(t *testing.T) {
	tests := []struct {
		name    string
		plan    Plan
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid plan",
			plan: Plan{
				ID:          "plan.abc123",
				WorkspaceID: "ws-test",
				Title:       "Test Plan",
				Content:     "Plan content here",
			},
			wantErr: false,
		},
		{
			name: "missing title",
			plan: Plan{
				ID:          "plan.abc123",
				WorkspaceID: "ws-test",
				Title:       "",
				Content:     "Plan content",
			},
			wantErr: true,
			errMsg:  "plan title is required",
		},
		{
			name: "title too long",
			plan: Plan{
				ID:          "plan.abc123",
				WorkspaceID: "ws-test",
				Title:       string(make([]byte, 201)), // 201 chars
				Content:     "Content",
			},
			wantErr: true,
			errMsg:  "plan title must be 200 characters or less",
		},
		{
			name: "missing workspace_id",
			plan: Plan{
				ID:          "plan.abc123",
				WorkspaceID: "",
				Title:       "Test Plan",
				Content:     "Content",
			},
			wantErr: true,
			errMsg:  "workspace_id is required",
		},
		{
			name: "empty content is valid",
			plan: Plan{
				ID:          "plan.abc123",
				WorkspaceID: "ws-test",
				Title:       "Test Plan",
				Content:     "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.plan.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Plan.Validate() expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("Plan.Validate() error = %q, want %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Plan.Validate() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestPlanContextHasPlan(t *testing.T) {
	tests := []struct {
		name    string
		pc      *PlanContext
		want    bool
	}{
		{
			name: "nil context",
			pc:   nil,
			want: false,
		},
		{
			name: "empty context",
			pc:   &PlanContext{},
			want: false,
		},
		{
			name: "with inline plan",
			pc: &PlanContext{
				InlinePlan: &Comment{
					ID:   1,
					Text: "Test plan",
				},
			},
			want: true,
		},
		{
			name: "with parent plan",
			pc: &PlanContext{
				ParentPlan: &Comment{
					ID:   2,
					Text: "Parent plan",
				},
				ParentIssueID: "issue-123",
			},
			want: true,
		},
		{
			name: "with shared plans",
			pc: &PlanContext{
				SharedPlans: []*Plan{
					{ID: "plan.abc123", Title: "Shared"},
				},
			},
			want: true,
		},
		{
			name: "with all plan types",
			pc: &PlanContext{
				InlinePlan:    &Comment{ID: 1, Text: "Inline"},
				ParentPlan:    &Comment{ID: 2, Text: "Parent"},
				ParentIssueID: "issue-123",
				SharedPlans:   []*Plan{{ID: "plan.abc123", Title: "Shared"}},
			},
			want: true,
		},
		{
			name: "with empty shared plans slice",
			pc: &PlanContext{
				SharedPlans: []*Plan{},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pc.HasPlan()
			if got != tt.want {
				t.Errorf("PlanContext.HasPlan() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommentTypeIsValid(t *testing.T) {
	tests := []struct {
		name string
		ct   CommentType
		want bool
	}{
		{"comment type", CommentTypeComment, true},
		{"plan type", CommentTypePlan, true},
		{"empty string", CommentType(""), false},
		{"invalid type", CommentType("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ct.IsValid(); got != tt.want {
				t.Errorf("CommentType.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatusIsValid(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{"open", StatusOpen, true},
		{"in_progress", StatusInProgress, true},
		{"blocked", StatusBlocked, true},
		{"deferred", StatusDeferred, true},
		{"closed", StatusClosed, true},
		{"empty", Status(""), false},
		{"invalid", Status("pending"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.want {
				t.Errorf("Status.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIssueTypeIsValid(t *testing.T) {
	tests := []struct {
		name      string
		issueType IssueType
		want      bool
	}{
		{"bug", TypeBug, true},
		{"feature", TypeFeature, true},
		{"task", TypeTask, true},
		{"epic", TypeEpic, true},
		{"chore", TypeChore, true},
		{"empty", IssueType(""), false},
		{"invalid", IssueType("story"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.issueType.IsValid(); got != tt.want {
				t.Errorf("IssueType.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDependencyTypeIsValid(t *testing.T) {
	tests := []struct {
		name    string
		depType DependencyType
		want    bool
	}{
		{"blocks", DepBlocks, true},
		{"parent-child", DepParentChild, true},
		{"related", DepRelated, true},
		{"discovered-from", DepDiscoveredFrom, true},
		{"empty", DependencyType(""), false},
		{"invalid", DependencyType("after"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.depType.IsValid(); got != tt.want {
				t.Errorf("DependencyType.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDependencyTypeAffectsReadyWork(t *testing.T) {
	tests := []struct {
		name    string
		depType DependencyType
		want    bool
	}{
		{"blocks affects ready work", DepBlocks, true},
		{"parent-child affects ready work", DepParentChild, true},
		{"related does not affect ready work", DepRelated, false},
		{"discovered-from does not affect ready work", DepDiscoveredFrom, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.depType.AffectsReadyWork(); got != tt.want {
				t.Errorf("DependencyType.AffectsReadyWork() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSortPolicyIsValid(t *testing.T) {
	tests := []struct {
		name   string
		policy SortPolicy
		want   bool
	}{
		{"hybrid", SortPolicyHybrid, true},
		{"priority", SortPolicyPriority, true},
		{"oldest", SortPolicyOldest, true},
		{"empty", SortPolicy(""), false},
		{"invalid", SortPolicy("random"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.policy.IsValid(); got != tt.want {
				t.Errorf("SortPolicy.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIssueValidate(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		issue   Issue
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid open issue",
			issue: Issue{
				Title:       "Test Issue",
				WorkspaceID: "ws-test",
				Status:      StatusOpen,
				Priority:    2,
				IssueType:   TypeTask,
			},
			wantErr: false,
		},
		{
			name: "valid closed issue",
			issue: Issue{
				Title:       "Test Issue",
				WorkspaceID: "ws-test",
				Status:      StatusClosed,
				Priority:    2,
				IssueType:   TypeTask,
				ClosedAt:    &now,
			},
			wantErr: false,
		},
		{
			name: "missing title",
			issue: Issue{
				Title:       "",
				WorkspaceID: "ws-test",
				Status:      StatusOpen,
				Priority:    2,
				IssueType:   TypeTask,
			},
			wantErr: true,
			errMsg:  "title is required",
		},
		{
			name: "missing workspace_id",
			issue: Issue{
				Title:       "Test Issue",
				WorkspaceID: "",
				Status:      StatusOpen,
				Priority:    2,
				IssueType:   TypeTask,
			},
			wantErr: true,
			errMsg:  "workspace_id is required",
		},
		{
			name: "invalid priority too low",
			issue: Issue{
				Title:       "Test Issue",
				WorkspaceID: "ws-test",
				Status:      StatusOpen,
				Priority:    -1,
				IssueType:   TypeTask,
			},
			wantErr: true,
		},
		{
			name: "invalid priority too high",
			issue: Issue{
				Title:       "Test Issue",
				WorkspaceID: "ws-test",
				Status:      StatusOpen,
				Priority:    5,
				IssueType:   TypeTask,
			},
			wantErr: true,
		},
		{
			name: "invalid status",
			issue: Issue{
				Title:       "Test Issue",
				WorkspaceID: "ws-test",
				Status:      Status("invalid"),
				Priority:    2,
				IssueType:   TypeTask,
			},
			wantErr: true,
		},
		{
			name: "closed without closed_at",
			issue: Issue{
				Title:       "Test Issue",
				WorkspaceID: "ws-test",
				Status:      StatusClosed,
				Priority:    2,
				IssueType:   TypeTask,
				ClosedAt:    nil,
			},
			wantErr: true,
			errMsg:  "closed issues must have closed_at timestamp",
		},
		{
			name: "not closed but has closed_at",
			issue: Issue{
				Title:       "Test Issue",
				WorkspaceID: "ws-test",
				Status:      StatusOpen,
				Priority:    2,
				IssueType:   TypeTask,
				ClosedAt:    &now,
			},
			wantErr: true,
			errMsg:  "non-closed issues cannot have closed_at timestamp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.issue.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Issue.Validate() expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("Issue.Validate() error = %q, want %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Issue.Validate() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestIssueSetDefaults(t *testing.T) {
	issue := Issue{
		Title:       "Test Issue",
		WorkspaceID: "ws-test",
	}

	issue.SetDefaults()

	if issue.Status != StatusOpen {
		t.Errorf("SetDefaults() Status = %v, want %v", issue.Status, StatusOpen)
	}
	if issue.IssueType != TypeTask {
		t.Errorf("SetDefaults() IssueType = %v, want %v", issue.IssueType, TypeTask)
	}
	if issue.Priority != 2 {
		t.Errorf("SetDefaults() Priority = %v, want %v", issue.Priority, 2)
	}
}

func TestWorkspaceValidate(t *testing.T) {
	tests := []struct {
		name    string
		ws      Workspace
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid workspace",
			ws: Workspace{
				Name:   "Test Workspace",
				Prefix: "test",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			ws: Workspace{
				Name:   "",
				Prefix: "test",
			},
			wantErr: true,
			errMsg:  "workspace name is required",
		},
		{
			name: "name too long",
			ws: Workspace{
				Name:   string(make([]byte, 101)),
				Prefix: "test",
			},
			wantErr: true,
			errMsg:  "workspace name must be 100 characters or less",
		},
		{
			name: "missing prefix",
			ws: Workspace{
				Name:   "Test",
				Prefix: "",
			},
			wantErr: true,
			errMsg:  "workspace prefix is required",
		},
		{
			name: "prefix too long",
			ws: Workspace{
				Name:   "Test",
				Prefix: "verylongprefix",
			},
			wantErr: true,
			errMsg:  "workspace prefix must be 10 characters or less",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ws.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Workspace.Validate() expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("Workspace.Validate() error = %q, want %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Workspace.Validate() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestAllStatuses(t *testing.T) {
	statuses := AllStatuses()
	expected := []Status{StatusOpen, StatusInProgress, StatusBlocked, StatusDeferred, StatusClosed}

	if len(statuses) != len(expected) {
		t.Errorf("AllStatuses() returned %d items, want %d", len(statuses), len(expected))
	}

	for i, s := range expected {
		if statuses[i] != s {
			t.Errorf("AllStatuses()[%d] = %v, want %v", i, statuses[i], s)
		}
	}
}

func TestAllIssueTypes(t *testing.T) {
	types := AllIssueTypes()
	expected := []IssueType{TypeBug, TypeFeature, TypeTask, TypeEpic, TypeChore}

	if len(types) != len(expected) {
		t.Errorf("AllIssueTypes() returned %d items, want %d", len(types), len(expected))
	}

	for i, it := range expected {
		if types[i] != it {
			t.Errorf("AllIssueTypes()[%d] = %v, want %v", i, types[i], it)
		}
	}
}

func TestAllDependencyTypes(t *testing.T) {
	types := AllDependencyTypes()
	expected := []DependencyType{DepBlocks, DepParentChild, DepRelated, DepDiscoveredFrom}

	if len(types) != len(expected) {
		t.Errorf("AllDependencyTypes() returned %d items, want %d", len(types), len(expected))
	}

	for i, dt := range expected {
		if types[i] != dt {
			t.Errorf("AllDependencyTypes()[%d] = %v, want %v", i, types[i], dt)
		}
	}
}

func TestAllSortPolicies(t *testing.T) {
	policies := AllSortPolicies()
	expected := []SortPolicy{SortPolicyHybrid, SortPolicyPriority, SortPolicyOldest}

	if len(policies) != len(expected) {
		t.Errorf("AllSortPolicies() returned %d items, want %d", len(policies), len(expected))
	}

	for i, sp := range expected {
		if policies[i] != sp {
			t.Errorf("AllSortPolicies()[%d] = %v, want %v", i, policies[i], sp)
		}
	}
}
