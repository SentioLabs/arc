package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelfChannelShowDefault(t *testing.T) {
	tmp := t.TempDir()
	cfgFile := filepath.Join(tmp, "cli-config.json")
	require.NoError(t, os.WriteFile(cfgFile, []byte(`{"server_url":"http://localhost:7432"}`), 0o600))

	origPath := configPath
	configPath = cfgFile
	t.Cleanup(func() { configPath = origPath })

	// Run with no args — should show default channel
	cfg, err := loadConfig()
	require.NoError(t, err)

	channel := cfg.Channel
	if channel == "" {
		channel = "stable"
	}
	assert.Equal(t, "stable", channel)
}

func TestSelfChannelSwitchNightly(t *testing.T) {
	tmp := t.TempDir()
	cfgFile := filepath.Join(tmp, "cli-config.json")
	require.NoError(t, os.WriteFile(cfgFile, []byte(`{"server_url":"http://localhost:7432"}`), 0o600))

	origPath := configPath
	configPath = cfgFile
	t.Cleanup(func() { configPath = origPath })

	// Simulate setting channel (what runSelfChannel does with --yes)
	cfg, err := loadConfig()
	require.NoError(t, err)

	err = setSelfChannel(cfg, "nightly")
	require.NoError(t, err)

	// Re-read and verify
	cfg, err = loadConfig()
	require.NoError(t, err)
	assert.Equal(t, "nightly", cfg.Channel)
}

func TestSelfChannelSwitchRC(t *testing.T) {
	tmp := t.TempDir()
	cfgFile := filepath.Join(tmp, "cli-config.json")
	require.NoError(t, os.WriteFile(cfgFile, []byte(`{"server_url":"http://localhost:7432"}`), 0o600))

	origPath := configPath
	configPath = cfgFile
	t.Cleanup(func() { configPath = origPath })

	cfg, err := loadConfig()
	require.NoError(t, err)

	err = setSelfChannel(cfg, "rc")
	require.NoError(t, err)

	cfg, err = loadConfig()
	require.NoError(t, err)
	assert.Equal(t, "rc", cfg.Channel)
}

func TestSelfChannelInvalid(t *testing.T) {
	tmp := t.TempDir()
	cfgFile := filepath.Join(tmp, "cli-config.json")
	require.NoError(t, os.WriteFile(cfgFile, []byte(`{"server_url":"http://localhost:7432"}`), 0o600))

	origPath := configPath
	configPath = cfgFile
	t.Cleanup(func() { configPath = origPath })

	cfg, err := loadConfig()
	require.NoError(t, err)

	err = setSelfChannel(cfg, "beta")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid channel")
}
