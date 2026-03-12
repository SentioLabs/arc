package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/project"
	"github.com/spf13/cobra"
)

// pathsAddLabel is the --label flag for paths add.
var pathsAddLabel string

// pathsAddHostname is the --hostname flag for paths add.
var pathsAddHostname string

// pathsListAll is the --all flag for paths list.
var pathsListAll bool

// pathsCmd is the parent command for workspace path management.
var pathsCmd = &cobra.Command{
	Use:   "paths",
	Short: "Manage workspace path registrations",
	Long: `List, add, and remove filesystem paths associated with the current project.

When run without a subcommand, lists paths for the current project.`,
	RunE: runPathsList,
}

// pathsAddCmd registers a new path to the current project.
var pathsAddCmd = &cobra.Command{
	Use:   "add <dir>",
	Short: "Register a path to the current project",
	Args:  cobra.ExactArgs(1),
	RunE:  runPathsAdd,
}

// pathsRemoveCmd unregisters a path from the current project.
var pathsRemoveCmd = &cobra.Command{
	Use:   "remove <path-or-id>",
	Short: "Unregister a path from the current project",
	Args:  cobra.ExactArgs(1),
	RunE:  runPathsRemove,
}

// pathsListCmd lists paths, optionally across all projects.
var pathsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List paths (use --all for all projects)",
	RunE:  runPathsListCmd,
}

func init() {
	rootCmd.AddCommand(pathsCmd)

	pathsCmd.AddCommand(pathsAddCmd)
	pathsCmd.AddCommand(pathsRemoveCmd)
	pathsCmd.AddCommand(pathsListCmd)

	pathsAddCmd.Flags().StringVar(&pathsAddLabel, "label", "", "Label for the path")
	pathsAddCmd.Flags().StringVar(&pathsAddHostname, "hostname", "", "Hostname for the path")

	pathsListCmd.Flags().BoolVar(&pathsListAll, "all", false, "List paths across all projects")
}

// runPathsList lists paths for the current project (default behavior).
func runPathsList(cmd *cobra.Command, args []string) error {
	c, err := getClient()
	if err != nil {
		return err
	}

	projID, err := getProjectID()
	if err != nil {
		return err
	}

	paths, err := c.ListWorkspaces(projID)
	if err != nil {
		return err
	}

	if outputJSON {
		outputResult(paths)
		return nil
	}

	if len(paths) == 0 {
		fmt.Println("No paths registered for this project.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, tabwriterPadding, ' ', 0)
	_, _ = fmt.Fprintln(w, "PATH\tLABEL\tHOST\tLAST ACCESSED")
	_, _ = fmt.Fprintln(w, "────\t─────\t────\t─────────────")

	for _, p := range paths {
		label := p.Label
		if label == "" {
			label = "-"
		}
		host := p.Hostname
		if host == "" {
			host = "-"
		}
		lastAccessed := p.UpdatedAt
		if lastAccessed.IsZero() {
			lastAccessed = p.CreatedAt
		}
		var lastAccessedStr string
		if lastAccessed.IsZero() {
			lastAccessedStr = "-"
		} else {
			lastAccessedStr = lastAccessed.Format("2006-01-02 15:04")
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Path, label, host, lastAccessedStr)
	}

	return w.Flush()
}

// runPathsAdd registers a new path to the current project.
func runPathsAdd(cmd *cobra.Command, args []string) error {
	dir := args[0]

	c, err := getClient()
	if err != nil {
		return err
	}

	projID, err := getProjectID()
	if err != nil {
		return err
	}

	normalizedPath := project.NormalizePath(dir)

	// Auto-detect git remote
	gitRemote := detectGitRemote(dir)

	hostname := pathsAddHostname
	if hostname == "" {
		h, hErr := os.Hostname()
		if hErr == nil {
			hostname = h
		}
	}

	// Determine path type by checking if the path is a symlink
	pathType := "canonical"
	resolved, evalErr := filepath.EvalSymlinks(dir)
	if evalErr == nil && resolved != normalizedPath {
		pathType = "symlink"
	}

	req := client.CreateWorkspaceRequest{
		Path:      normalizedPath,
		Label:     pathsAddLabel,
		Hostname:  hostname,
		GitRemote: gitRemote,
		PathType:  pathType,
	}

	wp, err := c.CreateWorkspace(projID, req)
	if err != nil {
		return err
	}

	if outputJSON {
		outputResult(wp)
		return nil
	}

	fmt.Printf("Registered path: %s\n", wp.Path)
	return nil
}

// runPathsRemove unregisters a path from the current project.
func runPathsRemove(cmd *cobra.Command, args []string) error {
	arg := args[0]

	c, err := getClient()
	if err != nil {
		return err
	}

	projID, err := getProjectID()
	if err != nil {
		return err
	}

	pathID := arg

	// If argument looks like a path (contains /), find the matching path ID
	if strings.Contains(arg, "/") {
		paths, listErr := c.ListWorkspaces(projID)
		if listErr != nil {
			return listErr
		}

		normalized := project.NormalizePath(arg)
		found := false
		for _, p := range paths {
			if p.Path == arg || p.Path == normalized {
				pathID = p.ID
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("no matching path found for %q", arg)
		}
	}

	if err := c.DeleteWorkspace(projID, pathID); err != nil {
		return err
	}

	if !outputJSON {
		fmt.Printf("Removed path: %s\n", arg)
	}
	return nil
}

// runPathsListCmd lists paths, optionally across all projects.
func runPathsListCmd(cmd *cobra.Command, args []string) error {
	if !pathsListAll {
		return runPathsList(cmd, args)
	}

	c, err := getClient()
	if err != nil {
		return err
	}

	projects, err := c.ListProjects()
	if err != nil {
		return err
	}

	type projPath struct {
		Project string `json:"project"`
		Path    string `json:"path"`
		Label   string `json:"label,omitempty"`
		Host    string `json:"host,omitempty"`
	}

	var allPaths []projPath

	for _, p := range projects {
		paths, pErr := c.ListWorkspaces(p.ID)
		if pErr != nil {
			continue
		}
		for _, wp := range paths {
			allPaths = append(allPaths, projPath{
				Project: p.Name,
				Path:    wp.Path,
				Label:   wp.Label,
				Host:    wp.Hostname,
			})
		}
	}

	if outputJSON {
		outputResult(allPaths)
		return nil
	}

	if len(allPaths) == 0 {
		fmt.Println("No paths registered across any project.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, tabwriterPadding, ' ', 0)
	_, _ = fmt.Fprintln(w, "PROJECT\tPATH\tLABEL\tHOST")
	_, _ = fmt.Fprintln(w, "───────\t────\t─────\t────")

	for _, p := range allPaths {
		label := p.Label
		if label == "" {
			label = "-"
		}
		host := p.Host
		if host == "" {
			host = "-"
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Project, p.Path, label, host)
	}

	return w.Flush()
}

// detectGitRemote attempts to get the git remote URL for a directory.
func detectGitRemote(dir string) string {
	cmd := exec.Command("git", "-C", dir, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
