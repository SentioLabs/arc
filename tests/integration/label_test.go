//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// apiRequest creates and executes an HTTP request against the test server.
// If body is non-nil it is marshalled as JSON. Returns the response for
// the caller to inspect and close.
func apiRequest(t *testing.T, method, path string, body any) *http.Response {
	t.Helper()

	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(data)
	}

	url := serverURL + path
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("create request %s %s: %v", method, path, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("execute request %s %s: %v", method, path, err)
	}
	return resp
}

// readBody reads and closes the response body, returning the bytes.
func readBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return data
}

// TestLabelCreateListDelete creates a label via POST, lists labels via GET
// and verifies it appears, deletes via DELETE, and verifies it is gone.
func TestLabelCreateListDelete(t *testing.T) {
	labelName := fmt.Sprintf("test-cld-%d", uniqueSuffix())

	// Create label.
	resp := apiRequest(t, http.MethodPost, "/api/v1/labels", map[string]string{
		"name": labelName,
	})
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 creating label, got %d: %s", resp.StatusCode, body)
	}

	// List labels and verify ours appears.
	resp = apiRequest(t, http.MethodGet, "/api/v1/labels", nil)
	body = readBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 listing labels, got %d: %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), labelName) {
		t.Errorf("expected label %q in list response, got: %s", labelName, body)
	}

	// Delete label.
	resp = apiRequest(t, http.MethodDelete, "/api/v1/labels/"+labelName, nil)
	readBody(t, resp)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 deleting label, got %d", resp.StatusCode)
	}

	// List again and verify label is gone.
	resp = apiRequest(t, http.MethodGet, "/api/v1/labels", nil)
	body = readBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 listing labels after delete, got %d: %s", resp.StatusCode, body)
	}
	if strings.Contains(string(body), labelName) {
		t.Errorf("label %q should not appear after deletion, got: %s", labelName, body)
	}
}

// TestLabelAddToIssue creates a label and an issue, adds the label to the
// issue via the API, then verifies the label appears in `arc show` output.
func TestLabelAddToIssue(t *testing.T) {
	home := setupHome(t)
	labelName := fmt.Sprintf("add-label-%d", uniqueSuffix())

	// Create label via API.
	resp := apiRequest(t, http.MethodPost, "/api/v1/labels", map[string]string{
		"name": labelName,
	})
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 creating label, got %d: %s", resp.StatusCode, body)
	}

	// Create project and issue via CLI.
	arcCmdSuccess(t, home, "init", "label-add-proj", "--server", serverURL)
	createOut := arcCmdSuccess(t, home, "create", "Label add test issue", "--type", "task", "--server", serverURL)
	id, ok := extractID(createOut)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", createOut)
	}

	// Get project ID from JSON output.
	jsonOut := arcCmdSuccess(t, home, "show", id, "--json", "--server", serverURL)
	var issueData map[string]interface{}
	if err := json.Unmarshal([]byte(jsonOut), &issueData); err != nil {
		t.Fatalf("failed to parse show --json output: %v\noutput: %s", err, jsonOut)
	}
	projID, ok := issueData["project_id"].(string)
	if !ok || projID == "" {
		t.Fatalf("could not extract project_id from JSON: %v", issueData)
	}

	// Add label to issue via API.
	path := fmt.Sprintf("/api/v1/projects/%s/issues/%s/labels", projID, id)
	resp = apiRequest(t, http.MethodPost, path, map[string]string{
		"label": labelName,
	})
	readBody(t, resp)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 adding label to issue, got %d", resp.StatusCode)
	}

	// Verify label appears in arc show output.
	showOut := arcCmdSuccess(t, home, "show", id, "--server", serverURL)
	if !strings.Contains(showOut, labelName) {
		t.Errorf("expected label %q in show output, got: %s", labelName, showOut)
	}
}

// TestLabelRemoveFromIssue creates a label, adds it to an issue, removes it
// via the API, and verifies it no longer appears in `arc show` output.
func TestLabelRemoveFromIssue(t *testing.T) {
	home := setupHome(t)
	labelName := fmt.Sprintf("rm-label-%d", uniqueSuffix())

	// Create label via API.
	resp := apiRequest(t, http.MethodPost, "/api/v1/labels", map[string]string{
		"name": labelName,
	})
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 creating label, got %d: %s", resp.StatusCode, body)
	}

	// Create project and issue via CLI.
	arcCmdSuccess(t, home, "init", "label-rm-proj", "--server", serverURL)
	createOut := arcCmdSuccess(t, home, "create", "Label remove test issue", "--type", "task", "--server", serverURL)
	id, ok := extractID(createOut)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", createOut)
	}

	// Get project ID.
	jsonOut := arcCmdSuccess(t, home, "show", id, "--json", "--server", serverURL)
	var issueData map[string]interface{}
	if err := json.Unmarshal([]byte(jsonOut), &issueData); err != nil {
		t.Fatalf("failed to parse show --json output: %v\noutput: %s", err, jsonOut)
	}
	projID, ok := issueData["project_id"].(string)
	if !ok || projID == "" {
		t.Fatalf("could not extract project_id from JSON: %v", issueData)
	}

	// Add label to issue.
	addPath := fmt.Sprintf("/api/v1/projects/%s/issues/%s/labels", projID, id)
	resp = apiRequest(t, http.MethodPost, addPath, map[string]string{
		"label": labelName,
	})
	readBody(t, resp)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 adding label, got %d", resp.StatusCode)
	}

	// Verify label is present before removal.
	showOut := arcCmdSuccess(t, home, "show", id, "--server", serverURL)
	if !strings.Contains(showOut, labelName) {
		t.Fatalf("label %q should appear before removal, got: %s", labelName, showOut)
	}

	// Remove label from issue.
	rmPath := fmt.Sprintf("/api/v1/projects/%s/issues/%s/labels/%s", projID, id, labelName)
	resp = apiRequest(t, http.MethodDelete, rmPath, nil)
	readBody(t, resp)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 removing label, got %d", resp.StatusCode)
	}

	// Verify label is gone from show output.
	showOut = arcCmdSuccess(t, home, "show", id, "--server", serverURL)
	if strings.Contains(showOut, labelName) {
		t.Errorf("label %q should not appear after removal, got: %s", labelName, showOut)
	}
}

// TestLabelWithColor creates a label with a color, lists labels, and
// verifies the color is stored correctly.
func TestLabelWithColor(t *testing.T) {
	labelName := fmt.Sprintf("urgent-%d", uniqueSuffix())
	color := "#ff0000"

	// Create label with color.
	resp := apiRequest(t, http.MethodPost, "/api/v1/labels", map[string]string{
		"name":  labelName,
		"color": color,
	})
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 creating label with color, got %d: %s", resp.StatusCode, body)
	}

	// List labels and verify color is present.
	resp = apiRequest(t, http.MethodGet, "/api/v1/labels", nil)
	body = readBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 listing labels, got %d: %s", resp.StatusCode, body)
	}

	// Parse the response — labels API returns a plain JSON array.
	var labels []struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.Unmarshal(body, &labels); err != nil {
		t.Fatalf("failed to parse labels list: %v\nresponse: %s", err, body)
	}

	var found bool
	for _, l := range labels {
		if l.Name == labelName {
			found = true
			if l.Color != color {
				t.Errorf("expected color %q for label %q, got %q", color, labelName, l.Color)
			}
			break
		}
	}
	if !found {
		t.Errorf("label %q not found in list response: %s", labelName, body)
	}

	// Cleanup.
	resp = apiRequest(t, http.MethodDelete, "/api/v1/labels/"+labelName, nil)
	readBody(t, resp)
}

// TestLabelDuplicateCreate creates a label and then attempts to create
// another label with the same name, expecting a 4xx error.
func TestLabelDuplicateCreate(t *testing.T) {
	labelName := fmt.Sprintf("dup-label-%d", uniqueSuffix())

	// Create label first time — should succeed.
	resp := apiRequest(t, http.MethodPost, "/api/v1/labels", map[string]string{
		"name": labelName,
	})
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 creating label, got %d: %s", resp.StatusCode, body)
	}

	// Create same label again — arc's label API is idempotent, so this
	// returns 201 again rather than a conflict error. Verify it succeeds
	// and doesn't create a second copy.
	resp = apiRequest(t, http.MethodPost, "/api/v1/labels", map[string]string{
		"name": labelName,
	})
	body = readBody(t, resp)
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201 for idempotent label creation, got %d: %s", resp.StatusCode, body)
	}

	// Verify only one label with this name exists.
	resp = apiRequest(t, http.MethodGet, "/api/v1/labels", nil)
	body = readBody(t, resp)
	count := strings.Count(string(body), labelName)
	if count != 1 {
		t.Errorf("expected exactly 1 label named %q, found %d in: %s", labelName, count, body)
	}

	// Cleanup.
	resp = apiRequest(t, http.MethodDelete, "/api/v1/labels/"+labelName, nil)
	readBody(t, resp)
}

// labelCounter is an atomic counter for generating unique label names.
var labelCounter int64

// uniqueSuffix returns a value suitable for creating unique label names
// across test runs.
func uniqueSuffix() int64 {
	return atomic.AddInt64(&labelCounter, 1) + time.Now().UnixNano()%100000
}
