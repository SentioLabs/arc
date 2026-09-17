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

// durableCommentFixture gives ownership tests two real parents and an anchored comment.
func durableCommentFixture(
	t *testing.T,
	s *Server,
) (path, other string, comment types.PlanComment) {
	t.Helper()
	project := createTestProject(t, s.echo)
	base := "/api/v1/projects/" + project + "/plans"
	paths := make([]string, 0, 2)
	for _, key := range []string{"one", "two"} {
		rec := planRequest(s.echo, "POST", base, `{"content":"design"}`, key)
		require.Equal(t, 201, rec.Code, rec.Body.String())
		var result storage.PlanWriteResult
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
		paths = append(paths, base+"/"+result.Plan.ID)
	}
	rec := planRequest(
		s.echo,
		"POST",
		paths[0]+"/revisions/1/comments",
		`{"content":"question","anchor":{"line_start":2,"line_end":4,"quoted_text":"quote",
 "occurrence":1,"heading_slug":"heading","context_before":"before","context_after":"after"}}`,
		"",
	)
	require.Equal(t, 201, rec.Code, rec.Body.String())
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &comment))
	return paths[0], paths[1], comment
}

func TestDurableAPICommentAnchorContract(t *testing.T) {
	s, cleanup := testServer(t)
	defer cleanup()
	path, _, original := durableCommentFixture(t, s)
	comments := path + "/revisions/1/comments"
	for _, anchor := range []string{
		`{"line_start":0,"line_end":1,"quoted_text":"q","occurrence":0}`,
		`{"line_start":1,"line_end":1,"quoted_text":"","occurrence":0}`,
		`{"line_start":3,"line_end":2,"quoted_text":"q","occurrence":0}`,
		`{"line_start":1,"line_end":1,"quoted_text":"q","occurrence":-1}`,
	} {
		for _, method := range []string{"POST", "PATCH"} {
			target := comments
			if method == "PATCH" {
				target += "/" + original.ID
			}
			rec := planRequest(
				s.echo,
				method,
				target,
				`{"content":"invalid","expected_version":1,"anchor":`+anchor+`}`,
				"",
			)
			require.Equal(t, 400, rec.Code, rec.Body.String())
		}
	}
	rec := planRequest(
		s.echo,
		"PATCH",
		comments+"/"+original.ID,
		`{"expected_version":1,"content":"edited"}`,
		"",
	)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	var edited types.PlanComment
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &edited))
	require.Equal(t, original.Anchor, edited.Anchor)
	require.Equal(t, original.LineNumber, edited.LineNumber)
	require.Equal(t, original.CreatedAt, edited.CreatedAt)
	require.Equal(t, "edited", edited.Content)
	require.EqualValues(t, 2, edited.Version)
	require.NotNil(t, edited.UpdatedAt)
	rec = planRequest(
		s.echo,
		"PATCH",
		comments+"/"+original.ID,
		`{"expected_version":2,"anchor":{"line_start":7,"line_end":8,"quoted_text":"new quote","occurrence":0}}`,
		"",
	)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	edited = types.PlanComment{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &edited))
	require.Equal(
		t,
		&types.PlanCommentAnchor{LineStart: 7, LineEnd: 8, QuotedText: "new quote"},
		edited.Anchor,
	)
	require.Equal(t, 7, *edited.LineNumber)
	require.Equal(t, "edited", edited.Content)
	require.EqualValues(t, 3, edited.Version)
}

func TestDurableAPICommentMutationOwnership(t *testing.T) {
	s, cleanup := testServer(t)
	defer cleanup()
	path, other, original := durableCommentFixture(t, s)
	rec := planRequest(
		s.echo,
		"POST",
		path+"/revisions",
		`{"expected_revision":1,"content":"second"}`,
		"second",
	)
	require.Equal(t, 201, rec.Code, rec.Body.String())
	for _, target := range []string{
		other + "/revisions/1/comments/" + original.ID,
		path + "/revisions/2/comments/" + original.ID,
		path + "/revisions/99/comments/" + original.ID,
		path + "/revisions/1/comments/missing",
		path + "-missing/revisions/1/comments/" + original.ID,
	} {
		for _, method := range []string{"PATCH", "DELETE"} {
			rec = planRequest(
				s.echo,
				method,
				target,
				`{"expected_version":1,"content":"intrusion"}`,
				"",
			)
			require.Equal(t, 404, rec.Code, rec.Body.String())
		}
	}
	rec = planRequest(s.echo, "GET", path+"/revisions/1/comments", "", "")
	require.Equal(t, 200, rec.Code, rec.Body.String())
	var retained []types.PlanComment
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &retained))
	require.Equal(t, []types.PlanComment{original}, retained)
}

func TestDurableAPIHistoryReaders(t *testing.T) {
	s, cleanup := testServer(t)
	defer cleanup()
	path, other, comment := durableCommentFixture(t, s)
	rev := path + "/revisions/1"
	rec := planRequest(
		s.echo,
		"POST",
		rev+"/decisions",
		`{"status":"in_review","expected_head":1,"expected_review_version":0,"expected_feedback_version":1}`,
		"",
	)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	rec = planRequest(s.echo, "GET", rev+"/decisions", "", "")
	require.Equal(t, 200, rec.Code, rec.Body.String())
	var events []storage.PlanReviewEvent
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &events))
	require.Len(t, events, 1)
	require.Equal(t, "in_review", events[0].Status)
	require.EqualValues(t, 1, events[0].ReviewVersion)
	require.EqualValues(t, 1, events[0].FeedbackVersion)
	require.Equal(t, "anonymous", events[0].Actor)
	require.Empty(t, events[0].SessionID)
	require.Empty(t, events[0].Dispositions)
	require.False(t, events[0].CreatedAt.IsZero())
	versions := rev + "/comments/" + comment.ID + "/versions"
	rec = planRequest(s.echo, "GET", versions, "", "")
	require.Equal(t, 200, rec.Code, rec.Body.String())
	var snapshots []storage.PlanCommentVersion
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &snapshots))
	require.Len(t, snapshots, 1)
	require.Equal(t, comment, snapshots[0].Comment)

	approval := mutateReviewedAPIHistory(t, s, path, comment.ID)
	rec = planRequest(s.echo, "GET", rev+"/decisions?limit=1&offset=1", "", "")
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &events))
	require.Equal(t, []storage.PlanReviewEvent{approval}, events)
	rec = planRequest(s.echo, "GET", versions, "", "")
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &snapshots))
	require.Len(t, snapshots, 4)
	require.Equal(t, comment, snapshots[0].Comment)
	require.Equal(t, "edited after approval", snapshots[1].Comment.Content)
	require.EqualValues(t, 3, snapshots[2].Comment.Version)
	require.NotNil(t, snapshots[3].Comment.DeletedAt)
	rec = planRequest(s.echo, "GET", versions+"?limit=1&offset=1", "", "")
	require.Equal(t, 200, rec.Code, rec.Body.String())
	var page []storage.PlanCommentVersion
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
	require.Equal(t, snapshots[1:2], page)
	for _, endpoint := range []string{rev + "/decisions", versions} {
		rec = planRequest(s.echo, "GET", endpoint+"?limit=1&offset=200", "", "")
		require.Equal(t, 200, rec.Code, rec.Body.String())
		require.JSONEq(t, `[]`, rec.Body.String())
		for _, query := range []string{"?limit=0", "?limit=201", "?offset=-1"} {
			rec = planRequest(s.echo, "GET", endpoint+query, "", "")
			require.Equal(t, 400, rec.Code, rec.Body.String())
		}
	}
	for _, endpoint := range []string{
		other + "/revisions/1/comments/" + comment.ID + "/versions",
		rev + "/comments/missing/versions",
		path + "/revisions/99/decisions",
		path + "/revisions/2/comments/" + comment.ID + "/versions",
		path + "/revisions/99/comments/" + comment.ID + "/versions",
		strings.Replace(versions, "/projects/", "/projects/missing-", 1),
		strings.Replace(rev+"/decisions", "/projects/", "/projects/missing-", 1),
	} {
		rec = planRequest(s.echo, "GET", endpoint, "", "")
		require.Equal(t, 404, rec.Code, rec.Body.String())
	}
}

// mutateReviewedAPIHistory changes every mutable surface after capturing approval.
// Returning the original HTTP event protects the exact evidence from later changes.
func mutateReviewedAPIHistory(
	t *testing.T,
	s *Server,
	path, commentID string,
) storage.PlanReviewEvent {
	t.Helper()
	rev := path + "/revisions/1"
	rec := planRequest(
		s.echo,
		"POST",
		rev+"/dispositions",
		`{"comment_id":"`+commentID+`","expected_comment_version":1,
 "expected_feedback_version":1,"disposition":"addressed","reason":"original answer"}`,
		"",
	)
	require.Equal(t, 201, rec.Code, rec.Body.String())
	var disposition types.PlanFeedbackDisposition
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &disposition))
	rec = planRequest(
		s.echo,
		"POST",
		rev+"/decisions",
		`{"status":"approved","expected_head":1,"expected_review_version":1,"expected_feedback_version":2}`,
		"",
	)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	rec = planRequest(s.echo, "GET", rev+"/decisions?limit=1&offset=1", "", "")
	require.Equal(t, 200, rec.Code, rec.Body.String())
	var events []storage.PlanReviewEvent
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &events))
	require.Len(t, events, 1)
	require.Equal(t, []*types.PlanFeedbackDisposition{&disposition}, events[0].Dispositions)
	require.Equal(t, "approved", events[0].Status)
	require.EqualValues(t, 2, events[0].ReviewVersion)
	require.EqualValues(t, 2, events[0].FeedbackVersion)
	for _, change := range []struct {
		method, endpoint, body, key string
		status                      int
	}{
		{"PATCH", rev + "/comments/" + commentID, `{"expected_version":1,"content":"edited after approval"}`, "", 200},
		{"PATCH", rev + "/comments/" + commentID, `{"expected_version":2,"reopen":true}`, "", 200},
		{"POST", path + "/revisions", `{"expected_revision":1,"content":"second revision"}`, "second", 201},
		{"DELETE", rev + "/comments/" + commentID, `{"expected_version":3}`, "", 200},
	} {
		rec = planRequest(s.echo, change.method, change.endpoint, change.body, change.key)
		require.Equal(t, change.status, rec.Code, rec.Body.String())
	}
	rec = planRequest(s.echo, "GET", path, "", "")
	require.Equal(t, 200, rec.Code, rec.Body.String())
	var plan types.Plan
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &plan))
	body, err := json.Marshal(
		storage.PlanMetadataUpdate{ExpectedVersion: plan.Version, Lifecycle: "archived"},
	)
	require.NoError(t, err)
	rec = planRequest(s.echo, "PATCH", path, string(body), "")
	require.Equal(t, 200, rec.Code, rec.Body.String())
	return events[0]
}
