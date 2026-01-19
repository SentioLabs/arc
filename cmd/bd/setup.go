package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	setupProject bool
	setupCheck   bool
	setupRemove  bool
)

var setupCmd = &cobra.Command{
	Use:   "setup [recipe]",
	Short: "Setup integration with AI editors",
	Long: `Setup integration files for AI editors and coding assistants.

Currently supports:
  claude    - Claude Code hooks (SessionStart, PreCompact)

Examples:
  bd setup claude          # Install Claude Code hooks globally
  bd setup claude --project  # Install for this project only
  bd setup claude --check    # Verify installation
  bd setup claude --remove   # Uninstall hooks`,
	Args: cobra.ExactArgs(1),
	RunE: runSetup,
}

func init() {
	setupCmd.Flags().BoolVar(&setupCheck, "check", false, "Check if integration is installed")
	setupCmd.Flags().BoolVar(&setupRemove, "remove", false, "Remove the integration")
	setupCmd.Flags().BoolVar(&setupProject, "project", false, "Install for this project only")
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	recipe := args[0]

	switch recipe {
	case "claude":
		if setupCheck {
			return checkClaudeHooks()
		}
		if setupRemove {
			return removeClaudeHooks(setupProject)
		}
		return installClaudeHooks(!setupProject)
	default:
		return fmt.Errorf("unknown recipe: %s (supported: claude)", recipe)
	}
}

// installClaudeHooks installs Claude Code hooks for bd prime
func installClaudeHooks(verbose bool) error {
	var settingsPath string
	if setupProject {
		cwd, _ := os.Getwd()
		settingsPath = filepath.Join(cwd, ".claude", "settings.local.json")
		if verbose {
			fmt.Println("Installing Claude hooks for this project...")
		}
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("get home directory: %w", err)
		}
		settingsPath = filepath.Join(home, ".claude", "settings.json")
		if verbose {
			fmt.Println("Installing Claude hooks globally...")
		}
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Load existing settings
	settings := make(map[string]interface{})
	if data, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parse settings: %w", err)
		}
	}

	// Get or create hooks section
	hooks, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		hooks = make(map[string]interface{})
		settings["hooks"] = hooks
	}

	// Clean up any null values
	for key, val := range hooks {
		if val == nil {
			delete(hooks, key)
		}
	}

	command := "bd prime"

	// Add hooks
	if addHookCommand(hooks, "SessionStart", command, verbose) {
		if verbose {
			fmt.Println("✓ Registered SessionStart hook")
		}
	}
	if addHookCommand(hooks, "PreCompact", command, verbose) {
		if verbose {
			fmt.Println("✓ Registered PreCompact hook")
		}
	}

	// Write settings
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, data, 0o644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}

	if verbose {
		fmt.Println("\n✓ Claude Code integration installed")
		fmt.Printf("  Settings: %s\n", settingsPath)
		fmt.Println("\nRestart Claude Code for changes to take effect.")
	}

	return nil
}

// checkClaudeHooks checks if Claude hooks are installed
func checkClaudeHooks() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	cwd, _ := os.Getwd()
	globalSettings := filepath.Join(home, ".claude", "settings.json")
	projectSettings := filepath.Join(cwd, ".claude", "settings.local.json")

	if hasBeadsHooks(globalSettings) {
		fmt.Printf("✓ Global hooks installed: %s\n", globalSettings)
		return nil
	}
	if hasBeadsHooks(projectSettings) {
		fmt.Printf("✓ Project hooks installed: %s\n", projectSettings)
		return nil
	}

	fmt.Println("✗ No hooks installed")
	fmt.Println("  Run: bd setup claude")
	return fmt.Errorf("hooks not installed")
}

// removeClaudeHooks removes Claude Code hooks
func removeClaudeHooks(project bool) error {
	var settingsPath string
	if project {
		cwd, _ := os.Getwd()
		settingsPath = filepath.Join(cwd, ".claude", "settings.local.json")
		fmt.Println("Removing Claude hooks from project...")
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("get home directory: %w", err)
		}
		settingsPath = filepath.Join(home, ".claude", "settings.json")
		fmt.Println("Removing Claude hooks globally...")
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		fmt.Println("No settings file found")
		return nil
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("parse settings: %w", err)
	}

	hooks, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		fmt.Println("No hooks found")
		return nil
	}

	removeHookCommand(hooks, "SessionStart", "bd prime")
	removeHookCommand(hooks, "PreCompact", "bd prime")

	data, err = json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, data, 0o644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}

	fmt.Println("✓ Claude hooks removed")
	return nil
}

// addHookCommand adds a hook command to an event if not already present
func addHookCommand(hooks map[string]interface{}, event, command string, verbose bool) bool {
	eventHooks, ok := hooks[event].([]interface{})
	if !ok {
		eventHooks = []interface{}{}
	}

	// Check if hook already registered
	for _, hook := range eventHooks {
		hookMap, ok := hook.(map[string]interface{})
		if !ok {
			continue
		}
		commands, ok := hookMap["hooks"].([]interface{})
		if !ok {
			continue
		}
		for _, cmd := range commands {
			cmdMap, ok := cmd.(map[string]interface{})
			if !ok {
				continue
			}
			if cmdMap["command"] == command {
				if verbose {
					fmt.Printf("✓ Hook already registered: %s\n", event)
				}
				return false
			}
		}
	}

	// Add new hook
	newHook := map[string]interface{}{
		"matcher": "",
		"hooks": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": command,
			},
		},
	}

	eventHooks = append(eventHooks, newHook)
	hooks[event] = eventHooks
	return true
}

// removeHookCommand removes a hook command from an event
func removeHookCommand(hooks map[string]interface{}, event, command string) {
	eventHooks, ok := hooks[event].([]interface{})
	if !ok {
		return
	}

	filtered := make([]interface{}, 0, len(eventHooks))
	for _, hook := range eventHooks {
		hookMap, ok := hook.(map[string]interface{})
		if !ok {
			filtered = append(filtered, hook)
			continue
		}

		commands, ok := hookMap["hooks"].([]interface{})
		if !ok {
			filtered = append(filtered, hook)
			continue
		}

		keepHook := true
		for _, cmd := range commands {
			cmdMap, ok := cmd.(map[string]interface{})
			if !ok {
				continue
			}
			if cmdMap["command"] == command {
				keepHook = false
				fmt.Printf("✓ Removed %s hook\n", event)
				break
			}
		}

		if keepHook {
			filtered = append(filtered, hook)
		}
	}

	if len(filtered) == 0 {
		delete(hooks, event)
	} else {
		hooks[event] = filtered
	}
}

// hasBeadsHooks checks if a settings file has bd prime hooks
func hasBeadsHooks(settingsPath string) bool {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return false
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return false
	}

	hooks, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		return false
	}

	for _, event := range []string{"SessionStart", "PreCompact"} {
		eventHooks, ok := hooks[event].([]interface{})
		if !ok {
			continue
		}

		for _, hook := range eventHooks {
			hookMap, ok := hook.(map[string]interface{})
			if !ok {
				continue
			}
			commands, ok := hookMap["hooks"].([]interface{})
			if !ok {
				continue
			}
			for _, cmd := range commands {
				cmdMap, ok := cmd.(map[string]interface{})
				if !ok {
					continue
				}
				if cmdMap["command"] == "bd prime" {
					return true
				}
			}
		}
	}

	return false
}
