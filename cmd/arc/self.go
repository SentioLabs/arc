// Package main provides the self-management commands for the arc CLI,
// including self-update functionality to fetch the latest version from GitHub.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"

	"github.com/sentiolabs/arc/internal/version"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

// selfForce forces update even if already up-to-date.
var selfForce bool

// selfCheck enables check-only mode without installing.
var selfCheck bool

// selfYes skips confirmation prompts.
var selfYes bool

var selfCmd = &cobra.Command{
	Use:   "self",
	Short: "Manage the arc CLI itself",
}

var selfUpdateCmd = &cobra.Command{
	Use:          "update",
	Short:        "Update arc to the latest version",
	SilenceUsage: true,
	Long: `Update arc to the latest version from GitHub releases.

The update channel determines which releases are considered:
  stable   Official releases (default)
  rc       Release candidates
  nightly  Daily builds from main branch

Use 'arc self channel' to view or switch the update channel.

Examples:
  arc self update          Update if a new version is available
  arc self update --check  Check for updates without installing
  arc self update --force  Force reinstall even if up-to-date`,
	RunE: runSelfUpdate,
}

var selfChannelCmd = &cobra.Command{
	Use:   "channel [stable|rc|nightly]",
	Short: "View or switch the update channel",
	Long: `View or switch the update channel for arc self update.

Channels:
  stable   Official releases (default)
  rc       Release candidates
  nightly  Daily builds from main branch

Examples:
  arc self channel              Show current channel
  arc self channel nightly      Switch to nightly (prompts for confirmation)
  arc self channel nightly -y   Switch without prompting`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSelfChannel,
}

func init() {
	selfCmd.AddCommand(selfUpdateCmd)
	selfCmd.AddCommand(selfChannelCmd)

	selfUpdateCmd.Flags().BoolVarP(&selfForce, "force", "f", false, "Force update even if already up-to-date")
	selfUpdateCmd.Flags().BoolVarP(&selfCheck, "check", "c", false, "Check for updates without installing")
	selfChannelCmd.Flags().BoolVarP(&selfYes, "yes", "y", false, "Skip confirmation prompt")
}

// githubReleasesURL is the base URL for GitHub releases API (var for testing).
var githubReleasesURL = "https://api.github.com/repos/sentiolabs/arc/releases"

// githubRelease represents a GitHub release.
type githubRelease struct {
	TagName    string `json:"tag_name"`
	Prerelease bool   `json:"prerelease"`
}

// channelTagPattern maps channel names to their tag matching patterns.
var channelTagPattern = map[string]*regexp.Regexp{
	"rc":      regexp.MustCompile(`^v\d+\.\d+\.\d+-rc\.?\d+$`),
	"nightly": regexp.MustCompile(`^v\d+\.\d+\.\d+-nightly\.\d{8}$`),
}

func runSelfUpdate(cmd *cobra.Command, args []string) error {
	current := ensureVPrefix(version.Short())

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	channel := cfg.Channel
	if channel == "" {
		channel = "stable"
	}

	latest, err := resolveChannelVersion(channel)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	cmp := semver.Compare(latest, current)

	// Check-only mode
	if selfCheck {
		switch {
		case cmp == 0:
			_, _ = fmt.Printf("arc %s (%s) is the latest version\n", current, channel)
		case cmp > 0:
			_, _ = fmt.Printf("arc %s installed (%s channel)\n", current, channel)
			_, _ = fmt.Printf("arc %s available\n", latest)
			_, _ = fmt.Println("\nRun 'arc self update' to upgrade")
		default:
			_, _ = fmt.Printf("arc %s installed (newer than latest %s release %s)\n", current, channel, latest)
		}
		return nil
	}

	// Compare versions
	if cmp == 0 && !selfForce {
		fmt.Printf("arc %s (%s) is already the latest version\n", current, channel)
		return nil
	}

	if cmp < 0 && !selfForce {
		fmt.Printf("arc %s is newer than latest %s release %s\n", current, channel, latest)
		return nil
	}

	// Run the install script
	fmt.Printf("Updating arc %s → %s...\n", current, latest)
	return runInstallScript(latest)
}

// resolveChannelVersion finds the latest release tag for the given channel.
func resolveChannelVersion(channel string) (string, error) {
	if channel == "" || channel == "stable" {
		return getLatestVersion()
	}

	pattern, ok := channelTagPattern[channel]
	if !ok {
		return "", fmt.Errorf("unknown channel: %s", channel)
	}

	resp, err := http.Get(githubReleasesURL + "?per_page=20")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var releases []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return "", err
	}

	for _, r := range releases {
		if pattern.MatchString(r.TagName) {
			return r.TagName, nil
		}
	}

	return "", fmt.Errorf("no %s release found", channel)
}

// getLatestVersion fetches just the version tag from GitHub.
func getLatestVersion() (string, error) {
	resp, err := http.Get(githubReleasesURL + "/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	return release.TagName, nil
}

// runInstallScript downloads and runs the install script.
func runInstallScript(tag string) error {
	scriptArgs := "--force"
	if tag != "" {
		scriptArgs += " --tag=" + tag
	}
	script := fmt.Sprintf(
		"curl -fsSL https://raw.githubusercontent.com/sentiolabs/arc/main/scripts/install.sh | bash -s -- %s",
		scriptArgs,
	)

	cmd := exec.Command("bash", "-c", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// ensureVPrefix adds a "v" prefix if not already present.
func ensureVPrefix(v string) string {
	if v == "" || v == "dev" {
		return "v0.0.0-dev"
	}
	if v[0] != 'v' {
		return "v" + v
	}
	return v
}

func runSelfChannel(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Show current channel if no args
	if len(args) == 0 {
		channel := cfg.Channel
		if channel == "" {
			channel = "stable"
		}
		fmt.Printf("Current update channel: %s\n", channel)
		return nil
	}

	newChannel := args[0]

	// Warn and confirm for non-stable channels
	if newChannel != "stable" && !selfYes {
		var warning string
		switch newChannel {
		case "rc":
			warning = "Release candidates may contain bugs that haven't been fully tested."
		case "nightly":
			warning = "Nightly builds are built from the latest main branch and may be unstable."
		}
		_, _ = fmt.Fprintf(os.Stderr, "\n⚠  %s\n\n", warning)
		_, _ = fmt.Fprintf(os.Stderr, "Switch to %s channel? [y/N] ", newChannel)

		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	return setSelfChannel(cfg, newChannel)
}

// setSelfChannel validates and persists the update channel.
func setSelfChannel(cfg *Config, channel string) error {
	switch channel {
	case "stable", "rc", "nightly":
		// valid
	default:
		return fmt.Errorf("invalid channel %q: must be stable, rc, or nightly", channel)
	}

	cfg.Channel = channel
	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	_, _ = fmt.Printf("\n✓ Switched to %s channel\n", channel)
	if channel != "stable" {
		_, _ = fmt.Printf("  Run 'arc self update' to get the latest %s build\n", channel)
	}
	return nil
}
