package api //nolint:testpackage // Route tests use the server fixture.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

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
