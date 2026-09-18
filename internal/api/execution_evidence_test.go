package api //nolint:testpackage // Route tests use the server fixture.

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

func TestExecutionExpectedWirePresence(t *testing.T) {
	s, cleanup := testServer(t)
	defer cleanup()
	p := createTestProject(t, s.echo)
	id := createTestIssue(t, s.echo, p, "legacy")
	for _, raw := range []string{
		`{"contract_version":1}`,
		`{"governing":null}`,
		`{"governing":null,"contract_version":null}`,
	} {
		body := `{"expected":` + raw + `,"phase":"build","evidence":"tests"}`
		r := planRequest(
			s.echo,
			http.MethodPost,
			"/api/v1/projects/"+p+"/issues/"+id+"/execution-evidence",
			body,
			"",
		)
		require.Equal(t, http.StatusPreconditionRequired, r.Code, r.Body.String())
	}
	i, err := s.store.GetIssue(context.Background(), id)
	require.NoError(t, err)
	expected := fmt.Sprintf(`{"governing":null,"contract_version":%d}`, i.ContractVersion)
	r := planRequest(
		s.echo,
		http.MethodPost,
		"/api/v1/projects/"+p+"/issues/"+id+"/execution-evidence",
		`{"expected":`+expected+`,"phase":"build","evidence":"tests"}`,
		"",
	)
	require.Equal(t, http.StatusCreated, r.Code, r.Body.String())
	r = planRequest(
		s.echo,
		http.MethodPut,
		"/api/v1/issues/"+id,
		`{"status":"closed","expected":`+expected+`}`,
		"",
	)
	require.Equal(t, http.StatusOK, r.Code, r.Body.String())
	var got types.Issue
	require.NoError(t, json.Unmarshal(r.Body.Bytes(), &got))
	require.Equal(t, types.StatusClosed, got.Status)
}

func TestExecutionEvidenceRoutePersistsRequestProvenanceAndRejectsStaleWrites(t *testing.T) {
	s, cleanup := testServer(t)
	defer cleanup()
	p := createTestProject(t, s.echo)
	id := createTestIssue(t, s.echo, p, "claimed task")
	require.NoError(t, s.store.UpdateIssue(
		context.Background(), id, map[string]any{"ai_session_id": "claimant-session"}, "claimant",
	))
	expected := apiCaptured(t, s, p, id)
	path := "/api/v1/projects/" + p + "/issues/" + id + "/execution-evidence"
	body := fmt.Sprintf(`{"expected":%s,"phase":"build","evidence":"tested"}`, jsonBody(t, expected))

	response := planRequestWithProvenance(
		s.echo,
		http.MethodPost,
		path,
		body,
		planRequestProvenance{actor: "caller", sessionID: "caller-session"},
	)
	require.Equal(t, http.StatusCreated, response.Code, response.Body.String())
	var created storage.ExecutionEvidence
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &created))
	require.Equal(t, "caller", created.Actor)
	require.Equal(t, "caller-session", created.SessionID)

	require.NoError(t, s.store.UpdateIssue(
		context.Background(), id, map[string]any{"ai_session_id": "new-claimant-session"}, "new-claimant",
	))
	stale := planRequestWithProvenance(
		s.echo,
		http.MethodPost,
		path,
		body,
		planRequestProvenance{actor: "later-caller", sessionID: "later-session"},
	)
	require.Equal(t, http.StatusConflict, stale.Code, stale.Body.String())
	history := planRequest(s.echo, http.MethodGet, path, "", "")
	require.Equal(t, http.StatusOK, history.Code, history.Body.String())
	var records []storage.ExecutionEvidence
	require.NoError(t, json.Unmarshal(history.Body.Bytes(), &records))
	require.Len(t, records, 1)
	require.Equal(t, "caller", records[0].Actor)
	require.Equal(t, "caller-session", records[0].SessionID)

	anonymousID := createTestIssue(t, s.echo, p, "anonymous task")
	anonymousExpected := apiCaptured(t, s, p, anonymousID)
	anonymousPath := "/api/v1/projects/" + p + "/issues/" + anonymousID + "/execution-evidence"
	anonymous := planRequestWithProvenance(
		s.echo,
		http.MethodPost,
		anonymousPath,
		fmt.Sprintf(`{"expected":%s,"phase":"review","evidence":"reviewed"}`, jsonBody(t, anonymousExpected)),
		planRequestProvenance{},
	)
	require.Equal(t, http.StatusCreated, anonymous.Code, anonymous.Body.String())
	var anonymousRecord storage.ExecutionEvidence
	require.NoError(t, json.Unmarshal(anonymous.Body.Bytes(), &anonymousRecord))
	require.Equal(t, "anonymous", anonymousRecord.Actor)
	require.Empty(t, anonymousRecord.SessionID)
}
