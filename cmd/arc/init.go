package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sentiolabs/arc/internal/templates"
	"github.com/sentiolabs/arc/internal/workspace"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Initialize arc in the current directory",
	Long: `Initialize arc in the current directory by creating a workspace.

This command:
1. Creates a workspace on the server (or connects to existing)
2. Sets the workspace as default for this directory
3. Creates .arc.json with workspace configuration
4. Creates AGENTS.md with session completion instructions

For Claude Code users: Install the arc plugin for full integration
(hooks, skills, agents). The plugin's onboard skill will handle
workspace initialization automatically.

Examples:
  arc init                    # Use directory name as workspace
  arc init my-project         # Use custom name`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringP("description", "d", "", "Workspace description")
	initCmd.Flags().BoolP("quiet", "q", false, "Suppress output")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	quiet, _ := cmd.Flags().GetBool("quiet")
	description, _ := cmd.Flags().GetString("description")

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	// Determine workspace name
	var name string
	if len(args) > 0 {
		// User provided explicit name
		name = args[0]
	} else {
		// Auto-generate: sanitized-basename-hash
		name, err = workspace.GenerateName(cwd)
		if err != nil {
			return fmt.Errorf("generate workspace name: %w", err)
		}
	}

	// Generate prefix with hash for guaranteed uniqueness
	prefix, err := workspace.GeneratePrefix(cwd)
	if err != nil {
		return fmt.Errorf("generate prefix: %w", err)
	}

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

	// Update CLAUDE.md to reference AGENTS.md for session completion (if it exists)
	if err := updateClaudeMdReference(!quiet); err != nil {
		if !quiet {
			fmt.Fprintf(os.Stderr, "Warning: failed to update CLAUDE.md: %v\n", err)
		}
	}

	if !quiet {
		fmt.Printf("\n✓ arc initialized successfully!\n\n")
		fmt.Printf("  Workspace: %s\n", ws.Name)
		fmt.Printf("  ID: %s\n", ws.ID)
		fmt.Printf("  Prefix: %s\n", ws.Prefix)
		fmt.Printf("  Issues will be named: %s-<hash> (e.g., %s-a3f2dd)\n\n", ws.Prefix, ws.Prefix)
		fmt.Printf("Run %s to get started.\n", "arc quickstart")
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

// addLandingThePlaneInstructions adds "landing the plane" instructions to AGENTS.md
func addLandingThePlaneInstructions(verbose bool) error {
	filename := "AGENTS.md"

	// Get the full AGENTS.md content from template
	agentsMdContent, err := templates.RenderAgentsMd()
	if err != nil {
		return fmt.Errorf("failed to render AGENTS.md template: %w", err)
	}

	content, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		// Create new file from template
		if err := os.WriteFile(filename, []byte(agentsMdContent), 0644); err != nil {
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

	// Extract just the landing the plane section from template and append it
	landingSection := extractLandingThePlaneSection(agentsMdContent)
	newContent := string(content)
	if !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}
	newContent += landingSection

	if err := os.WriteFile(filename, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to update %s: %w", filename, err)
	}
	if verbose {
		fmt.Printf("✓ Added landing-the-plane instructions to %s\n", filename)
	}
	return nil
}

// extractLandingThePlaneSection extracts the landing the plane section from full AGENTS.md content
func extractLandingThePlaneSection(content string) string {
	idx := strings.Index(content, "## Landing the Plane")
	if idx == -1 {
		return ""
	}
	return "\n" + content[idx:]
}

// sessionCompletionPattern matches a "Session Completion" section with git commands
// This indicates the section is duplicating content that should be in AGENTS.md
var sessionCompletionPattern = regexp.MustCompile(`(?s)## Session Completion.*?` + "```" + `(?:bash)?\s*\ngit .*?` + "```")

// updateClaudeMdReference updates CLAUDE.md to reference AGENTS.md for session completion.
// If CLAUDE.md doesn't exist, creates a minimal one with just the reference.
// If it exists with a duplicated session completion section, replaces it with a reference.
func updateClaudeMdReference(verbose bool) error {
	filename := "CLAUDE.md"

	// Generate the reference text from template
	reference, err := templates.RenderClaudeMdReference(templates.ClaudeMdReferenceData{
		AgentsFile: "AGENTS.md",
	})
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	content, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		// No CLAUDE.md - create minimal one with just the reference
		// Claude Code's /init will fill in the rest
		if err := os.WriteFile(filename, []byte(reference), 0644); err != nil {
			return fmt.Errorf("failed to create %s: %w", filename, err)
		}
		if verbose {
			fmt.Printf("✓ Created %s with session completion reference\n", filename)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to read %s: %w", filename, err)
	}

	contentStr := string(content)

	// Check if it already references AGENTS.md for session completion
	if strings.Contains(contentStr, "AGENTS.md") && strings.Contains(contentStr, "Landing the Plane") {
		if verbose {
			fmt.Printf("  %s already references AGENTS.md for session completion\n", filename)
		}
		return nil
	}

	// Look for a "Session Completion" section with git commands (indicating duplication)
	if !sessionCompletionPattern.MatchString(contentStr) {
		// No duplicated session completion section found
		if verbose {
			fmt.Printf("  %s has no session completion section to update\n", filename)
		}
		return nil
	}

	// Find the section boundaries using string manipulation
	// (Go's regexp doesn't support lookahead)
	newContent := replaceSectionUntilNextHeader(contentStr, "## Session Completion", reference)

	if newContent == contentStr {
		// No changes made
		return nil
	}

	if err := os.WriteFile(filename, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to update %s: %w", filename, err)
	}
	if verbose {
		fmt.Printf("✓ Updated %s to reference AGENTS.md for session completion\n", filename)
	}
	return nil
}

// replaceSectionUntilNextHeader replaces a markdown section (starting with header)
// with replacement text, stopping at the next ## header or end of file
func replaceSectionUntilNextHeader(content, header, replacement string) string {
	start := strings.Index(content, header)
	if start == -1 {
		return content
	}

	// Find the next ## header after the start
	rest := content[start+len(header):]
	nextHeader := strings.Index(rest, "\n## ")

	var end int
	if nextHeader == -1 {
		// No next header - replace until end of file
		end = len(content)
	} else {
		// Found next header - replace until there (including the newline before it)
		end = start + len(header) + nextHeader + 1 // +1 for the newline
	}

	return content[:start] + replacement + content[end:]
}
