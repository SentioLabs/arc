package api

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

// mockWPStore implements storage.Storage for workspace path tests.
// Only workspace path methods are implemented; all others panic.
type mockWPStore struct {
	paths   []*types.WorkspacePath
	touched string // last path ID touched
}

func newMockWPStore() *mockWPStore {
	return &mockWPStore{
		paths: []*types.WorkspacePath{},
	}
}

// Workspace path methods - the ones under test

func (m *mockWPStore) CreateWorkspacePath(_ context.Context, wp *types.WorkspacePath) error {
	for _, existing := range m.paths {
		if existing.Path == wp.Path && existing.WorkspaceID == wp.WorkspaceID {
			return fmt.Errorf("workspace path already exists: %s", wp.Path)
		}
	}
	wp.ID = fmt.Sprintf("p-%d", len(m.paths)+1)
	wp.CreatedAt = time.Now()
	wp.UpdatedAt = time.Now()
	m.paths = append(m.paths, wp)
	return nil
}

func (m *mockWPStore) GetWorkspacePath(_ context.Context, id string) (*types.WorkspacePath, error) {
	for _, p := range m.paths {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, fmt.Errorf("workspace path not found: %s", id)
}

func (m *mockWPStore) ListWorkspacePaths(_ context.Context, workspaceID string) ([]*types.WorkspacePath, error) {
	var result []*types.WorkspacePath
	for _, p := range m.paths {
		if p.WorkspaceID == workspaceID {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockWPStore) UpdateWorkspacePath(_ context.Context, wp *types.WorkspacePath) error {
	for i, p := range m.paths {
		if p.ID == wp.ID {
			wp.UpdatedAt = time.Now()
			m.paths[i] = wp
			return nil
		}
	}
	return fmt.Errorf("workspace path not found: %s", wp.ID)
}

func (m *mockWPStore) DeleteWorkspacePath(_ context.Context, id string) error {
	for i, p := range m.paths {
		if p.ID == id {
			m.paths = append(m.paths[:i], m.paths[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("workspace path not found: %s", id)
}

func (m *mockWPStore) ResolveWorkspaceByPath(_ context.Context, path string) (*types.WorkspacePath, error) {
	for _, p := range m.paths {
		if p.Path == path {
			return p, nil
		}
	}
	return nil, fmt.Errorf("no workspace found for path: %s", path)
}

func (m *mockWPStore) UpdatePathLastAccessed(_ context.Context, id string) error {
	m.touched = id
	return nil
}

// Stub methods to satisfy storage.Storage interface - these are not called in workspace path tests.

func (m *mockWPStore) CreateWorkspace(_ context.Context, _ *types.Workspace) error {
	panic("not implemented")
}
func (m *mockWPStore) GetWorkspace(_ context.Context, id string) (*types.Workspace, error) {
	return &types.Workspace{ID: id, Name: "test-workspace"}, nil
}
func (m *mockWPStore) GetWorkspaceByName(_ context.Context, _ string) (*types.Workspace, error) {
	panic("not implemented")
}
func (m *mockWPStore) ListWorkspaces(_ context.Context) ([]*types.Workspace, error) {
	panic("not implemented")
}
func (m *mockWPStore) UpdateWorkspace(_ context.Context, _ *types.Workspace) error {
	panic("not implemented")
}
func (m *mockWPStore) DeleteWorkspace(_ context.Context, _ string) error {
	panic("not implemented")
}
func (m *mockWPStore) MergeWorkspaces(_ context.Context, _ string, _ []string, _ string) (*types.MergeResult, error) {
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
func (m *mockWPStore) ListPlans(_ context.Context, _ string) ([]*types.Plan, error) {
	panic("not implemented")
}
func (m *mockWPStore) UpdatePlan(_ context.Context, _, _, _ string) error {
	panic("not implemented")
}
func (m *mockWPStore) DeletePlan(_ context.Context, _ string) error { panic("not implemented") }
func (m *mockWPStore) LinkIssueToPlan(_ context.Context, _, _ string) error {
	panic("not implemented")
}
func (m *mockWPStore) UnlinkIssueFromPlan(_ context.Context, _, _ string) error {
	panic("not implemented")
}
func (m *mockWPStore) GetLinkedPlans(_ context.Context, _ string) ([]*types.Plan, error) {
	panic("not implemented")
}
func (m *mockWPStore) GetLinkedIssues(_ context.Context, _ string) ([]string, error) {
	panic("not implemented")
}
func (m *mockWPStore) SetInlinePlan(_ context.Context, _, _, _ string) (*types.Comment, error) {
	panic("not implemented")
}
func (m *mockWPStore) GetInlinePlan(_ context.Context, _ string) (*types.Comment, error) {
	panic("not implemented")
}
func (m *mockWPStore) GetPlanHistory(_ context.Context, _ string) ([]*types.Comment, error) {
	panic("not implemented")
}
func (m *mockWPStore) GetPlanContext(_ context.Context, _ string) (*types.PlanContext, error) {
	panic("not implemented")
}
func (m *mockWPStore) GetEvents(_ context.Context, _ string, _ int) ([]*types.Event, error) {
	panic("not implemented")
}
func (m *mockWPStore) GetStatistics(_ context.Context, _ string) (*types.Statistics, error) {
	panic("not implemented")
}
func (m *mockWPStore) Close() error  { return nil }
func (m *mockWPStore) Path() string  { return "" }

func setupWorkspacePathTest(t *testing.T) (*echo.Echo, *mockWPStore) {
	t.Helper()
	store := newMockWPStore()
	srv := New(Config{
		Address: ":0",
		Store:   store,
	})
	return srv.Echo(), store
}

func TestListWorkspacePaths(t *testing.T) {
	e, store := setupWorkspacePathTest(t)

	// Seed a path
	store.paths = append(store.paths, &types.WorkspacePath{
		ID:          "p-1",
		WorkspaceID: "ws-abc",
		Path:        "/home/user/project",
		Label:       "main",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-abc/paths", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var paths []*types.WorkspacePath
	if err := json.Unmarshal(rec.Body.Bytes(), &paths); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
	if paths[0].ID != "p-1" {
		t.Errorf("expected ID p-1, got %s", paths[0].ID)
	}
}

func TestCreateWorkspacePath(t *testing.T) {
	e, _ := setupWorkspacePathTest(t)

	body := `{"path":"/home/user/project","label":"main","hostname":"dev-machine","git_remote":"git@github.com:user/repo.git"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws-abc/paths", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var wp types.WorkspacePath
	if err := json.Unmarshal(rec.Body.Bytes(), &wp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if wp.Path != "/home/user/project" {
		t.Errorf("expected path /home/user/project, got %s", wp.Path)
	}
	if wp.Label != "main" {
		t.Errorf("expected label main, got %s", wp.Label)
	}
	if wp.Hostname != "dev-machine" {
		t.Errorf("expected hostname dev-machine, got %s", wp.Hostname)
	}
	if wp.GitRemote != "git@github.com:user/repo.git" {
		t.Errorf("expected git_remote, got %s", wp.GitRemote)
	}
	if wp.WorkspaceID != "ws-abc" {
		t.Errorf("expected workspace_id ws-abc, got %s", wp.WorkspaceID)
	}
}

func TestCreateWorkspacePath_Duplicate(t *testing.T) {
	e, store := setupWorkspacePathTest(t)

	// Seed an existing path
	store.paths = append(store.paths, &types.WorkspacePath{
		ID:          "p-1",
		WorkspaceID: "ws-abc",
		Path:        "/home/user/project",
	})

	body := `{"path":"/home/user/project"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws-abc/paths", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateWorkspacePath(t *testing.T) {
	e, store := setupWorkspacePathTest(t)

	// Seed a path
	store.paths = append(store.paths, &types.WorkspacePath{
		ID:          "p-1",
		WorkspaceID: "ws-abc",
		Path:        "/home/user/project",
		Label:       "original",
	})

	body := `{"label":"updated-label","hostname":"new-host"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/workspaces/ws-abc/paths/p-1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var wp types.WorkspacePath
	if err := json.Unmarshal(rec.Body.Bytes(), &wp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if wp.Label != "updated-label" {
		t.Errorf("expected label updated-label, got %s", wp.Label)
	}
	if wp.Hostname != "new-host" {
		t.Errorf("expected hostname new-host, got %s", wp.Hostname)
	}
	if wp.Path != "/home/user/project" {
		t.Errorf("expected path unchanged, got %s", wp.Path)
	}
}

func TestDeleteWorkspacePath(t *testing.T) {
	e, store := setupWorkspacePathTest(t)

	// Seed a path
	store.paths = append(store.paths, &types.WorkspacePath{
		ID:          "p-1",
		WorkspaceID: "ws-abc",
		Path:        "/home/user/project",
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/ws-abc/paths/p-1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d: %s", rec.Code, rec.Body.String())
	}

	if len(store.paths) != 0 {
		t.Errorf("expected 0 paths after delete, got %d", len(store.paths))
	}
}

func TestResolveWorkspace(t *testing.T) {
	e, store := setupWorkspacePathTest(t)

	// Seed a path
	store.paths = append(store.paths, &types.WorkspacePath{
		ID:          "p-1",
		WorkspaceID: "ws-abc",
		Path:        "/home/user/project",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/resolve?path=/home/user/project", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result types.WorkspaceResolution
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.WorkspaceID != "ws-abc" {
		t.Errorf("expected workspace_id ws-abc, got %s", result.WorkspaceID)
	}
	if result.PathID != "p-1" {
		t.Errorf("expected path_id p-1, got %s", result.PathID)
	}
	if result.WorkspaceName != "test-workspace" {
		t.Errorf("expected workspace_name test-workspace, got %s", result.WorkspaceName)
	}

	// Verify last_accessed_at was touched
	if store.touched != "p-1" {
		t.Errorf("expected UpdatePathLastAccessed called with p-1, got %s", store.touched)
	}
}
