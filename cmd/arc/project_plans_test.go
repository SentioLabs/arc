package main

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureStdout runs fn with os.Stdout redirected to a pipe and returns
// everything written to it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()

	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(out)
}

func TestProjectPlansCommandsRegistered(t *testing.T) {
	found := false
	for _, cmd := range projectCmd.Commands() {
		if cmd.Use == projectPlansCmdName {
			found = true
			subNames := map[string]bool{}
			for _, sub := range cmd.Commands() {
				subNames[sub.Name()] = true
			}
			assert.True(t, subNames["set"], "plans should have 'set' subcommand")
			assert.True(t, subNames["get"], "plans should have 'get' subcommand")
			assert.True(t, subNames["unset"], "plans should have 'unset' subcommand")
			break
		}
	}
	assert.True(t, found, "project command should have a 'plans' subcommand")
}

func TestProjectPlansSetRejectsInvalidType(t *testing.T) {
	plansSetDir, plansSetType = "", "notion"
	defer func() { plansSetDir, plansSetType = "", "" }()

	err := projectPlansSetCmd.RunE(projectPlansSetCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --type")
}

func TestProjectPlansSetRequiresDirOrType(t *testing.T) {
	plansSetDir, plansSetType = "", ""
	defer func() { plansSetDir, plansSetType = "", "" }()

	err := projectPlansSetCmd.RunE(projectPlansSetCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provide --dir and/or --type")
}

func TestProjectPlansSetRejectsPathTraversal(t *testing.T) {
	plansSetDir, plansSetType = "../escape", ""
	defer func() { plansSetDir, plansSetType = "", "" }()

	err := projectPlansSetCmd.RunE(projectPlansSetCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not contain '..'")
}

func TestProjectPlansGetJSON(t *testing.T) {
	_, _ = loadConfigForTest(t)

	origProjectID := projectID
	origOutputJSON := outputJSON
	projectID = "proj-test"
	outputJSON = true
	defer func() {
		projectID = origProjectID
		outputJSON = origOutputJSON
	}()

	out := captureStdout(t, func() {
		err := runProjectPlansGet(projectPlansGetCmd, nil)
		require.NoError(t, err)
	})

	var result map[string]string
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out)), &result))
	assert.Contains(t, result, "plans_dir")
	assert.Contains(t, result, "plans_type")
	assert.Contains(t, result, "plans_source")
	assert.NotEmpty(t, result["plans_dir"])
	assert.NotEmpty(t, result["plans_type"])
	assert.NotEmpty(t, result["plans_source"])
}

func TestResolvePlansForProjectDefaults(t *testing.T) {
	_, _ = loadConfigForTest(t)

	dir, ptype, source, err := resolvePlansForProject("proj-test", "My Project", "mp")
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(dir, "docs/plans"), "expected dir ending in docs/plans, got %q", dir)
	assert.Equal(t, "markdown", ptype)
	assert.Equal(t, "default", source)
}

func TestWhichResultHasPlansFields(t *testing.T) {
	// Struct-level contract check: JSON tags must be present so downstream
	// consumers (the Claude plugin) can rely on plans_dir/plans_type/plans_source.
	result := whichResult{PlansDir: "/abs/dir", PlansType: "markdown", PlansSource: "default"}
	b, err := json.Marshal(result)
	require.NoError(t, err)
	encoded := string(b)
	assert.Contains(t, encoded, `"plans_dir":"/abs/dir"`)
	assert.Contains(t, encoded, `"plans_type":"markdown"`)
	assert.Contains(t, encoded, `"plans_source":"default"`)
}
