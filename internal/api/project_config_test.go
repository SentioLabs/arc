package api //nolint:testpackage // tests use internal helpers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

// getProjectConfigMap issues GET /projects/:id/config and returns the config map.
func getProjectConfigMap(t *testing.T, e *echo.Echo, projectID string) map[string]string {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID+"/config", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET config returned %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Config map[string]string `json:"config"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse config response: %v", err)
	}
	return body.Config
}

// putProjectConfigKey issues PUT /projects/:id/config and returns the status code.
func putProjectConfigKey(t *testing.T, e *echo.Echo, projectID, body string) int {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+projectID+"/config", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	return rec.Code
}

func TestProjectConfigRoundTrip(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	projectID := createTestProject(t, e)

	code := putProjectConfigKey(t, e, projectID, `{"key":"docs.path","value":"~/vault/plans"}`)
	if code != http.StatusOK {
		t.Fatalf("PUT config returned %d, want 200", code)
	}
	if code = putProjectConfigKey(t, e, projectID, `{"key":"docs.type","value":"obsidian"}`); code != http.StatusOK {
		t.Fatalf("PUT second key returned %d, want 200", code)
	}

	cfg := getProjectConfigMap(t, e, projectID)
	if cfg["docs.path"] != "~/vault/plans" {
		t.Errorf("docs.path = %q, want %q", cfg["docs.path"], "~/vault/plans")
	}
	if cfg["docs.type"] != "obsidian" {
		t.Errorf("docs.type = %q, want %q", cfg["docs.type"], "obsidian")
	}

	// PUT on an existing key overwrites it.
	if code = putProjectConfigKey(t, e, projectID, `{"key":"docs.path","value":"/elsewhere"}`); code != http.StatusOK {
		t.Fatalf("PUT overwrite returned %d, want 200", code)
	}
	if cfg = getProjectConfigMap(t, e, projectID); cfg["docs.path"] != "/elsewhere" {
		t.Errorf("docs.path = %q, want %q", cfg["docs.path"], "/elsewhere")
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+projectID+"/config/docs.path", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE config returned %d, want 200: %s", rec.Code, rec.Body.String())
	}

	cfg = getProjectConfigMap(t, e, projectID)
	if _, ok := cfg["docs.path"]; ok {
		t.Error("docs.path still present after DELETE")
	}
	if cfg["docs.type"] != "obsidian" {
		t.Errorf("DELETE removed unrelated key: docs.type = %q", cfg["docs.type"])
	}
}

func TestGetProjectConfigEmpty(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	if cfg := getProjectConfigMap(t, e, createTestProject(t, e)); len(cfg) != 0 {
		t.Errorf("expected empty config, got %v", cfg)
	}
}

func TestProjectConfigUnknownProject(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/proj-missing/config", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET config for unknown project returned %d, want 404", rec.Code)
	}

	code := putProjectConfigKey(t, e, "proj-missing", `{"key":"docs.path","value":"/x"}`)
	if code != http.StatusNotFound {
		t.Errorf("PUT config for unknown project returned %d, want 404", code)
	}
}

func TestPutProjectConfigRejectsEmptyKey(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	projectID := createTestProject(t, e)

	if code := putProjectConfigKey(t, e, projectID, `{"value":"/x"}`); code != http.StatusBadRequest {
		t.Errorf("PUT config with no key returned %d, want 400", code)
	}
}
