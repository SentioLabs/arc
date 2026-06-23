// Package jjtest provides helpers for constructing real Jujutsu (jj)
// repositories in tests. Unlike production code, tests may shell out to the
// jj binary. Tests using these helpers skip when jj is not installed.
package jjtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const (
	// dirPerm is the directory mode for test scaffolding (matches gittest).
	dirPerm = 0o755
	// cfgPerm is the mode for the isolated jj config file.
	cfgPerm = 0o600
)

// RequireJJ skips the test if the jj binary is not on PATH.
func RequireJJ(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("jj"); err != nil {
		t.Skip("jj binary not installed; skipping native-jj test")
	}
}

// isolatedConfig writes a minimal jj config under dir and returns its path,
// so jj never reads or writes the developer's real configuration.
func isolatedConfig(t *testing.T, dir string) string {
	t.Helper()
	cfg := filepath.Join(dir, "jjconfig.toml")
	content := "[user]\nname = \"arc-test\"\nemail = \"arc-test@example.com\"\n"
	if err := os.WriteFile(cfg, []byte(content), cfgPerm); err != nil {
		t.Fatal(err)
	}
	return cfg
}

// Run executes a jj subcommand in workdir, failing the test on error.
func Run(t *testing.T, workdir string, args ...string) {
	t.Helper()
	cmd := exec.Command("jj", args...)
	cmd.Dir = workdir
	cmd.Env = append(os.Environ(), "JJ_CONFIG="+isolatedConfig(t, workdir))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("jj %v (in %s): %v\n%s", args, workdir, err, out)
	}
}

// InitNative creates a native (non-colocated) jj repo in dir and returns dir.
func InitNative(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		t.Fatal(err)
	}
	Run(t, dir, "git", "init")
	return dir
}

// AddRemote registers a git remote on the jj repo at dir.
func AddRemote(t *testing.T, dir, name, url string) {
	t.Helper()
	Run(t, dir, "git", "remote", "add", name, url)
}

// AddWorkspace adds a secondary jj workspace at wsPath from the main repo at
// mainDir, with the given workspace name.
func AddWorkspace(t *testing.T, mainDir, wsPath, name string) {
	t.Helper()
	Run(t, mainDir, "workspace", "add", "--name", name, wsPath)
}
