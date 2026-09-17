package api //nolint:testpackage // Tests exercise project routes and real storage.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

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
