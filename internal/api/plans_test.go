package api //nolint:testpackage // tests use internal helpers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/storage/sqlite"
	"github.com/sentiolabs/arc/internal/types"
)

const testPlanBody = `{"text": "Plan content"}`

// testServer creates a test server with a temporary SQLite database.
func testServer(t *testing.T) (*Server, func()) {
	t.Helper()

	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	server := New(Config{
		Address: ":0",
		Store:   store,
	})

	cleanup := func() {
		store.Close()
	}

	return server, cleanup
}

// createTestWorkspace creates a project for testing and returns its ID.
func createTestWorkspace(t *testing.T, e *echo.Echo) string {
	t.Helper()

	body := `{"name": "Test Workspace", "prefix": "test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create project: %s", rec.Body.String())
	}

	var ws types.Workspace
	if err := json.Unmarshal(rec.Body.Bytes(), &ws); err != nil {
		t.Fatalf("failed to parse project response: %v", err)
	}

	return ws.ID
}

// createTestIssue creates an issue for testing and returns its ID.
func createTestIssue(t *testing.T, e *echo.Echo, wsID, title string) string {
	t.Helper()

	body := `{"title": "` + title + `", "type": "task", "priority": 2}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+wsID+"/issues", bytes.NewBufferString(body))
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

func TestSetIssuePlanCreatesDraft(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)
	issueID := createTestIssue(t, e, wsID, "Issue with Plan")

	// Set plan with explicit draft status
	body := `{"text": "Plan content here", "status": "draft"}`
	planURL := "/api/v1/projects/" + wsID + "/issues/" + issueID + "/plan"
	req := httptest.NewRequest(http.MethodPost, planURL, bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("X-Actor", "test-user")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("setIssuePlan returned %d: %s", rec.Code, rec.Body.String())
	}

	var plan types.Plan
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("failed to parse plan: %v", err)
	}

	if plan.Status != "draft" {
		t.Errorf("plan.Status = %q, want %q", plan.Status, "draft")
	}
	if plan.Content != "Plan content here" {
		t.Errorf("plan.Content = %q, want %q", plan.Content, "Plan content here")
	}
	if plan.IssueID != issueID {
		t.Errorf("plan.IssueID = %q, want %q", plan.IssueID, issueID)
	}
	if plan.ProjectID != wsID {
		t.Errorf("plan.ProjectID = %q, want %q", plan.ProjectID, wsID)
	}
}

func TestSetIssuePlanDefaultsDraft(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)
	issueID := createTestIssue(t, e, wsID, "Issue Default Status")

	// Set plan without status - should default to "draft"
	body := `{"text": "Plan without status"}`
	planURL := "/api/v1/projects/" + wsID + "/issues/" + issueID + "/plan"
	req := httptest.NewRequest(http.MethodPost, planURL, bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("setIssuePlan returned %d: %s", rec.Code, rec.Body.String())
	}

	var plan types.Plan
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("failed to parse plan: %v", err)
	}

	if plan.Status != "draft" {
		t.Errorf("plan.Status = %q, want %q", plan.Status, "draft")
	}
}

func TestSetIssuePlanEmptyText(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)
	issueID := createTestIssue(t, e, wsID, "Issue")

	body := `{"text": ""}`
	planURL := "/api/v1/projects/" + wsID + "/issues/" + issueID + "/plan"
	req := httptest.NewRequest(http.MethodPost, planURL, bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("setIssuePlan with empty text returned %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetIssuePlan(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)
	issueID := createTestIssue(t, e, wsID, "Issue with Plan")

	// Set plan first
	planText := "This is the plan"
	reqBody := map[string]string{"text": planText}
	bodyBytes, _ := json.Marshal(reqBody)
	planURL := "/api/v1/projects/" + wsID + "/issues/" + issueID + "/plan"
	req := httptest.NewRequest(http.MethodPost, planURL, bytes.NewBuffer(bodyBytes))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("setIssuePlan returned %d: %s", rec.Code, rec.Body.String())
	}

	// Get plan
	req = httptest.NewRequest(http.MethodGet, planURL, nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("getIssuePlan returned %d: %s", rec.Code, rec.Body.String())
	}

	var plan types.Plan
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("failed to parse plan: %v", err)
	}

	if plan.Content != planText {
		t.Errorf("plan.Content = %q, want %q", plan.Content, planText)
	}
	if plan.IssueID != issueID {
		t.Errorf("plan.IssueID = %q, want %q", plan.IssueID, issueID)
	}
}

func TestGetIssuePlanNotFound(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)
	issueID := createTestIssue(t, e, wsID, "Issue without Plan")

	// Get plan for issue with no plan - should 404
	planURL := "/api/v1/projects/" + wsID + "/issues/" + issueID + "/plan"
	req := httptest.NewRequest(http.MethodGet, planURL, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("getIssuePlan with no plan returned %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestListPlans(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)

	// Create plans via issues
	for i := range 3 {
		issueID := createTestIssue(t, e, wsID, "Issue "+string(rune('A'+i)))
		body := `{"text": "Plan content ` + string(rune('A'+i)) + `"}`
		planURL := "/api/v1/projects/" + wsID + "/issues/" + issueID + "/plan"
		req := httptest.NewRequest(http.MethodPost, planURL, bytes.NewBufferString(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("setIssuePlan failed: %s", rec.Body.String())
		}
	}

	// List all plans
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+wsID+"/plans", nil)
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

func TestListPlansFilterByStatus(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)

	// Create two plans (both default to "draft")
	planIDs := make([]string, 0, 2)
	for i := range 2 {
		issueID := createTestIssue(t, e, wsID, "Issue "+string(rune('A'+i)))
		body := testPlanBody
		planURL := "/api/v1/projects/" + wsID + "/issues/" + issueID + "/plan"
		req := httptest.NewRequest(http.MethodPost, planURL, bytes.NewBufferString(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("setIssuePlan failed: %s", rec.Body.String())
		}
		var plan types.Plan
		if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
			t.Fatalf("failed to parse plan: %v", err)
		}
		planIDs = append(planIDs, plan.ID)
	}

	// Approve the first plan
	statusBody := `{"status": "approved"}`
	statusURL := "/api/v1/projects/" + wsID + "/plans/" + planIDs[0] + "/status"
	req := httptest.NewRequest(http.MethodPatch, statusURL, bytes.NewBufferString(statusBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("updatePlanStatus returned %d: %s", rec.Code, rec.Body.String())
	}

	// List only draft plans
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+wsID+"/plans?status=draft", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("listPlans returned %d: %s", rec.Code, rec.Body.String())
	}

	var plans []*types.Plan
	if err := json.Unmarshal(rec.Body.Bytes(), &plans); err != nil {
		t.Fatalf("failed to parse plans: %v", err)
	}

	if len(plans) != 1 {
		t.Errorf("listPlans with status=draft returned %d plans, want 1", len(plans))
	}
}

func TestUpdatePlanStatus(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)
	issueID := createTestIssue(t, e, wsID, "Issue for Status")

	// Create plan
	body := testPlanBody
	planURL := "/api/v1/projects/" + wsID + "/issues/" + issueID + "/plan"
	req := httptest.NewRequest(http.MethodPost, planURL, bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var plan types.Plan
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("failed to parse plan: %v", err)
	}

	// Update status to approved
	statusBody := `{"status": "approved"}`
	statusURL := "/api/v1/projects/" + wsID + "/plans/" + plan.ID + "/status"
	req = httptest.NewRequest(http.MethodPatch, statusURL, bytes.NewBufferString(statusBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("updatePlanStatus returned %d: %s", rec.Code, rec.Body.String())
	}

	// Verify by getting the plan
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+wsID+"/plans/"+plan.ID, nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var updated types.Plan
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("failed to parse plan: %v", err)
	}

	if updated.Status != "approved" {
		t.Errorf("plan.Status = %q, want %q", updated.Status, "approved")
	}
}

func TestUpdatePlanStatusInvalid(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)
	issueID := createTestIssue(t, e, wsID, "Issue for Invalid Status")

	// Create plan
	body := testPlanBody
	planURL := "/api/v1/projects/" + wsID + "/issues/" + issueID + "/plan"
	req := httptest.NewRequest(http.MethodPost, planURL, bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var plan types.Plan
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("failed to parse plan: %v", err)
	}

	// Try invalid status
	statusBody := `{"status": "invalid_status"}`
	statusURL := "/api/v1/projects/" + wsID + "/plans/" + plan.ID + "/status"
	req = httptest.NewRequest(http.MethodPatch, statusURL, bytes.NewBufferString(statusBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("updatePlanStatus with invalid status returned %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUpdatePlanContent(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)
	issueID := createTestIssue(t, e, wsID, "Issue for Update")

	// Create plan
	body := `{"text": "Original content"}`
	planURL := "/api/v1/projects/" + wsID + "/issues/" + issueID + "/plan"
	req := httptest.NewRequest(http.MethodPost, planURL, bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var plan types.Plan
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("failed to parse plan: %v", err)
	}

	// Update content
	updateBody := `{"title": "Updated Title", "content": "Updated content"}`
	updateURL := "/api/v1/projects/" + wsID + "/plans/" + plan.ID
	req = httptest.NewRequest(http.MethodPut, updateURL, bytes.NewBufferString(updateBody))
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

	if updated.Title != "Updated Title" {
		t.Errorf("updated.Title = %q, want %q", updated.Title, "Updated Title")
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
	issueID := createTestIssue(t, e, wsID, "Issue for Delete")

	// Create plan
	body := `{"text": "Plan to delete"}`
	planURL := "/api/v1/projects/" + wsID + "/issues/" + issueID + "/plan"
	req := httptest.NewRequest(http.MethodPost, planURL, bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var plan types.Plan
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("failed to parse plan: %v", err)
	}

	// Delete plan
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+wsID+"/plans/"+plan.ID, nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("deletePlan returned %d: %s", rec.Code, rec.Body.String())
	}

	// Verify deletion
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+wsID+"/plans/"+plan.ID, nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("getPlan after delete returned %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetPendingCount(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)

	// Create two draft plans
	for i := range 2 {
		issueID := createTestIssue(t, e, wsID, "Issue "+string(rune('A'+i)))
		body := testPlanBody
		planURL := "/api/v1/projects/" + wsID + "/issues/" + issueID + "/plan"
		req := httptest.NewRequest(http.MethodPost, planURL, bytes.NewBufferString(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("setIssuePlan failed: %s", rec.Body.String())
		}
	}

	// Get pending count
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+wsID+"/plans/pending-count", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("getPendingCount returned %d: %s", rec.Code, rec.Body.String())
	}

	var result map[string]int
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse pending count: %v", err)
	}

	if result["count"] != 2 {
		t.Errorf("pending count = %d, want 2", result["count"])
	}
}

func TestGetPendingCountZero(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)

	// Get pending count with no plans
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+wsID+"/plans/pending-count", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("getPendingCount returned %d: %s", rec.Code, rec.Body.String())
	}

	var result map[string]int
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse pending count: %v", err)
	}

	if result["count"] != 0 {
		t.Errorf("pending count = %d, want 0", result["count"])
	}
}

func TestGetPlanNotFound(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+wsID+"/plans/plan.nonexistent", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("getPlan for nonexistent plan returned %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestPlanWorkspaceIsolation(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	// Create two workspaces
	wsID1 := createTestWorkspace(t, e)

	body := `{"name": "Other Workspace", "prefix": "other"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	var ws2 types.Workspace
	if err := json.Unmarshal(rec.Body.Bytes(), &ws2); err != nil {
		t.Fatalf("failed to parse workspace: %v", err)
	}
	wsID2 := ws2.ID

	// Create plan in workspace 1 via issue
	issueID := createTestIssue(t, e, wsID1, "WS1 Issue")
	body = `{"text": "WS1 plan content"}`
	planURL := "/api/v1/projects/" + wsID1 + "/issues/" + issueID + "/plan"
	req = httptest.NewRequest(http.MethodPost, planURL, bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var plan types.Plan
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("failed to parse plan: %v", err)
	}

	// Try to access plan from workspace 2 - should fail
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+wsID2+"/plans/"+plan.ID, nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("getPlan from wrong workspace returned %d, want %d", rec.Code, http.StatusForbidden)
	}
}
