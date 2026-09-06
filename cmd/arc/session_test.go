package main

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
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
		explicitSet, withoutTake                                      bool
	}{
		{name: "explicit", explicit: "explicit-session", explicitSet: true, wantID: "explicit-session"},
		{name: "arc environment", arcID: "arc-session", wantID: "arc-session"},
		{name: "native Codex environment is ignored", codexID: "codex-thread", wantErr: "no session ID available"},
		{name: "native Pi environment is ignored", piID: "pi-session", wantErr: "no session ID available"},
		{
			name: "missing generic identity with native environments", codexID: "codex", piID: "pi",
			wantErr: "no session ID available",
		},
		{
			name: "flag wins conflicts", explicit: "flag", explicitSet: true, arcID: "arc-session",
			codexID: "codex", piID: "pi", wantID: "flag",
		},
		{name: "arc wins native conflict", arcID: "arc-session", codexID: "codex", piID: "pi", wantID: "arc-session"},
		{
			name: "preserve status", explicit: "explicit", explicitSet: true,
			status: string(types.StatusBlocked), wantID: "explicit",
		},
		{
			name: "empty explicit value does not fall back", explicitSet: true, arcID: "arc-session",
			codexID: "codex", piID: "pi", wantErr: "--session-id cannot be empty",
		},
		{
			name: "flag requires take", explicit: "flag", explicitSet: true, codexID: "codex", withoutTake: true,
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
			if tt.explicitSet {
				require.NoError(t, cmd.Flags().Set("session-id", tt.explicit))
			}
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
		})
	}
}

func TestHarnessRegistrationAndExplicitTake(t *testing.T) {
	tests := []struct {
		name, id string
	}{
		{name: "Claude", id: "983a7cf7-bcb6-48fc-b485-129b4f1aaa45"},
		{name: "Codex", id: "72b52c9a-2f32-4ae8-b387-ecde0e36e70f"},
		{name: "Pi", id: "455df4ac-2b0f-4d28-93b7-f811a8fb4b03"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, projID := setupSessionTest(t)
			cwd, err := os.Getwd()
			require.NoError(t, err)
			registerCmd := &cobra.Command{}
			registerCmd.Flags().String("id", "", "")
			registerCmd.Flags().String("transcript-path", "", "")
			registerCmd.Flags().String("cwd", "", "")
			require.NoError(t, registerCmd.Flags().Set("id", tt.id))
			require.NoError(t, registerCmd.Flags().Set("cwd", cwd))
			require.NoError(t, runSessionStart(registerCmd, false))
			registered, err := c.GetAISession(projID, tt.id)
			require.NoError(t, err)
			issue, err := c.CreateIssue(projID, client.CreateIssueRequest{Title: tt.name + " claim fixture"})
			require.NoError(t, err)
			claimCmd := takeTestCommand()
			require.NoError(t, claimCmd.Flags().Set("session-id", tt.id))
			require.NoError(t, updateCmd.RunE(claimCmd, []string{issue.ID}))
			claimed, err := c.GetIssueByID(issue.ID)
			require.NoError(t, err)
			require.Equal(t, registered.ID, claimed.AISessionID)
			require.Equal(t, tt.id, claimed.AISessionID)
			require.Equal(t, types.StatusInProgress, claimed.Status)
		})
	}
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

func primeTestCommand() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().String("session-id", "", "")
	return cmd
}

func TestPrimeSessionIdentity(t *testing.T) {
	const hookID = "983a7cf7-bcb6-48fc-b485-129b4f1aaa45"
	tests := []struct {
		name, explicit, hook, arcID, codexID, piID, wantID, wantErr string
		explicitSet                                                 bool
	}{
		{
			name: "explicit wins conflicts", explicit: "explicit", explicitSet: true, hook: hookID,
			arcID: "arc-session", codexID: "codex-thread", piID: "pi", wantID: "explicit",
		},
		{
			name: "hook wins ARC fallback", hook: hookID, arcID: "arc-session",
			codexID: "codex-thread", piID: "pi", wantID: hookID,
		},
		{
			name: "ARC fallback ignores native environments", arcID: "arc-session",
			codexID: "codex-thread", piID: "pi", wantID: "arc-session",
		},
		{name: "native environments are ignored", codexID: "codex-thread", piID: "pi"},
		{
			name: "empty explicit fails", explicitSet: true, hook: hookID, arcID: "arc-session",
			wantErr: "--session-id cannot be empty",
		},
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
			cmd := primeTestCommand()
			if tt.explicitSet {
				require.NoError(t, cmd.Flags().Set("session-id", tt.explicit))
			}
			var runErr error
			out := captureStdout(t, func() { runErr = primeCmd.RunE(cmd, nil) })
			if tt.wantErr != "" {
				require.ErrorContains(t, runErr, tt.wantErr)
				require.Empty(t, out)
				require.NoFileExists(t, envFile)
				return
			}
			require.NoError(t, runErr)
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
