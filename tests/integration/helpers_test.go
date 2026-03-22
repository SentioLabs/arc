//go:build integration

// Package integration provides end-to-end tests that exercise the arc CLI
// binary against a running arc-server. Tests are gated behind the
// "integration" build tag and require ARC_BINARY to point at the compiled
// arc binary.
package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// serverURL is the address of the test server used for health checks
// in TestMain. The arc CLI reads ARC_SERVER from the environment directly.
const serverURL = "http://localhost:7432"

// arcBinary holds the path to the arc CLI binary, set from ARC_BINARY env var.
var arcBinary string

// TestMain sets up the integration test environment. It reads ARC_BINARY,
// waits for the test server to be reachable, and then runs the tests.
func TestMain(m *testing.M) {
	arcBinary = os.Getenv("ARC_BINARY")
	if arcBinary == "" {
		fmt.Fprintln(os.Stderr, "ARC_BINARY not set; skipping integration tests")
		os.Exit(0)
	}

	// Resolve to absolute path so tests can change directories freely.
	abs, err := filepath.Abs(arcBinary)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve ARC_BINARY: %v\n", err)
		os.Exit(1)
	}
	arcBinary = abs

	if err := waitForServer(serverURL, 30*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "server not reachable at %s: %v\n", serverURL, err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// waitForServer polls the server's health endpoint until it responds with
// HTTP 200 or the timeout elapses.
func waitForServer(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get(url + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timed out after %s waiting for %s", timeout, url)
}

// arcCmd runs the arc binary with the given arguments inside an isolated
// HOME directory. It returns the combined output and any error. The
// isolated HOME ensures that each call gets its own ~/.arc/ config
// directory, preventing test pollution.
func arcCmd(t *testing.T, homeDir string, args ...string) (string, error) {
	t.Helper()

	cmd := exec.Command(arcBinary, args...)
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"ARC_SERVER="+serverURL,
	)

	out, err := cmd.CombinedOutput()
	return string(out), err
}

// arcCmdSuccess runs the arc binary and fails the test if the command
// returns a non-zero exit code.
func arcCmdSuccess(t *testing.T, homeDir string, args ...string) string {
	t.Helper()

	output, err := arcCmd(t, homeDir, args...)
	if err != nil {
		t.Fatalf("arc %v failed: %v\noutput: %s", args, err, output)
	}
	return output
}

// arcCmdSuccessWithEnv runs the arc binary with extra env vars and fails the test on error.
func arcCmdSuccessWithEnv(t *testing.T, home string, env []string, args ...string) string {
	t.Helper()
	baseEnv := append(os.Environ(), "HOME="+home, "ARC_SERVER="+serverURL)
	cmd := exec.Command(arcBinary, args...)
	cmd.Env = append(baseEnv, env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("arc %v failed: %v\noutput: %s", args, err, string(out))
	}
	return string(out)
}

// arcCmdWithStdin runs the arc binary with the given stdin data and returns
// the combined output and any error.
func arcCmdWithStdin(t *testing.T, homeDir string, stdin string, args ...string) (string, error) {
	t.Helper()

	cmd := exec.Command(arcBinary, args...)
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"ARC_SERVER="+serverURL,
	)
	cmd.Stdin = strings.NewReader(stdin)

	out, err := cmd.CombinedOutput()
	return string(out), err
}

// arcCmdWithStdinSuccess runs the arc binary with stdin and fails the test on error.
func arcCmdWithStdinSuccess(t *testing.T, homeDir string, stdin string, args ...string) string {
	t.Helper()

	output, err := arcCmdWithStdin(t, homeDir, stdin, args...)
	if err != nil {
		t.Fatalf("arc %v failed: %v\noutput: %s", args, err, output)
	}
	return output
}

// setupHome creates an isolated temporary directory to serve as HOME for
// a single test. It writes a minimal arc CLI config pointing at the test
// server. The directory is cleaned up when the test finishes.
func setupHome(t *testing.T) string {
	t.Helper()

	home := t.TempDir()

	// Create ~/.arc/ directory and write config.
	arcDir := filepath.Join(home, ".arc")
	if err := os.MkdirAll(arcDir, 0o755); err != nil {
		t.Fatalf("create arc config dir: %v", err)
	}

	cfg := map[string]string{"server_url": serverURL}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}

	cfgPath := filepath.Join(arcDir, "cli-config.json")
	if err := os.WriteFile(cfgPath, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	return home
}

