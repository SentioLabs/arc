package planmigration_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sentiolabs/arc/internal/planfiles"
	"github.com/sentiolabs/arc/internal/planmigration"
	"github.com/sentiolabs/arc/internal/storage/sqlite"
	"github.com/sentiolabs/arc/internal/types"
	"github.com/stretchr/testify/require"
)

func TestCleanupRejectsDatabaseRootOverlap(t *testing.T) {
	for _, alias := range []string{"direct", "database-symlink", "parent-symlink", "root-parent-symlink"} {
		t.Run(alias, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "plans")
			publisher, err := planfiles.New(root)
			require.NoError(t, err)
			require.NoError(t, publisher.Close())
			database := filepath.Join(root, "data.db")
			store, err := sqlite.New(database)
			require.NoError(t, err)
			defer store.Close()
			project := &types.Project{ID: "preserve", Name: "metadata must survive", Prefix: "safe"}
			require.NoError(t, store.CreateProject(t.Context(), project))
			configuredDB, configuredRoot := database, root
			link := filepath.Join(t.TempDir(), "alias")
			switch alias {
			case "database-symlink":
				require.NoError(t, os.Symlink(database, link))
				configuredDB = link
			case "parent-symlink":
				require.NoError(t, os.Symlink(root, link))
				configuredDB = filepath.Join(link, "data.db")
			case "root-parent-symlink":
				require.NoError(t, os.Symlink(filepath.Dir(root), link))
				configuredRoot = filepath.Join(link, "plans")
			}
			before := snapshotRegularFiles(t, root)
			for _, selection := range [][]string{{"data.db"}, {"data.db-wal"}, {"data.db-shm"}} {
				err := planmigration.Cleanup(t.Context(), configuredDB, configuredRoot, selection, false)
				require.ErrorContains(t, err, "database and SQLite sidecars must be outside the plan root")
				require.Equal(t, before, snapshotRegularFiles(t, root))
			}
			reports, err := planmigration.Inspect(t.Context(), configuredDB, configuredRoot)
			require.ErrorContains(t, err, "database and SQLite sidecars must be outside the plan root")
			require.Empty(t, reports)
			destination := filepath.Join(t.TempDir(), "backup")
			_, err = planmigration.Backup(t.Context(), configuredDB, configuredRoot, destination)
			require.ErrorContains(t, err, "database and SQLite sidecars must be outside the plan root")
			_, err = os.Stat(destination)
			require.ErrorIs(t, err, os.ErrNotExist)
			require.Equal(t, before, snapshotRegularFiles(t, root))
			got, err := store.GetProject(t.Context(), project.ID)
			require.NoError(t, err)
			require.Equal(t, project.Name, got.Name)
			require.NoError(t, store.Close())
			reopened, err := sqlite.OpenPlanOperator(database, false)
			require.NoError(t, err)
			defer reopened.Close()
			got, err = reopened.GetProject(t.Context(), project.ID)
			require.NoError(t, err)
			require.Equal(t, project.Name, got.Name)
		})
	}
}

func snapshotRegularFiles(t *testing.T, root string) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	files := map[string][]byte{}
	for _, entry := range entries {
		if entry.Type().IsRegular() {
			data, err := os.ReadFile(filepath.Join(root, entry.Name()))
			require.NoError(t, err)
			files[entry.Name()] = data
		}
	}
	return files
}

func TestCleanupRejectsSidecarAliasIntoRoot(t *testing.T) {
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		t.Run(suffix, func(t *testing.T) {
			store, root, _ := fixture(t)
			database := store.Path()
			require.NoError(t, store.Close())
			artifact := filepath.Join(root, "retained-sidecar")
			require.NoError(t, os.WriteFile(artifact, []byte("must survive"), 0o600))
			require.NoError(t, os.Symlink(artifact, database+suffix))
			before := snapshotRegularFiles(t, root)
			err := planmigration.Cleanup(t.Context(), database, root, []string{"retained-sidecar"}, false)
			require.ErrorContains(t, err, "database and SQLite sidecars must be outside the plan root")
			require.Equal(t, before, snapshotRegularFiles(t, root))
		})
	}
}

func TestCaseAliasedRootRejectsMetadata(t *testing.T) {
	root := filepath.Join(t.TempDir(), "PlanRoot")
	require.NoError(t, os.Mkdir(root, 0o700))
	alias := filepath.Join(filepath.Dir(root), "planroot")
	requireCaseAlias(t, root, alias)
	publisher, err := planfiles.New(alias)
	require.NoError(t, err)
	require.NoError(t, publisher.Close())
	database := filepath.Join(root, "configured.db")
	store, err := sqlite.New(database)
	require.NoError(t, err)
	defer store.Close()
	project := &types.Project{ID: "keep", Name: "case alias metadata", Prefix: "safe"}
	require.NoError(t, store.CreateProject(t.Context(), project))
	before := snapshotRegularFiles(t, root)
	for _, name := range []string{"configured.db", "configured.db-wal", "configured.db-shm"} {
		cleanupErr := planmigration.Cleanup(t.Context(), database, alias, []string{name}, false)
		_, statErr := os.Stat(filepath.Join(root, name))
		t.Logf("selected=%s cleanup_error=%v file_present=%v", name, cleanupErr, statErr == nil)
		require.ErrorContains(t, cleanupErr, "database and SQLite sidecars must be outside the plan root")
		require.NoError(t, statErr)
		require.Equal(t, before, snapshotRegularFiles(t, root))
	}
	reports, err := planmigration.Inspect(t.Context(), database, alias)
	require.ErrorContains(t, err, "database and SQLite sidecars must be outside the plan root")
	require.Empty(t, reports)
	require.Equal(t, before, snapshotRegularFiles(t, root))
	got, err := store.GetProject(t.Context(), project.ID)
	require.NoError(t, err)
	require.Equal(t, project.Name, got.Name)
	require.NoError(t, store.Close())
	reopened, err := sqlite.OpenPlanOperator(database, false)
	require.NoError(t, err)
	defer reopened.Close()
	got, err = reopened.GetProject(t.Context(), project.ID)
	require.NoError(t, err)
	require.Equal(t, project.Name, got.Name)
}

func TestBackupRejectsCaseAliasedNestedDestination(t *testing.T) {
	store, root, _ := fixture(t)
	upper := filepath.Join(filepath.Dir(root), "PlanRoot")
	require.NoError(t, os.Rename(root, upper))
	alias := filepath.Join(filepath.Dir(root), "planroot")
	requireCaseAlias(t, upper, alias)
	nested := filepath.Join(upper, "new-backup")
	maintenance, err := planfiles.OpenMaintenance(alias)
	require.NoError(t, err)
	// Assert the live containment check before invoking copy, so RED cannot recurse.
	checkErr := maintenance.CheckDestination(nested)
	require.NoError(t, maintenance.Close())
	require.EqualError(t, checkErr, "backup destination must be outside the plan root")
	_, err = planmigration.Backup(t.Context(), store.Path(), alias, nested)
	require.EqualError(t, err, "backup destination must be outside the plan root")
	_, err = os.Stat(nested)
	require.ErrorIs(t, err, os.ErrNotExist)
	// A similar-spelled sibling is physically separate and remains a valid backup.
	sibling := filepath.Join(filepath.Dir(root), "PlanRootSibling")
	_, err = planmigration.Backup(t.Context(), store.Path(), alias, sibling)
	require.NoError(t, err)
	require.NoError(t, planmigration.VerifyBackup(t.Context(), sibling))
}

func requireCaseAlias(t *testing.T, original, alias string) {
	t.Helper()
	originalInfo, err := os.Stat(original)
	require.NoError(t, err)
	aliasInfo, err := os.Stat(alias)
	if os.IsNotExist(err) {
		t.Skip("fixture filesystem does not resolve differently cased paths to one directory")
	}
	require.NoError(t, err)
	if !os.SameFile(originalInfo, aliasInfo) {
		t.Skip("fixture filesystem treats these case variants as distinct directories")
	}
	t.Logf("case-insensitive fixture proven by os.SameFile: %s == %s", original, alias)
}
