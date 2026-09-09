package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/sentiolabs/selfupdate-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stagedFixture struct {
	commit func(context.Context) error
	close  func() error
}

func (s stagedFixture) Commit(ctx context.Context) error { return s.commit(ctx) }
func (s stagedFixture) Close() error                     { return s.close() }

type installerFixture struct {
	prepare func(context.Context, selfupdate.Release) (selfupdate.Staged, error)
	sweep   func() error
}

func (i installerFixture) Prepare(ctx context.Context, rel selfupdate.Release) (selfupdate.Staged, error) {
	return i.prepare(ctx, rel)
}

func (i installerFixture) Sweep() error {
	if i.sweep != nil {
		return i.sweep()
	}
	return nil
}

type lifecycleScenario struct {
	name                                                                                        string
	running, prepareFail, stopFail, staysRunning, commitFail, cancel, cancelCommit, restartFail bool
	want                                                                                        []string
	wantError                                                                                   string
}

func TestUpdateLifecycle(t *testing.T) {
	scenarios := []lifecycleScenario{
		{
			name: "running", running: true,
			want: []string{"sweep", "prepare", "backup", "stop", "commit", "start", "close"},
		},
		{name: "stopped", want: []string{"sweep", "prepare", "backup", "commit", "close"}},
		{
			name: "prepare failure", running: true, prepareFail: true,
			want: []string{"sweep", "prepare"}, wantError: "prepare update",
		},
		{
			name: "stop failure after exit", running: true, stopFail: true,
			want: []string{"sweep", "prepare", "backup", "stop", "start", "close"}, wantError: "stop server",
		},
		{
			name: "still running", running: true, staysRunning: true,
			want: []string{"sweep", "prepare", "backup", "stop", "close"}, wantError: "still running",
		},
		{
			name: "commit failure", running: true, commitFail: true,
			want:      []string{"sweep", "prepare", "backup", "stop", "commit", "start", "close"},
			wantError: "install update",
		},
		{
			name: "cancel after stop", running: true, cancel: true,
			want: []string{"sweep", "prepare", "backup", "stop", "start", "close"}, wantError: "context canceled",
		},
		{
			name: "cancel after commit", running: true, cancelCommit: true,
			want: []string{"sweep", "prepare", "backup", "stop", "commit", "start", "close"},
		},
		{
			name: "restart failure", running: true, restartFail: true,
			want:      []string{"sweep", "prepare", "backup", "stop", "commit", "start", "close"},
			wantError: "binary was updated",
		},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) { runLifecycleScenario(t, scenario) })
	}
}

func runLifecycleScenario(t *testing.T, scenario lifecycleScenario) {
	t.Helper()
	var events []string
	running := scenario.running
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	l := fixtureLifecycle(t, scenario, &running, &events, cancel)
	u := fixtureUpdater(l, fakeSource{tag: "v1.1.0"})
	err := u.Update(ctx, selfupdate.UpdateOptions{Yes: true})
	if scenario.wantError != "" {
		require.ErrorContains(t, err, scenario.wantError)
	} else {
		require.NoError(t, err)
	}
	assert.Equal(t, scenario.want, events)
	assert.Equal(t, scenario.running && !scenario.restartFail, running)
}

func fixtureLifecycle(t *testing.T, scenario lifecycleScenario, running *bool,
	events *[]string, cancel context.CancelFunc,
) *updateLifecycle {
	t.Helper()
	l := &updateLifecycle{
		running: func() bool { return *running },
		backup:  func(context.Context, string, string) error { *events = append(*events, "backup"); return nil },
		stop: func(context.Context) error {
			*events = append(*events, "stop")
			*running = scenario.staysRunning
			if scenario.cancel {
				cancel()
			}
			if scenario.stopFail {
				return errors.New("stop failed")
			}
			return nil
		},
		start: func(ctx context.Context) error {
			*events = append(*events, "start")
			require.NoError(t, ctx.Err(), "recovery must not inherit cancellation")
			if scenario.restartFail {
				return errors.New("start failed")
			}
			*running = true
			return nil
		},
	}
	l.installer = fixtureInstaller(scenario, events, cancel)
	return l
}

func fixtureInstaller(scenario lifecycleScenario, events *[]string, cancel context.CancelFunc) selfupdate.Installer {
	return installerFixture{
		sweep: func() error { *events = append(*events, "sweep"); return nil },
		prepare: func(context.Context, selfupdate.Release) (selfupdate.Staged, error) {
			*events = append(*events, "prepare")
			if scenario.prepareFail {
				return nil, errors.New("download failed")
			}
			return stagedFixture{
				commit: func(context.Context) error {
					*events = append(*events, "commit")
					if scenario.cancelCommit {
						cancel()
					}
					if scenario.commitFail {
						return errors.New("rename failed")
					}
					return nil
				},
				close: func() error { *events = append(*events, "close"); return nil },
			}, nil
		},
	}
}

func fixtureUpdater(l *updateLifecycle, source selfupdate.Source) *selfupdate.Updater {
	return &selfupdate.Updater{
		Name: "arc", Version: "v1.0.0", Source: source, Installer: l,
		PreInstall: l.preInstall, PostInstall: l.postInstall, Out: io.Discard,
	}
}

func TestArchiveUpdateIntegration(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		t.Run(fmt.Sprintf("corrupt=%v", corrupt), func(t *testing.T) { runArchiveScenario(t, corrupt) })
	}
}

func runArchiveScenario(t *testing.T, corrupt bool) {
	t.Helper()
	dir := t.TempDir()
	target, link := filepath.Join(dir, "arc-real"), filepath.Join(dir, "arc")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o600))
	require.NoError(t, os.Symlink(target, link))
	payload := []byte("new executable")
	rel := archiveRelease(t, payload, corrupt)
	var events []string
	running := true
	l := fixtureLifecycle(t, lifecycleScenario{}, &running, &events, func() {})
	l.installer = &selfupdate.ArchiveInstaller{Name: "arc", TargetPath: link, Out: io.Discard}
	u := fixtureUpdater(l, releaseFixture{rel})
	err := u.Update(t.Context(), selfupdate.UpdateOptions{Yes: true})
	actual, readErr := os.ReadFile(target)
	require.NoError(t, readErr)
	if corrupt {
		require.Error(t, err)
		assert.Equal(t, "old", string(actual))
		assert.Empty(t, events)
	} else {
		require.NoError(t, err)
		assert.Equal(t, payload, actual)
		assert.Equal(t, []string{"backup", "stop", "start"}, events)
	}
	assert.True(t, running)
	info, err := os.Lstat(link)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&os.ModeSymlink)
}

func archiveRelease(t *testing.T, payload []byte, corrupt bool) selfupdate.Release {
	t.Helper()
	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gz)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "arc", Mode: 0o755, Size: int64(len(payload))}))
	_, err := tw.Write(payload)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	name := fmt.Sprintf("arc_1.1.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	sum := sha256.Sum256(archive.Bytes())
	if corrupt {
		sum[0] ^= 1
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/checksums.txt" {
			_, _ = fmt.Fprintf(w, "%x  %s\n", sum, name)
		} else {
			_, _ = w.Write(archive.Bytes())
		}
	}))
	t.Cleanup(server.Close)
	return selfupdate.Release{Tag: "v1.1.0", Assets: []selfupdate.Asset{
		{Name: name, URL: server.URL + "/" + name},
		{Name: "checksums.txt", URL: server.URL + "/checksums.txt"},
	}}
}

type releaseFixture struct{ release selfupdate.Release }

func (s releaseFixture) Latest(context.Context) (selfupdate.Release, error) { return s.release, nil }

func (s releaseFixture) List(context.Context, int) ([]selfupdate.Release, error) {
	return []selfupdate.Release{s.release}, nil
}

func TestPreInstallBackupFailureDoesNotAbort(t *testing.T) {
	calls := 0
	hook := preInstallBackup(func() error { calls++; return errors.New("fixture backup failed") })
	require.NoError(t, hook(t.Context(), "v1.0.0", "v1.1.0"))
	assert.Equal(t, 1, calls)
}
