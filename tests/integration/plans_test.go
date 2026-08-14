//go:build integration

package integration

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// plansWhichResult mirrors the plans_* fields of the `arc which --json` and
// `arc project plans get --json` cross-repo contract.
type plansWhichResult struct {
	PlansDir    string `json:"plans_dir"`
	PlansType   string `json:"plans_type"`
	PlansSource string `json:"plans_source"`
}

func decodePlansResult(t *testing.T, out string) plansWhichResult {
	t.Helper()
	var r plansWhichResult
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("parse plans JSON: %v\noutput: %s", err, out)
	}
	return r
}

// TestProjectPlansLifecycle exercises the configurable plan-destination
// surface end-to-end through the CLI -> server -> SQLite stack: default
// resolution, a per-project override via `arc project plans set`, and revert
// via `unset` -- verified through both `arc project plans get --json` and the
// `arc which --json` contract that the companion plugin consumes.
func TestProjectPlansLifecycle(t *testing.T) {
	home := setupHome(t)
	dir := t.TempDir()

	arcCmdInDirSuccess(t, home, dir, "init", "plans-proj", "--server", serverURL)

	// 1. Default layer: source=default, type=markdown, dir absolute ending docs/plans.
	got := decodePlansResult(t, arcCmdInDirSuccess(t, home, dir, "which", "--json", "--server", serverURL))
	if got.PlansSource != "default" {
		t.Errorf("default: expected plans_source=default, got %q", got.PlansSource)
	}
	if got.PlansType != "markdown" {
		t.Errorf("default: expected plans_type=markdown, got %q", got.PlansType)
	}
	if !filepath.IsAbs(got.PlansDir) || !strings.HasSuffix(got.PlansDir, filepath.Join("docs", "plans")) {
		t.Errorf("default: expected absolute plans_dir ending in docs/plans, got %q", got.PlansDir)
	}

	// 2. Per-project override persists and wins.
	vault := filepath.Join(t.TempDir(), "vault")
	arcCmdInDirSuccess(t, home, dir, "project", "plans", "set", "--dir", vault, "--type", "obsidian", "--server", serverURL)

	got = decodePlansResult(t, arcCmdInDirSuccess(t, home, dir, "which", "--json", "--server", serverURL))
	if got.PlansSource != "project" {
		t.Errorf("override: expected plans_source=project, got %q", got.PlansSource)
	}
	if got.PlansType != "obsidian" {
		t.Errorf("override: expected plans_type=obsidian, got %q", got.PlansType)
	}
	if got.PlansDir != vault {
		t.Errorf("override: expected plans_dir=%q, got %q", vault, got.PlansDir)
	}

	// 3. `project plans get --json` reports the same resolved values.
	getGot := decodePlansResult(t, arcCmdInDirSuccess(t, home, dir, "project", "plans", "get", "--json", "--server", serverURL))
	if getGot.PlansSource != "project" || getGot.PlansType != "obsidian" || getGot.PlansDir != vault {
		t.Errorf("plans get: expected project/obsidian/%q, got %+v", vault, getGot)
	}

	// 4. Unset reverts to the default layer.
	arcCmdInDirSuccess(t, home, dir, "project", "plans", "unset", "--server", serverURL)
	got = decodePlansResult(t, arcCmdInDirSuccess(t, home, dir, "which", "--json", "--server", serverURL))
	if got.PlansSource != "default" || got.PlansType != "markdown" {
		t.Errorf("unset: expected default/markdown, got %+v", got)
	}
}

// TestProjectPlansValidation verifies the plans.* namespace rejects an invalid
// type and a traversal path rather than persisting them.
func TestProjectPlansValidation(t *testing.T) {
	home := setupHome(t)
	dir := t.TempDir()
	arcCmdInDirSuccess(t, home, dir, "init", "plans-valid-proj", "--server", serverURL)

	if out, err := arcCmdInDir(t, home, dir, "project", "plans", "set", "--type", "notion", "--server", serverURL); err == nil {
		t.Errorf("expected `plans set --type notion` to fail, got success: %s", out)
	}
	if out, err := arcCmdInDir(t, home, dir, "project", "plans", "set", "--dir", "../escape", "--server", serverURL); err == nil {
		t.Errorf("expected `plans set --dir ../escape` to fail, got success: %s", out)
	}
}
