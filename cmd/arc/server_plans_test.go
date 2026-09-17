package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestServerPlansDryRunDoesNotWriteConfig(t *testing.T) {
	for _, mode := range []string{"missing-default", "missing-explicit", "legacy-default", "legacy-explicit"} {
		t.Run(mode, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			selected := filepath.Join(home, ".arc", "config.toml")
			old := configPath
			configPath = ""
			t.Cleanup(func() { configPath = old })
			if strings.HasSuffix(mode, "explicit") {
				selected = filepath.Join(home, "selected", "config.toml")
				configPath = selected
			}
			legacy := filepath.Join(filepath.Dir(selected), "cli-config.json")
			var original []byte
			if strings.HasPrefix(mode, "legacy") {
				require.NoError(t, os.MkdirAll(filepath.Dir(legacy), 0o700))
				original = []byte(`{"server_url":"http://localhost:12345","channel":"stable"}`)
				require.NoError(t, os.WriteFile(legacy, original, 0o600))
			}
			manifest := filepath.Join(t.TempDir(), "manifest.json")
			require.NoError(t, os.WriteFile(manifest, []byte(`{"entries":[
 {"legacy_id":"plan.test","project_id":"project","source_file":"/explicit/source"}
 ]}`), 0o600))
			command := newServerPlansCommand()
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&output)
			command.SetArgs([]string{"migrate", "--manifest", manifest, "--dry-run"})
			require.Error(t, command.Execute()) // No DB exists in the temporary home.
			_, err := os.Stat(selected)
			require.ErrorIs(t, err, os.ErrNotExist)
			_, err = os.Stat(legacy + ".bak")
			require.ErrorIs(t, err, os.ErrNotExist)
			if original != nil {
				retained, err := os.ReadFile(legacy)
				require.NoError(t, err)
				require.Equal(t, original, retained)
				entries, err := os.ReadDir(filepath.Dir(legacy))
				require.NoError(t, err)
				require.Len(t, entries, 1)
			} else {
				_, err = os.Stat(filepath.Dir(selected))
				require.ErrorIs(t, err, os.ErrNotExist)
			}
		})
	}
}
