package types //nolint:testpackage // tests use internal validation functions

import (
	"fmt"
	"testing"
	"time"
)

// TestPlanValidate removed — Plan no longer has Validate() method.
// New plan types are tested in plan_test.go.

func TestCommentTypeIsValid(t *testing.T) {
	tests := []struct {
		name string
		ct   CommentType
		want bool
	}{
		{"comment type", CommentTypeComment, true},
		{"plan type is no longer valid", CommentType("plan"), false},
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
				Title:     "Test Issue",
				ProjectID: "proj-test",
				Status:    StatusOpen,
				Priority:  2,
				IssueType: TypeTask,
			},
			wantErr: false,
		},
		{
			name: "valid closed issue",
			issue: Issue{
				Title:     "Test Issue",
				ProjectID: "proj-test",
				Status:    StatusClosed,
				Priority:  2,
				IssueType: TypeTask,
				ClosedAt:  &now,
			},
			wantErr: false,
		},
		{
			name: "missing title",
			issue: Issue{
				Title:     "",
				ProjectID: "proj-test",
				Status:    StatusOpen,
				Priority:  2,
				IssueType: TypeTask,
			},
			wantErr: true,
			errMsg:  "title is required",
		},
		{
			name: "missing project_id",
			issue: Issue{
				Title:     "Test Issue",
				ProjectID: "",
				Status:    StatusOpen,
				Priority:  2,
				IssueType: TypeTask,
			},
			wantErr: true,
			errMsg:  "project_id is required",
		},
		{
			name: "invalid priority too low",
			issue: Issue{
				Title:     "Test Issue",
				ProjectID: "proj-test",
				Status:    StatusOpen,
				Priority:  -1,
				IssueType: TypeTask,
			},
			wantErr: true,
		},
		{
			name: "invalid priority too high",
			issue: Issue{
				Title:     "Test Issue",
				ProjectID: "proj-test",
				Status:    StatusOpen,
				Priority:  5,
				IssueType: TypeTask,
			},
			wantErr: true,
		},
		{
			name: "invalid status",
			issue: Issue{
				Title:     "Test Issue",
				ProjectID: "proj-test",
				Status:    Status("invalid"),
				Priority:  2,
				IssueType: TypeTask,
			},
			wantErr: true,
		},
		{
			name: "closed without closed_at",
			issue: Issue{
				Title:     "Test Issue",
				ProjectID: "proj-test",
				Status:    StatusClosed,
				Priority:  2,
				IssueType: TypeTask,
				ClosedAt:  nil,
			},
			wantErr: true,
			errMsg:  "closed issues must have closed_at timestamp",
		},
		{
			name: "not closed but has closed_at",
			issue: Issue{
				Title:     "Test Issue",
				ProjectID: "proj-test",
				Status:    StatusOpen,
				Priority:  2,
				IssueType: TypeTask,
				ClosedAt:  &now,
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
		Title:     "Test Issue",
		ProjectID: "proj-test",
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

func TestProjectValidate(t *testing.T) {
	tests := []struct {
		name    string
		proj    Project
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid project",
			proj: Project{
				Name:   "Test Project",
				Prefix: "test",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			proj: Project{
				Name:   "",
				Prefix: "test",
			},
			wantErr: true,
			errMsg:  "project name is required",
		},
		{
			name: "name too long",
			proj: Project{
				Name:   string(make([]byte, 101)),
				Prefix: "test",
			},
			wantErr: true,
			errMsg:  "project name must be 100 characters or less",
		},
		{
			name: "missing prefix",
			proj: Project{
				Name:   "Test",
				Prefix: "",
			},
			wantErr: true,
			errMsg:  "project prefix is required",
		},
		{
			name: "prefix too long",
			proj: Project{
				Name:   "Test",
				Prefix: "thisprefixtoolong",
			},
			wantErr: true,
			errMsg:  fmt.Sprintf("project prefix must be %d characters or less", MaxPrefixLength),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.proj.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Project.Validate() expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("Project.Validate() error = %q, want %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Project.Validate() unexpected error: %v", err)
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

func TestOpenChildrenError(t *testing.T) {
	t.Run("implements error interface", func(t *testing.T) {
		children := []Issue{
			{ID: "child-1", Title: "Child 1", Status: StatusOpen},
			{ID: "child-2", Title: "Child 2", Status: StatusInProgress},
		}
		err := &OpenChildrenError{
			IssueID:  "parent-1",
			Children: children,
		}

		var _ error = err // compile-time check

		msg := err.Error()
		if msg == "" {
			t.Error("OpenChildrenError.Error() returned empty string")
		}
	})

	t.Run("error message contains issue ID and child count", func(t *testing.T) {
		children := []Issue{
			{ID: "child-1", Title: "Child 1", Status: StatusOpen},
			{ID: "child-2", Title: "Child 2", Status: StatusInProgress},
		}
		err := &OpenChildrenError{
			IssueID:  "parent-1",
			Children: children,
		}

		msg := err.Error()
		// Should mention the parent issue ID
		if !containsString(msg, "parent-1") {
			t.Errorf("OpenChildrenError.Error() = %q, want it to contain %q", msg, "parent-1")
		}
		// Should mention the count of open children
		if !containsString(msg, "2") {
			t.Errorf("OpenChildrenError.Error() = %q, want it to contain the count %q", msg, "2")
		}
	})

	t.Run("single child message", func(t *testing.T) {
		err := &OpenChildrenError{
			IssueID: "parent-1",
			Children: []Issue{
				{ID: "child-1", Title: "Only Child", Status: StatusOpen},
			},
		}

		msg := err.Error()
		if !containsString(msg, "parent-1") {
			t.Errorf("OpenChildrenError.Error() = %q, want it to contain %q", msg, "parent-1")
		}
		if !containsString(msg, "1") {
			t.Errorf("OpenChildrenError.Error() = %q, want it to contain the count %q", msg, "1")
		}
	})

	t.Run("stores fields correctly", func(t *testing.T) {
		children := []Issue{
			{ID: "child-1", Title: "Child 1"},
		}
		err := &OpenChildrenError{
			IssueID:  "parent-1",
			Children: children,
		}

		if err.IssueID != "parent-1" {
			t.Errorf("OpenChildrenError.IssueID = %q, want %q", err.IssueID, "parent-1")
		}
		if len(err.Children) != 1 {
			t.Errorf("OpenChildrenError.Children length = %d, want 1", len(err.Children))
		}
		if err.Children[0].ID != "child-1" {
			t.Errorf("OpenChildrenError.Children[0].ID = %q, want %q", err.Children[0].ID, "child-1")
		}
	})
}

// containsString checks if s contains substr.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
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
				ID:        "ws-abc123",
				ProjectID: "proj-test",
				Path:      "/home/user/project",
			},
			wantErr: false,
		},
		{
			name: "missing path",
			ws: Workspace{
				ID:        "ws-abc123",
				ProjectID: "proj-test",
				Path:      "",
			},
			wantErr: true,
			errMsg:  "path is required",
		},
		{
			name: "missing project_id",
			ws: Workspace{
				ID:        "ws-abc123",
				ProjectID: "",
				Path:      "/home/user/project",
			},
			wantErr: true,
			errMsg:  "project_id is required",
		},
		{
			name: "both missing",
			ws: Workspace{
				ID: "ws-abc123",
			},
			wantErr: true,
		},
		{
			name: "with optional fields",
			ws: Workspace{
				ID:        "ws-abc123",
				ProjectID: "proj-test",
				Path:      "/home/user/project",
				Label:     "main",
				Hostname:  "dev-machine",
				GitRemote: "git@github.com:user/repo.git",
			},
			wantErr: false,
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

func TestIssueFilterParentIDField(t *testing.T) {
	// Verify that IssueFilter has a ProjectID field and it works as expected
	filter := IssueFilter{
		ProjectID: "proj-test",
		ParentID:  "arc-abc123",
	}

	if filter.ParentID != "arc-abc123" {
		t.Errorf("IssueFilter.ParentID = %q, want %q", filter.ParentID, "arc-abc123")
	}

	// Verify empty ParentID is the zero value
	emptyFilter := IssueFilter{
		ProjectID: "proj-test",
	}
	if emptyFilter.ParentID != "" {
		t.Errorf("IssueFilter.ParentID should be empty by default, got %q", emptyFilter.ParentID)
	}
}

func TestIssueFilterMultiValueFields(t *testing.T) {
	filter := IssueFilter{
		ProjectID:  "proj-test",
		Statuses:   []Status{StatusOpen, StatusInProgress},
		Priorities: []int{0, 1, 2},
		IssueTypes: []IssueType{TypeBug, TypeFeature},
	}

	if len(filter.Statuses) != 2 {
		t.Errorf("Statuses length = %d, want 2", len(filter.Statuses))
	}
	if len(filter.Priorities) != 3 {
		t.Errorf("Priorities length = %d, want 3", len(filter.Priorities))
	}
	if len(filter.IssueTypes) != 2 {
		t.Errorf("IssueTypes length = %d, want 2", len(filter.IssueTypes))
	}

	// Zero-value filter should have nil slices
	empty := IssueFilter{ProjectID: "proj-test"}
	if empty.Statuses != nil || empty.Priorities != nil || empty.IssueTypes != nil {
		t.Error("zero-value IssueFilter should have nil slices")
	}
}

func TestMergeResultTargetProject(t *testing.T) {
	// Verify MergeResult uses TargetProject (not TargetWorkspace)
	result := MergeResult{
		TargetProject:  &Project{ID: "proj-1", Name: "Test"},
		IssuesMoved:    5,
		SourcesDeleted: []string{"proj-2"},
	}

	if result.TargetProject == nil {
		t.Fatal("MergeResult.TargetProject should not be nil")
	}
	if result.TargetProject.ID != "proj-1" {
		t.Errorf("MergeResult.TargetProject.ID = %q, want %q", result.TargetProject.ID, "proj-1")
	}
}

func TestProjectResolution(t *testing.T) {
	// Verify ProjectResolution type exists with correct fields
	res := ProjectResolution{
		ProjectID:   "proj-test",
		ProjectName: "test-project",
		PathID:      "ws-abc123",
	}

	if res.ProjectID != "proj-test" {
		t.Errorf("ProjectResolution.ProjectID = %q, want %q", res.ProjectID, "proj-test")
	}
	if res.ProjectName != "test-project" {
		t.Errorf("ProjectResolution.ProjectName = %q, want %q", res.ProjectName, "test-project")
	}
	if res.PathID != "ws-abc123" {
		t.Errorf("ProjectResolution.PathID = %q, want %q", res.PathID, "ws-abc123")
	}
}

func TestStatisticsProjectID(t *testing.T) {
	// Verify Statistics uses ProjectID
	stats := Statistics{
		ProjectID:   "proj-test",
		TotalIssues: 10,
	}

	if stats.ProjectID != "proj-test" {
		t.Errorf("Statistics.ProjectID = %q, want %q", stats.ProjectID, "proj-test")
	}
}

func TestAISessionStruct(t *testing.T) {
	now := time.Now()
	session := AISession{
		ID:             "session-abc123",
		TranscriptPath: "/home/user/.claude/projects/test/transcript.jsonl",
		CWD:            "/home/user/project",
		StartedAt:      now,
	}

	if session.ID != "session-abc123" {
		t.Errorf("AISession.ID = %q, want %q", session.ID, "session-abc123")
	}
	if session.TranscriptPath != "/home/user/.claude/projects/test/transcript.jsonl" {
		t.Errorf("AISession.TranscriptPath = %q, want expected path", session.TranscriptPath)
	}
	if session.CWD != "/home/user/project" {
		t.Errorf("AISession.CWD = %q, want %q", session.CWD, "/home/user/project")
	}
	if !session.StartedAt.Equal(now) {
		t.Errorf("AISession.StartedAt = %v, want %v", session.StartedAt, now)
	}
}

func TestAISessionOptionalCWD(t *testing.T) {
	session := AISession{
		ID:             "session-abc123",
		TranscriptPath: "/path/to/transcript.jsonl",
		StartedAt:      time.Now(),
	}

	if session.CWD != "" {
		t.Errorf("AISession.CWD should be empty by default, got %q", session.CWD)
	}
}

func TestAIAgentStruct(t *testing.T) {
	now := time.Now()
	durationMs := 5000
	totalTokens := 1500
	toolUseCount := 10

	agent := AIAgent{
		ID:           "agent-xyz789",
		SessionID:    "session-abc123",
		Description:  "Implementing feature X",
		Prompt:       "Build the thing",
		AgentType:    "task",
		Model:        "claude-sonnet-4-20250514",
		Status:       "completed",
		DurationMs:   &durationMs,
		TotalTokens:  &totalTokens,
		ToolUseCount: &toolUseCount,
		CreatedAt:    now,
	}

	if agent.ID != "agent-xyz789" {
		t.Errorf("AIAgent.ID = %q, want %q", agent.ID, "agent-xyz789")
	}
	if agent.SessionID != "session-abc123" {
		t.Errorf("AIAgent.SessionID = %q, want %q", agent.SessionID, "session-abc123")
	}
	if agent.Description != "Implementing feature X" {
		t.Errorf("AIAgent.Description = %q, want expected value", agent.Description)
	}
	if agent.Status != "completed" {
		t.Errorf("AIAgent.Status = %q, want %q", agent.Status, "completed")
	}
	if *agent.DurationMs != 5000 {
		t.Errorf("AIAgent.DurationMs = %d, want 5000", *agent.DurationMs)
	}
	if *agent.TotalTokens != 1500 {
		t.Errorf("AIAgent.TotalTokens = %d, want 1500", *agent.TotalTokens)
	}
	if *agent.ToolUseCount != 10 {
		t.Errorf("AIAgent.ToolUseCount = %d, want 10", *agent.ToolUseCount)
	}
}

func TestAIAgentOptionalFields(t *testing.T) {
	agent := AIAgent{
		ID:        "agent-xyz789",
		SessionID: "session-abc123",
		Status:    "running",
		CreatedAt: time.Now(),
	}

	if agent.Description != "" {
		t.Errorf("AIAgent.Description should be empty by default, got %q", agent.Description)
	}
	if agent.Prompt != "" {
		t.Errorf("AIAgent.Prompt should be empty by default, got %q", agent.Prompt)
	}
	if agent.AgentType != "" {
		t.Errorf("AIAgent.AgentType should be empty by default, got %q", agent.AgentType)
	}
	if agent.Model != "" {
		t.Errorf("AIAgent.Model should be empty by default, got %q", agent.Model)
	}
	if agent.DurationMs != nil {
		t.Errorf("AIAgent.DurationMs should be nil by default, got %v", agent.DurationMs)
	}
	if agent.TotalTokens != nil {
		t.Errorf("AIAgent.TotalTokens should be nil by default, got %v", agent.TotalTokens)
	}
	if agent.ToolUseCount != nil {
		t.Errorf("AIAgent.ToolUseCount should be nil by default, got %v", agent.ToolUseCount)
	}
}
