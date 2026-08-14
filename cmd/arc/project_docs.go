// Per-project docs-destination commands for the arc CLI.
// These commands let a project override where design-spec plan files live
// (docs.path) and what format they're written in (docs.type), independent
// of the global plans.dir/plans.type config.
package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	cfgpkg "github.com/sentiolabs/arc/internal/config"
)

var (
	docsSetPath string
	docsSetType string
)

// projectDocsCmd is the parent command for per-project docs destination management.
var projectDocsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Manage the project's docs destination (plan files)",
}

// projectDocsSetCmd persists a per-project docs.path and/or docs.type override.
var projectDocsSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set the per-project docs path and/or type",
	RunE: func(cmd *cobra.Command, args []string) error {
		if docsSetPath == "" && docsSetType == "" {
			return errors.New("provide --path and/or --type")
		}
		if docsSetType != "" && !cfgpkg.ValidDocsType(docsSetType) {
			return fmt.Errorf("invalid --type %q (want %q or %q)",
				docsSetType, cfgpkg.DocsTypeMarkdown, cfgpkg.DocsTypeObsidian)
		}
		if docsSetPath != "" && strings.Contains(docsSetPath, "..") {
			return errors.New("--path: must not contain '..'")
		}
		projID, _, _, err := resolveProject()
		if err != nil {
			return err
		}
		c, err := getClient()
		if err != nil {
			return err
		}
		if docsSetPath != "" {
			if err := c.SetProjectConfig(projID, cfgpkg.ProjectDocsPathKey, docsSetPath); err != nil {
				return err
			}
		}
		if docsSetType != "" {
			if err := c.SetProjectConfig(projID, cfgpkg.ProjectDocsTypeKey, docsSetType); err != nil {
				return err
			}
		}
		return runProjectDocsGet(cmd, args) // echo the resolved result back
	},
}

// projectDocsGetCmd prints the fully-resolved docs path, type, and source layer.
var projectDocsGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show the resolved docs path, type, and source layer",
	RunE:  runProjectDocsGet,
}

// projectDocsUnsetCmd clears the per-project docs.path/docs.type overrides.
var projectDocsUnsetCmd = &cobra.Command{
	Use:   "unset",
	Short: "Clear the per-project docs override",
	RunE: func(cmd *cobra.Command, args []string) error {
		projID, _, _, err := resolveProject()
		if err != nil {
			return err
		}
		c, err := getClient()
		if err != nil {
			return err
		}
		if err := c.DeleteProjectConfig(projID, cfgpkg.ProjectDocsPathKey); err != nil {
			return err
		}
		if err := c.DeleteProjectConfig(projID, cfgpkg.ProjectDocsTypeKey); err != nil {
			return err
		}
		return runProjectDocsGet(cmd, args)
	},
}

// resolveDocsForProject resolves the effective docs destination for a project.
// Client failures degrade to nil rows so resolution still returns config/default layers.
func resolveDocsForProject(projID, projName, prefix string) (path, dtype, source string, err error) {
	cfg, err := loadConfig()
	if err != nil {
		return "", "", "", err
	}
	var rows map[string]string
	if c, cerr := getClient(); cerr == nil {
		rows, _ = c.GetProjectConfig(projID)
	}
	cwd, _ := os.Getwd()
	vars := map[string]string{"project": cfgpkg.SanitizeSlug(projName), "prefix": cfgpkg.SanitizeSlug(prefix)}
	return cfgpkg.ResolveDocs(rows, cfg, vars, cwd)
}

// runProjectDocsGet resolves and prints the current project's docs destination.
func runProjectDocsGet(cmd *cobra.Command, args []string) error {
	projID, _, _, err := resolveProject()
	if err != nil {
		return err
	}
	var projName, prefix string
	if c, cerr := getClient(); cerr == nil {
		if p, perr := c.GetProject(projID); perr == nil {
			projName, prefix = p.Name, p.Prefix
		}
	}
	path, dtype, source, err := resolveDocsForProject(projID, projName, prefix)
	if err != nil {
		return err
	}
	if outputJSON {
		outputResult(map[string]string{"docs_path": path, "docs_type": dtype, "docs_source": source})
		return nil
	}
	fmt.Printf("Path:   %s\n", path)
	fmt.Printf("Type:   %s\n", dtype)
	fmt.Printf("Source: %s\n", source)
	return nil
}

// init wires the docs subcommand's flags and registers it under `arc project`.
func init() {
	projectDocsSetCmd.Flags().StringVar(&docsSetPath, "path", "",
		"docs directory (absolute, ~, or repo-relative; {project}/{prefix} templates allowed)")
	// --type is validated against cfgpkg.ValidDocsType before being persisted.
	projectDocsSetCmd.Flags().StringVar(&docsSetType, "type", "", "docs type: markdown or obsidian")
	projectDocsCmd.AddCommand(projectDocsSetCmd, projectDocsGetCmd, projectDocsUnsetCmd)
	projectCmd.AddCommand(projectDocsCmd)
}
