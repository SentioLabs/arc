package api //nolint:testpackage // tests use internal mock store types

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/types"
)

const (
	testPathID          = "p-1"
	testUserProjectPath = "/home/user/project"
)

// mockWPStore implements storage.Storage for workspace (directory path) tests.
// Only workspace methods are implemented; all others panic.
type mockWPStore struct {
	workspaces []*types.Workspace
	touched    string // last workspace ID touched
}

func newMockWPStore() *mockWPStore {
	return &mockWPStore{
		workspaces: []*types.Workspace{},
	}
}

// Workspace methods - the ones under test

func (m *mockWPStore) CreateWorkspace(_ context.Context, ws *types.Workspace) error {
	for _, existing := range m.workspaces {
		if existing.Path == ws.Path && existing.ProjectID == ws.ProjectID {
			return fmt.Errorf("workspace already exists: %s", ws.Path)
		}
	}
	ws.ID = fmt.Sprintf("p-%d", len(m.workspaces)+1)
	ws.CreatedAt = time.Now()
	ws.UpdatedAt = time.Now()
	m.workspaces = append(m.workspaces, ws)
	return nil
}

func (m *mockWPStore) GetWorkspace(_ context.Context, id string) (*types.Workspace, error) {
	for _, ws := range m.workspaces {
		if ws.ID == id {
			return ws, nil
		}
	}
	return nil, fmt.Errorf("workspace not found: %s", id)
}

func (m *mockWPStore) ListWorkspaces(_ context.Context, projectID string) ([]*types.Workspace, error) {
	var result []*types.Workspace
	for _, ws := range m.workspaces {
		if ws.ProjectID == projectID {
			result = append(result, ws)
		}
	}
	return result, nil
}

func (m *mockWPStore) UpdateWorkspace(_ context.Context, ws *types.Workspace) error {
	for i, existing := range m.workspaces {
		if existing.ID == ws.ID {
			ws.UpdatedAt = time.Now()
			m.workspaces[i] = ws
			return nil
		}
	}
	return fmt.Errorf("workspace not found: %s", ws.ID)
}

func (m *mockWPStore) DeleteWorkspace(_ context.Context, id string) error {
	for i, ws := range m.workspaces {
		if ws.ID == id {
			m.workspaces = append(m.workspaces[:i], m.workspaces[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("workspace not found: %s", id)
}

func (m *mockWPStore) ResolveProjectByPath(_ context.Context, path string) (*types.Workspace, error) {
	for _, ws := range m.workspaces {
		if ws.Path == path {
			return ws, nil
		}
	}
	return nil, fmt.Errorf("no project found for path: %s", path)
}

func (m *mockWPStore) UpdateWorkspaceLastAccessed(_ context.Context, id string) error {
	m.touched = id
	return nil
}

// Stub methods to satisfy storage.Storage interface - these are not called in workspace tests.

func (m *mockWPStore) CreateProject(_ context.Context, _ *types.Project) error {
	panic("not implemented")
}

func (m *mockWPStore) GetProject(_ context.Context, id string) (*types.Project, error) {
	return &types.Project{ID: id, Name: "test-project"}, nil
}

func (m *mockWPStore) GetProjectByName(_ context.Context, _ string) (*types.Project, error) {
	panic("not implemented")
}

func (m *mockWPStore) ListProjects(_ context.Context) ([]*types.Project, error) {
	panic("not implemented")
}

func (m *mockWPStore) UpdateProject(_ context.Context, _ *types.Project) error {
	panic("not implemented")
}

func (m *mockWPStore) DeleteProject(_ context.Context, _ string) error {
	panic("not implemented")
}

func (m *mockWPStore) MergeProjects(_ context.Context, _ string, _ []string, _ string) (*types.MergeResult, error) {
	panic("not implemented")
}

func (m *mockWPStore) CreateIssue(_ context.Context, _ *types.Issue, _ string) error {
	panic("not implemented")
}

func (m *mockWPStore) GetIssue(_ context.Context, _ string) (*types.Issue, error) {
	panic("not implemented")
}

func (m *mockWPStore) GetIssueByExternalRef(_ context.Context, _ string) (*types.Issue, error) {
	panic("not implemented")
}

func (m *mockWPStore) ListIssues(_ context.Context, _ types.IssueFilter) ([]*types.Issue, error) {
	panic("not implemented")
}

func (m *mockWPStore) UpdateIssue(_ context.Context, _ string, _ map[string]any, _ string) error {
	panic("not implemented")
}

func (m *mockWPStore) CloseIssue(_ context.Context, _ string, _ string, _ bool, _ string) error {
	panic("not implemented")
}

func (m *mockWPStore) ReopenIssue(_ context.Context, _ string, _ string) error {
	panic("not implemented")
}
func (m *mockWPStore) DeleteIssue(_ context.Context, _ string) error { panic("not implemented") }
func (m *mockWPStore) GetIssueDetails(_ context.Context, _ string) (*types.IssueDetails, error) {
	panic("not implemented")
}

func (m *mockWPStore) GetReadyWork(_ context.Context, _ types.WorkFilter) ([]*types.Issue, error) {
	panic("not implemented")
}

func (m *mockWPStore) GetBlockedIssues(_ context.Context, _ types.WorkFilter) ([]*types.BlockedIssue, error) {
	panic("not implemented")
}

func (m *mockWPStore) IsBlocked(_ context.Context, _ string) (bool, []string, error) {
	panic("not implemented")
}

func (m *mockWPStore) AddDependency(_ context.Context, _ *types.Dependency, _ string) error {
	panic("not implemented")
}

func (m *mockWPStore) RemoveDependency(_ context.Context, _, _ string, _ string) error {
	panic("not implemented")
}

func (m *mockWPStore) GetDependencies(_ context.Context, _ string) ([]*types.Dependency, error) {
	panic("not implemented")
}

func (m *mockWPStore) GetDependents(_ context.Context, _ string) ([]*types.Dependency, error) {
	panic("not implemented")
}

func (m *mockWPStore) CreateLabel(_ context.Context, _ *types.Label) error {
	panic("not implemented")
}

func (m *mockWPStore) GetLabel(_ context.Context, _ string) (*types.Label, error) {
	panic("not implemented")
}

func (m *mockWPStore) ListLabels(_ context.Context) ([]*types.Label, error) {
	panic("not implemented")
}

func (m *mockWPStore) UpdateLabel(_ context.Context, _ *types.Label) error {
	panic("not implemented")
}
func (m *mockWPStore) DeleteLabel(_ context.Context, _ string) error { panic("not implemented") }
func (m *mockWPStore) AddLabelToIssue(_ context.Context, _, _, _ string) error {
	panic("not implemented")
}

func (m *mockWPStore) RemoveLabelFromIssue(_ context.Context, _, _, _ string) error {
	panic("not implemented")
}

func (m *mockWPStore) GetIssueLabels(_ context.Context, _ string) ([]string, error) {
	panic("not implemented")
}

func (m *mockWPStore) GetLabelsForIssues(_ context.Context, _ []string) (map[string][]string, error) {
	panic("not implemented")
}

func (m *mockWPStore) AddComment(_ context.Context, _, _, _ string) (*types.Comment, error) {
	panic("not implemented")
}

func (m *mockWPStore) GetComments(_ context.Context, _ string) ([]*types.Comment, error) {
	panic("not implemented")
}

func (m *mockWPStore) UpdateComment(_ context.Context, _ int64, _ string) error {
	panic("not implemented")
}
func (m *mockWPStore) DeleteComment(_ context.Context, _ int64) error { panic("not implemented") }
func (m *mockWPStore) CreatePlan(_ context.Context, _ *types.Plan) error {
	panic("not implemented")
}

func (m *mockWPStore) GetPlan(_ context.Context, _ string) (*types.Plan, error) {
	panic("not implemented")
}

func (m *mockWPStore) UpdatePlanStatus(_ context.Context, _, _ string) error {
	panic("not implemented")
}

func (m *mockWPStore) DeletePlan(_ context.Context, _ string) error { panic("not implemented") }

func (m *mockWPStore) CreatePlanComment(_ context.Context, _ *types.PlanComment) error {
	panic("not implemented")
}

func (m *mockWPStore) ListPlanComments(_ context.Context, _ string) ([]*types.PlanComment, error) {
	panic("not implemented")
}

func (m *mockWPStore) GetEvents(_ context.Context, _ string, _ int) ([]*types.Event, error) {
	panic("not implemented")
}

func (m *mockWPStore) GetStatistics(_ context.Context, _ string) (*types.Statistics, error) {
	panic("not implemented")
}

func (m *mockWPStore) CreateAISession(_ context.Context, _ *types.AISession) error {
	panic("not implemented")
}

func (m *mockWPStore) GetAISession(_ context.Context, _ string) (*types.AISession, error) {
	panic("not implemented")
}

func (m *mockWPStore) ListAISessions(_ context.Context, _, _ int) ([]*types.AISession, error) {
	panic("not implemented")
}

func (m *mockWPStore) DeleteAISession(_ context.Context, _ string) error {
	panic("not implemented")
}

func (m *mockWPStore) CreateAIAgent(_ context.Context, _ *types.AIAgent) error {
	panic("not implemented")
}

func (m *mockWPStore) GetAIAgent(_ context.Context, _ string) (*types.AIAgent, error) {
	panic("not implemented")
}

func (m *mockWPStore) ListAIAgents(_ context.Context, _ string) ([]*types.AIAgent, error) {
	panic("not implemented")
}

func (m *mockWPStore) Close() error { return nil }
func (m *mockWPStore) Path() string { return "" }

func setupWorkspaceTest(t *testing.T) (*echo.Echo, *mockWPStore) {
	t.Helper()
	store := newMockWPStore()
	srv := New(Config{
		Address: ":0",
		Store:   store,
	})
	return srv.Echo(), store
}

func TestListWorkspaces(t *testing.T) {
	e, store := setupWorkspaceTest(t)

	// Seed a workspace
	store.workspaces = append(store.workspaces, &types.Workspace{
		ID:        "p-1",
		ProjectID: "proj-abc",
		Path:      testUserProjectPath,
		Label:     "main",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/proj-abc/workspaces", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var workspaces []*types.Workspace
	if err := json.Unmarshal(rec.Body.Bytes(), &workspaces); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(workspaces) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(workspaces))
	}
	if workspaces[0].ID != testPathID {
		t.Errorf("expected ID %s, got %s", testPathID, workspaces[0].ID)
	}
}

func TestCreateWorkspace(t *testing.T) {
	e, _ := setupWorkspaceTest(t)

	body := `{"path":"` + testUserProjectPath + `","label":"main",` +
		`"hostname":"dev-machine","git_remote":"git@github.com:user/repo.git"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/proj-abc/workspaces", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var ws types.Workspace
	if err := json.Unmarshal(rec.Body.Bytes(), &ws); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if ws.Path != testUserProjectPath {
		t.Errorf("expected path /home/user/project, got %s", ws.Path)
	}
	if ws.Label != "main" {
		t.Errorf("expected label main, got %s", ws.Label)
	}
	if ws.Hostname != "dev-machine" {
		t.Errorf("expected hostname dev-machine, got %s", ws.Hostname)
	}
	if ws.GitRemote != "git@github.com:user/repo.git" {
		t.Errorf("expected git_remote, got %s", ws.GitRemote)
	}
	if ws.ProjectID != "proj-abc" {
		t.Errorf("expected project_id proj-abc, got %s", ws.ProjectID)
	}
}

func TestCreateWorkspace_Duplicate(t *testing.T) {
	e, store := setupWorkspaceTest(t)

	// Seed an existing workspace
	store.workspaces = append(store.workspaces, &types.Workspace{
		ID:        "p-1",
		ProjectID: "proj-abc",
		Path:      testUserProjectPath,
	})

	body := `{"path":"` + testUserProjectPath + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/proj-abc/workspaces", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateWorkspace(t *testing.T) {
	e, store := setupWorkspaceTest(t)

	// Seed a workspace
	store.workspaces = append(store.workspaces, &types.Workspace{
		ID:        "p-1",
		ProjectID: "proj-abc",
		Path:      testUserProjectPath,
		Label:     "original",
	})

	body := `{"label":"updated-label","hostname":"new-host"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/projects/proj-abc/workspaces/p-1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var ws types.Workspace
	if err := json.Unmarshal(rec.Body.Bytes(), &ws); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if ws.Label != "updated-label" {
		t.Errorf("expected label updated-label, got %s", ws.Label)
	}
	if ws.Hostname != "new-host" {
		t.Errorf("expected hostname new-host, got %s", ws.Hostname)
	}
	if ws.Path != testUserProjectPath {
		t.Errorf("expected path unchanged, got %s", ws.Path)
	}
}

func TestDeleteWorkspace(t *testing.T) {
	e, store := setupWorkspaceTest(t)

	// Seed a workspace
	store.workspaces = append(store.workspaces, &types.Workspace{
		ID:        "p-1",
		ProjectID: "proj-abc",
		Path:      testUserProjectPath,
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/proj-abc/workspaces/p-1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d: %s", rec.Code, rec.Body.String())
	}

	if len(store.workspaces) != 0 {
		t.Errorf("expected 0 workspaces after delete, got %d", len(store.workspaces))
	}
}

func TestResolveProject(t *testing.T) {
	e, store := setupWorkspaceTest(t)

	// Seed a workspace
	store.workspaces = append(store.workspaces, &types.Workspace{
		ID:        "p-1",
		ProjectID: "proj-abc",
		Path:      testUserProjectPath,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/resolve?path=/home/user/project", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result types.ProjectResolution
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.ProjectID != "proj-abc" {
		t.Errorf("expected project_id proj-abc, got %s", result.ProjectID)
	}
	if result.PathID != "p-1" {
		t.Errorf("expected path_id p-1, got %s", result.PathID)
	}
	if result.ProjectName != "test-project" {
		t.Errorf("expected project_name test-project, got %s", result.ProjectName)
	}

	// Verify last_accessed_at was touched
	if store.touched != "p-1" {
		t.Errorf("expected UpdateWorkspaceLastAccessed called with p-1, got %s", store.touched)
	}
}
