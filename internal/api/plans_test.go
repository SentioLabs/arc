package api //nolint:testpackage // tests use internal helpers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sentiolabs/arc/internal/planfiles"
	"github.com/sentiolabs/arc/internal/storage"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/storage/sqlite"
	"github.com/sentiolabs/arc/internal/types"
	"github.com/stretchr/testify/require"
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

	publisher, err := planfiles.New(filepath.Join(tmpDir, "plans"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = publisher.Close() })
	server := New(ServerOptions{
		PlanFiles: publisher,
		Address:   ":0",
		Store:     store,
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
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/projects/"+pID+"/issues",
		bytes.NewBufferString(body),
	)
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

func planRequest(e *echo.Echo, method, path, body, key string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestDurableAPIRequiredFieldsAndIsolation(t *testing.T) {
	s, cleanup := testServer(t)
	defer cleanup()
	p := createTestProject(t, s.echo)
	base := "/api/v1/projects/" + p + "/plans"
	for _, tc := range []struct {
		body, key string
		code      int
	}{
		{`{"file_path":"/never/open"}`, "", 400},
		{`{"content":""}`, "", 428},
		{`{"title":"t"}`, "key", 400},
		{`{"content":null}`, "key", 400},
	} {
		r := planRequest(s.echo, "POST", base, tc.body, tc.key)
		if r.Code != tc.code {
			t.Fatalf("create %s: %d %s", tc.body, r.Code, r.Body.String())
		}
	}
	r := planRequest(s.echo, "POST", base, `{"title":"title","content":""}`, "create")
	if r.Code != 201 {
		t.Fatalf("create: %d %s", r.Code, r.Body.String())
	}
	var result storage.PlanWriteResult
	if err := json.Unmarshal(r.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	path := base + "/" + result.Plan.ID
	for _, body := range []string{`{"content":"next"}`, `{"content":"next","expected_revision":null}`} {
		r = planRequest(s.echo, "POST", path+"/revisions", body, "save")
		if r.Code != 428 {
			t.Fatalf("save missing: %d %s", r.Code, r.Body.String())
		}
	}
	for _, body := range []string{`{"status":"in_review","expected_head":1,"expected_feedback_version":0}`, `{
  "status": "in_review",
  "expected_head": 1,
  "expected_review_version": null,
  "expected_feedback_version": 0
}`} {
		r = planRequest(s.echo, "POST", path+"/revisions/1/decisions", body, "")
		if r.Code != 428 {
			t.Fatalf("decision missing: %d %s", r.Code, r.Body.String())
		}
	}
	r = planRequest(
		s.echo,
		"POST",
		path+"/revisions/1/decisions",
		`{"status":"in_review","expected_head":1,"expected_review_version":0,"expected_feedback_version":0}`,
		"",
	)
	if r.Code != 200 {
		t.Fatalf("explicit zero: %d %s", r.Code, r.Body.String())
	}
	for _, suffix := range []string{
		"",
		"/revisions",
		"/revisions/1",
		"/revisions/1/comments",
		"/revisions/1/dispositions",
	} {
		r = planRequest(
			s.echo,
			"GET",
			"/api/v1/projects/unknown/plans/"+result.Plan.ID+suffix,
			"",
			"",
		)
		if r.Code != 404 {
			t.Fatalf("isolation %s: %d %s", suffix, r.Code, r.Body.String())
		}
	}
}

func TestLegacyPlanRoutesRequireUpgrade(t *testing.T) {
	s, cleanup := testServer(t)
	defer cleanup()
	for _, tc := range []struct{ method, path string }{
		{"POST", "/plans"},
		{"GET", "/plans/old"},
		{"PUT", "/plans/old"},
		{"PATCH", "/plans/old/status"},
		{"DELETE", "/plans/old"},
		{"POST", "/plans/old/comments"},
		{"PATCH", "/plans/old/comments/c"},
		{"DELETE", "/plans/old/comments/c"},
	} {
		r := planRequest(s.echo, tc.method, "/api/v1"+tc.path, `{"file_path":"/not/read"}`, "")
		if r.Code != 400 || !strings.Contains(r.Body.String(), "upgrade") {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, r.Code, r.Body.String())
		}
	}
}

func TestDurableAPIRestartRetainsExactClientBytes(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "server.db")
	root := filepath.Join(t.TempDir(), "server-root")
	open := func() (*Server, func()) {
		store, err := sqlite.New(dbPath)
		if err != nil {
			t.Fatal(err)
		}
		publisher, err := planfiles.New(root)
		if err != nil {
			t.Fatal(err)
		}
		return New(
			ServerOptions{Store: store, PlanFiles: publisher},
		), func() { _ = store.Close(); _ = publisher.Close() }
	}
	server, closeServer := open()
	project := createTestProject(t, server.echo)
	content := "# retained\r\n\nUTF-8: λ\n"
	source := filepath.Join(t.TempDir(), "client.md")
	if err := os.WriteFile(source, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(
		storage.PlanUpload{Title: "retained", Content: content, SourceName: source},
	)
	if err != nil {
		t.Fatal(err)
	}
	base := "/api/v1/projects/" + project + "/plans"
	rec := planRequest(server.echo, "POST", base, string(body), "create")
	if rec.Code != 201 {
		t.Fatal(rec.Body.String())
	}
	var first storage.PlanWriteResult
	if err = json.Unmarshal(rec.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(source); err != nil {
		t.Fatal(err)
	}
	closeServer()
	server, closeServer = open()
	defer closeServer()
	rec = planRequest(server.echo, "GET", base+"/"+first.Plan.ID+"/revisions/1", "", "")
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var retained types.PlanRevisionWithContent
	if err = json.Unmarshal(rec.Body.Bytes(), &retained); err != nil {
		t.Fatal(err)
	}
	if retained.Content != content || retained.ContentSHA256 != first.Revision.ContentSHA256 ||
		retained.ContentBytes != int64(len(content)) {
		t.Fatalf("bytes changed after restart: %+v", retained)
	}
}

func TestDurableAPICommentVersionAndReviewIsolation(t *testing.T) {
	s, cleanup := testServer(t)
	defer cleanup()
	p := createTestProject(t, s.echo)
	base := "/api/v1/projects/" + p + "/plans"
	rec := planRequest(s.echo, "POST", base, `{"content":"anchored bytes"}`, "create")
	var result storage.PlanWriteResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	rev := base + "/" + result.Plan.ID + "/revisions/1"
	rec = planRequest(
		s.echo,
		"POST",
		rev+"/comments",
		`{
  "content": "feedback",
  "anchor": {
    "line_start": 1,
    "line_end": 1,
    "quoted_text": "anchored",
    "occurrence": 0
  }
}`,
		"",
	)
	if rec.Code != 201 {
		t.Fatal(rec.Body.String())
	}
	var comment types.PlanComment
	if err := json.Unmarshal(rec.Body.Bytes(), &comment); err != nil {
		t.Fatal(err)
	}
	if comment.LineNumber == nil || *comment.LineNumber != 1 || comment.Version != 1 {
		t.Fatalf("anchor: %+v", comment)
	}
	for _, body := range []string{`{"content":"updated"}`, `{"expected_version":null,"content":"updated"}`} {
		rec = planRequest(s.echo, "PATCH", rev+"/comments/"+comment.ID, body, "")
		if rec.Code != 428 {
			t.Fatalf("missing version: %d %s", rec.Code, rec.Body.String())
		}
	}
	rec = planRequest(
		s.echo,
		"PATCH",
		rev+"/comments/"+comment.ID,
		`{"expected_version":1,"content":"updated"}`,
		"",
	)
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	rec = planRequest(
		s.echo,
		"PATCH",
		rev+"/comments/"+comment.ID,
		`{"expected_version":1,"content":"stale"}`,
		"",
	)
	if rec.Code != 409 {
		t.Fatal(rec.Body.String())
	}
	other := strings.Replace(rev, p, "other-project", 1)
	for _, tc := range []struct{ method, suffix, body string }{
		{"POST", "/comments", `{"content":"intrusion"}`},
		{"PATCH", "/comments/" + comment.ID, `{"expected_version":2,"content":"intrusion"}`},
		{"DELETE", "/comments/" + comment.ID, `{"expected_version":2}`},
		{
			"POST",
			"/decisions",
			`{"status":"in_review","expected_head":1,"expected_review_version":0,"expected_feedback_version":2}`,
		},
		{
			"POST",
			"/dispositions",
			`{"comment_id":"` + comment.ID + `","expected_comment_version":2,` +
				`"expected_feedback_version":2,"disposition":"addressed","reason":"intrusion"}`,
		},
	} {
		rec = planRequest(s.echo, tc.method, other+tc.suffix, tc.body, "")
		if rec.Code != 404 {
			t.Fatalf("%s isolation: %d %s", tc.suffix, rec.Code, rec.Body.String())
		}
	}
}

func TestDurableUploadRejectsUnicodeReplacement(t *testing.T) {
	s, cleanup := testServer(t)
	defer cleanup()
	p := createTestProject(t, s.echo)
	base := "/api/v1/projects/" + p + "/plans"
	for _, body := range []string{"{\"content\":\"\xff\"}", `{"content":"\ud800"}`, `{"content":"\udc00"}`} {
		rec := planRequest(s.echo, "POST", base, body, "unicode")
		if rec.Code != 400 {
			t.Fatalf("accepted replacement for %q: %d %s", body, rec.Code, rec.Body.String())
		}
	}
	rec := planRequest(s.echo, "POST", base, `{"content":"\ud800\udc00"}`, "unicode")
	if rec.Code != 201 {
		t.Fatalf("valid surrogate pair: %d %s", rec.Code, rec.Body.String())
	}
	var result storage.PlanWriteResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Revision.Content != "\U00010000" {
		t.Fatalf("valid content transformed: %q", result.Revision.Content)
	}
}

func TestDurableAPIZeroPreconditionsArePresentButInvalid(t *testing.T) {
	s, cleanup := testServer(t)
	defer cleanup()
	p := createTestProject(t, s.echo)
	base := "/api/v1/projects/" + p + "/plans"
	rec := planRequest(s.echo, "POST", base, `{"content":"base"}`, "create")
	var first storage.PlanWriteResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &first))
	path := base + "/" + first.Plan.ID
	for _, request := range []struct{ method, path, body, key string }{
		{"PATCH", path, `{"expected_version":0,"lifecycle":"archived"}`, ""},
		{"POST", path + "/revisions", `{"content":"next","expected_revision":0}`, "save"},
	} {
		rec = planRequest(s.echo, request.method, request.path, request.body, request.key)
		if rec.Code != 400 {
			t.Fatalf("explicit zero conflated with absence: %d %s", rec.Code, rec.Body.String())
		}
	}
}
