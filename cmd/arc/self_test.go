package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/semver"
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
		channel = channelStable
	}
	assert.Equal(t, channelStable, channel)
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

	err = setSelfChannel(cfg, channelNightly)
	require.NoError(t, err)

	// Re-read and verify
	cfg, err = loadConfig()
	require.NoError(t, err)
	assert.Equal(t, channelNightly, cfg.Channel)
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

	err = setSelfChannel(cfg, channelRC)
	require.NoError(t, err)

	cfg, err = loadConfig()
	require.NoError(t, err)
	assert.Equal(t, channelRC, cfg.Channel)
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
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid channel")
}

// mockReleases creates test data for GitHub API mocking.
// Uses real-world tag formats: rc3 (no dot) and nightly.20260302 (with dot).
// Nightly builds use the next unreleased version (v0.11.0) so they sort
// higher than the latest stable (v0.10.0) per semver.
func mockReleases() []githubRelease {
	return []githubRelease{
		{TagName: "v0.11.0-rc3", Prerelease: true},
		{TagName: "v0.11.0-rc2", Prerelease: true},
		{TagName: "v0.11.0-rc.1", Prerelease: true}, // also accept dotted format
		{TagName: "v0.11.0-nightly.20260302", Prerelease: true},
		{TagName: "v0.11.0-nightly.20260301", Prerelease: true},
		{TagName: "v0.10.0", Prerelease: false},
	}
}

func TestResolveChannelVersion_Stable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/releases/latest" {
			_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": "v0.10.0"})
		}
	}))
	defer srv.Close()

	origURL := githubReleasesURL
	githubReleasesURL = srv.URL + "/releases"
	t.Cleanup(func() { githubReleasesURL = origURL })

	tag, err := resolveChannelVersion(channelStable)
	require.NoError(t, err)
	assert.Equal(t, "v0.10.0", tag)
}

func TestResolveChannelVersion_RC(t *testing.T) {
	releases := mockReleases()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(releases)
	}))
	defer srv.Close()

	origURL := githubReleasesURL
	githubReleasesURL = srv.URL + "/releases"
	t.Cleanup(func() { githubReleasesURL = origURL })

	tag, err := resolveChannelVersion(channelRC)
	require.NoError(t, err)
	assert.Equal(t, "v0.11.0-rc3", tag)
}

func TestResolveChannelVersion_RC_StableIsNewer(t *testing.T) {
	// When stable surpasses the latest RC, channel should return stable
	releases := []githubRelease{
		{TagName: "v0.12.0", Prerelease: false},
		{TagName: "v0.11.0-rc3", Prerelease: true},
		{TagName: "v0.11.0-rc2", Prerelease: true},
		{TagName: "v0.10.0", Prerelease: false},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(releases)
	}))
	defer srv.Close()

	origURL := githubReleasesURL
	githubReleasesURL = srv.URL + "/releases"
	t.Cleanup(func() { githubReleasesURL = origURL })

	tag, err := resolveChannelVersion(channelRC)
	require.NoError(t, err)
	assert.Equal(t, "v0.12.0", tag)
}

func TestResolveChannelVersion_Nightly(t *testing.T) {
	releases := mockReleases()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(releases)
	}))
	defer srv.Close()

	origURL := githubReleasesURL
	githubReleasesURL = srv.URL + "/releases"
	t.Cleanup(func() { githubReleasesURL = origURL })

	tag, err := resolveChannelVersion(channelNightly)
	require.NoError(t, err)
	assert.Equal(t, "v0.11.0-nightly.20260302", tag)
}

func TestResolveChannelVersion_Nightly_StableIsNewer(t *testing.T) {
	releases := []githubRelease{
		{TagName: "v0.11.0", Prerelease: false},
		{TagName: "v0.10.0-nightly.20260302", Prerelease: true},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(releases)
	}))
	defer srv.Close()

	origURL := githubReleasesURL
	githubReleasesURL = srv.URL + "/releases"
	t.Cleanup(func() { githubReleasesURL = origURL })

	tag, err := resolveChannelVersion(channelNightly)
	require.NoError(t, err)
	assert.Equal(t, "v0.11.0", tag)
}

func TestResolveChannelVersion_NoChannelMatch_FallsBackToStable(t *testing.T) {
	// No nightly releases exist, but stable does — should return stable
	releases := []githubRelease{
		{TagName: "v0.10.0", Prerelease: false},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(releases)
	}))
	defer srv.Close()

	origURL := githubReleasesURL
	githubReleasesURL = srv.URL + "/releases"
	t.Cleanup(func() { githubReleasesURL = origURL })

	tag, err := resolveChannelVersion(channelNightly)
	require.NoError(t, err)
	assert.Equal(t, "v0.10.0", tag)
}

func TestResolveChannelVersion_NoMatch(t *testing.T) {
	// Empty releases — no match for RC
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]githubRelease{})
	}))
	defer srv.Close()

	origURL := githubReleasesURL
	githubReleasesURL = srv.URL + "/releases"
	t.Cleanup(func() { githubReleasesURL = origURL })

	_, err := resolveChannelVersion(channelRC)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no rc release found")
}

func TestSemverComparison(t *testing.T) {
	tests := []struct {
		a, b string
		want int // +1 if a > b, 0 if equal, -1 if a < b
	}{
		{"v0.10.0", "v0.9.0", 1},
		{"v0.11.0-rc.2", "v0.11.0-rc.1", 1},
		{"v0.11.0", "v0.11.0-rc.1", 1}, // stable > prerelease per semver
		{"v0.11.0-nightly.20260302", "v0.11.0-nightly.20260301", 1},
		{"v0.10.0", "v0.10.0", 0},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_vs_%s", tt.a, tt.b), func(t *testing.T) {
			got := semver.Compare(tt.a, tt.b)
			assert.Equal(t, tt.want, got)
		})
	}
}
