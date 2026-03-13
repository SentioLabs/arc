package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/project"
	"github.com/sentiolabs/arc/internal/templates"
	"github.com/sentiolabs/arc/internal/types"
	"github.com/spf13/cobra"
)

// filePermissions is the permission mode for project config files that need to be
// readable by other tools (AGENTS.md, CLAUDE.md).
const filePermissions = 0o644

var initCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Initialize arc in the current directory",
	Long: `Initialize arc in the current directory by creating a project.

This command:
1. Creates a project on the server (or connects to existing)
2. Saves project config to ~/.arc/projects/
3. Creates AGENTS.md with session completion instructions

For Claude Code users: Install the arc plugin for full integration
(hooks, skills, agents). The plugin's onboard skill will handle
project initialization automatically.

For Codex CLI users: Run arc setup codex to install the repo-scoped
arc skill bundle under .codex/skills.

Examples:
  arc init                    # Use directory name as project
  arc init my-project         # Use custom name
  arc init --prefix cxsh      # Custom issue prefix (e.g., cxsh-0b7w)`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringP("description", "d", "", "Project description")
	initCmd.Flags().StringP("prefix", "p", "", "Custom issue prefix (alphanumeric, max 10 chars)")
	initCmd.Flags().BoolP("quiet", "q", false, "Suppress output")
	rootCmd.AddCommand(initCmd)
}

//nolint:revive,gocognit // cognitive-complexity: init orchestrates multiple setup steps
func runInit(cmd *cobra.Command, args []string) error {
	quiet, _ := cmd.Flags().GetBool("quiet")
	description, _ := cmd.Flags().GetString("description")

	// Get current working directory, resolving symlinks for consistent path storage
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}
	cwd = project.NormalizePath(cwd)

	// Determine project name
	var name string
	if len(args) > 0 {
		// User provided explicit name
		name = args[0]
	} else {
		// Auto-generate: sanitized-basename-hash
		name, err = project.GenerateName(cwd)
		if err != nil {
			return fmt.Errorf("generate project name: %w", err)
		}
	}

	// Generate prefix with hash for guaranteed uniqueness
	customPrefix, _ := cmd.Flags().GetString("prefix")

	var prefix string
	if customPrefix != "" {
		prefix, err = project.GeneratePrefixWithCustomName(cwd, customPrefix)
		if err != nil {
			return fmt.Errorf("generate prefix: %w", err)
		}
	} else {
		prefix, err = project.GeneratePrefix(cwd)
		if err != nil {
			return fmt.Errorf("generate prefix: %w", err)
		}
	}

	// Create project on server
	c, err := getClient()
	if err != nil {
		return fmt.Errorf("connect to server: %w", err)
	}

	// Check if project already exists
	projects, err := c.ListProjects()
	if err != nil {
		return fmt.Errorf("list projects: %w", err)
	}

	var proj *types.Project

	// Look for existing project by name
	for _, existing := range projects {
		if existing.Name == name {
			proj = existing
			if !quiet {
				fmt.Printf("Using existing project: %s (%s)\n", proj.Name, proj.ID)
			}
			break
		}
	}

	// Also check server-side path resolution
	if proj == nil {
		proj = resolveExistingProject(c, cwd, quiet)
	}

	// Create new project if not found
	if proj == nil {
		proj, err = c.CreateProject(name, prefix, description)
		if err != nil {
			return fmt.Errorf("create project: %w", err)
		}
		if !quiet {
			fmt.Printf("Created project: %s (%s)\n", proj.Name, proj.ID)
		}
	}

	// Register the current directory as a workspace path
	hostname, _ := os.Hostname()
	absPath, resolvedPath := project.NormalizePathPair(cwd)
	if regErr := registerPathPair(c, proj.ID, absPath, resolvedPath, hostname); regErr != nil {
		if !quiet {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to register workspace path: %v\n", regErr)
		}
	}

	// Add "landing the plane" instructions to AGENTS.md
	if err := addLandingThePlaneInstructions(!quiet); err != nil {
		if !quiet {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to update AGENTS.md: %v\n", err)
		}
	}

	// Update CLAUDE.md to reference AGENTS.md for session completion (if it exists)
	if err := updateClaudeMdReference(!quiet); err != nil {
		if !quiet {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to update CLAUDE.md: %v\n", err)
		}
	}

	if !quiet {
		fmt.Printf("\n✓ arc initialized successfully!\n\n")
		fmt.Printf("  Project: %s\n", proj.Name)
		fmt.Printf("  ID: %s\n", proj.ID)
		fmt.Printf("  Prefix: %s\n", proj.Prefix)
		fmt.Printf("  Issues will be named: %s.<hash> (e.g., %s.a3f2dd)\n\n",
			proj.Prefix, proj.Prefix)
		fmt.Printf("Run %s to get started.\n", "arc quickstart")
	}

	return nil
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
		if err := os.WriteFile(filename, []byte(agentsMdContent), filePermissions); err != nil {
			return fmt.Errorf("failed to create %s: %w", filename, err)
		}
		if verbose {
			_, _ = fmt.Printf("✓ Created %s with landing-the-plane instructions\n", filename)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to read %s: %w", filename, err)
	}

	// File exists - check if it already has landing the plane section
	if strings.Contains(string(content), "Landing the Plane") {
		if verbose {
			_, _ = fmt.Printf("  %s already has landing-the-plane instructions\n", filename)
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

	//nolint:gosec // filename is a hardcoded constant ("AGENTS.md")
	if err := os.WriteFile(filename, []byte(newContent), filePermissions); err != nil {
		return fmt.Errorf("failed to update %s: %w", filename, err)
	}
	if verbose {
		_, _ = fmt.Printf("✓ Added landing-the-plane instructions to %s\n", filename)
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
var sessionCompletionPattern = regexp.MustCompile(
	`(?s)## Session Completion.*?` + "```" + `(?:bash)?\s*\ngit .*?` + "```",
)

// updateClaudeMdReference updates CLAUDE.md to reference AGENTS.md for session completion.
// If CLAUDE.md doesn't exist, creates a minimal one with just the reference.
// If it exists with a duplicated session completion section, replaces it with a reference.
// If it exists without any session completion section, appends the reference.
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
		if err := os.WriteFile(filename, []byte(reference), filePermissions); err != nil {
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

	// Check if there's a "Session Completion" section with git commands (indicating duplication)
	var newContent string
	if sessionCompletionPattern.MatchString(contentStr) {
		// Replace duplicated section with reference
		newContent = replaceSectionUntilNextHeader(contentStr, "## Session Completion", reference)
		if newContent == contentStr {
			// No changes made
			return nil
		}
	} else {
		// No session completion section - append the reference
		newContent = contentStr
		if !strings.HasSuffix(newContent, "\n") {
			newContent += "\n"
		}
		newContent += "\n" + reference
	}

	//nolint:gosec // filename is a hardcoded constant ("CLAUDE.md")
	if err := os.WriteFile(filename, []byte(newContent), filePermissions); err != nil {
		return fmt.Errorf("failed to update %s: %w", filename, err)
	}
	if verbose {
		_, _ = fmt.Printf("✓ Updated %s to reference AGENTS.md for session completion\n", filename)
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

// resolveExistingProject checks server-side path resolution for an existing project.
func resolveExistingProject(c *client.Client, cwd string, quiet bool) *types.Project {
	res, resolveErr := c.ResolveProjectByPath(cwd)
	if resolveErr != nil || res.ProjectID == "" {
		return nil
	}
	existing, getErr := c.GetProject(res.ProjectID)
	if getErr != nil {
		return nil
	}
	if !quiet {
		fmt.Printf("Using existing project: %s (%s)\n", existing.Name, existing.ID)
	}
	return existing
}
