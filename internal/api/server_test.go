package api //nolint:testpackage // tests use internal helpers that access unexported fields

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthCheck(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("health check returned %d: %s", rec.Code, rec.Body.String())
	}

	var health HealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &health); err != nil {
		t.Fatalf("failed to parse health response: %v", err)
	}

	if health.Status != "healthy" {
		t.Errorf("status = %q, want %q", health.Status, "healthy")
	}
	if health.Version == "" {
		t.Error("version should not be empty")
	}
	if health.Uptime < 0 {
		t.Errorf("uptime = %f, want >= 0", health.Uptime)
	}
	// testServer uses ":0" (ephemeral port), so parsed port should be 0
	if health.Port != 0 {
		t.Errorf("port = %d, want 0", health.Port)
	}
	// With port 0 and webui tag not set in tests, WebUIURL should be empty
	if health.WebUIURL != "" {
		t.Errorf("webui_url = %q, want empty (port is 0 and webui not compiled)", health.WebUIURL)
	}
}
