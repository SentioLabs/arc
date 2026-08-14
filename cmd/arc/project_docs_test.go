package main

import (
	"bytes"
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

func TestProjectDocsCommandsRegistered(t *testing.T) {
	found := false
	for _, cmd := range projectCmd.Commands() {
		if cmd.Use == "docs" {
			found = true
			subNames := map[string]bool{}
			for _, sub := range cmd.Commands() {
				subNames[sub.Name()] = true
			}
			assert.True(t, subNames["set"], "docs should have 'set' subcommand")
			assert.True(t, subNames["get"], "docs should have 'get' subcommand")
			assert.True(t, subNames["unset"], "docs should have 'unset' subcommand")
			break
		}
	}
	assert.True(t, found, "project command should have a 'docs' subcommand")
}

func TestProjectDocsSetRejectsInvalidType(t *testing.T) {
	docsSetPath, docsSetType = "", "notion"
	defer func() { docsSetPath, docsSetType = "", "" }()

	err := projectDocsSetCmd.RunE(projectDocsSetCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --type")
}

func TestProjectDocsSetRequiresPathOrType(t *testing.T) {
	docsSetPath, docsSetType = "", ""
	defer func() { docsSetPath, docsSetType = "", "" }()

	err := projectDocsSetCmd.RunE(projectDocsSetCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provide --path and/or --type")
}

func TestProjectDocsSetRejectsPathTraversal(t *testing.T) {
	docsSetPath, docsSetType = "../escape", ""
	defer func() { docsSetPath, docsSetType = "", "" }()

	err := projectDocsSetCmd.RunE(projectDocsSetCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not contain '..'")
}

func TestProjectDocsGetJSON(t *testing.T) {
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
		err := runProjectDocsGet(projectDocsGetCmd, nil)
		require.NoError(t, err)
	})

	var result map[string]string
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out)), &result))
	assert.Contains(t, result, "docs_path")
	assert.Contains(t, result, "docs_type")
	assert.Contains(t, result, "docs_source")
	assert.NotEmpty(t, result["docs_path"])
	assert.NotEmpty(t, result["docs_type"])
	assert.NotEmpty(t, result["docs_source"])
}

func TestResolveDocsForProjectDefaults(t *testing.T) {
	_, _ = loadConfigForTest(t)

	path, dtype, source, err := resolveDocsForProject("proj-test", "My Project", "mp")
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(path, "docs/plans"), "expected path ending in docs/plans, got %q", path)
	assert.Equal(t, "markdown", dtype)
	assert.Equal(t, "default", source)
}

func TestWhichResultHasDocsFields(t *testing.T) {
	// Struct-level contract check: JSON tags must be present so downstream
	// consumers (the Claude plugin) can rely on docs_path/docs_type/docs_source.
	result := whichResult{DocsPath: "/abs/dir", DocsType: "markdown", DocsSource: "default"}
	b, err := json.Marshal(result)
	require.NoError(t, err)
	var buf bytes.Buffer
	buf.Write(b)
	assert.Contains(t, buf.String(), `"docs_path":"/abs/dir"`)
	assert.Contains(t, buf.String(), `"docs_type":"markdown"`)
	assert.Contains(t, buf.String(), `"docs_source":"default"`)
}
