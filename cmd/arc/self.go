// Package main provides the self-management commands for the arc CLI,
// including self-update functionality to fetch the latest version from GitHub.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/sentiolabs/arc/internal/version"
	"github.com/spf13/cobra"
)

// selfForce forces update even if already up-to-date.
var selfForce bool

// selfCheck enables check-only mode without installing.
var selfCheck bool

var selfCmd = &cobra.Command{
	Use:   "self",
	Short: "Manage the arc CLI itself",
}

var selfUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update arc to the latest version",
	Long: `Update arc to the latest version from GitHub releases.

Examples:
  arc self update          Update if a new version is available
  arc self update --check  Check for updates without installing
  arc self update --force  Force reinstall even if up-to-date`,
	RunE: runSelfUpdate,
}

func init() {
	selfCmd.AddCommand(selfUpdateCmd)

	selfUpdateCmd.Flags().BoolVarP(&selfForce, "force", "f", false, "Force update even if already up-to-date")
	selfUpdateCmd.Flags().BoolVarP(&selfCheck, "check", "c", false, "Check for updates without installing")
}

func runSelfUpdate(cmd *cobra.Command, args []string) error {
	current := version.Short()

	// Fetch latest release version
	latest, err := getLatestVersion()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	// Normalize versions for comparison
	currentNorm := normalizeVersion(current)
	latestNorm := normalizeVersion(latest)

	// Check-only mode
	if selfCheck {
		switch {
		case currentNorm == latestNorm:
			_, _ = fmt.Printf("arc %s is the latest version\n", current)
		case isNewer(latestNorm, currentNorm):
			_, _ = fmt.Printf("arc %s installed\n", current)
			_, _ = fmt.Printf("arc %s available\n", latest)
			_, _ = fmt.Println("\nRun 'arc self update' to upgrade")
		default:
			fmt.Printf("arc %s installed (newer than latest release %s)\n", current, latest)
		}
		return nil
	}

	// Compare versions
	if currentNorm == latestNorm && !selfForce {
		fmt.Printf("arc %s is already the latest version\n", current)
		return nil
	}

	if !isNewer(latestNorm, currentNorm) && !selfForce {
		fmt.Printf("arc %s is newer than latest release %s\n", current, latest)
		return nil
	}

	// Run the install script with --force
	fmt.Printf("Updating arc %s → %s...\n", current, latest)
	return runInstallScript()
}

// getLatestVersion fetches just the version tag from GitHub
func getLatestVersion() (string, error) {
	url := "https://api.github.com/repos/sentiolabs/arc/releases/latest"

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	return release.TagName, nil
}

// runInstallScript downloads and runs the install script
func runInstallScript() error {
	// Use curl to fetch and bash to run the install script
	script := "curl -fsSL https://raw.githubusercontent.com/sentiolabs/arc/main/scripts/install.sh | bash -s -- --force"

	cmd := exec.Command("bash", "-c", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// normalizeVersion removes 'v' prefix and handles dev builds
func normalizeVersion(v string) string {
	v = strings.TrimPrefix(v, "v")
	if v == "dev" || v == "" {
		return "0.0.0"
	}
	return v
}

// semverPartCount is the number of parts in a semantic version string (major.minor.patch).
const semverPartCount = 3

// isNewer returns true if a is newer than b (semver comparison).
func isNewer(a, b string) bool {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	for i := range semverPartCount {
		var aNum, bNum int
		if i < len(aParts) {
			aNum, _ = strconv.Atoi(aParts[i])
		}
		if i < len(bParts) {
			bNum, _ = strconv.Atoi(bParts[i])
		}
		if aNum > bNum {
			return true
		}
		if aNum < bNum {
			return false
		}
	}
	return false
}
