package api //nolint:testpackage // Tests exercise project routes and real storage.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/types"
	"github.com/stretchr/testify/require"
)

// adoptionRouteFixture keeps plan publication and request validation real while
// using a disposable server root and database for each route scenario.
//
//nolint:revive // Fixture returns server, project, root, task and approved pin in that order.
func adoptionRouteFixture(t *testing.T) (*Server, string, string, string, *types.PlanReference) {
	t.Helper()
	s, cleanup := testServer(t)
	t.Cleanup(cleanup)
	p := createTestProject(t, s.echo)
	ctx := context.Background()
	root := &types.Issue{ProjectID: p, Title: "root", IssueType: types.TypeEpic}
	require.NoError(t, s.store.CreateIssue(ctx, root, "fixture"))
	task := &types.Issue{ProjectID: p, Title: "task", ParentID: root.ID}
	require.NoError(t, s.store.CreateIssue(ctx, task, "fixture"))
	plan, err := s.store.CreateDurablePlan(ctx, p, "design", storage.PlanUpload{Content: "design"})
	require.NoError(t, err)
	_, err = s.store.DecidePlanRevision(
		ctx,
		p,
		plan.Plan.ID,
		1,
		types.PlanReviewRequest{Status: "in_review", ExpectedHead: 1, ExpectedReviewVersion: 0},
	)
	require.NoError(t, err)
	_, err = s.store.DecidePlanRevision(
		ctx,
		p,
		plan.Plan.ID,
		1,
		types.PlanReviewRequest{Status: "approved", ExpectedHead: 1, ExpectedReviewVersion: 1},
	)
	require.NoError(t, err)
	return s, p, root.ID, task.ID, &types.PlanReference{PlanID: plan.Plan.ID, Revision: 1}
}

func apiCaptured(t *testing.T, s *Server, p, id string) types.ExpectedGovernance {
	t.Helper()
	i, err := s.store.GetIssue(context.Background(), id)
	require.NoError(t, err)
	g, err := s.store.ResolveGoverningPlan(context.Background(), p, id)
	require.NoError(t, err)
	return types.ExpectedGovernance{Governing: g, ContractVersion: i.ContractVersion}
}

//
//nolint:revive // Fixture requires project, container, task and target reference.
func apiAdoption(
	t *testing.T,
	s *Server,
	p, root, task string,
	pin *types.PlanReference,
) types.PlanAdoptionRequest {
	t.Helper()
	i, err := s.store.GetIssue(context.Background(), root)
	require.NoError(t, err)
	project, err := s.store.GetProject(context.Background(), p)
	require.NoError(t, err)
	return types.PlanAdoptionRequest{
		ExpectedContainerVersion:     i.ContractVersion,
		ExpectedGovernanceGeneration: project.GovernanceGeneration,
		ExpectedPin:                  i.GoverningPlan,
		TargetPin:                    pin,
		Tasks: []types.ReconciledTask{
			{
				IssueID:     task,
				Expected:    apiCaptured(t, s, p, task),
				Disposition: "unchanged",
				Reason:      "consistent",
			},
		},
	}
}

func jsonBody(t *testing.T, value any) string {
	t.Helper()
	b, err := json.Marshal(value)
	require.NoError(t, err)
	return string(b)
}

type planRequestProvenance struct {
	key       string
	actor     string
	sessionID string
}

func planRequestWithProvenance(
	e *echo.Echo,
	method, path, body string,
	provenance planRequestProvenance,
) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if provenance.key != "" {
		req.Header.Set("Idempotency-Key", provenance.key)
	}
	if provenance.actor != "" {
		req.Header.Set("X-Actor", provenance.actor)
	}
	if provenance.sessionID != "" {
		req.Header.Set("X-AI-Session-ID", provenance.sessionID)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestAdoptionRoutePersistsRequestProvenanceAndOriginalReplay(t *testing.T) {
	s, p, root, task, pin := adoptionRouteFixture(t)
	require.NoError(t, s.store.UpdateIssue(
		context.Background(), root, map[string]any{"ai_session_id": "claimant-session"}, "claimant",
	))
	path := "/api/v1/projects/" + p + "/issues/" + root + "/plan-adoption"
	request := apiAdoption(t, s, p, root, task, pin)
	request.Tasks[0].Disposition = "follow_up"
	request.Tasks[0].FollowUpKeys = []string{"repair"}
	request.FollowUps = []types.ReconciliationFollowUp{{
		Key:       "repair",
		ParentID:  root,
		Title:     "repair",
		IssueType: types.TypeTask,
		Priority:  2,
	}}

	response := planRequestWithProvenance(
		s.echo,
		http.MethodPost,
		path,
		jsonBody(t, request),
		planRequestProvenance{key: "adopt-key", actor: "caller", sessionID: "caller-session"},
	)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var created storage.PlanAdoptionResult
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &created))
	require.Equal(t, "caller", created.Actor)
	require.Equal(t, "caller-session", created.SessionID)
	require.Len(t, created.FollowUpIDs, 1)

	replay := planRequestWithProvenance(
		s.echo,
		http.MethodPost,
		path,
		jsonBody(t, request),
		planRequestProvenance{key: "adopt-key", actor: "later-caller", sessionID: "later-session"},
	)
	require.Equal(t, http.StatusOK, replay.Code, replay.Body.String())
	var replayed storage.PlanAdoptionResult
	require.NoError(t, json.Unmarshal(replay.Body.Bytes(), &replayed))
	require.True(t, replayed.Replay)
	require.Equal(t, "caller", replayed.Actor)
	require.Equal(t, "caller-session", replayed.SessionID)
	require.Equal(t, created.FollowUpIDs, replayed.FollowUpIDs)

	detach := apiAdoption(t, s, p, root, task, nil)
	followUpID := created.FollowUpIDs["repair"]
	detach.Tasks = append(detach.Tasks, types.ReconciledTask{
		IssueID:     followUpID,
		Expected:    apiCaptured(t, s, p, followUpID),
		Disposition: "unchanged",
		Reason:      "consistent",
	})
	dryRun := detach
	dryRun.DryRun = true
	valid := planRequestWithProvenance(
		s.echo,
		http.MethodPost,
		path,
		jsonBody(t, dryRun),
		planRequestProvenance{actor: "later-caller", sessionID: "later-session"},
	)
	require.Equal(t, http.StatusOK, valid.Code, valid.Body.String())
	var proposal storage.PlanAdoptionResult
	require.NoError(t, json.Unmarshal(valid.Body.Bytes(), &proposal))
	require.Empty(t, proposal.Errors)
	require.NoError(t, s.store.UpdateIssue(
		context.Background(), task, map[string]any{"ai_session_id": "changed-claimant-session"}, "changed-claimant",
	))
	stale := planRequestWithProvenance(
		s.echo,
		http.MethodPost,
		path,
		jsonBody(t, detach),
		planRequestProvenance{key: "stale-detach-key", actor: "later-caller", sessionID: "later-session"},
	)
	require.Equal(t, http.StatusConflict, stale.Code, stale.Body.String())
	history := planRequest(s.echo, http.MethodGet, "/api/v1/projects/"+p+"/issues/"+root+"/plan-adoptions", "", "")
	require.Equal(t, http.StatusOK, history.Code, history.Body.String())
	var records []storage.PlanAdoptionResult
	require.NoError(t, json.Unmarshal(history.Body.Bytes(), &records))
	require.Len(t, records, 1)
	require.Equal(t, "caller", records[0].Actor)
	require.Equal(t, "caller-session", records[0].SessionID)
	issues, err := s.store.ListIssues(context.Background(), types.IssueFilter{ProjectID: p})
	require.NoError(t, err)
	require.Len(t, issues, 3)

	anonymousServer, anonymousProject, anonymousRoot, anonymousTask, anonymousPin := adoptionRouteFixture(t)
	anonymousRequest := apiAdoption(
		t, anonymousServer, anonymousProject, anonymousRoot, anonymousTask, anonymousPin,
	)
	anonymous := planRequestWithProvenance(
		anonymousServer.echo,
		http.MethodPost,
		"/api/v1/projects/"+anonymousProject+"/issues/"+anonymousRoot+"/plan-adoption",
		jsonBody(t, anonymousRequest),
		planRequestProvenance{key: "anonymous-key"},
	)
	require.Equal(t, http.StatusOK, anonymous.Code, anonymous.Body.String())
	var anonymousResult storage.PlanAdoptionResult
	require.NoError(t, json.Unmarshal(anonymous.Body.Bytes(), &anonymousResult))
	require.Equal(t, "anonymous", anonymousResult.Actor)
	require.Empty(t, anonymousResult.SessionID)
}

func TestAdoptionRouteRequiredPinsDryRunAndReplay(t *testing.T) {
	s, p, root, task, pin := adoptionRouteFixture(t)
	path := "/api/v1/projects/" + p + "/issues/" + root + "/plan-adoption"
	req := apiAdoption(t, s, p, root, task, pin)
	for _, key := range []string{
		expectedPinField,
		"target_pin",
		expectedContainerVersionField,
		"expected_governance_generation",
	} {
		var fields map[string]any
		require.NoError(t, json.Unmarshal([]byte(jsonBody(t, req)), &fields))
		delete(fields, key)
		r := planRequest(s.echo, "POST", path, jsonBody(t, fields), "key")
		require.Equal(t, 428, r.Code, r.Body.String())
	}
	req.DryRun = true
	req.Tasks = nil
	r := planRequest(s.echo, "POST", path, jsonBody(t, req), "")
	require.Equal(t, 200, r.Code, r.Body.String())
	var proposal storage.PlanAdoptionResult
	require.NoError(t, json.Unmarshal(r.Body.Bytes(), &proposal))
	require.Len(t, proposal.Tasks, 1)
	require.NotEmpty(t, proposal.Errors)
	req = apiAdoption(t, s, p, root, task, pin)
	r = planRequest(s.echo, "POST", path, jsonBody(t, req), "")
	require.Equal(t, 428, r.Code, r.Body.String())
	r = planRequest(s.echo, "POST", path, jsonBody(t, req), "key")
	require.Equal(t, 200, r.Code, r.Body.String())
	r = planRequest(s.echo, "POST", path, jsonBody(t, req), "key")
	require.Equal(t, 200, r.Code, r.Body.String())
	require.Contains(t, r.Body.String(), `"replay":true`)
	r = planRequest(s.echo, "GET", "/api/v1/projects/"+p+"/issues/"+root+"/plan-adoptions", "", "")
	require.Equal(t, 200, r.Code, r.Body.String())
	require.Contains(t, r.Body.String(), `"reason":"consistent"`)
}

func TestAllCompletionRoutesRejectMissingAndStaleContext(t *testing.T) {
	for _, route := range []string{"global_put", "project_put", "global_close", "project_close", evidenceField} {
		t.Run(route, func(t *testing.T) {
			s, p, root, task, pin := adoptionRouteFixture(t)
			before := apiCaptured(t, s, p, task)
			req := apiAdoption(t, s, p, root, task, pin)
			_, err := s.store.AdoptPlan(context.Background(), p, root, "attach", req)
			require.NoError(t, err)
			path := "/api/v1/projects/" + p + "/issues/" + task
			method := "PUT"
			body := map[string]any{"status": "closed"}
			switch route {
			case "global_put":
				path = "/api/v1/issues/" + task
			case "global_close":
				path = "/api/v1/issues/" + task + "/close"
				method = "POST"
				body = map[string]any{closeReasonField: "done"}
			case "project_close":
				path += "/close"
				method = "POST"
				body = map[string]any{closeReasonField: "done"}
			case evidenceField:
				path += "/execution-evidence"
				method = "POST"
				body = map[string]any{"phase": "verify", evidenceField: "tests"}
			}
			for _, expected := range []any{nil, before} {
				if expected == nil {
					delete(body, "expected")
				} else {
					body["expected"] = expected
				}
				r := planRequest(s.echo, method, path, jsonBody(t, body), "")
				code := 409
				if expected == nil {
					code = 428
				}
				require.Equal(t, code, r.Code, r.Body.String())
			}
			body["expected"] = apiCaptured(t, s, p, task)
			r := planRequest(s.echo, method, path, jsonBody(t, body), "")
			want := http.StatusOK
			if route == evidenceField {
				want = http.StatusCreated
			}
			require.Equal(t, want, r.Code, r.Body.String())
		})
	}
}

func TestGovernedCascadeAndExplicitNullCompletion(t *testing.T) {
	s, p, root, task, pin := adoptionRouteFixture(t)
	_, err := s.store.AdoptPlan(
		context.Background(),
		p,
		root,
		"attach",
		apiAdoption(t, s, p, root, task, pin),
	)
	require.NoError(t, err)
	body := fmt.Sprintf(`{"cascade":true,"expected":%s}`, jsonBody(t, apiCaptured(t, s, p, root)))
	r := planRequest(s.echo, "POST", "/api/v1/issues/"+root+"/close", body, "")
	require.Equal(t, 409, r.Code, r.Body.String())
	r = planRequest(s.echo, "POST", "/api/v1/issues/"+task+"/close", `{"expected":null}`, "")
	require.Equal(t, 428, r.Code, r.Body.String())
}

func TestAdoptionNestedPreconditionPresence(t *testing.T) {
	s, p, root, task, pin := adoptionRouteFixture(t)
	request := apiAdoption(t, s, p, root, task, pin)
	path := "/api/v1/projects/" + p + "/issues/" + root + "/plan-adoption"
	for _, expected := range []any{
		nil, map[string]any{}, map[string]any{"governing": nil}, map[string]any{"contract_version": 1},
	} {
		var fields map[string]any
		require.NoError(t, json.Unmarshal([]byte(jsonBody(t, request)), &fields))
		tasks := fields["tasks"].([]any)
		tasks[0].(map[string]any)["expected"] = expected
		response := planRequest(s.echo, http.MethodPost, path, jsonBody(t, fields), "key")
		require.Equal(t, http.StatusPreconditionRequired, response.Code, response.Body.String())
	}
}

func TestCloseOpenAPIConflictResponses(t *testing.T) {
	schema, err := GetSwagger()
	require.NoError(t, err)
	for _, path := range []string{"/issues/{issueId}/close", "/projects/{projectId}/issues/{issueId}/close"} {
		response := schema.Paths.Value(path).Post.Responses.Value("409")
		require.NotNil(t, response, path)
		require.NotNil(t, response.Value)
		require.Equal(
			t,
			"#/components/schemas/Error",
			response.Value.Content["application/json"].Schema.Ref,
		)
	}
}
