// Package main provides the self-management commands for the arc CLI,
// delegating update and channel logic to github.com/sentiolabs/go-selfupdate.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sentiolabs/arc/internal/project"
	"github.com/sentiolabs/arc/internal/storage/sqlite"
	"github.com/sentiolabs/arc/internal/version"
	"github.com/sentiolabs/go-selfupdate"
	"github.com/sentiolabs/go-selfupdate/cobracmd"
	"golang.org/x/mod/semver"
)

// cliName is the binary name, which is also arc's GitHub repository name.
const cliName = "arc"

// installScriptURL is arc's install script, run with --force --tag=<tag>.
const installScriptURL = "https://raw.githubusercontent.com/sentiolabs/arc/main/scripts/install.sh"

// selfCmd is registered by main.go. It is the go-selfupdate command tree with
// arc's wiring: GitHub releases of sentiolabs/arc, the channel from
// ~/.arc/config.toml, and a database backup before a major or minor bump.
var selfCmd = cobracmd.New(newSelfUpdater(), cobracmd.WithCheckShorthand("c"))

// newSelfUpdater builds arc's updater.
func newSelfUpdater() *selfupdate.Updater {
	return &selfupdate.Updater{
		Name:       cliName,
		Version:    version.Short(),
		Source:     &selfupdate.GitHubSource{Owner: "sentiolabs", Repo: cliName},
		Store:      arcChannelStore(),
		Installer:  &selfupdate.ScriptInstaller{ScriptURL: installScriptURL},
		PreInstall: preInstallBackup(backupArcDatabase),
	}
}

// arcChannelStore persists the channel in cfg.Updates.Channel. An empty
// value reads as stable.
func arcChannelStore() selfupdate.Store {
	return selfupdate.FuncStore{
		Get: func() (selfupdate.Channel, error) {
			cfg, err := loadConfig()
			if err != nil {
				return "", err
			}
			if cfg.Updates.Channel == "" {
				return selfupdate.ChannelStable, nil
			}
			return selfupdate.Channel(cfg.Updates.Channel), nil
		},
		Set: func(c selfupdate.Channel) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			cfg.Updates.Channel = string(c)
			if err := saveConfig(cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}
			return nil
		},
	}
}

// preInstallBackup returns a PreInstall hook that calls backup when the
// update crosses a major or minor version boundary. Backup failures are
// reported on stderr and do not abort the update, matching prior behavior.
func preInstallBackup(backup func() error) func(context.Context, string, string) error {
	return func(_ context.Context, current, latest string) error {
		if semver.MajorMinor(latest) == semver.MajorMinor(current) {
			return nil
		}
		if err := backup(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: pre-update backup failed: %v\n", err)
		}
		return nil
	}
}

// backupArcDatabase backs up ~/.arc/data.db and prints the result.
func backupArcDatabase() error {
	dbPath := filepath.Join(project.DefaultArcHome(), "data.db")
	result, err := sqlite.BackupDatabase(dbPath)
	if err != nil {
		return err
	}
	if result != nil {
		_, _ = fmt.Printf("Pre-update backup: %s (%s -> %s)\n",
			result.Path, formatSize(result.OriginalSize), formatSize(result.BackupSize))
	}
	return nil
}
