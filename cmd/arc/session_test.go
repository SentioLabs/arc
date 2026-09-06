package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/sentiolabs/arc/internal/api"
	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/storage/sqlite"
	"github.com/sentiolabs/arc/internal/types"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// setupSessionTest isolates the server, database, config, cwd, and runtime env.
func setupSessionTest(t *testing.T) (*client.Client, string) {
	t.Helper()
	workDir := t.TempDir()
	t.Chdir(workDir)
	t.Setenv("ARC_SESSION_ID", "")
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("PI_SESSION_ID", "")
	t.Setenv("CLAUDE_ENV_FILE", "")
	t.Setenv("ARC_TEAMMATE_ROLE", "")

	store, err := sqlite.New(filepath.Join(workDir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	server := api.New(api.ServerOptions{Address: ":0", Store: store})
	ts := httptest.NewServer(server.Echo())
	t.Cleanup(ts.Close)
	c := client.New(ts.URL)
	proj, err := c.CreateProject("Session Test", "sess", "")
	require.NoError(t, err)
	_, err = c.CreateWorkspace(proj.ID, client.CreateWorkspaceRequest{Path: workDir})
	require.NoError(t, err)

	origURL, origConfig, origJSON := serverURL, configPath, outputJSON
	serverURL, configPath, outputJSON = ts.URL, filepath.Join(workDir, "config.toml"), false
	t.Cleanup(func() { serverURL, configPath, outputJSON = origURL, origConfig, origJSON })
	return c, proj.ID
}

// sessionTestStdin supplies a hook payload without depending on the test runner's stdin.
func sessionTestStdin(t *testing.T, input string) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stdin")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, f.Close()) })
	_, err = f.WriteString(input)
	require.NoError(t, err)
	_, err = f.Seek(0, 0)
	require.NoError(t, err)
	origStdin := os.Stdin
	os.Stdin = f
	t.Cleanup(func() { os.Stdin = origStdin })
}

// takeTestCommand uses fresh flags to avoid changing the package-global Cobra flags.
func takeTestCommand() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("take", true, "")
	cmd.Flags().String("session-id", "", "")
	cmd.Flags().String("status", "", "")
	return cmd
}

func TestUpdateTakeSessionIdentity(t *testing.T) {
	tests := []struct {
		name, explicit, arcID, codexID, piID, status, wantID, wantErr string
		withoutTake                                                   bool
	}{
		{name: "explicit", explicit: "explicit-session", wantID: "explicit-session"},
		{name: "arc environment", arcID: "arc-session", wantID: "arc-session"},
		{name: "codex", codexID: "codex-thread", wantID: "codex-thread"},
		{name: "pi", piID: "pi-session", wantID: "pi-session"},
		{name: "native conflict", codexID: "codex", piID: "pi", wantErr: "conflicting runtime session IDs"},
		{name: "same native identity", codexID: "same", piID: "same", wantID: "same"},
		{name: "flag resolves native conflict", explicit: "flag", codexID: "codex", piID: "pi", wantID: "flag"},
		{
			name: "arc resolves native conflict", arcID: "arc-session",
			codexID: "codex", piID: "pi", wantID: "arc-session",
		},
		{name: "missing", wantErr: "no session ID available"},
		{name: "flag wins conflicts", explicit: "flag", arcID: "arc-session", codexID: "codex", wantID: "flag"},
		{name: "arc wins conflict", arcID: "arc-session", codexID: "codex", wantID: "arc-session"},
		{name: "matching env", arcID: "same", codexID: "same", wantID: "same"},
		{name: "preserve status", codexID: "codex", status: string(types.StatusBlocked), wantID: "codex"},
		{
			name: "flag requires take", explicit: "flag", codexID: "codex", withoutTake: true,
			wantErr: "--session-id requires --take",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, projID := setupSessionTest(t)
			t.Setenv("ARC_SESSION_ID", tt.arcID)
			t.Setenv("CODEX_THREAD_ID", tt.codexID)
			t.Setenv("PI_SESSION_ID", tt.piID)
			issue, err := c.CreateIssue(projID, client.CreateIssueRequest{Title: "Claim fixture"})
			require.NoError(t, err)
			cmd := takeTestCommand()
			if tt.withoutTake {
				require.NoError(t, cmd.Flags().Set("take", "false"))
			}
			require.NoError(t, cmd.Flags().Set("session-id", tt.explicit))
			if tt.status != "" {
				require.NoError(t, cmd.Flags().Set("status", tt.status))
			}
			err = updateCmd.RunE(cmd, []string{issue.ID})
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			got, err := c.GetIssueByID(issue.ID)
			require.NoError(t, err)
			require.Equal(t, tt.wantID, got.AISessionID)
			switch {
			case tt.wantErr != "":
				require.Equal(t, types.StatusOpen, got.Status)
			case tt.status != "":
				require.Equal(t, types.Status(tt.status), got.Status)
			default:
				require.Equal(t, types.StatusInProgress, got.Status)
			}
			sessions, err := c.ListAISessions(projID, 10, 0)
			require.NoError(t, err)
			if tt.wantErr == "" {
				require.Len(t, sessions, 1, "every claim must have a registered session")
				require.Equal(t, tt.wantID, sessions[0].ID)
			} else {
				require.Empty(t, sessions)
			}
		})
	}
}

func TestRuntimeHookRegistrationAndTake(t *testing.T) {
	for _, runtime := range []string{"claude", "codex", "pi"} {
		t.Run(runtime, func(t *testing.T) {
			c, projID := setupSessionTest(t)
			const threadID = "983a7cf7-bcb6-48fc-b485-129b4f1aaa45"
			switch runtime {
			case "claude":
				envFile := filepath.Join(t.TempDir(), "claude-env")
				t.Setenv("CLAUDE_ENV_FILE", envFile)
				sessionTestStdin(t, `{"session_id":"`+threadID+`"}`)
				captureStdout(t, func() { primeCmd.Run(primeCmd, nil) })
				data, err := os.ReadFile(envFile)
				require.NoError(t, err)
				require.Equal(t, "export ARC_SESSION_ID="+threadID+"\n", string(data))
				t.Setenv("ARC_SESSION_ID", threadID)
			case "codex":
				t.Setenv("CODEX_THREAD_ID", threadID)
			case "pi":
				t.Setenv("PI_SESSION_ID", threadID)
			}
			cwd, err := os.Getwd()
			require.NoError(t, err)
			payload, err := json.Marshal(hookInput{
				SessionID: threadID, CWD: cwd, TranscriptPath: "/recorded/transcript.jsonl",
			})
			require.NoError(t, err)
			sessionTestStdin(t, string(payload))
			require.NoError(t, runSessionStart(aiSessionStartCmd, true))
			registered, err := c.GetAISession(projID, threadID)
			require.NoError(t, err)
			issue, err := c.CreateIssue(projID, client.CreateIssueRequest{Title: "Runtime claim fixture"})
			require.NoError(t, err)
			// A registered parent remains usable from a worker's different directory.
			t.Chdir(t.TempDir())
			require.NoError(t, updateCmd.RunE(takeTestCommand(), []string{issue.ID}))
			claimed, err := c.GetIssueByID(issue.ID)
			require.NoError(t, err)
			require.Equal(t, registered.ID, claimed.AISessionID)
			require.Equal(t, types.StatusInProgress, claimed.Status)
			require.NoError(t, updateCmd.RunE(takeTestCommand(), []string{issue.ID}))
			retried, err := c.GetAISession(projID, threadID)
			require.NoError(t, err)
			require.Equal(t, registered.AISession, retried.AISession, "claims must retain original hook metadata")
		})
	}
}

func TestClaimRejectsProjectMismatch(t *testing.T) {
	for _, mismatch := range []string{"cwd", "registered session"} {
		t.Run(mismatch, func(t *testing.T) {
			c, projID := setupSessionTest(t)
			other, err := c.CreateProject("Other Project", "other", "")
			require.NoError(t, err)
			t.Setenv("CODEX_THREAD_ID", "worker-thread")
			targetProject := projID
			if mismatch == "cwd" {
				targetProject = other.ID
			} else {
				_, err = c.CreateAISession(other.ID, &types.AISession{ID: "worker-thread"})
				require.NoError(t, err)
			}
			issue, err := c.CreateIssue(targetProject, client.CreateIssueRequest{Title: "Must remain unclaimed"})
			require.NoError(t, err)
			err = updateCmd.RunE(takeTestCommand(), []string{issue.ID})
			require.ErrorContains(t, err, "register AI session")
			got, err := c.GetIssueByID(issue.ID)
			require.NoError(t, err)
			require.Empty(t, got.AISessionID)
			require.Equal(t, types.StatusOpen, got.Status)
		})
	}
}

func TestClaimValidatesRegistrationResponse(t *testing.T) {
	for _, failure := range []string{"wrong project", "wrong id", "server error"} {
		t.Run(failure, func(t *testing.T) {
			t.Setenv("ARC_SESSION_ID", "")
			t.Setenv("CODEX_THREAD_ID", "worker-thread")
			t.Setenv("PI_SESSION_ID", "")
			var updated atomic.Bool
			ts := claimRegistrationFailureServer(failure, &updated)
			t.Cleanup(ts.Close)
			origURL, origConfig := serverURL, configPath
			serverURL, configPath = ts.URL, filepath.Join(t.TempDir(), "config.toml")
			t.Cleanup(func() { serverURL, configPath = origURL, origConfig })
			err := updateCmd.RunE(takeTestCommand(), []string{"fixture"})
			require.ErrorContains(t, err, "register AI session")
			require.False(t, updated.Load(), "registration failure must stop before issue mutation")
		})
	}
}

func claimRegistrationFailureServer(failure string, updated *atomic.Bool) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			if strings.Contains(r.URL.Path, "/ai/sessions/") {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":"not found"}`))
				return
			}
			_ = json.NewEncoder(w).Encode(types.Issue{ID: "fixture", ProjectID: "target-project"})
		case http.MethodPost:
			if failure == "server error" {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":"registration unavailable"}`))
				return
			}
			session := types.AISession{ID: "worker-thread", ProjectID: "target-project"}
			if failure == "wrong project" {
				session.ProjectID = "other-project"
			} else {
				session.ID = "other-thread"
			}
			_ = json.NewEncoder(w).Encode(session)
		case http.MethodPut:
			updated.Store(true)
			_ = json.NewEncoder(w).Encode(types.Issue{ID: "fixture"})
		}
	}))
}

func TestSessionStartKeepsExplicitIdentity(t *testing.T) {
	c, projID := setupSessionTest(t)
	t.Setenv("ARC_SESSION_ID", "arc-env")
	t.Setenv("CODEX_THREAD_ID", "codex-thread")

	// Runtime environment must not turn a missing hook identity into registration.
	t.Run("missing hook identity", func(t *testing.T) {
		sessionTestStdin(t, `{}`)
		require.ErrorIs(t, runSessionStart(aiSessionStartCmd, true), errSkipSession)
	})
	t.Run("missing manual identity", func(t *testing.T) {
		require.EqualError(t, runSessionStart(&cobra.Command{}, false), "--id is required")
	})
	sessions, err := c.ListAISessions(projID, 10, 0)
	require.NoError(t, err)
	require.Empty(t, sessions)

	// A supplied hook payload remains authoritative even when both env IDs differ.
	cwd, err := os.Getwd()
	require.NoError(t, err)
	payload, err := json.Marshal(hookInput{SessionID: "hook-session", CWD: cwd})
	require.NoError(t, err)
	sessionTestStdin(t, string(payload))
	require.NoError(t, runSessionStart(aiSessionStartCmd, true))
	sessions, err = c.ListAISessions(projID, 10, 0)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	require.Equal(t, "hook-session", sessions[0].ID)
}

func TestPrimeSessionIdentity(t *testing.T) {
	const hookID = "983a7cf7-bcb6-48fc-b485-129b4f1aaa45"
	tests := []struct{ name, hook, arcID, codexID, piID, wantID string }{
		{name: "codex", codexID: "codex-thread", wantID: "codex-thread"},
		{name: "pi", piID: "pi-session", wantID: "pi-session"},
		{name: "native conflict", codexID: "codex", piID: "pi"},
		{name: "hook resolves native conflict", hook: hookID, codexID: "codex", piID: "pi", wantID: hookID},
		{name: "arc wins conflict", arcID: "arc-session", codexID: "codex-thread", wantID: "arc-session"},
		{name: "hook wins conflicts", hook: hookID, arcID: "arc-session", codexID: "codex-thread", wantID: hookID},
		{name: "missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupSessionTest(t)
			t.Setenv("ARC_SESSION_ID", tt.arcID)
			t.Setenv("CODEX_THREAD_ID", tt.codexID)
			t.Setenv("PI_SESSION_ID", tt.piID)
			envFile := filepath.Join(t.TempDir(), "claude-env")
			t.Setenv("CLAUDE_ENV_FILE", envFile)
			payload, err := json.Marshal(hookInput{SessionID: tt.hook})
			require.NoError(t, err)
			sessionTestStdin(t, string(payload))
			out := captureStdout(t, func() { primeCmd.Run(primeCmd, nil) })
			if tt.wantID == "" {
				require.NotContains(t, out, "> **Session**:")
			} else {
				require.Contains(t, out, "> **Session**: `"+tt.wantID+"`")
			}
			if tt.hook == "" {
				require.NoFileExists(t, envFile, "environment fallback must not be persisted as a hook identity")
			} else {
				data, err := os.ReadFile(envFile)
				require.NoError(t, err)
				require.Equal(t, "export ARC_SESSION_ID="+tt.hook+"\n", string(data))
			}
		})
	}
}
