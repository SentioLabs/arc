package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sentiolabs/arc/internal/version"
	"github.com/spf13/cobra"
)

var (
	selfForce bool
	selfCheck bool
)

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

// GitHubRelease represents a GitHub release API response
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func runSelfUpdate(cmd *cobra.Command, args []string) error {
	current := version.Short()

	// Fetch latest release
	latest, downloadURL, err := getLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	// Normalize versions for comparison
	currentNorm := normalizeVersion(current)
	latestNorm := normalizeVersion(latest)

	// Check-only mode
	if selfCheck {
		if currentNorm == latestNorm {
			fmt.Printf("arc %s is the latest version\n", current)
		} else if isNewer(latestNorm, currentNorm) {
			fmt.Printf("arc %s installed\n", current)
			fmt.Printf("arc %s available\n", latest)
			fmt.Println("\nRun 'arc self update' to upgrade")
		} else {
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

	if downloadURL == "" {
		return fmt.Errorf("no download available for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	fmt.Printf("Updating arc %s → %s...\n", current, latest)

	// Download to temp file
	tmpDir, err := os.MkdirTemp("", "arc-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, "arc.tar.gz")
	fmt.Printf("Downloading %s...\n", filepath.Base(downloadURL))

	if err := downloadFile(archivePath, downloadURL); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// Extract binary
	binaryPath := filepath.Join(tmpDir, "arc")
	if err := extractBinary(archivePath, binaryPath); err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	// Get current binary path
	currentBinary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to find current binary: %w", err)
	}
	currentBinary, err = filepath.EvalSymlinks(currentBinary)
	if err != nil {
		return fmt.Errorf("failed to resolve binary path: %w", err)
	}

	// Check if server was running
	serverWasRunning := checkServerRunning()

	// Stop server before replacing binary
	if serverWasRunning {
		fmt.Println("Stopping arc server...")
		selfStopServer()
	}

	// Replace binary
	fmt.Println("Installing new version...")
	if err := replaceBinary(binaryPath, currentBinary); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	// Re-sign on macOS
	if runtime.GOOS == "darwin" {
		resignBinary(currentBinary)
	}

	// Restart server if it was running
	if serverWasRunning {
		fmt.Println("Restarting arc server...")
		selfStartServer()
	}

	fmt.Printf("Successfully updated to arc %s\n", latest)
	return nil
}

// getLatestRelease fetches the latest release info from GitHub
func getLatestRelease() (version string, downloadURL string, err error) {
	url := "https://api.github.com/repos/sentiolabs/arc/releases/latest"

	resp, err := http.Get(url)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", "", err
	}

	// Find asset for current platform
	platform := fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH)
	assetName := fmt.Sprintf("arc_%s_%s.tar.gz", strings.TrimPrefix(release.TagName, "v"), platform)

	for _, asset := range release.Assets {
		if asset.Name == assetName {
			return release.TagName, asset.BrowserDownloadURL, nil
		}
	}

	return release.TagName, "", nil
}

// normalizeVersion removes 'v' prefix and handles dev builds
func normalizeVersion(v string) string {
	v = strings.TrimPrefix(v, "v")
	if v == "dev" || v == "" {
		return "0.0.0"
	}
	return v
}

// isNewer returns true if a is newer than b (semver comparison)
func isNewer(a, b string) bool {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	for i := 0; i < 3; i++ {
		var aNum, bNum int
		if i < len(aParts) {
			fmt.Sscanf(aParts[i], "%d", &aNum)
		}
		if i < len(bParts) {
			fmt.Sscanf(bParts[i], "%d", &bNum)
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

// downloadFile downloads a URL to a local file
func downloadFile(filepath string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// extractBinary extracts the arc binary from a tar.gz archive
func extractBinary(archivePath, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Look for the arc binary (handles both "arc" and "./arc")
		name := strings.TrimPrefix(header.Name, "./")
		if name == "arc" && header.Typeflag == tar.TypeReg {
			out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return err
			}
			defer out.Close()

			_, err = io.Copy(out, tr)
			return err
		}
	}

	return fmt.Errorf("arc binary not found in archive")
}

// replaceBinary replaces the current binary with the new one
func replaceBinary(newPath, currentPath string) error {
	// Check if we have write permission
	dir := filepath.Dir(currentPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Try direct copy first
	newBinary, err := os.ReadFile(newPath)
	if err != nil {
		return err
	}

	err = os.WriteFile(currentPath, newBinary, 0755)
	if err == nil {
		return nil
	}

	// If direct write fails (e.g., permission denied), try with sudo
	if os.IsPermission(err) {
		fmt.Println("Elevated permissions required...")
		cmd := exec.Command("sudo", "cp", newPath, currentPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	return err
}

// checkServerRunning checks if the arc server is running (wraps isServerRunning from server.go)
func checkServerRunning() bool {
	_, running := isServerRunning()
	return running
}

// selfStopServer stops the arc server by calling runServerStop directly
func selfStopServer() {
	// runServerStop doesn't use cmd/args, so nil is safe
	runServerStop(nil, nil)
}

// selfStartServer starts the arc server with default settings
// Uses subprocess because runServerStart needs cmd.Flags() for port/db
func selfStartServer() {
	cmd := exec.Command(os.Args[0], "server", "start")
	cmd.Start() // Don't wait - it daemonizes itself
}

// resignBinary re-signs a binary on macOS to avoid Gatekeeper delays
func resignBinary(path string) {
	// Remove existing signature
	exec.Command("codesign", "--remove-signature", path).Run()
	// Ad-hoc sign
	exec.Command("codesign", "--force", "--sign", "-", path).Run()
}
