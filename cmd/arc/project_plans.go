// Per-project plans-destination commands for the arc CLI.
// These commands let a project override where design-spec plan files live
// (plans.dir) and what format they're written in (plans.type), independent
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

// projectPlansCmdName is the CLI name of the per-project plans command group.
const projectPlansCmdName = "plans"

var (
	plansSetDir  string
	plansSetType string
)

// projectPlansCmd is the parent command for per-project plans destination management.
var projectPlansCmd = &cobra.Command{
	Use:   projectPlansCmdName,
	Short: "Manage the project's plans destination (plan files)",
}

// projectPlansSetCmd persists a per-project plans.dir and/or plans.type override.
var projectPlansSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set the per-project plans dir and/or type",
	RunE: func(cmd *cobra.Command, args []string) error {
		if plansSetDir == "" && plansSetType == "" {
			return errors.New("provide --dir and/or --type")
		}
		if plansSetType != "" && !cfgpkg.ValidPlansType(plansSetType) {
			return fmt.Errorf("invalid --type %q (want %q or %q)",
				plansSetType, cfgpkg.PlansTypeMarkdown, cfgpkg.PlansTypeObsidian)
		}
		if plansSetDir != "" && strings.Contains(plansSetDir, "..") {
			return errors.New("--dir: must not contain '..'")
		}
		projID, _, _, err := resolveProject()
		if err != nil {
			return err
		}
		c, err := getClient()
		if err != nil {
			return err
		}
		if plansSetDir != "" {
			if err := c.SetProjectConfig(projID, cfgpkg.ProjectPlansDirKey, plansSetDir); err != nil {
				return err
			}
		}
		if plansSetType != "" {
			if err := c.SetProjectConfig(projID, cfgpkg.ProjectPlansTypeKey, plansSetType); err != nil {
				return err
			}
		}
		return runProjectPlansGet(cmd, args) // echo the resolved result back
	},
}

// projectPlansGetCmd prints the fully-resolved plans dir, type, and source layer.
var projectPlansGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show the resolved plans dir, type, and source layer",
	RunE:  runProjectPlansGet,
}

// projectPlansUnsetCmd clears the per-project plans.dir/plans.type overrides.
var projectPlansUnsetCmd = &cobra.Command{
	Use:   "unset",
	Short: "Clear the per-project plans override",
	RunE: func(cmd *cobra.Command, args []string) error {
		projID, _, _, err := resolveProject()
		if err != nil {
			return err
		}
		c, err := getClient()
		if err != nil {
			return err
		}
		if err := c.DeleteProjectConfig(projID, cfgpkg.ProjectPlansDirKey); err != nil {
			return err
		}
		if err := c.DeleteProjectConfig(projID, cfgpkg.ProjectPlansTypeKey); err != nil {
			return err
		}
		return runProjectPlansGet(cmd, args)
	},
}

// resolvePlansForProject resolves the effective plans destination for a project.
// Client failures degrade to nil rows so resolution still returns config/default layers.
func resolvePlansForProject(projID, projName, prefix string) (dir, ptype, source string, err error) {
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
	return cfgpkg.ResolvePlans(rows, cfg, vars, cwd)
}

// runProjectPlansGet resolves and prints the current project's plans destination.
func runProjectPlansGet(cmd *cobra.Command, args []string) error {
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
	dir, ptype, source, err := resolvePlansForProject(projID, projName, prefix)
	if err != nil {
		return err
	}
	if outputJSON {
		outputResult(map[string]string{"plans_dir": dir, "plans_type": ptype, "plans_source": source})
		return nil
	}
	fmt.Printf("Dir:    %s\n", dir)
	fmt.Printf("Type:   %s\n", ptype)
	fmt.Printf("Source: %s\n", source)
	return nil
}

// init wires the plans subcommand's flags and registers it under `arc project`.
func init() {
	projectPlansSetCmd.Flags().StringVar(&plansSetDir, "dir", "",
		"plans directory (absolute, ~, or repo-relative; {project}/{prefix} templates allowed)")
	// --type is validated against cfgpkg.ValidPlansType before being persisted.
	projectPlansSetCmd.Flags().StringVar(&plansSetType, "type", "", "plans type: markdown or obsidian")
	projectPlansCmd.AddCommand(projectPlansSetCmd, projectPlansGetCmd, projectPlansUnsetCmd)
	projectCmd.AddCommand(projectPlansCmd)
}
