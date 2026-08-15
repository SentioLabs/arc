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

// testServer creates a test server with a temporary SQLite database.
func testServer(t *testing.T) (*Server, func()) {
	t.Helper()

	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	server := New(ServerOptions{
		Address: ":0",
		Store:   store,
	})

	cleanup := func() {
		store.Close()
	}

	return server, cleanup
}

// createTestProject creates a project for testing and returns its ID.
func createTestProject(t *testing.T, e *echo.Echo) string {
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
func createTestIssue(t *testing.T, e *echo.Echo, pID, title string) string {
	t.Helper()

	body := `{"title": "` + title + `", "type": "task", "priority": 2}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+pID+"/issues", bytes.NewBufferString(body))
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

// createTestPlan creates a plan for testing and returns its ID.
func createTestPlan(t *testing.T, e *echo.Echo) string {
	t.Helper()

	filePath := filepath.Join(t.TempDir(), "plan.md")
	encodedPath, err := json.Marshal(filePath)
	if err != nil {
		t.Fatalf("failed to encode file path: %v", err)
	}
	body := `{"file_path": ` + string(encodedPath) + `}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/plans", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create plan: %s", rec.Body.String())
	}

	var plan types.Plan
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("failed to parse plan response: %v", err)
	}

	return plan.ID
}

// createTestPlanComment creates a plan comment for testing and returns the decoded comment.
func createTestPlanComment(t *testing.T, e *echo.Echo, planID, body string) types.PlanComment {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/plans/"+planID+"/comments", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create plan comment: %s", rec.Body.String())
	}

	var comment types.PlanComment
	if err := json.Unmarshal(rec.Body.Bytes(), &comment); err != nil {
		t.Fatalf("failed to parse plan comment response: %v", err)
	}

	return comment
}

// patchPlanComment sends a PATCH request to a plan comment and returns the raw response.
func patchPlanComment(e *echo.Echo, planID, commentID, body string) *httptest.ResponseRecorder {
	url := "/api/v1/plans/" + planID + "/comments/" + commentID
	req := httptest.NewRequest(http.MethodPatch, url, bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestCreatePlanComment_WithAnchorMirrorsLineNumber(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.Echo()

	planID := createTestPlan(t, e)

	body := `{
		"content": "please clarify",
		"anchor": {
			"line_start": 5,
			"line_end": 7,
			"quoted_text": "some quoted text",
			"occurrence": 0
		}
	}`
	comment := createTestPlanComment(t, e, planID, body)

	if comment.Anchor == nil {
		t.Fatalf("expected anchor to be present in response")
	}
	if comment.Anchor.LineStart != 5 || comment.Anchor.LineEnd != 7 {
		t.Errorf("anchor not stored correctly: %+v", comment.Anchor)
	}
	if comment.Anchor.QuotedText != "some quoted text" {
		t.Errorf("expected quoted_text to round-trip, got %q", comment.Anchor.QuotedText)
	}
	if comment.LineNumber == nil || *comment.LineNumber != 5 {
		t.Errorf("expected line_number to mirror anchor.line_start (5), got %v", comment.LineNumber)
	}
}

func TestCreatePlanComment_InvalidAnchor(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.Echo()

	planID := createTestPlan(t, e)

	tests := []struct {
		name   string
		anchor string
	}{
		{"line_start zero", `{"line_start":0,"line_end":1,"quoted_text":"x","occurrence":0}`},
		{"empty quoted_text", `{"line_start":1,"line_end":1,"quoted_text":"","occurrence":0}`},
		{"line_end less than line_start", `{"line_start":5,"line_end":4,"quoted_text":"x","occurrence":0}`},
		{"negative occurrence", `{"line_start":1,"line_end":1,"quoted_text":"x","occurrence":-1}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"content":"hi","anchor":` + tt.anchor + `}`
			url := "/api/v1/plans/" + planID + "/comments"
			req := httptest.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestUpdatePlanComment_ContentOnly(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.Echo()

	planID := createTestPlan(t, e)
	comment := createTestPlanComment(t, e, planID, `{
		"content": "original",
		"anchor": {"line_start": 2, "line_end": 3, "quoted_text": "quote", "occurrence": 0}
	}`)

	rec := patchPlanComment(e, planID, comment.ID, `{"content":"updated content"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var updated types.PlanComment
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if updated.Content != "updated content" {
		t.Errorf("expected content to be updated, got %q", updated.Content)
	}
	if updated.UpdatedAt == nil {
		t.Errorf("expected updated_at to be set")
	}
	if updated.Anchor == nil || updated.Anchor.LineStart != 2 || updated.Anchor.QuotedText != "quote" {
		t.Errorf("expected anchor to be left untouched, got %+v", updated.Anchor)
	}
}

func TestUpdatePlanComment_AnchorReplacesAndMirrorsLineNumber(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.Echo()

	planID := createTestPlan(t, e)
	comment := createTestPlanComment(t, e, planID, `{"content":"original"}`)

	body := `{"anchor":{"line_start":10,"line_end":12,"quoted_text":"new quote","occurrence":1}}`
	rec := patchPlanComment(e, planID, comment.ID, body)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var updated types.PlanComment
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if updated.Anchor == nil || updated.Anchor.LineStart != 10 || updated.Anchor.QuotedText != "new quote" {
		t.Errorf("expected anchor to be replaced, got %+v", updated.Anchor)
	}
	if updated.LineNumber == nil || *updated.LineNumber != 10 {
		t.Errorf("expected line_number to re-mirror anchor.line_start (10), got %v", updated.LineNumber)
	}
}

func TestUpdatePlanComment_ResolvedSetsAndClearsResolvedAt(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.Echo()

	planID := createTestPlan(t, e)
	comment := createTestPlanComment(t, e, planID, `{"content":"original"}`)

	// Resolve.
	rec := patchPlanComment(e, planID, comment.ID, `{"resolved":true}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resolved types.PlanComment
	if err := json.Unmarshal(rec.Body.Bytes(), &resolved); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resolved.ResolvedAt == nil {
		t.Fatalf("expected resolved_at to be set")
	}

	// Un-resolve.
	rec2 := patchPlanComment(e, planID, comment.ID, `{"resolved":false}`)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec2.Code, rec2.Body.String())
	}

	var unresolved types.PlanComment
	if err := json.Unmarshal(rec2.Body.Bytes(), &unresolved); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if unresolved.ResolvedAt != nil {
		t.Errorf("expected resolved_at to be cleared, got %v", unresolved.ResolvedAt)
	}
}

func TestUpdatePlanComment_DifferentPlanReturns404(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.Echo()

	planA := createTestPlan(t, e)
	planB := createTestPlan(t, e)
	comment := createTestPlanComment(t, e, planA, `{"content":"original"}`)

	rec := patchPlanComment(e, planB, comment.ID, `{"content":"hijack"}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdatePlanComment_UnknownCommentOrPlanReturns404(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.Echo()

	planID := createTestPlan(t, e)

	// Unknown comment on a known plan.
	rec := patchPlanComment(e, planID, "pc.unknown", `{"content":"x"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown comment, got %d: %s", rec.Code, rec.Body.String())
	}

	// Known comment on an unknown plan.
	comment := createTestPlanComment(t, e, planID, `{"content":"original"}`)
	rec2 := patchPlanComment(e, "plan.unknown", comment.ID, `{"content":"x"}`)
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown plan, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

func TestDeletePlanComment(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.Echo()

	planID := createTestPlan(t, e)
	comment := createTestPlanComment(t, e, planID, `{"content":"original"}`)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/plans/"+planID+"/comments/"+comment.ID, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify it's gone from the list.
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/plans/"+planID+"/comments", nil)
	listRec := httptest.NewRecorder()
	e.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}

	var comments []types.PlanComment
	if err := json.Unmarshal(listRec.Body.Bytes(), &comments); err != nil {
		t.Fatalf("failed to parse list response: %v", err)
	}
	for _, c := range comments {
		if c.ID == comment.ID {
			t.Fatalf("expected comment %s to be deleted, but it's still in the list", comment.ID)
		}
	}
}

func TestDeletePlanComment_UnknownCommentOrPlanReturns404(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.Echo()

	planID := createTestPlan(t, e)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/plans/"+planID+"/comments/pc.unknown", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown comment, got %d: %s", rec.Code, rec.Body.String())
	}

	comment := createTestPlanComment(t, e, planID, `{"content":"original"}`)
	req2 := httptest.NewRequest(http.MethodDelete, "/api/v1/plans/plan.unknown/comments/"+comment.ID, nil)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown plan, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

func TestUpdatePlanStatus_ChangesRequestedAccepted(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.Echo()

	planID := createTestPlan(t, e)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/plans/"+planID+"/status",
		bytes.NewBufferString(`{"status":"changes_requested"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var plan types.Plan
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if plan.Status != types.PlanStatusChangesRequested {
		t.Errorf("expected status changes_requested, got %q", plan.Status)
	}
}
