package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Initialize arc in the current directory",
	Long: `Initialize arc in the current directory by creating a workspace and
setting up Claude Code integration.

This command:
1. Creates a workspace on the server (or connects to existing)
2. Sets the workspace as default for this directory
3. Creates AGENTS.md with landing-the-plane instructions
4. Optionally sets up Claude Code hooks

Examples:
  arc init                    # Use directory name as workspace
  arc init my-project         # Use custom name
  arc init --prefix mp        # Custom issue prefix
  arc init --setup-claude     # Also install Claude hooks`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringP("prefix", "p", "", "Issue ID prefix (default: workspace name)")
	initCmd.Flags().StringP("description", "d", "", "Workspace description")
	initCmd.Flags().Bool("setup-claude", false, "Also install Claude Code hooks")
	initCmd.Flags().BoolP("quiet", "q", false, "Suppress output")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	quiet, _ := cmd.Flags().GetBool("quiet")
	setupClaude, _ := cmd.Flags().GetBool("setup-claude")
	prefix, _ := cmd.Flags().GetString("prefix")
	description, _ := cmd.Flags().GetString("description")

	// Determine workspace name
	var name string
	if len(args) > 0 {
		name = args[0]
	} else {
		// Use directory name
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get current directory: %w", err)
		}
		name = filepath.Base(cwd)
	}

	// Determine prefix
	if prefix == "" {
		prefix = name
		// Normalize: lowercase, no spaces
		prefix = strings.ToLower(strings.ReplaceAll(prefix, " ", "-"))
		// Truncate if too long
		if len(prefix) > 10 {
			prefix = prefix[:10]
		}
	}

	// Get path for the workspace
	cwd, _ := os.Getwd()

	// Create workspace on server
	c, err := getClient()
	if err != nil {
		return fmt.Errorf("connect to server: %w", err)
	}

	// Check if workspace already exists
	workspaces, err := c.ListWorkspaces()
	if err != nil {
		return fmt.Errorf("list workspaces: %w", err)
	}

	var ws *struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Path        string `json:"path"`
		Description string `json:"description"`
		Prefix      string `json:"prefix"`
	}

	// Look for existing workspace by path or name
	for _, existing := range workspaces {
		if existing.Path == cwd || existing.Name == name {
			ws = &struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				Path        string `json:"path"`
				Description string `json:"description"`
				Prefix      string `json:"prefix"`
			}{
				ID:          existing.ID,
				Name:        existing.Name,
				Path:        existing.Path,
				Description: existing.Description,
				Prefix:      existing.Prefix,
			}
			if !quiet {
				fmt.Printf("Using existing workspace: %s (%s)\n", ws.Name, ws.ID)
			}
			break
		}
	}

	// Create new workspace if not found
	if ws == nil {
		newWs, err := c.CreateWorkspace(name, prefix, cwd, description)
		if err != nil {
			return fmt.Errorf("create workspace: %w", err)
		}
		ws = &struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Path        string `json:"path"`
			Description string `json:"description"`
			Prefix      string `json:"prefix"`
		}{
			ID:          newWs.ID,
			Name:        newWs.Name,
			Path:        newWs.Path,
			Description: newWs.Description,
			Prefix:      newWs.Prefix,
		}
		if !quiet {
			fmt.Printf("Created workspace: %s (%s)\n", ws.Name, ws.ID)
		}
	}

	// Save as default workspace
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	cfg.DefaultWorkspace = ws.ID
	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	// Create project-local config
	if err := createProjectConfig(cwd, ws.ID, ws.Name); err != nil {
		if !quiet {
			fmt.Fprintf(os.Stderr, "Warning: failed to create .arc.json: %v\n", err)
		}
	}

	// Add "landing the plane" instructions to AGENTS.md
	if err := addLandingThePlaneInstructions(!quiet); err != nil {
		if !quiet {
			fmt.Fprintf(os.Stderr, "Warning: failed to update AGENTS.md: %v\n", err)
		}
	}

	// Setup Claude hooks if requested
	if setupClaude {
		if err := installClaudeHooks(!quiet); err != nil {
			if !quiet {
				fmt.Fprintf(os.Stderr, "Warning: failed to setup Claude hooks: %v\n", err)
			}
		}
	}

	if !quiet {
		fmt.Printf("\n✓ arc initialized successfully!\n\n")
		fmt.Printf("  Workspace: %s\n", ws.Name)
		fmt.Printf("  ID: %s\n", ws.ID)
		fmt.Printf("  Prefix: %s\n", ws.Prefix)
		fmt.Printf("  Issues will be named: %s-<hash> (e.g., %s-a3f2dd)\n\n", ws.Prefix, ws.Prefix)
		fmt.Printf("Run %s to get started.\n\n", "arc quickstart")
		if !setupClaude {
			fmt.Printf("Tip: Run %s to add Claude Code hooks.\n", "arc setup claude")
		}
	}

	return nil
}

// createProjectConfig creates a .arc.json file in the project root
func createProjectConfig(dir, workspaceID, workspaceName string) error {
	configPath := filepath.Join(dir, ".arc.json")

	// Don't overwrite existing config
	if _, err := os.Stat(configPath); err == nil {
		return nil
	}

	config := map[string]string{
		"workspace_id":   workspaceID,
		"workspace_name": workspaceName,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// landingThePlaneSection is the "landing the plane" instructions for AI agents
const landingThePlaneSection = `
## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **Commit and push**:
   ` + "```bash" + `
   git add .
   git commit -m "description of changes"
   git push
   git status  # MUST show "up to date with origin"
   ` + "```" + `
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until ` + "`git push`" + ` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
`

// addLandingThePlaneInstructions adds "landing the plane" instructions to AGENTS.md
func addLandingThePlaneInstructions(verbose bool) error {
	filename := "AGENTS.md"

	content, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		// Create new file with basic structure
		newContent := fmt.Sprintf(`# Agent Instructions

This project uses **arc** for issue tracking. Run `+"`arc onboard`"+` to get started.

## Quick Reference

`+"```bash"+`
arc ready              # Find available work
arc show <id>          # View issue details
arc update <id> --status in_progress  # Claim work
arc close <id>         # Complete work
`+"```"+`
%s
`, landingThePlaneSection)

		if err := os.WriteFile(filename, []byte(newContent), 0644); err != nil {
			return fmt.Errorf("failed to create %s: %w", filename, err)
		}
		if verbose {
			fmt.Printf("✓ Created %s with landing-the-plane instructions\n", filename)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to read %s: %w", filename, err)
	}

	// File exists - check if it already has landing the plane section
	if strings.Contains(string(content), "Landing the Plane") {
		if verbose {
			fmt.Printf("  %s already has landing-the-plane instructions\n", filename)
		}
		return nil
	}

	// Append the landing the plane section
	newContent := string(content)
	if !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}
	newContent += landingThePlaneSection

	if err := os.WriteFile(filename, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to update %s: %w", filename, err)
	}
	if verbose {
		fmt.Printf("✓ Added landing-the-plane instructions to %s\n", filename)
	}
	return nil
}
