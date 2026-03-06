package api //nolint:testpackage // tests use internal helpers that access unexported fields

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/storage/sqlite"
	"github.com/sentiolabs/arc/internal/types"
)

// setupGitRepo creates a temp dir with a git repo that has two branches with a diff.
// Returns the repo path, base branch name, and head branch name.
func setupGitRepo(t *testing.T) (repoPath, base, head string) {
	t.Helper()

	repoPath = t.TempDir()

	// Initialize git repo
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	run("init", "-b", "main")
	// Create initial file and commit on main
	if err := os.WriteFile(filepath.Join(repoPath, "file.txt"), []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "initial")

	// Create a feature branch with changes
	run("checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(repoPath, "file.txt"), []byte("hello\nworld\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "new.txt"), []byte("new file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "add changes")

	return repoPath, "main", "feature"
}

// createTestWorkspaceWithPath creates a workspace with a path field set.
func createTestWorkspaceWithPath(t *testing.T, e *echo.Echo, path string) string {
	t.Helper()

	reqBody := map[string]string{
		"name":   "Review Test Workspace",
		"prefix": "rv",
		"path":   path,
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces", bytes.NewBuffer(bodyBytes))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create workspace: %s", rec.Body.String())
	}

	var ws types.Workspace
	if err := json.Unmarshal(rec.Body.Bytes(), &ws); err != nil {
		t.Fatalf("failed to parse workspace: %v", err)
	}
	return ws.ID
}

func TestReviewCreateAndGetDiff(t *testing.T) {
	// Override HOME so diff files go to a temp dir
	t.Setenv("HOME", t.TempDir())

	repoPath, base, head := setupGitRepo(t)

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	server := New(Config{Address: ":0", Store: store})
	e := server.echo

	wsID := createTestWorkspaceWithPath(t, e, repoPath)

	// POST /review — create a review session
	createBody, _ := json.Marshal(map[string]string{
		"base": base,
		"head": head,
	})
	reviewURL := fmt.Sprintf("/api/v1/workspaces/%s/review", wsID)
	req := httptest.NewRequest(http.MethodPost, reviewURL, bytes.NewBuffer(createBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("createReview returned %d: %s", rec.Code, rec.Body.String())
	}

	var session reviewSession
	if err := json.Unmarshal(rec.Body.Bytes(), &session); err != nil {
		t.Fatalf("failed to parse review session: %v", err)
	}

	if session.ID == "" {
		t.Error("session ID should not be empty")
	}
	if session.WorkspaceID != wsID {
		t.Errorf("workspace_id = %q, want %q", session.WorkspaceID, wsID)
	}
	if session.Status != "pending" {
		t.Errorf("status = %q, want %q", session.Status, "pending")
	}
	if session.Stats == nil {
		t.Fatal("stats should not be nil")
	}
	if session.Stats.FilesChanged < 1 {
		t.Errorf("files_changed = %d, want >= 1", session.Stats.FilesChanged)
	}
	if session.Stats.Insertions < 1 {
		t.Errorf("insertions = %d, want >= 1", session.Stats.Insertions)
	}

	// GET /review/:rid/diff — get the diff as text/plain
	diffURL := fmt.Sprintf(
		"/api/v1/workspaces/%s/review/%s/diff", wsID, session.ID,
	)
	req = httptest.NewRequest(http.MethodGet, diffURL, nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("getReviewDiff returned %d: %s", rec.Code, rec.Body.String())
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "text/plain; charset=UTF-8" {
		t.Errorf("content-type = %q, want text/plain", contentType)
	}

	diffBody := rec.Body.String()
	if diffBody == "" {
		t.Error("diff body should not be empty")
	}
	if len(diffBody) < 10 {
		t.Errorf("diff body too short: %q", diffBody)
	}
}

func TestReviewGetStatus(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	repoPath, base, head := setupGitRepo(t)

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	server := New(Config{Address: ":0", Store: store})
	e := server.echo

	wsID := createTestWorkspaceWithPath(t, e, repoPath)

	// Create review
	createBody, _ := json.Marshal(map[string]string{"base": base, "head": head})
	reviewURL := fmt.Sprintf("/api/v1/workspaces/%s/review", wsID)
	req := httptest.NewRequest(http.MethodPost, reviewURL, bytes.NewBuffer(createBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var session reviewSession
	if err := json.Unmarshal(rec.Body.Bytes(), &session); err != nil {
		t.Fatalf("failed to parse session: %v", err)
	}

	// GET /review/:rid/status
	statusURL := fmt.Sprintf(
		"/api/v1/workspaces/%s/review/%s/status", wsID, session.ID,
	)
	req = httptest.NewRequest(http.MethodGet, statusURL, nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("getReviewStatus returned %d: %s", rec.Code, rec.Body.String())
	}

	var status reviewSession
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("failed to parse status: %v", err)
	}
	if status.Status != "pending" {
		t.Errorf("status = %q, want %q", status.Status, "pending")
	}
}

func TestReviewSubmit(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	repoPath, base, head := setupGitRepo(t)

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	server := New(Config{Address: ":0", Store: store})
	e := server.echo

	wsID := createTestWorkspaceWithPath(t, e, repoPath)

	// Create review
	createBody, _ := json.Marshal(map[string]string{"base": base, "head": head})
	reviewURL := fmt.Sprintf("/api/v1/workspaces/%s/review", wsID)
	req := httptest.NewRequest(http.MethodPost, reviewURL, bytes.NewBuffer(createBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var session reviewSession
	if err := json.Unmarshal(rec.Body.Bytes(), &session); err != nil {
		t.Fatalf("failed to parse session: %v", err)
	}

	// POST /review/:rid/submit — submit review decision
	submitBody, _ := json.Marshal(map[string]any{
		"decision":      "approved",
		"comment":       "LGTM",
		"file_comments": map[string]string{"file.txt": "nice change"},
	})
	submitURL := fmt.Sprintf(
		"/api/v1/workspaces/%s/review/%s/submit", wsID, session.ID,
	)
	req = httptest.NewRequest(http.MethodPost, submitURL, bytes.NewBuffer(submitBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("submitReview returned %d: %s", rec.Code, rec.Body.String())
	}

	var updated reviewSession
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("failed to parse submit response: %v", err)
	}
	if updated.Status != "approved" {
		t.Errorf("status = %q, want %q", updated.Status, "approved")
	}
	if updated.Comment != "LGTM" {
		t.Errorf("comment = %q, want %q", updated.Comment, "LGTM")
	}
	if updated.FileComments == nil || updated.FileComments["file.txt"] != "nice change" {
		t.Errorf("file_comments = %v, want file.txt='nice change'", updated.FileComments)
	}
}

func TestReviewNotFound(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)

	// GET status for nonexistent review
	statusURL := fmt.Sprintf(
		"/api/v1/workspaces/%s/review/nonexistent/status", wsID,
	)
	req := httptest.NewRequest(http.MethodGet, statusURL, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestReviewNoPath(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	// Create workspace without path
	wsID := createTestWorkspace(t, e)

	createBody, _ := json.Marshal(map[string]string{"base": "main", "head": "feature"})
	reviewURL := fmt.Sprintf("/api/v1/workspaces/%s/review", wsID)
	req := httptest.NewRequest(http.MethodPost, reviewURL, bytes.NewBuffer(createBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for workspace without path, got %d: %s", rec.Code, rec.Body.String())
	}
}
