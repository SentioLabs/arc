package planfiles_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sentiolabs/arc/internal/planfiles"
	"github.com/stretchr/testify/require"
)

func TestPathWithinRootUsesPhysicalAncestors(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "Root")
	nested := filepath.Join(root, "nested")
	sibling := filepath.Join(base, "RootSibling")
	require.NoError(t, os.MkdirAll(nested, 0o700))
	require.NoError(t, os.Mkdir(sibling, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(nested, "existing.db"), []byte("data"), 0o600))
	alias := filepath.Join(base, "alias")
	require.NoError(t, os.Symlink(nested, alias))
	for _, test := range []struct {
		path   string
		inside bool
	}{
		{root, true},
		{nested, true},
		{filepath.Join(nested, "existing.db"), true},
		{filepath.Join(nested, "absent.db-wal"), true},
		{filepath.Join(root, "new", "nested", "backup"), true},
		{alias, true},
		{filepath.Join(alias, "future"), true},
		{filepath.Join(sibling, "future"), false},
		{base, false},
	} {
		inside, err := planfiles.PathWithinRoot(root, test.path)
		require.NoError(t, err, test.path)
		require.Equal(t, test.inside, inside, test.path)
	}
	dangling := filepath.Join(base, "dangling")
	require.NoError(t, os.Symlink(filepath.Join(root, "absent"), dangling))
	_, err := planfiles.PathWithinRoot(root, dangling)
	require.Error(t, err)
	_, err = planfiles.PathWithinRoot(root, filepath.Join(dangling, "child"))
	require.Error(t, err)
	_, err = planfiles.PathWithinRoot(root, "relative")
	require.Error(t, err)
	_, err = planfiles.PathWithinRoot(filepath.Join(nested, "existing.db"), sibling)
	require.Error(t, err)
}

func TestPathWithinRootRetainsCaseSensitiveSiblings(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "Root")
	sibling := filepath.Join(base, "root")
	require.NoError(t, os.Mkdir(root, 0o700))
	if _, err := os.Stat(sibling); err == nil {
		t.Skip("filesystem cannot create distinct case-only sibling directories")
	} else {
		require.ErrorIs(t, err, os.ErrNotExist)
	}
	require.NoError(t, os.Mkdir(sibling, 0o700))
	inside, err := planfiles.PathWithinRoot(root, filepath.Join(sibling, "new-backup"))
	require.NoError(t, err)
	require.False(t, inside)
}
