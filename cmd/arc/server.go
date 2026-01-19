package main

import (
	"bufio"
	"encoding/json"
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
	serverStartCmd.Flags().IntP("port", "p", 7432, "Server port")
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
	if err := os.MkdirAll(server.DefaultDataDir(), 0o755); err != nil {
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
	logFile, err := os.OpenFile(server.LogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	daemonCmd.Stdout = logFile
	daemonCmd.Stderr = logFile

	if err := daemonCmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start daemon: %w", err)
	}

	// Write PID file
	pidPath := server.PIDPath()
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(daemonCmd.Process.Pid)), 0o644); err != nil {
		// Try to kill the child if we can't write PID
		daemonCmd.Process.Kill()
		logFile.Close()
		return fmt.Errorf("write PID file: %w", err)
	}

	// Don't wait for the child - it's now a daemon
	logFile.Close()

	// Wait for health check to confirm startup
	if err := waitForHealth(port, 10*time.Second); err != nil {
		// Cleanup on failure
		os.Remove(pidPath)
		return fmt.Errorf("server failed to start: %w", err)
	}

	fmt.Printf("Server started (PID %d)\n", daemonCmd.Process.Pid)
	fmt.Printf("  Address: http://localhost:%d\n", port)
	fmt.Printf("  Logs:    %s\n", server.LogPath())
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
		fmt.Println("Server is not running")
		return nil
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process: %w", err)
	}

	// Send SIGTERM for graceful shutdown
	if err := process.Signal(syscall.SIGTERM); err != nil {
		// Process might already be dead
		os.Remove(server.PIDPath())
		fmt.Println("Server stopped")
		return nil
	}

	// Wait up to 10 seconds for graceful shutdown
	for i := 0; i < 100; i++ {
		if !isProcessRunning(pid) {
			os.Remove(server.PIDPath())
			fmt.Println("Server stopped")
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Force kill if still running
	process.Signal(syscall.SIGKILL)
	time.Sleep(100 * time.Millisecond)
	os.Remove(server.PIDPath())
	fmt.Println("Server killed (forced)")
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
			outputResult(map[string]interface{}{
				"running": false,
			})
		} else {
			fmt.Println("Server is not running")
		}
		return nil
	}

	// Try to get health info
	port := 7432 // Default port
	health, err := getHealth(port)

	if err != nil {
		if outputJSON {
			outputResult(map[string]interface{}{
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
		outputResult(map[string]interface{}{
			"running":    true,
			"pid":        pid,
			"responding": true,
			"status":     health.Status,
			"version":    health.Version,
			"uptime":     health.Uptime,
		})
	} else {
		fmt.Printf("Server running (PID %d)\n", pid)
		fmt.Printf("  Status:  %s\n", health.Status)
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
	serverLogsCmd.Flags().IntP("lines", "n", 50, "Number of lines to show")
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
	serverRestartCmd.Flags().IntP("port", "p", 7432, "Server port")
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
		time.Sleep(500 * time.Millisecond)
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
	url := fmt.Sprintf("http://localhost:%d/health", port)

	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for server to become healthy")
}

func getHealth(port int) (*api.HealthResponse, error) {
	url := fmt.Sprintf("http://localhost:%d/health", port)
	resp, err := http.Get(url)
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
	if d < 24*time.Hour {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
	return fmt.Sprintf("%.1fd", d.Hours()/24)
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
	file.Seek(0, io.SeekEnd)

	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return err
		}
		fmt.Print(line)
	}
}
