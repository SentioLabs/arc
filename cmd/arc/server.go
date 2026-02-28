package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/sentiolabs/arc/internal/api"
	"github.com/sentiolabs/arc/internal/server"
	"github.com/spf13/cobra"
)

// Server-related constants.
const (
	// defaultServerPort is the default port for the arc server.
	defaultServerPort = 7432

	// defaultLogLines is the default number of log lines to display.
	defaultLogLines = 50

	// serverPollInterval is the interval between process status checks during shutdown.
	serverPollInterval = 100 * time.Millisecond

	// serverRestartDelay is the time to wait between stop and start during a restart.
	serverRestartDelay = 500 * time.Millisecond

	// hoursPerDay is the number of hours in a day, used for duration formatting.
	hoursPerDay = 24

	// logFilePerm is the permission for log files.
	logFilePerm = 0o644

	// serverDirPerm is the permission for server data directories.
	serverDirPerm = 0o755

	// gracefulShutdownChecks is the number of checks (at serverPollInterval) before force kill.
	gracefulShutdownChecks = 100

	// healthCheckTimeout is how long to wait for the server to pass a health check after starting.
	healthCheckTimeout = 10 * time.Second
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage the arc server",
	Long:  `Start, stop, and manage the arc server daemon.`,
}

func init() {
	serverCmd.AddCommand(serverStartCmd)
	serverCmd.AddCommand(serverStopCmd)
	serverCmd.AddCommand(serverStatusCmd)
	serverCmd.AddCommand(serverLogsCmd)
	serverCmd.AddCommand(serverRestartCmd)
}

// ============ Server Start ============

var serverStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the arc server",
	Long: `Start the arc server as a background daemon.

Use --foreground to run in the foreground (blocking).
The server stores data in ~/.arc/ by default.`,
	RunE: runServerStart,
}

func init() {
	serverStartCmd.Flags().BoolP("foreground", "f", false, "Run in foreground (don't daemonize)")
	serverStartCmd.Flags().IntP("port", "p", defaultServerPort, "Server port")
	serverStartCmd.Flags().String("db", "", "Database path (default: ~/.arc/data.db)")
}

func runServerStart(cmd *cobra.Command, args []string) error {
	foreground, _ := cmd.Flags().GetBool("foreground")
	port, _ := cmd.Flags().GetInt("port")
	dbPath, _ := cmd.Flags().GetString("db")

	addr := fmt.Sprintf(":%d", port)

	if foreground {
		// Run server directly (blocking)
		return server.Run(server.Config{
			Address: addr,
			DBPath:  dbPath,
		})
	}

	// Check if already running
	if pid, running := isServerRunning(); running {
		return fmt.Errorf("server already running (PID %d)", pid)
	}

	// Ensure data directory exists
	if err := os.MkdirAll(server.DefaultDataDir(), serverDirPerm); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	// Fork: re-exec with --foreground flag
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	cmdArgs := []string{"server", "start", "--foreground", "--port", strconv.Itoa(port)}
	if dbPath != "" {
		cmdArgs = append(cmdArgs, "--db", dbPath)
	}

	daemonCmd := exec.Command(execPath, cmdArgs...)
	daemonCmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	// Redirect output to log file
	logFile, err := os.OpenFile(server.LogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, logFilePerm)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	daemonCmd.Stdout = logFile
	daemonCmd.Stderr = logFile

	if err := daemonCmd.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("start daemon: %w", err)
	}

	// Write PID file
	pidPath := server.PIDPath()
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(daemonCmd.Process.Pid)), logFilePerm); err != nil {
		// Try to kill the child if we can't write PID
		_ = daemonCmd.Process.Kill()
		_ = logFile.Close()
		return fmt.Errorf("write PID file: %w", err)
	}

	// Don't wait for the child - it's now a daemon
	_ = logFile.Close()

	// Wait for health check to confirm startup
	if err := waitForHealth(port, healthCheckTimeout); err != nil {
		// Cleanup on failure
		_ = os.Remove(pidPath)
		return fmt.Errorf("server failed to start: %w", err)
	}

	_, _ = fmt.Printf("Server started (PID %d)\n", daemonCmd.Process.Pid)
	_, _ = fmt.Printf("  WebUI:   http://localhost:%d\n", port)
	_, _ = fmt.Printf("  Logs:    %s\n", server.LogPath())
	return nil
}

// ============ Server Stop ============

var serverStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the arc server",
	RunE:  runServerStop,
}

func runServerStop(cmd *cobra.Command, args []string) error {
	pid, running := isServerRunning()
	if !running {
		_, _ = fmt.Println("Server is not running")
		return nil
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process: %w", err)
	}

	// Send SIGTERM for graceful shutdown
	if err := process.Signal(syscall.SIGTERM); err != nil {
		// Process might already be dead - clean up PID file
		_ = os.Remove(server.PIDPath())
		_, _ = fmt.Println("Server stopped (process already exited)")
		return nil //nolint:nilerr // SIGTERM failure means process already exited
	}

	// Wait up to 10 seconds for graceful shutdown
	for range gracefulShutdownChecks {
		if !isProcessRunning(pid) {
			_ = os.Remove(server.PIDPath())
			_, _ = fmt.Println("Server stopped")
			return nil
		}
		time.Sleep(serverPollInterval)
	}

	// Force kill if still running
	_ = process.Signal(syscall.SIGKILL)
	time.Sleep(serverPollInterval)
	_ = os.Remove(server.PIDPath())
	_, _ = fmt.Println("Server killed (forced)")
	return nil
}

// ============ Server Status ============

var serverStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show server status",
	RunE:  runServerStatus,
}

func runServerStatus(cmd *cobra.Command, args []string) error {
	pid, running := isServerRunning()

	if !running {
		if outputJSON {
			outputResult(map[string]any{
				"running": false,
			})
		} else {
			_, _ = fmt.Println("Server is not running")
		}
		return nil
	}

	// Try to get health info
	port := defaultServerPort
	health, err := getHealth(port)
	if err != nil {
		if outputJSON {
			outputResult(map[string]any{
				"running":    true,
				"pid":        pid,
				"responding": false,
			})
		} else {
			fmt.Printf("Server running (PID %d) but not responding\n", pid)
		}
		return nil
	}

	if outputJSON {
		outputResult(map[string]any{
			"running":    true,
			"pid":        pid,
			"responding": true,
			"status":     health.Status,
			"port":       health.Port,
			"webui_url":  health.WebUIURL,
			"version":    health.Version,
			"uptime":     health.Uptime,
		})
	} else {
		fmt.Printf("Server running (PID %d)\n", pid)
		fmt.Printf("  Status:  %s\n", health.Status)
		fmt.Printf("  Port:    %d\n", health.Port)
		fmt.Printf("  WebUI:   %s\n", health.WebUIURL)
		fmt.Printf("  Version: %s\n", health.Version)
		fmt.Printf("  Uptime:  %s\n", formatDuration(time.Duration(health.Uptime)*time.Second))
	}
	return nil
}

// ============ Server Logs ============

var serverLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View server logs",
	RunE:  runServerLogs,
}

func init() {
	serverLogsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
	serverLogsCmd.Flags().IntP("lines", "n", defaultLogLines, "Number of lines to show")
}

func runServerLogs(cmd *cobra.Command, args []string) error {
	follow, _ := cmd.Flags().GetBool("follow")
	lines, _ := cmd.Flags().GetInt("lines")

	logPath := server.LogPath()

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		fmt.Println("No log file found")
		return nil
	}

	if follow {
		return tailFollow(logPath, lines)
	}

	return tailLines(logPath, lines)
}

// ============ Server Restart ============

var serverRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the arc server",
	RunE:  runServerRestart,
}

func init() {
	serverRestartCmd.Flags().IntP("port", "p", defaultServerPort, "Server port")
	serverRestartCmd.Flags().String("db", "", "Database path")
}

func runServerRestart(cmd *cobra.Command, args []string) error {
	// Stop if running
	if pid, running := isServerRunning(); running {
		fmt.Printf("Stopping server (PID %d)...\n", pid)
		if err := runServerStop(cmd, args); err != nil {
			return err
		}
		// Give it a moment
		time.Sleep(serverRestartDelay)
	}

	// Start
	return runServerStart(cmd, args)
}

// ============ Helper Functions ============

func readPIDFile() (int, error) {
	data, err := os.ReadFile(server.PIDPath())
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(data))
}

func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds. Send signal 0 to check if alive.
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func isServerRunning() (int, bool) {
	pid, err := readPIDFile()
	if err != nil {
		return 0, false
	}
	return pid, isProcessRunning(pid)
}

func waitForHealth(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	healthURL := fmt.Sprintf("http://localhost:%d/health", port)

	for time.Now().Before(deadline) {
		//nolint:gosec // URL is constructed from localhost with a numeric port, not user input
		resp, err := http.Get(healthURL)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(serverPollInterval)
	}
	return errors.New("timeout waiting for server to become healthy")
}

func getHealth(port int) (*api.HealthResponse, error) {
	healthURL := fmt.Sprintf("http://localhost:%d/health", port)
	//nolint:gosec // URL is constructed from localhost with a numeric port, not user input
	resp, err := http.Get(healthURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var health api.HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil, err
	}
	return &health, nil
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	}
	if d < hoursPerDay*time.Hour {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
	return fmt.Sprintf("%.1fd", d.Hours()/hoursPerDay)
}

// tailLines reads the last n lines from a file
func tailLines(path string, n int) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Read all lines (simple approach for reasonable log files)
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	start := 0
	if len(lines) > n {
		start = len(lines) - n
	}

	for _, line := range lines[start:] {
		fmt.Println(line)
	}
	return scanner.Err()
}

// tailFollow follows a log file (like tail -f)
func tailFollow(path string, initialLines int) error {
	// Print initial lines
	if err := tailLines(path, initialLines); err != nil {
		return err
	}

	// Now follow
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Seek to end
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("seek to end of log: %w", err)
	}

	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				time.Sleep(serverPollInterval)
				continue
			}
			return err
		}
		_, _ = fmt.Print(line)
	}
}
