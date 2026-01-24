package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/storage/sqlite"
	"github.com/sentiolabs/arc/internal/types"
)

// testServer creates a test server with a temporary SQLite database.
func testServer(t *testing.T) (*Server, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "arc-api-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create store: %v", err)
	}

	server := New(Config{
		Address: ":0",
		Store:   store,
	})

	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}

	return server, cleanup
}

// createTestWorkspace creates a workspace for testing and returns its ID.
func createTestWorkspace(t *testing.T, e *echo.Echo) string {
	t.Helper()

	body := `{"name": "Test Workspace", "prefix": "test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create workspace: %s", rec.Body.String())
	}

	var ws types.Workspace
	if err := json.Unmarshal(rec.Body.Bytes(), &ws); err != nil {
		t.Fatalf("failed to parse workspace response: %v", err)
	}

	return ws.ID
}

// createTestIssue creates an issue for testing and returns its ID.
func createTestIssue(t *testing.T, e *echo.Echo, wsID, title string) string {
	t.Helper()

	body := `{"title": "` + title + `", "type": "task", "priority": 2}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+wsID+"/issues", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create issue: %s", rec.Body.String())
	}

	var issue types.Issue
	if err := json.Unmarshal(rec.Body.Bytes(), &issue); err != nil {
		t.Fatalf("failed to parse issue response: %v", err)
	}

	return issue.ID
}

func TestSetAndGetIssuePlan(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)
	issueID := createTestIssue(t, e, wsID, "Issue with Plan")

	// Set inline plan - use JSON marshal to properly escape
	planText := "This is the plan: Step one then Step two"
	reqBody := map[string]string{"text": planText}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+wsID+"/issues/"+issueID+"/plan", bytes.NewBuffer(bodyBytes))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("X-Actor", "test-user")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("setIssuePlan returned %d: %s", rec.Code, rec.Body.String())
	}

	// Get plan context
	req = httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+wsID+"/issues/"+issueID+"/plan", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("getIssuePlan returned %d: %s", rec.Code, rec.Body.String())
	}

	var pc types.PlanContext
	if err := json.Unmarshal(rec.Body.Bytes(), &pc); err != nil {
		t.Fatalf("failed to parse plan context: %v", err)
	}

	if pc.InlinePlan == nil {
		t.Fatal("InlinePlan should not be nil")
	}
	if pc.InlinePlan.Text != planText {
		t.Errorf("InlinePlan.Text = %q, want %q", pc.InlinePlan.Text, planText)
	}
}

func TestSetIssuePlanEmptyText(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)
	issueID := createTestIssue(t, e, wsID, "Issue")

	body := `{"text": ""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+wsID+"/issues/"+issueID+"/plan", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("setIssuePlan with empty text returned %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetIssuePlanHistory(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)
	issueID := createTestIssue(t, e, wsID, "Issue with History")

	// Create multiple plan versions
	versions := []string{"Version 1", "Version 2", "Version 3"}
	for _, v := range versions {
		body := `{"text": "` + v + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+wsID+"/issues/"+issueID+"/plan", bytes.NewBufferString(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("setIssuePlan failed: %s", rec.Body.String())
		}
	}

	// Get history
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+wsID+"/issues/"+issueID+"/plan/history", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("getIssuePlanHistory returned %d: %s", rec.Code, rec.Body.String())
	}

	var history []*types.Comment
	if err := json.Unmarshal(rec.Body.Bytes(), &history); err != nil {
		t.Fatalf("failed to parse history: %v", err)
	}

	if len(history) != len(versions) {
		t.Errorf("history length = %d, want %d", len(history), len(versions))
	}
}

func TestCreateAndGetSharedPlan(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)

	// Create shared plan
	body := `{"title": "Shared Plan", "content": "Plan content here"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+wsID+"/plans", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("createPlan returned %d: %s", rec.Code, rec.Body.String())
	}

	var plan types.Plan
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("failed to parse plan: %v", err)
	}

	if plan.Title != "Shared Plan" {
		t.Errorf("plan.Title = %q, want %q", plan.Title, "Shared Plan")
	}
	if plan.Content != "Plan content here" {
		t.Errorf("plan.Content = %q, want %q", plan.Content, "Plan content here")
	}

	// Get plan
	req = httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+wsID+"/plans/"+plan.ID, nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("getPlan returned %d: %s", rec.Code, rec.Body.String())
	}

	var retrieved types.Plan
	if err := json.Unmarshal(rec.Body.Bytes(), &retrieved); err != nil {
		t.Fatalf("failed to parse plan: %v", err)
	}

	if retrieved.ID != plan.ID {
		t.Errorf("retrieved.ID = %q, want %q", retrieved.ID, plan.ID)
	}
}

func TestCreatePlanEmptyTitle(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)

	body := `{"title": "", "content": "content"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+wsID+"/plans", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("createPlan with empty title returned %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestListPlans(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)

	// Create multiple plans
	for i := 0; i < 3; i++ {
		body := `{"title": "Plan ` + string(rune('A'+i)) + `", "content": "Content"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+wsID+"/plans", bytes.NewBufferString(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("createPlan failed: %s", rec.Body.String())
		}
	}

	// List plans
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+wsID+"/plans", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("listPlans returned %d: %s", rec.Code, rec.Body.String())
	}

	var plans []*types.Plan
	if err := json.Unmarshal(rec.Body.Bytes(), &plans); err != nil {
		t.Fatalf("failed to parse plans: %v", err)
	}

	if len(plans) != 3 {
		t.Errorf("listPlans returned %d plans, want 3", len(plans))
	}
}

func TestUpdatePlan(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)

	// Create plan
	body := `{"title": "Original", "content": "Original content"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+wsID+"/plans", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var plan types.Plan
	json.Unmarshal(rec.Body.Bytes(), &plan)

	// Update plan
	body = `{"title": "Updated", "content": "Updated content"}`
	req = httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/"+wsID+"/plans/"+plan.ID, bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("updatePlan returned %d: %s", rec.Code, rec.Body.String())
	}

	var updated types.Plan
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("failed to parse plan: %v", err)
	}

	if updated.Title != "Updated" {
		t.Errorf("updated.Title = %q, want %q", updated.Title, "Updated")
	}
	if updated.Content != "Updated content" {
		t.Errorf("updated.Content = %q, want %q", updated.Content, "Updated content")
	}
}

func TestDeletePlan(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)

	// Create plan
	body := `{"title": "To Delete", "content": ""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+wsID+"/plans", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var plan types.Plan
	json.Unmarshal(rec.Body.Bytes(), &plan)

	// Delete plan
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/"+wsID+"/plans/"+plan.ID, nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("deletePlan returned %d: %s", rec.Code, rec.Body.String())
	}

	// Verify deletion
	req = httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+wsID+"/plans/"+plan.ID, nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("getPlan after delete returned %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestLinkAndUnlinkIssuesToPlan(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)
	issueID := createTestIssue(t, e, wsID, "Issue to Link")

	// Create plan
	body := `{"title": "Linkable Plan", "content": ""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+wsID+"/plans", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var plan types.Plan
	json.Unmarshal(rec.Body.Bytes(), &plan)

	// Link issue to plan
	body = `{"issue_ids": ["` + issueID + `"]}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+wsID+"/plans/"+plan.ID+"/link", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("linkIssuesToPlan returned %d: %s", rec.Code, rec.Body.String())
	}

	// Verify link via getPlan (should include linked_issues)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+wsID+"/plans/"+plan.ID, nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var retrieved types.Plan
	json.Unmarshal(rec.Body.Bytes(), &retrieved)

	if len(retrieved.LinkedIssues) != 1 {
		t.Errorf("LinkedIssues length = %d, want 1", len(retrieved.LinkedIssues))
	}
	if len(retrieved.LinkedIssues) > 0 && retrieved.LinkedIssues[0] != issueID {
		t.Errorf("LinkedIssues[0] = %q, want %q", retrieved.LinkedIssues[0], issueID)
	}

	// Unlink issue
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/"+wsID+"/plans/"+plan.ID+"/link/"+issueID, nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("unlinkIssueFromPlan returned %d: %s", rec.Code, rec.Body.String())
	}

	// Verify unlink - use new variable to avoid stale data
	req = httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+wsID+"/plans/"+plan.ID, nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var afterUnlink types.Plan
	json.Unmarshal(rec.Body.Bytes(), &afterUnlink)

	if len(afterUnlink.LinkedIssues) != 0 {
		t.Errorf("LinkedIssues after unlink length = %d, want 0", len(afterUnlink.LinkedIssues))
	}
}

func TestLinkIssuesToPlanEmptyList(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)

	// Create plan
	body := `{"title": "Plan", "content": ""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+wsID+"/plans", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var plan types.Plan
	json.Unmarshal(rec.Body.Bytes(), &plan)

	// Try to link empty list
	body = `{"issue_ids": []}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+wsID+"/plans/"+plan.ID+"/link", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("linkIssuesToPlan with empty list returned %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestPlanWorkspaceIsolation(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	// Create two workspaces
	wsID1 := createTestWorkspace(t, e)

	body := `{"name": "Other Workspace", "prefix": "other"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	var ws2 types.Workspace
	json.Unmarshal(rec.Body.Bytes(), &ws2)
	wsID2 := ws2.ID

	// Create plan in workspace 1
	body = `{"title": "WS1 Plan", "content": ""}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+wsID1+"/plans", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var plan types.Plan
	json.Unmarshal(rec.Body.Bytes(), &plan)

	// Try to access plan from workspace 2 - should fail
	req = httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+wsID2+"/plans/"+plan.ID, nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("getPlan from wrong workspace returned %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestGetPlanNotFound(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+wsID+"/plans/plan.nonexistent", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("getPlan for nonexistent plan returned %d, want %d", rec.Code, http.StatusNotFound)
	}
}
