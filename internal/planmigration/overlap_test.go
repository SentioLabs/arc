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
