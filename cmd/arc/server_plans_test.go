package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sentiolabs/arc/internal/planfiles"
	"github.com/sentiolabs/arc/internal/planmigration"
	"github.com/sentiolabs/arc/internal/storage/sqlite"
	"github.com/sentiolabs/arc/internal/types"
	"github.com/stretchr/testify/require"
)

func TestServerPlansRequiresExplicitModeAndConfiguredRoot(t *testing.T) {
	base := t.TempDir()
	s, err := sqlite.New(filepath.Join(base, "data.db"))
	require.NoError(t, err)
	defer s.Close()
	require.NoError(
		t,
		s.CreateProject(t.Context(), &types.Project{ID: "migration-project", Name: "operator", Prefix: "op"}),
	)
	require.NoError(
		t,
		s.CreatePlan(t.Context(), &types.LegacyPlan{ID: "plan.cli", FilePath: "/wrong/source", Status: "approved"}),
	)
	source := filepath.Join(base, "draft.md")
	require.NoError(t, os.WriteFile(source, []byte("exact\r\n"), 0o600))
	manifest := filepath.Join(base, "import.json")
	body, err := json.Marshal(planmigration.Manifest{Entries: []planmigration.Entry{
		{LegacyID: "plan.cli", ProjectID: "migration-project", SourceFile: source},
	}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(manifest, body, 0o600))
	config := filepath.Join(base, "config.toml")
	require.NoError(
		t,
		os.WriteFile(
			config,
			[]byte("[server]\ndb_path = '"+s.Path()+"'\n"+
				"plans_dir = '"+filepath.Join(base, "server-plans")+"'\n[plans]\n"+
				"dir = '"+filepath.Join(base, "client-drafts")+"'\n"),
			0o600,
		),
	)
	old := configPath
	configPath = config
	t.Cleanup(func() { configPath = old })
	run := func(args ...string) (string, error) {
		cmd := newServerPlansCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(args)
		err := cmd.Execute()
		return out.String(), err
	}
	_, err = run("migrate", "--manifest", manifest)
	require.Error(t, err)
	out, err := run("migrate", "--manifest", manifest, "--dry-run")
	require.NoError(t, err)
	require.Contains(t, out, "ready")
	_, err = os.Stat(filepath.Join(base, "server-plans"))
	require.ErrorIs(t, err, os.ErrNotExist)
	out, err = run("migrate", "--manifest", manifest, "--apply")
	require.NoError(t, err)
	require.Contains(t, out, "imported")
	_, err = os.Stat(filepath.Join(base, "client-drafts"))
	require.ErrorIs(t, err, os.ErrNotExist)
	p, err := planfiles.New(filepath.Join(base, "server-plans"))
	require.NoError(t, err)
	orphan, err := p.Publish(t.Context(), "migration-project", "plan.orphan", 1, []byte("orphan"))
	require.NoError(t, err)
	require.NoError(t, p.Close())
	out, err = run("integrity")
	require.NoError(t, err)
	report := filepath.Join(base, "report.json")
	require.NoError(t, os.WriteFile(report, []byte(out), 0o600))
	_, err = run("cleanup", "--report", report)
	require.Error(t, err)
	_, err = run("cleanup", "--report", report, "--dry-run")
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(base, "server-plans", orphan.RelativePath))
	require.NoError(t, err)
	_, err = run("cleanup", "--report", report, "--apply")
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(base, "server-plans", orphan.RelativePath))
	require.ErrorIs(t, err, os.ErrNotExist)
	verifyPlansCLIBackup(t, base, run)
}

func verifyPlansCLIBackup(t *testing.T, base string, run func(...string) (string, error)) {
	t.Helper()
	backup := filepath.Join(base, "backup")
	_, err := run("backup", "--output", backup)
	require.NoError(t, err)
	out, err := run("verify-backup", "--directory", backup)
	require.NoError(t, err)
	require.Contains(t, out, "verified")
}
