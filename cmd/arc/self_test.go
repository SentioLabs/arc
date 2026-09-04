package main

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/sentiolabs/go-selfupdate"
	"github.com/sentiolabs/go-selfupdate/cobracmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// useTempConfig points configPath at a throwaway file for the test.
func useTempConfig(t *testing.T) {
	t.Helper()
	origPath := configPath
	configPath = filepath.Join(t.TempDir(), "config.toml")
	t.Cleanup(func() { configPath = origPath })
}

func TestChannelStore_DefaultIsStable(t *testing.T) {
	useTempConfig(t)
	ch, err := arcChannelStore().Channel()
	require.NoError(t, err)
	assert.Equal(t, selfupdate.ChannelStable, ch)
}

func TestChannelStore_PersistsRCAndNightly(t *testing.T) {
	useTempConfig(t)
	store := arcChannelStore()
	for _, ch := range []selfupdate.Channel{selfupdate.ChannelNightly, selfupdate.ChannelRC} {
		require.NoError(t, store.SetChannel(ch))
		cfg, err := loadConfig()
		require.NoError(t, err)
		assert.Equal(t, string(ch), cfg.Updates.Channel)
		got, err := store.Channel()
		require.NoError(t, err)
		assert.Equal(t, ch, got)
	}
}

func TestSelfUpdaterWiring(t *testing.T) {
	useTempConfig(t)
	u := newSelfUpdater()
	assert.Equal(t, "arc", u.Name)
	src, ok := u.Source.(*selfupdate.GitHubSource)
	require.True(t, ok)
	assert.Equal(t, "sentiolabs", src.Owner)
	assert.Equal(t, "arc", src.Repo)
	inst, ok := u.Installer.(*selfupdate.ScriptInstaller)
	require.True(t, ok)
	assert.Contains(t, inst.ScriptURL, "sentiolabs/arc/main/scripts/install.sh")
	assert.NotNil(t, u.PreInstall)
}

// channelCmdName is the "channel" subcommand's name.
const channelCmdName = "channel"

func TestSelfCommandKeepsCheckShorthand(t *testing.T) {
	update, _, err := selfCmd.Find([]string{"update"})
	require.NoError(t, err)
	check := update.Flags().Lookup("check")
	require.NotNil(t, check)
	assert.Equal(t, "c", check.Shorthand)
	channel, _, err := selfCmd.Find([]string{channelCmdName})
	require.NoError(t, err)
	assert.Equal(t, "y", channel.Flags().Lookup("yes").Shorthand)
}

func TestInvalidChannelRejected(t *testing.T) {
	useTempConfig(t)
	u := newSelfUpdater()
	u.Store = &selfupdate.MemStore{}
	err := u.SwitchChannel("beta", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid channel")
}

func TestPreInstallBacksUpOnlyOnMajorMinorChange(t *testing.T) {
	calls := 0
	hook := preInstallBackup(func() error { calls++; return nil })
	require.NoError(t, hook(t.Context(), "v0.15.0", "v0.15.1"))
	assert.Equal(t, 0, calls, "patch bump must not back up")
	require.NoError(t, hook(t.Context(), "v0.15.0", "v0.16.0"))
	assert.Equal(t, 1, calls, "minor bump must back up")
	require.NoError(t, hook(t.Context(), "v0.16.0", "v1.0.0"))
	assert.Equal(t, 2, calls, "major bump must back up")
}

// fakeSource serves one release tag without touching the network.
type fakeSource struct{ tag string }

func (f fakeSource) Latest(context.Context) (selfupdate.Release, error) {
	return selfupdate.Release{Tag: f.tag}, nil
}

func (f fakeSource) List(context.Context, int) ([]selfupdate.Release, error) {
	return []selfupdate.Release{{Tag: f.tag}}, nil
}

// blockingInstaller announces that Install started, then waits for the
// context it was handed and reports why it stopped.
type blockingInstaller struct{ started chan struct{} }

func (b *blockingInstaller) Install(ctx context.Context, _ string) error {
	close(b.started)
	<-ctx.Done()
	return ctx.Err()
}

// useFakeSelfCmd swaps rootCmd's real self command for one driving u, so the
// test exercises arc's own command tree without reaching GitHub. It silences
// rootCmd's streams and restores every change when the test ends.
func useFakeSelfCmd(t *testing.T, u *selfupdate.Updater) {
	t.Helper()
	fake := cobracmd.New(u, cobracmd.WithCheckShorthand("c"))
	rootCmd.RemoveCommand(selfCmd)
	rootCmd.AddCommand(fake)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	t.Cleanup(func() {
		rootCmd.RemoveCommand(fake)
		rootCmd.AddCommand(selfCmd)
		rootCmd.SetArgs(nil)
		// A nil writer restores cobra's os.Stdout and os.Stderr defaults.
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})
}

// TestSelfUpdateHonorsCancelledRootContext covers the reason main runs the
// root command with ExecuteContext: cobra must hand the installer a context
// whose Done channel actually closes.
func TestSelfUpdateHonorsCancelledRootContext(t *testing.T) {
	inst := &blockingInstaller{started: make(chan struct{})}
	useFakeSelfCmd(t, &selfupdate.Updater{
		Name:      cliName,
		Version:   "v0.1.0",
		Source:    fakeSource{tag: "v99.0.0"},
		Installer: inst,
	})
	rootCmd.SetArgs([]string{"self", "update", "-y"})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() {
		<-inst.started
		cancel()
	}()

	err := awaitCommand(t, func() error { return rootCmd.ExecuteContext(ctx) })
	require.ErrorIs(t, err, context.Canceled)
}
