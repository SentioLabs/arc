package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/sentiolabs/selfupdate-go"
)

const updateRecoveryTimeout = 30 * time.Second

// updateLifecycle keeps daemon downtime inside the verified installation phase.
// The callbacks also let tests exercise failure recovery without a real daemon.
type updateLifecycle struct {
	installer selfupdate.Installer
	resolve   func() error
	running   func() bool
	stop      func(context.Context) error
	start     func(context.Context) error
	backup    func(context.Context, string, string) error
	restart   bool
}

// newUpdateLifecycle wires the archive installer to the local daemon.
func newUpdateLifecycle() *updateLifecycle {
	archive := &selfupdate.ArchiveInstaller{Name: cliName}
	lifecycle := &updateLifecycle{
		installer: archive,
		running: func() bool {
			_, running := isServerRunning()
			return running
		},
		backup: preInstallBackup(backupArcDatabase),
	}
	lifecycle.resolve = func() error {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("locate executable: %w", err)
		}
		archive.TargetPath, err = filepath.EvalSymlinks(exe)
		if err != nil {
			return fmt.Errorf("resolve executable: %w", err)
		}
		// Capture the selected config after Cobra has parsed global flags.
		args := []string{}
		if configPath != "" {
			args = append(args, "--config", configPath)
		}
		run := func(ctx context.Context, action string) error {
			//nolint:gosec // Execute only our own canonical binary with fixed server actions.
			cmd := exec.CommandContext(ctx, archive.TargetPath, append(args, "server", action)...)
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
			return cmd.Run()
		}
		lifecycle.stop = func(ctx context.Context) error { return run(ctx, "stop") }
		lifecycle.start = func(ctx context.Context) error { return run(ctx, "start") }
		return nil
	}
	return lifecycle
}

// Sweep preserves upstream cleanup even when no newer release is available.
func (l *updateLifecycle) Sweep() error {
	if sweeper, ok := l.installer.(selfupdate.Sweeper); ok {
		return sweeper.Sweep()
	}
	return nil
}

// Prepare resolves the executable before it can become an unlinked inode.
// No daemon operation runs until download and verification have succeeded.
func (l *updateLifecycle) Prepare(ctx context.Context, rel selfupdate.Release) (selfupdate.Staged, error) {
	l.restart = false
	if l.resolve != nil {
		if err := l.resolve(); err != nil {
			return nil, err
		}
	}
	staged, err := l.installer.Prepare(ctx, rel)
	if err != nil {
		return nil, err
	}
	return &lifecycleStaged{Staged: staged, lifecycle: l}, nil
}

// preInstall preserves backup policy and verifies that stopping actually worked.
func (l *updateLifecycle) preInstall(ctx context.Context, current, latest string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := l.backup(ctx, current, latest); err != nil {
		return err
	}
	if !l.running() {
		return nil
	}
	// Arm recovery before stopping: cancellation can arrive after the daemon exits.
	l.restart = true
	if err := l.stop(ctx); err != nil {
		return fmt.Errorf("stop server: %w", err)
	}
	if l.running() {
		return errors.New("server is still running after stop; update aborted")
	}
	return ctx.Err()
}

func (l *updateLifecycle) postInstall(_ context.Context, _, _ string) error {
	// Once replaced, restore service even if the update context was cancelled.
	return l.recoverServer()
}

// restartServer makes a single restoration attempt, avoiding duplicate starts.
func (l *updateLifecycle) restartServer(ctx context.Context) error {
	if !l.restart {
		return nil
	}
	l.restart = false
	if l.running() {
		return nil
	}
	if err := l.start(ctx); err != nil {
		return fmt.Errorf("restart server (run 'arc server start' to retry): %w", err)
	}
	return nil
}

// recoverServer bounds service restoration independently of Ctrl-C.
func (l *updateLifecycle) recoverServer() error {
	ctx, cancel := context.WithTimeout(context.Background(), updateRecoveryTimeout)
	defer cancel()
	return l.restartServer(ctx)
}

// lifecycleStaged restores a stopped daemon even when PreInstall aborts.
// Upstream defers Close immediately after successful preparation.
type lifecycleStaged struct {
	selfupdate.Staged
	lifecycle *updateLifecycle
}

// Commit returns both replacement and recovery errors when both fail.
func (s *lifecycleStaged) Commit(ctx context.Context) error {
	if err := s.Staged.Commit(ctx); err != nil {
		return errors.Join(err, s.lifecycle.recoverServer())
	}
	return nil
}

func (s *lifecycleStaged) Close() error {
	// Upstream ignores Close errors, so recovery failure must also be visible.
	err := s.lifecycle.recoverServer()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: update recovery failed: %v\n", err)
	}
	return errors.Join(err, s.Staged.Close())
}
