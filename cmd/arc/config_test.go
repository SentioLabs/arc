package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigChannelRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	cfgFile := filepath.Join(tmp, "cli-config.json")

	// Write config with channel
	data := []byte(`{"server_url":"http://localhost:7432","channel":"nightly"}`)
	require.NoError(t, os.WriteFile(cfgFile, data, 0o600))

	// Point loadConfig at our temp file
	origPath := configPath
	configPath = cfgFile
	t.Cleanup(func() { configPath = origPath })

	cfg, err := loadConfig()
	require.NoError(t, err)
	assert.Equal(t, "nightly", cfg.Channel)

	// Save and re-read
	require.NoError(t, saveConfig(cfg))
	raw, err := os.ReadFile(cfgFile)
	require.NoError(t, err)

	var reloaded Config
	require.NoError(t, json.Unmarshal(raw, &reloaded))
	assert.Equal(t, "nightly", reloaded.Channel)
}

func TestConfigChannelDefault(t *testing.T) {
	tmp := t.TempDir()
	cfgFile := filepath.Join(tmp, "cli-config.json")

	data := []byte(`{"server_url":"http://localhost:7432"}`)
	require.NoError(t, os.WriteFile(cfgFile, data, 0o600))

	origPath := configPath
	configPath = cfgFile
	t.Cleanup(func() { configPath = origPath })

	cfg, err := loadConfig()
	require.NoError(t, err)
	assert.Equal(t, "", cfg.Channel)
}

func TestConfigSetInvalidChannel(t *testing.T) {
	tmp := t.TempDir()
	cfgFile := filepath.Join(tmp, "cli-config.json")

	data := []byte(`{"server_url":"http://localhost:7432"}`)
	require.NoError(t, os.WriteFile(cfgFile, data, 0o600))

	origPath := configPath
	configPath = cfgFile
	t.Cleanup(func() { configPath = origPath })

	// Simulate what configSetCmd does for an invalid channel
	cfg, err := loadConfig()
	require.NoError(t, err)

	value := "beta"
	var setErr error
	switch value {
	case "stable", "rc", "nightly":
		cfg.Channel = value
	default:
		setErr = fmt.Errorf("invalid channel %q: must be stable, rc, or nightly", value)
	}

	assert.Error(t, setErr)
	assert.Contains(t, setErr.Error(), "invalid channel")
}
