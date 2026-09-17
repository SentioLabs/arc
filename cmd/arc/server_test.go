package main

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/sentiolabs/arc/internal/config"
	"github.com/stretchr/testify/require"
)

// Exercise the actual background re-exec, with every persistent path under a
// temporary home. No command in this test reads the operator's default config.
func TestBackgroundServerPreservesSelectedPlanRoot(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "arc")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", binary, ".")
	output, err := build.CombinedOutput()
	require.NoError(t, err, string(output))
	for _, unusable := range []bool{false, true} {
		name := "custom-root"
		if unusable {
			name = "unusable-root"
		}
		t.Run(name, func(t *testing.T) {
			fixture := newDaemonFixture(t, unusable)
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			//nolint:gosec // binary and config are created exclusively by this isolated test
			command := exec.CommandContext(ctx, binary, "--config", fixture.configPath, "server", "start")
			command.Env = fixture.env
			output, err := command.CombinedOutput()
			if unusable {
				require.Error(t, err, string(output))
				log, readErr := os.ReadFile(filepath.Join(fixture.home, ".arc", "server.log"))
				require.NoError(t, readErr)
				require.Contains(t, string(log), "initialize plan files")
				require.NoFileExists(t, fixture.db)
			} else {
				require.NoError(t, err, string(output))
				require.DirExists(t, fixture.plans)
				require.FileExists(t, fixture.db)
			}
			require.NoDirExists(t, filepath.Join(fixture.home, ".arc", "plans"),
				"must not silently use the default root")
		})
	}
}

type daemonFixture struct {
	home, configPath, plans, db string
	env                         []string
}

func newDaemonFixture(t *testing.T, unusable bool) daemonFixture {
	t.Helper()
	root := t.TempDir()
	f := daemonFixture{
		home: filepath.Join(root, "home"), configPath: filepath.Join(root, "operator.toml"),
		plans: filepath.Join(root, "operator-content"), db: filepath.Join(root, "operator.sqlite"),
	}
	require.NoError(t, os.Mkdir(f.home, 0o700))
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "HOME=") {
			f.env = append(f.env, value)
		}
	}
	f.env = append(f.env, "HOME="+f.home)
	if unusable {
		require.NoError(t, os.WriteFile(f.plans, []byte("not a directory"), 0o600))
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	cfg := config.Default()
	cfg.Server.Port, cfg.Server.DBPath, cfg.Server.PlansDir = port, f.db, f.plans
	require.NoError(t, config.Save(f.configPath, cfg))
	t.Cleanup(func() {
		// Only signal the PID published inside this fixture's private home.
		data, readErr := os.ReadFile(filepath.Join(f.home, ".arc", "server.pid"))
		if os.IsNotExist(readErr) {
			return
		}
		require.NoError(t, readErr)
		pid, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
		require.NoError(t, parseErr)
		require.Positive(t, pid)
		child, findErr := os.FindProcess(pid)
		require.NoError(t, findErr)
		defer func() {
			_ = child.Kill()
			require.NoError(t, child.Release())
		}()
		_ = child.Signal(syscall.SIGTERM)
		require.Eventually(t, func() bool {
			address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
			conn, dialErr := net.DialTimeout("tcp", address, 100*time.Millisecond)
			if dialErr != nil {
				return true
			}
			_ = conn.Close()
			return false
		}, 5*time.Second, 50*time.Millisecond)
	})
	return f
}
