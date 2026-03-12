package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadProjectConfigs(t *testing.T) {
	// Set up a temp directory simulating ~/.arc with projects/ subdirectories
	arcHome := t.TempDir()
	projDir := filepath.Join(arcHome, "projects")

	// Create two project config directories
	dir1 := filepath.Join(projDir, "-home-user-project1")
	dir2 := filepath.Join(projDir, "-home-user-project2")
	require.NoError(t, os.MkdirAll(dir1, 0o755))
	require.NoError(t, os.MkdirAll(dir2, 0o755))

	cfg1 := map[string]string{
		"workspace_id":   "ws-001",
		"workspace_name": "project1",
		"project_root":   "/home/user/project1",
	}
	cfg2 := map[string]string{
		"workspace_id":   "ws-002",
		"workspace_name": "project2",
		"project_root":   "/home/user/project2",
	}

	data1, _ := json.Marshal(cfg1)
	data2, _ := json.Marshal(cfg2)
	require.NoError(t, os.WriteFile(filepath.Join(dir1, "config.json"), data1, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "config.json"), data2, 0o600))

	// Also create a non-directory file that should be skipped
	require.NoError(t, os.WriteFile(filepath.Join(projDir, "stray-file"), []byte("ignore"), 0o600))

	// Create a directory without config.json that should be skipped
	emptyDir := filepath.Join(projDir, "empty-dir")
	require.NoError(t, os.MkdirAll(emptyDir, 0o755))

	configs, err := readProjectConfigs(arcHome)
	require.NoError(t, err)
	assert.Len(t, configs, 2)

	// Verify configs were read (order may vary, so check by workspace ID)
	ids := map[string]bool{}
	for _, c := range configs {
		ids[c.WorkspaceID] = true
		if c.WorkspaceID == "ws-001" {
			assert.Equal(t, "project1", c.WorkspaceName)
			assert.Equal(t, "/home/user/project1", c.ProjectRoot)
		}
	}
	assert.True(t, ids["ws-001"])
	assert.True(t, ids["ws-002"])
}

func TestReadProjectConfigsNoProjectsDir(t *testing.T) {
	arcHome := t.TempDir()
	// projects/ doesn't exist
	configs, err := readProjectConfigs(arcHome)
	require.NoError(t, err)
	assert.Empty(t, configs)
}

func TestBackupProjectsDir(t *testing.T) {
	arcHome := t.TempDir()
	projDir := filepath.Join(arcHome, "projects")
	require.NoError(t, os.MkdirAll(projDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projDir, "test.txt"), []byte("data"), 0o600))

	err := backupProjectsDir(arcHome)
	require.NoError(t, err)

	// projects/ should be gone
	_, err = os.Stat(projDir)
	assert.True(t, os.IsNotExist(err))

	// projects.bak/ should exist with the file
	bakDir := filepath.Join(arcHome, "projects.bak")
	data, err := os.ReadFile(filepath.Join(bakDir, "test.txt"))
	require.NoError(t, err)
	assert.Equal(t, "data", string(data))
}

func TestBackupProjectsDirAlreadyExists(t *testing.T) {
	arcHome := t.TempDir()
	projDir := filepath.Join(arcHome, "projects")
	bakDir := filepath.Join(arcHome, "projects.bak")
	require.NoError(t, os.MkdirAll(projDir, 0o755))
	require.NoError(t, os.MkdirAll(bakDir, 0o755))

	err := backupProjectsDir(arcHome)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}
