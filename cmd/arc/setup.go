// Package main provides the setup commands for integrating arc with AI
// editors such as Claude Code and Codex CLI. The setup subcommands install,
// check, and remove lifecycle hooks and repo-scoped skill bundles so that
// the AI assistant receives arc workflow context automatically.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sentiolabs/arc/internal/templates"
	"github.com/spf13/cobra"
)

var (
	setupProject bool
	setupCheck   bool
	setupRemove  bool
)

// setupCmd is the parent command for editor integration recipes.
var setupCmd = &cobra.Command{
	Use:          "setup <recipe>",
	Short:        "Setup integration with AI editors (claude, codex)",
	SilenceUsage: true,
	Long: `Setup integration files for AI editors and coding assistants.

Currently supports:
  claude    - Claude Code hooks (SessionStart, PreCompact)
  codex     - Codex CLI repo-scoped skills (.codex/skills)

Examples:
  arc setup claude          # Install Claude Code hooks globally
  arc setup claude --project  # Install for this project only
  arc setup claude --check    # Verify installation
  arc setup claude --remove   # Uninstall hooks
  arc setup codex           # Install Codex repo skill bundle
  arc setup codex --check   # Verify installation
  arc setup codex --remove  # Uninstall repo skill bundle`,
	Args: validateSetupArgs,
	RunE: runSetup,
}

// init registers setup flags and adds the command to root.
func init() {
	setupCmd.Flags().BoolVar(&setupCheck, "check", false, "Check if integration is installed")
	setupCmd.Flags().BoolVar(&setupRemove, "remove", false, "Remove the integration")
	setupCmd.Flags().BoolVar(&setupProject, "project", false, "Install for this project only")
	rootCmd.AddCommand(setupCmd)
}

// runSetup dispatches to the appropriate recipe handler.
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
	case "codex":
		if setupCheck {
			return checkCodexSkill()
		}
		if setupRemove {
			return removeCodexSkill()
		}
		return installCodexSkill()
	default:
		return fmt.Errorf("unknown recipe: %s (supported: claude, codex)", recipe)
	}
}

// validateSetupArgs ensures exactly one recipe argument is provided.
func validateSetupArgs(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return errors.New(`missing recipe

Supported recipes:
  claude  - Claude Code hooks (SessionStart, PreCompact)
  codex   - Codex CLI repo-scoped skills (.codex/skills)

Examples:
  arc setup claude --check
  arc setup codex --check`)
	}
	return nil
}

// installClaudeHooks installs Claude Code hooks for arc prime.
//
//nolint:revive // cognitive-complexity: hook installation has inherent branching
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
	if err := os.MkdirAll(filepath.Dir(settingsPath), defaultDirPerm); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Load existing settings
	settings := make(map[string]any)
	if data, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parse settings: %w", err)
		}
	}

	// Get or create hooks section
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		hooks = make(map[string]any)
		settings["hooks"] = hooks
	}

	// Clean up any null values
	for key, val := range hooks {
		if val == nil {
			delete(hooks, key)
		}
	}

	// arc prime generates workflow context for AI assistants
	command := "arc prime"

	// Add hooks
	if addHookCommand(hooks, "SessionStart", command, verbose) {
		if verbose {
			_, _ = fmt.Println("✓ Registered SessionStart hook")
		}
	}
	if addHookCommand(hooks, "PreCompact", command, verbose) {
		if verbose {
			_, _ = fmt.Println("✓ Registered PreCompact hook")
		}
	}

	// Write settings
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, data, defaultFilePerm); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}

	if verbose {
		_, _ = fmt.Printf("\n✓ Claude Code integration installed\n  Settings: %s\n", settingsPath)
		_, _ = fmt.Println("\nRestart Claude Code for changes to take effect.")
	}

	return nil
}

// checkClaudeHooks checks if Claude hooks are installed.
// It looks in both the global (~/.claude/settings.json) and project-level
// (.claude/settings.local.json) settings files for arc prime hooks.
func checkClaudeHooks() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	cwd, _ := os.Getwd()
	globalSettings := filepath.Join(home, ".claude", "settings.json")
	projectSettings := filepath.Join(cwd, ".claude", "settings.local.json")

	if hasArcHooks(globalSettings) {
		fmt.Printf("✓ Global hooks installed: %s\n", globalSettings)
		return nil
	}
	if hasArcHooks(projectSettings) {
		fmt.Printf("✓ Project hooks installed: %s\n", projectSettings)
		return nil
	}

	fmt.Println("✗ No hooks installed")
	fmt.Println("  Run: arc setup claude")
	return errors.New("hooks not installed")
}

// removeClaudeHooks removes Claude Code hooks from either the global or
// project-level settings file. It strips "arc prime" from SessionStart and
// PreCompact events and persists the updated settings back to disk.
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
		if os.IsNotExist(err) {
			fmt.Println("No settings file found")
			return nil
		}
		return fmt.Errorf("read settings: %w", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("parse settings: %w", err)
	}

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		fmt.Println("No hooks found")
		return nil
	}

	removeHookCommand(hooks, "SessionStart", "arc prime")
	removeHookCommand(hooks, "PreCompact", "arc prime")

	data, err = json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, data, defaultFilePerm); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}

	fmt.Println("✓ Claude hooks removed")
	return nil
}

// addHookCommand adds a hook command to an event if not already present.
// It walks the nested hooks structure in settings.json to check for duplicates.
// Returns true if the hook was added, false if it was already registered.
func addHookCommand(hooks map[string]any, event, command string, verbose bool) bool {
	eventHooks, ok := hooks[event].([]any)
	if !ok {
		eventHooks = []any{}
	}

	// Check if hook already registered
	for _, hook := range eventHooks {
		hookMap, ok := hook.(map[string]any)
		if !ok {
			continue
		}
		commands, ok := hookMap["hooks"].([]any)
		if !ok {
			continue
		}
		for _, cmd := range commands {
			cmdMap, ok := cmd.(map[string]any)
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

	// Add new hook entry with matcher and command configuration
	newHook := map[string]any{
		"matcher": "",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": command,
			},
		},
	}

	eventHooks = append(eventHooks, newHook)
	hooks[event] = eventHooks
	return true
}

// removeHookCommand removes a hook command from an event.
// It filters out hooks whose "command" field matches the given command
// and deletes the event key entirely when no hooks remain.
func removeHookCommand(hooks map[string]any, event, command string) {
	eventHooks, ok := hooks[event].([]any)
	if !ok {
		return
	}

	filtered := make([]any, 0, len(eventHooks))
	for _, hook := range eventHooks {
		hookMap, ok := hook.(map[string]any)
		if !ok {
			filtered = append(filtered, hook)
			continue
		}

		commands, ok := hookMap["hooks"].([]any)
		if !ok {
			filtered = append(filtered, hook)
			continue
		}

		keepHook := true
		for _, cmd := range commands {
			cmdMap, ok := cmd.(map[string]any)
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

	// Remove empty event keys to keep settings clean
	if len(filtered) == 0 {
		delete(hooks, event)
	} else {
		hooks[event] = filtered
	}
}

// hasArcHooks checks if a settings file has arc prime hooks.
// It reads and parses the JSON settings file, then searches the hooks
// map for SessionStart or PreCompact events that contain an "arc prime" command.
//
//nolint:revive // cognitive-complexity: nested JSON structure traversal
func hasArcHooks(settingsPath string) bool {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return false
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return false
	}

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return false
	}

	for _, event := range []string{"SessionStart", "PreCompact"} {
		eventHooks, ok := hooks[event].([]any)
		if !ok {
			continue
		}

		for _, hook := range eventHooks {
			hookMap, ok := hook.(map[string]any)
			if !ok {
				continue
			}
			commands, ok := hookMap["hooks"].([]any)
			if !ok {
				continue
			}
			for _, cmd := range commands {
				cmdMap, ok := cmd.(map[string]any)
				if !ok {
					continue
				}
				if cmdMap["command"] == "arc prime" {
					return true
				}
			}
		}
	}

	return false
}

// installCodexSkill writes the arc skill bundle into .codex/skills/arc.
func installCodexSkill() error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	dstDir := filepath.Join(repoRoot, ".codex", "skills", "arc")
	if err := os.MkdirAll(dstDir, defaultDirPerm); err != nil {
		return fmt.Errorf("create codex skill directory: %w", err)
	}

	if err := writeCodexSkillFiles(repoRoot, dstDir); err != nil {
		return err
	}

	fmt.Println("✓ Codex skill bundle installed")
	fmt.Printf("  Location: %s\n", dstDir)
	fmt.Println("  Restart Codex to pick up repo-scoped skills.")
	return nil
}

// writeCodexSkillFiles renders or copies skill.toml and SKILL.md into dstDir.
// It first tries to copy from a local codex-plugin source directory, and
// falls back to rendering from embedded Go templates if the source is unavailable.
func writeCodexSkillFiles(repoRoot, dstDir string) error {
	srcDir := filepath.Join(repoRoot, "codex-plugin", "skills", "arc")
	if exists(srcDir) {
		if err := copyCodexSkillFromSource(srcDir, dstDir); err == nil {
			return nil
		}
	}

	skillToml, err := templates.RenderCodexSkillToml()
	if err != nil {
		return fmt.Errorf("render codex skill.toml: %w", err)
	}
	skillMd, err := templates.RenderCodexSkillMd()
	if err != nil {
		return fmt.Errorf("render codex SKILL.md: %w", err)
	}

	if err := os.WriteFile(filepath.Join(dstDir, "skill.toml"), []byte(skillToml), defaultFilePerm); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Join(dstDir, "skill.toml"), err)
	}
	if err := os.WriteFile(filepath.Join(dstDir, "SKILL.md"), []byte(skillMd), defaultFilePerm); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Join(dstDir, "SKILL.md"), err)
	}

	return nil
}

// copyCodexSkillFromSource copies skill files from a local source directory.
// It iterates over the known skill file names and copies each one.
func copyCodexSkillFromSource(srcDir, dstDir string) error {
	for _, name := range []string{"skill.toml", "SKILL.md"} {
		srcPath := filepath.Join(srcDir, name)
		dstPath := filepath.Join(dstDir, name)
		if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}

// checkCodexSkill verifies that the Codex skill bundle is installed.
// It checks for the presence of skill.toml and SKILL.md in the expected
// .codex/skills/arc directory under the repository root.
func checkCodexSkill() error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	skillDir := filepath.Join(repoRoot, ".codex", "skills", "arc")
	missing := false
	for _, name := range []string{"skill.toml", "SKILL.md"} {
		if _, err := os.Stat(filepath.Join(skillDir, name)); err != nil {
			missing = true
			break
		}
	}

	if missing {
		fmt.Println("✗ Codex skill bundle not installed")
		fmt.Println("  Run: arc setup codex")
		return errors.New("codex skill bundle not installed")
	}

	fmt.Printf("✓ Codex skill bundle installed: %s\n", skillDir)
	return nil
}

// removeCodexSkill deletes the Codex skill bundle directory.
// It removes the entire .codex/skills/arc directory tree.
func removeCodexSkill() error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	skillDir := filepath.Join(repoRoot, ".codex", "skills", "arc")
	if err := os.RemoveAll(skillDir); err != nil {
		return fmt.Errorf("remove codex skill bundle: %w", err)
	}

	fmt.Println("✓ Codex skill bundle removed")
	return nil
}

// findRepoRoot walks up from cwd looking for .codex, .arc.json, or .git markers.
// It returns the first ancestor directory that contains any of these marker files,
// or falls back to cwd if the filesystem root is reached without finding one.
func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get current directory: %w", err)
	}

	// Walk upward from cwd to filesystem root looking for project markers
	dir := cwd
	for {
		hasCodex := exists(filepath.Join(dir, ".codex"))
		hasArc := exists(filepath.Join(dir, ".arc.json"))
		hasGit := exists(filepath.Join(dir, ".git"))
		if hasCodex || hasArc || hasGit {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return cwd, nil
		}
		dir = parent
	}
}

// exists returns true if the given path exists on disk.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// copyFile reads srcPath and writes its contents to dstPath.
// It uses filePermissions (0o644) since copied skill files need
// to be world-readable by other tools.
func copyFile(srcPath, dstPath string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", srcPath, err)
	}
	if err := os.WriteFile(dstPath, data, filePermissions); err != nil {
		return fmt.Errorf("write %s: %w", dstPath, err)
	}
	return nil
}
