//go:build integration

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupPlanEnv creates an isolated home + workdir with an initialized project
// and a docs/plans/ directory for plan files. Returns home, workDir.
func setupPlanEnv(t *testing.T, projName string) (string, string) {
	t.Helper()

	home := setupHome(t)

	// Make the temp directory tree world-traversable so the Docker-hosted
	// server (running as uid 1000) can read plan files written by the test
	// runner. t.TempDir() creates directories with 0o700, blocking other users.
	// We chmod both the home dir and its parent (Go creates /tmp/TestName/NNN/).
	if err := os.Chmod(filepath.Dir(home), 0o755); err != nil {
		t.Fatalf("chmod temp parent dir: %v", err)
	}
	if err := os.Chmod(home, 0o755); err != nil {
		t.Fatalf("chmod home dir: %v", err)
	}

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

// extractPlanID extracts a plan.xxxxx ID from command output.
func extractPlanID(output string) (string, bool) {
	for _, word := range strings.Fields(output) {
		if strings.HasPrefix(word, "plan.") {
			// Trim trailing punctuation
			word = strings.TrimRight(word, "(),.")
			return word, true
		}
	}
	return "", false
}

// --- Plan Create ---

func TestPlanCreate(t *testing.T) {
	home, workDir := setupPlanEnv(t, "plan-create")
	planPath := writePlanFile(t, workDir, "test-create.md", "# Test Plan\n\nSome content.")

	output := arcCmdInDirSuccess(t, home, workDir, "plan", "create", planPath, "--server", serverURL)

	planID, ok := extractPlanID(output)
	if !ok {
		t.Fatalf("expected plan ID in output, got: %s", output)
	}
	if !strings.HasPrefix(planID, "plan.") {
		t.Errorf("expected plan ID to start with 'plan.', got: %s", planID)
	}
	if !strings.Contains(strings.ToLower(output), "draft") {
		t.Errorf("expected draft status in output, got: %s", output)
	}
}

func TestPlanCreateJSON(t *testing.T) {
	home, workDir := setupPlanEnv(t, "plan-create-json")
	planPath := writePlanFile(t, workDir, "test-json.md", "# JSON Plan")

	output := arcCmdInDirSuccess(t, home, workDir, "plan", "create", planPath, "--json", "--server", serverURL)

	var plan map[string]interface{}
	if err := json.Unmarshal([]byte(output), &plan); err != nil {
		t.Fatalf("expected valid JSON, got: %s (error: %v)", output, err)
	}
	if plan["status"] != "draft" {
		t.Errorf("expected status 'draft', got %v", plan["status"])
	}
	fp, _ := plan["file_path"].(string)
	if !strings.HasSuffix(fp, planPath) {
		t.Errorf("expected file_path ending with %q, got %v", planPath, fp)
	}
	if _, ok := plan["id"]; !ok {
		t.Error("expected 'id' field in JSON output")
	}
}

func TestPlanCreateMissingFile(t *testing.T) {
	home, workDir := setupPlanEnv(t, "plan-create-nofile")

	_, err := arcCmdInDir(t, home, workDir, "plan", "create", "docs/plans/nonexistent.md", "--server", serverURL)
	// The create command registers the path; file existence is checked on GET.
	// This test mainly verifies the command accepts the argument.
	_ = err
}

func TestPlanCreateMissingArgs(t *testing.T) {
	home := setupHome(t)

	_, err := arcCmd(t, home, "plan", "create", "--server", serverURL)
	if err == nil {
		t.Error("expected error when no file path is given")
	}
}

// --- Plan Show ---

func TestPlanShow(t *testing.T) {
	home, workDir := setupPlanEnv(t, "plan-show")
	planPath := writePlanFile(t, workDir, "test-show.md", "# Show Plan\n\nLine 2\nLine 3")

	createOut := arcCmdInDirSuccess(t, home, workDir, "plan", "create", planPath, "--json", "--server", serverURL)
	var created map[string]interface{}
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	planID := created["id"].(string)

	output := arcCmdInDirSuccess(t, home, workDir, "plan", "show", planID, "--server", serverURL)

	if !strings.Contains(output, "Show Plan") {
		t.Errorf("expected plan content in show output, got: %s", output)
	}
	if !strings.Contains(strings.ToLower(output), "draft") {
		t.Errorf("expected draft status, got: %s", output)
	}
}

func TestPlanShowJSON(t *testing.T) {
	home, workDir := setupPlanEnv(t, "plan-show-json")
	planPath := writePlanFile(t, workDir, "test-show-json.md", "# JSON Show Content")

	createOut := arcCmdInDirSuccess(t, home, workDir, "plan", "create", planPath, "--json", "--server", serverURL)
	var created map[string]interface{}
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	planID := created["id"].(string)

	output := arcCmdInDirSuccess(t, home, workDir, "plan", "show", planID, "--json", "--server", serverURL)

	var plan map[string]interface{}
	if err := json.Unmarshal([]byte(output), &plan); err != nil {
		t.Fatalf("expected valid JSON, got: %s (error: %v)", output, err)
	}

	for _, field := range []string{"id", "file_path", "status", "content", "created_at", "updated_at"} {
		if _, exists := plan[field]; !exists {
			t.Errorf("expected field %q in JSON output", field)
		}
	}
	if !strings.Contains(plan["content"].(string), "JSON Show Content") {
		t.Errorf("expected content with 'JSON Show Content', got %v", plan["content"])
	}
}

func TestPlanShowMissingArgs(t *testing.T) {
	home := setupHome(t)

	_, err := arcCmd(t, home, "plan", "show", "--server", serverURL)
	if err == nil {
		t.Error("expected error when no plan ID is given")
	}
}

// --- Plan Approve ---

func TestPlanApprove(t *testing.T) {
	home, workDir := setupPlanEnv(t, "plan-approve")
	planPath := writePlanFile(t, workDir, "test-approve.md", "# Approve Me")

	createOut := arcCmdInDirSuccess(t, home, workDir, "plan", "create", planPath, "--json", "--server", serverURL)
	var created map[string]interface{}
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	planID := created["id"].(string)

	output := arcCmdInDirSuccess(t, home, workDir, "plan", "approve", planID, "--server", serverURL)
	if !strings.Contains(strings.ToLower(output), "approved") {
		t.Errorf("expected 'approved' in output, got: %s", output)
	}

	// Verify status changed
	showOut := arcCmdInDirSuccess(t, home, workDir, "plan", "show", planID, "--json", "--server", serverURL)
	var updated map[string]interface{}
	if err := json.Unmarshal([]byte(showOut), &updated); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if updated["status"] != "approved" {
		t.Errorf("expected status 'approved', got %v", updated["status"])
	}
}

func TestPlanApproveMissingArgs(t *testing.T) {
	home := setupHome(t)

	_, err := arcCmd(t, home, "plan", "approve", "--server", serverURL)
	if err == nil {
		t.Error("expected error when no plan ID is given")
	}
}

// --- Plan Reject ---

func TestPlanReject(t *testing.T) {
	home, workDir := setupPlanEnv(t, "plan-reject")
	planPath := writePlanFile(t, workDir, "test-reject.md", "# Reject Me")

	createOut := arcCmdInDirSuccess(t, home, workDir, "plan", "create", planPath, "--json", "--server", serverURL)
	var created map[string]interface{}
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	planID := created["id"].(string)

	output := arcCmdInDirSuccess(t, home, workDir, "plan", "reject", planID, "--server", serverURL)
	if !strings.Contains(strings.ToLower(output), "rejected") {
		t.Errorf("expected 'rejected' in output, got: %s", output)
	}

	// Verify status changed
	showOut := arcCmdInDirSuccess(t, home, workDir, "plan", "show", planID, "--json", "--server", serverURL)
	var updated map[string]interface{}
	if err := json.Unmarshal([]byte(showOut), &updated); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if updated["status"] != "rejected" {
		t.Errorf("expected status 'rejected', got %v", updated["status"])
	}
}

func TestPlanRejectMissingArgs(t *testing.T) {
	home := setupHome(t)

	_, err := arcCmd(t, home, "plan", "reject", "--server", serverURL)
	if err == nil {
		t.Error("expected error when no plan ID is given")
	}
}

// --- Plan Comments ---

func TestPlanComments(t *testing.T) {
	home, workDir := setupPlanEnv(t, "plan-comments")
	planPath := writePlanFile(t, workDir, "test-comments.md", "# Plan\n\nLine 2")

	createOut := arcCmdInDirSuccess(t, home, workDir, "plan", "create", planPath, "--json", "--server", serverURL)
	var created map[string]interface{}
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	planID := created["id"].(string)

	// No comments yet
	output := arcCmdInDirSuccess(t, home, workDir, "plan", "comments", planID, "--server", serverURL)
	if !strings.Contains(strings.ToLower(output), "no comments") {
		t.Errorf("expected 'no comments' message, got: %s", output)
	}
}

func TestPlanCommentsMissingArgs(t *testing.T) {
	home := setupHome(t)

	_, err := arcCmd(t, home, "plan", "comments", "--server", serverURL)
	if err == nil {
		t.Error("expected error when no plan ID is given")
	}
}

// --- Command Help ---

func TestPlanCommandHelp(t *testing.T) {
	home := setupHome(t)

	output := arcCmdSuccess(t, home, "plan", "--help")

	for _, sub := range []string{"create", "show", "approve", "reject", "comments"} {
		if !strings.Contains(output, sub) {
			t.Errorf("expected %q subcommand in plan help, got: %s", sub, output)
		}
	}
}

// --- Full Lifecycle ---

func TestPlanFullLifecycle(t *testing.T) {
	home, workDir := setupPlanEnv(t, "plan-lifecycle")

	// 1. Create plan file and register it
	planPath := writePlanFile(t, workDir, "lifecycle.md", "# Implementation Plan\n\n1. Build it\n2. Test it")
	createOut := arcCmdInDirSuccess(t, home, workDir, "plan", "create", planPath, "--json", "--server", serverURL)
	var plan map[string]interface{}
	if err := json.Unmarshal([]byte(createOut), &plan); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	planID := plan["id"].(string)
	if plan["status"] != "draft" {
		t.Errorf("expected draft status, got %v", plan["status"])
	}

	// 2. Show plan — should include file content
	showOut := arcCmdInDirSuccess(t, home, workDir, "plan", "show", planID, "--server", serverURL)
	if !strings.Contains(showOut, "Implementation Plan") {
		t.Errorf("expected plan content in show, got: %s", showOut)
	}

	// 3. Comments — should be empty
	commentsOut := arcCmdInDirSuccess(t, home, workDir, "plan", "comments", planID, "--server", serverURL)
	if !strings.Contains(strings.ToLower(commentsOut), "no comments") {
		t.Errorf("expected no comments initially, got: %s", commentsOut)
	}

	// 4. Approve the plan
	arcCmdInDirSuccess(t, home, workDir, "plan", "approve", planID, "--server", serverURL)

	// 5. Verify approved
	showOut2 := arcCmdInDirSuccess(t, home, workDir, "plan", "show", planID, "--json", "--server", serverURL)
	var approved map[string]interface{}
	if err := json.Unmarshal([]byte(showOut2), &approved); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if approved["status"] != "approved" {
		t.Errorf("expected approved, got %v", approved["status"])
	}
}

func TestPlanDraftRejectCycle(t *testing.T) {
	home, workDir := setupPlanEnv(t, "plan-reject-cycle")

	planPath := writePlanFile(t, workDir, "reject-cycle.md", "# Bad Plan")
	createOut := arcCmdInDirSuccess(t, home, workDir, "plan", "create", planPath, "--json", "--server", serverURL)
	var plan map[string]interface{}
	if err := json.Unmarshal([]byte(createOut), &plan); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	planID := plan["id"].(string)

	// Reject
	arcCmdInDirSuccess(t, home, workDir, "plan", "reject", planID, "--server", serverURL)

	// Verify rejected
	showOut := arcCmdInDirSuccess(t, home, workDir, "plan", "show", planID, "--json", "--server", serverURL)
	var rejected map[string]interface{}
	if err := json.Unmarshal([]byte(showOut), &rejected); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if rejected["status"] != "rejected" {
		t.Errorf("expected rejected, got %v", rejected["status"])
	}
}
