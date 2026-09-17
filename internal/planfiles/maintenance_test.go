package planfiles //nolint:testpackage // Exercises publisher and maintenance lifetimes together.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMaintenanceExcludesIndependentPublishers(t *testing.T) {
	root := filepath.Join(t.TempDir(), "plans")
	p, err := New(root)
	require.NoError(t, err)
	second, err := New(root)
	require.NoError(t, err)
	_, err = OpenMaintenance(root)
	require.ErrorIs(t, err, ErrBusy)
	require.NoError(t, p.Close())
	_, err = OpenMaintenance(root)
	require.ErrorIs(t, err, ErrBusy)
	require.NoError(t, second.Close())
	maintenance, err := OpenMaintenance(root)
	require.NoError(t, err)
	defer maintenance.Close()
	_, err = New(root)
	require.ErrorIs(t, err, ErrBusy)
	_, err = p.Publish(context.Background(), "project", "plan", 1, []byte("closed"))
	require.Error(t, err)
}

func TestMaintenanceInspectAndSelectedCleanup(t *testing.T) {
	root := filepath.Join(t.TempDir(), "plans")
	p, err := New(root)
	require.NoError(t, err)
	good, err := p.Publish(context.Background(), "project", "plan", 1, []byte("retained"))
	require.NoError(t, err)
	orphan, err := p.Publish(context.Background(), "project", "plan", 2, []byte("orphan"))
	require.NoError(t, err)
	require.NoError(t, p.Close())
	m, err := OpenMaintenance(root)
	require.NoError(t, err)
	defer m.Close()
	reports, err := m.Inspect(context.Background(), []Blob{good})
	require.NoError(t, err)
	require.Len(t, reports, 2)
	require.NoError(t, m.Cleanup(context.Background(), []Blob{good}, []string{orphan.RelativePath}, true))
	_, err = os.Stat(filepath.Join(root, orphan.RelativePath))
	require.NoError(t, err)
	require.Error(t, m.Cleanup(context.Background(), []Blob{good}, []string{good.RelativePath}, false))
	require.Error(t, m.Cleanup(context.Background(), []Blob{good}, []string{"../outside"}, false))
	require.NoError(t, m.Cleanup(context.Background(), []Blob{good}, []string{orphan.RelativePath}, false))
	require.NoError(t, os.WriteFile(filepath.Join(root, good.RelativePath), []byte("tampered"), 0o600))
	outside := filepath.Join(t.TempDir(), "secret")
	require.NoError(t, os.WriteFile(outside, []byte("private"), 0o600))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "link")))
	reports, err = m.Inspect(context.Background(), []Blob{good})
	require.NoError(t, err)
	statuses := map[string]string{}
	for _, r := range reports {
		statuses[r.Path] = r.Status
	}
	require.Equal(t, "mismatched", statuses[good.RelativePath])
	require.Equal(t, "symlink", statuses["link"])
	require.Error(t, m.Cleanup(context.Background(), []Blob{good}, []string{"link"}, false))
	require.NoError(t, os.Remove(filepath.Join(root, good.RelativePath)))
	reports, err = m.Inspect(context.Background(), []Blob{good})
	require.NoError(t, err)
	for _, report := range reports {
		if report.Path == good.RelativePath {
			require.Equal(t, "missing", report.Status)
		}
	}
}

func TestMaintenanceRejectsSymlinkRootAndNestedBackup(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	p, err := New(root)
	require.NoError(t, err)
	require.NoError(t, p.Close())
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(root, link))
	_, err = OpenMaintenance(link)
	require.Error(t, err)
	m, err := OpenMaintenance(root)
	require.NoError(t, err)
	defer m.Close()
	// Use a canceled context to bound the currently unsafe recursive copy in RED.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	destination := filepath.Join(link, "nested")
	require.Error(t, m.CopyTo(ctx, destination))
	_, err = os.Stat(destination)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestMaintenanceIndependentProcesses(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	p, err := New(root)
	require.NoError(t, err)
	run := func(mode, want string) {
		t.Helper()
		executable, err := os.Executable()
		require.NoError(t, err)
		command := exec.Command(executable, "-test.run=^TestMaintenanceProcessHelper$")
		command.Env = append(
			os.Environ(),
			"ARC_TEST_LOCK_ROOT="+root,
			"ARC_TEST_LOCK_MODE="+mode,
			"ARC_TEST_LOCK_WANT="+want,
		)
		output, err := command.CombinedOutput()
		require.NoError(t, err, string(output))
	}
	run("maintenance", "busy")
	require.NoError(t, p.Close())
	m, err := OpenMaintenance(root)
	require.NoError(t, err)
	run("publisher", "busy")
	require.NoError(t, m.Close())
	run("publisher", "ok")
}

func TestMaintenanceProcessHelper(t *testing.T) {
	root := os.Getenv("ARC_TEST_LOCK_ROOT")
	if root == "" {
		return
	}
	var closer interface{ Close() error }
	var err error
	if os.Getenv("ARC_TEST_LOCK_MODE") == "publisher" {
		closer, err = New(root)
	} else {
		closer, err = OpenMaintenance(root)
	}
	if os.Getenv("ARC_TEST_LOCK_WANT") == "busy" {
		require.ErrorIs(t, err, ErrBusy)
		return
	}
	require.NoError(t, err)
	require.NoError(t, closer.Close())
}
