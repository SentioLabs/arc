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

	code := putProjectConfigKey(t, e, projectID, `{"key":"plans.dir","value":"~/vault/plans"}`)
	if code != http.StatusOK {
		t.Fatalf("PUT config returned %d, want 200", code)
	}
	if code = putProjectConfigKey(t, e, projectID, `{"key":"plans.type","value":"obsidian"}`); code != http.StatusOK {
		t.Fatalf("PUT second key returned %d, want 200", code)
	}

	cfg := getProjectConfigMap(t, e, projectID)
	if cfg["plans.dir"] != "~/vault/plans" {
		t.Errorf("plans.dir = %q, want %q", cfg["plans.dir"], "~/vault/plans")
	}
	if cfg["plans.type"] != "obsidian" {
		t.Errorf("plans.type = %q, want %q", cfg["plans.type"], "obsidian")
	}

	// PUT on an existing key overwrites it.
	if code = putProjectConfigKey(t, e, projectID, `{"key":"plans.dir","value":"/elsewhere"}`); code != http.StatusOK {
		t.Fatalf("PUT overwrite returned %d, want 200", code)
	}
	if cfg = getProjectConfigMap(t, e, projectID); cfg["plans.dir"] != "/elsewhere" {
		t.Errorf("plans.dir = %q, want %q", cfg["plans.dir"], "/elsewhere")
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+projectID+"/config/plans.dir", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE config returned %d, want 200: %s", rec.Code, rec.Body.String())
	}

	cfg = getProjectConfigMap(t, e, projectID)
	if _, ok := cfg["plans.dir"]; ok {
		t.Error("plans.dir still present after DELETE")
	}
	if cfg["plans.type"] != "obsidian" {
		t.Errorf("DELETE removed unrelated key: plans.type = %q", cfg["plans.type"])
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

	code := putProjectConfigKey(t, e, "proj-missing", `{"key":"plans.dir","value":"/x"}`)
	if code != http.StatusNotFound {
		t.Errorf("PUT config for unknown project returned %d, want 404", code)
	}
}

func TestPutProjectConfigPlansValidation(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	projectID := createTestProject(t, e)

	// plans.type must be a known enum value.
	code := putProjectConfigKey(t, e, projectID, `{"key":"plans.type","value":"notion"}`)
	if code != http.StatusBadRequest {
		t.Errorf("PUT plans.type=notion returned %d, want 400", code)
	}
	// plans.dir must not contain traversal.
	code = putProjectConfigKey(t, e, projectID, `{"key":"plans.dir","value":"../etc"}`)
	if code != http.StatusBadRequest {
		t.Errorf("PUT plans.dir=../etc returned %d, want 400", code)
	}

	// Valid values are still stored.
	code = putProjectConfigKey(t, e, projectID, `{"key":"plans.dir","value":"/abs/ok"}`)
	if code != http.StatusOK {
		t.Errorf("PUT plans.dir=/abs/ok returned %d, want 200", code)
	}
	code = putProjectConfigKey(t, e, projectID, `{"key":"plans.type","value":"obsidian"}`)
	if code != http.StatusOK {
		t.Errorf("PUT plans.type=obsidian returned %d, want 200", code)
	}
	cfg := getProjectConfigMap(t, e, projectID)
	if cfg["plans.dir"] != "/abs/ok" {
		t.Errorf("plans.dir = %q, want %q", cfg["plans.dir"], "/abs/ok")
	}
	if cfg["plans.type"] != "obsidian" {
		t.Errorf("plans.type = %q, want %q", cfg["plans.type"], "obsidian")
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
