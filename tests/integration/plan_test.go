//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/types"
)

// setupPlanEnv creates an isolated home + workdir with an initialized project
// and a docs/plans/ directory for plan files. Returns home, workDir.
func setupPlanEnv(t *testing.T, projName string) (string, string) {
	t.Helper()

	home := setupHome(t)
	workDir := filepath.Join(home, "project")
	if err := os.MkdirAll(filepath.Join(workDir, "docs", "plans"), 0o755); err != nil {
		t.Fatalf("create docs/plans dir: %v", err)
	}

	arcCmdInDirSuccess(t, home, workDir, "init", projName, "--server", serverURL)
	return home, workDir
}

// writePlanFile creates a markdown plan file under docs/plans/ in the workdir.
func writePlanFile(t *testing.T, workDir, filename, content string) string {
	t.Helper()
	path := filepath.Join("docs", "plans", filename)
	fullPath := filepath.Join(workDir, path)
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write plan file: %v", err)
	}
	return path
}

// T6 introduces durable CLI commands; this coordinated server upgrade rejects
// every path-only command explicitly while durable API lifecycle coverage below
// exercises retained bytes in the disposable Docker server.
func TestLegacyPlanCommandsRequireUpgrade(t *testing.T) {
	home, dir := setupPlanEnv(t, "plan-upgrade")
	file := writePlanFile(t, dir, "legacy.md", "# local draft")
	for _, args := range [][]string{
		{"create", file},
		{"show", "plan.old"},
		{"approve", "plan.old"},
		{"reject", "plan.old"},
		{"comments", "plan.old"},
		{"wait", "plan.old"},
	} {
		command := append([]string{"plan"}, args...)
		command = append(command, "--server", serverURL)
		out, err := arcCmdInDir(t, home, dir, command...)
		if err == nil || !strings.Contains(strings.ToLower(out), "upgrade") {
			t.Fatalf("%v: %v %s", args, err, out)
		}
	}
	for _, sub := range []string{"create", "show", "approve", "reject", "comments", "wait"} {
		if _, err := arcCmd(t, home, "plan", sub, "--server", serverURL); err == nil {
			t.Fatalf("%s accepted missing argument", sub)
		}
	}
	out := arcCmdSuccess(t, home, "plan", "--help")
	for _, sub := range []string{"create", "show", "approve", "reject", "comments", "wait"} {
		if !strings.Contains(out, sub) {
			t.Errorf("missing %s help", sub)
		}
	}
	content, err := os.ReadFile(filepath.Join(dir, file))
	if err != nil || string(content) != "# local draft" {
		t.Fatalf("upgrade changed local draft: %q %v", content, err)
	}
}

func durableAPI(t *testing.T, method, path, key string, body any, want int) []byte {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(t.Context(), method, serverURL+path, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	result := readBody(t, resp)
	if resp.StatusCode != want {
		t.Fatalf("%s %s: want %d got %d %s", method, path, want, resp.StatusCode, result)
	}
	return result
}

func TestDurablePlanAPILifecycle(t *testing.T) {
	body := durableAPI(
		t,
		"POST",
		"/api/v1/projects",
		"",
		map[string]string{"name": fmt.Sprintf("durable-%d", uniqueSuffix()), "prefix": "dp"},
		201,
	)
	var p types.Project
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatal(err)
	}
	base := "/api/v1/projects/" + p.ID + "/plans"
	source := filepath.Join(t.TempDir(), "client.md")
	content := "# design\r\n\nExact Unicode: λ\n"
	if err := os.WriteFile(source, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	body = durableAPI(
		t,
		"POST",
		base,
		"create",
		storage.PlanUpload{Title: "review", Content: content, SourceName: source},
		201,
	)
	var first storage.PlanWriteResult
	if err := json.Unmarshal(body, &first); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	plan := base + "/" + first.Plan.ID
	rev := plan + "/revisions/1"
	body = durableAPI(t, "GET", rev, "", nil, 200)
	var read types.PlanRevisionWithContent
	if err := json.Unmarshal(body, &read); err != nil {
		t.Fatal(err)
	}
	if read.Content != content || read.ContentSHA256 != first.Revision.ContentSHA256 {
		t.Fatal("server depended on deleted client source")
	}
	durableAPI(
		t,
		"POST",
		rev+"/decisions",
		"",
		types.PlanReviewRequest{Status: "in_review", ExpectedHead: 1},
		200,
	)
	durableAPI(
		t,
		"POST",
		rev+"/decisions",
		"",
		types.PlanReviewRequest{Status: "approved", ExpectedHead: 1, ExpectedReviewVersion: 1},
		200,
	)
	durableAPI(
		t,
		"POST",
		plan+"/revisions",
		"save",
		storage.PlanSave{Content: "next", ExpectedRevision: 1},
		201,
	)
	durableAPI(
		t,
		"POST",
		rev+"/decisions",
		"",
		types.PlanReviewRequest{Status: "rejected", ExpectedHead: 1, ExpectedReviewVersion: 2},
		409,
	)
	body = durableAPI(t, "GET", rev, "", nil, 200)
	if err := json.Unmarshal(body, &read); err != nil {
		t.Fatal(err)
	}
	if read.ReviewStatus != "approved" || read.Content != content {
		t.Fatal("save rewrote approved history")
	}
	body = durableAPI(t, "GET", plan, "", nil, 200)
	var meta types.Plan
	if err := json.Unmarshal(body, &meta); err != nil {
		t.Fatal(err)
	}
	durableAPI(
		t,
		"PATCH",
		plan,
		"",
		storage.PlanMetadataUpdate{ExpectedVersion: meta.Version, Lifecycle: "archived"},
		200,
	)
	body = durableAPI(
		t,
		"POST",
		plan+"/revisions",
		"save",
		storage.PlanSave{Content: "next", ExpectedRevision: 1},
		200,
	)
	var replay storage.PlanWriteResult
	if err := json.Unmarshal(body, &replay); err != nil {
		t.Fatal(err)
	}
	if !replay.Replay || replay.Revision.Revision != 2 {
		t.Fatal("lost original replay")
	}
	durableAPI(t, "GET", rev, "", nil, 200)
	durableAPI(t, "POST", "/api/v1/plans", "", map[string]string{"file_path": source}, 400)
}
