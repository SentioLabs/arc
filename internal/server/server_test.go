package server_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sentiolabs/arc/internal/server"
	"github.com/stretchr/testify/require"
)

func TestRunRejectsUnusablePlanRootBeforeOpeningDatabase(t *testing.T) {
	root := t.TempDir()
	blocked := filepath.Join(root, "file")
	require.NoError(t, os.WriteFile(blocked, nil, 0o600))
	db := filepath.Join(root, "db.sqlite")
	err := server.Run(server.Config{Address: "127.0.0.1:0", DBPath: db, PlansDir: filepath.Join(blocked, "plans")})
	require.ErrorContains(t, err, "plan")
	_, err = os.Stat(db)
	require.ErrorIs(t, err, os.ErrNotExist)
}
