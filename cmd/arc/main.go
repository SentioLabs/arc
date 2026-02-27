// Command arc is the CLI for arc.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/project"
	"github.com/sentiolabs/arc/internal/version"
	"github.com/sentiolabs/arc/internal/workspace"
	"github.com/spf13/cobra"
)

var (
	serverURL     string
	workspaceID   string
	workspaceName string
	outputJSON    bool
	configPath    string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// Config holds CLI configuration
type Config struct {
	ServerURL string `json:"server_url"`
}

// WorkspaceSource indicates how the workspace was resolved
type WorkspaceSource int

const (
	WorkspaceSourceFlag    WorkspaceSource = iota
	WorkspaceSourceProject                         // ~/.arc/projects/<path>/config.json
	WorkspaceSourceLegacy                          // .arc.json (migrated)
)

func (s WorkspaceSource) String() string {
	switch s {
	case WorkspaceSourceFlag:
		return "command line flag (-w)"
	case WorkspaceSourceProject:
		return "~/.arc/projects/ (local)"
	case WorkspaceSourceLegacy:
		return ".arc.json (legacy, migrated)"
	default:
		return "unknown"
	}
}

// defaultConfigPath returns the default config file path.
func defaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".arc", "cli-config.json")
}

func loadConfig() (*Config, error) {
	if configPath == "" {
		configPath = defaultConfigPath()
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create default config on first use
			cfg := &Config{
				ServerURL: "http://localhost:7432",
			}
			// Try to save, but don't fail if we can't
			_ = saveConfig(cfg)
			return cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.ServerURL == "" {
		cfg.ServerURL = "http://localhost:7432"
	}

	return &cfg, nil
}

func saveConfig(cfg *Config) error {
	path := configPath
	if path == "" {
		path = defaultConfigPath()
	}

	// Create directory
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(path, data, 0o644)
}

func getClient() (*client.Client, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}

	url := serverURL
	if url == "" {
		url = cfg.ServerURL
	}

	return client.New(url), nil
}

// getWorkspaceID resolves the workspace ID using the following priority:
// 1. CLI flag (-w/--workspace) - explicit override
// 2. Project config (~/.arc/projects/<path>/config.json)
// 3. Legacy .arc.json (auto-migrated)
//
// If none is available, an error is returned. There is no global fallback.
func getWorkspaceID() (string, error) {
	wsID, _, _, err := resolveWorkspace()
	return wsID, err
}

// resolveWorkspace returns the workspace ID, source, and error.
// Resolution priority:
//  1. CLI flag (-w/--workspace) - explicit override always works
//  2. Project config (~/.arc/projects/<path>/config.json)
//  3. Legacy .arc.json - auto-migrated to project config
//
// If none is available, an error is returned. There is no global fallback
// to prevent accidentally operating in the wrong workspace.
func resolveWorkspace() (wsID string, source WorkspaceSource, warning string, err error) {
	// Priority 1: CLI flag (explicit override)
	if workspaceID != "" {
		return workspaceID, WorkspaceSourceFlag, "", nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", 0, "", fmt.Errorf("get current directory: %w", err)
	}

	arcHome := project.DefaultArcHome()

	// Priority 2: Find project root and load config from ~/.arc/projects/
	projectRoot, rootErr := project.FindProjectRootWithArcHome(cwd, arcHome)
	if rootErr == nil {
		cfg, cfgErr := project.LoadConfig(arcHome, projectRoot)
		if cfgErr == nil && cfg.WorkspaceID != "" {
			// Validate workspace exists on server
			c, clientErr := getClient()
			if clientErr == nil {
				_, wsErr := c.GetWorkspace(cfg.WorkspaceID)
				if wsErr != nil {
					return "", 0, "", fmt.Errorf("workspace '%s' (%s) not found on server\n  Run 'arc init' to reconfigure this directory",
						cfg.WorkspaceName, cfg.WorkspaceID)
				}
			}
			return cfg.WorkspaceID, WorkspaceSourceProject, "", nil
		}
	}

	// Priority 3: Legacy .arc.json fallback with migration
	legacyPath, legacyErr := project.FindLegacyConfig(cwd)
	if legacyErr == nil {
		legacyRoot := filepath.Dir(legacyPath)
		cfg, migrateErr := project.MigrateLegacyConfig(legacyRoot, arcHome)
		if migrateErr == nil && cfg.WorkspaceID != "" {
			fmt.Fprintf(os.Stderr, "Migrated .arc.json → ~/.arc/projects/%s/config.json\n",
				project.PathToProjectDir(legacyRoot))

			// Validate workspace exists on server
			c, clientErr := getClient()
			if clientErr == nil {
				_, wsErr := c.GetWorkspace(cfg.WorkspaceID)
				if wsErr != nil {
					return "", 0, "", fmt.Errorf("workspace '%s' (%s) not found on server\n  Run 'arc init' to reconfigure this directory",
						cfg.WorkspaceName, cfg.WorkspaceID)
				}
			}
			return cfg.WorkspaceID, WorkspaceSourceLegacy, "", nil
		}
	}

	return "", 0, "", fmt.Errorf("no workspace configured for this directory\n  Run 'arc init' to set up a workspace, or use '-w <workspace>' to specify one")
}

func outputResult(data interface{}) {
	if outputJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(data)
	} else {
		fmt.Println(data)
	}
}

// isSubdirectory returns true if child is the same as or a subdirectory of parent.
func isSubdirectory(parent, child string) bool {
	// Clean paths for consistent comparison
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)

	// Exact match
	if parent == child {
		return true
	}

	// Check if child starts with parent + separator
	// This prevents /home/foo/project matching /home/foo/project2
	return strings.HasPrefix(child, parent+string(filepath.Separator))
}

var rootCmd = &cobra.Command{
	Use:     "arc",
	Short:   "arc CLI - central issue tracking",
	Long:    `arc is a central issue tracking server for AI-assisted coding workflows.`,
	Version: version.Info(),
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&serverURL, "server", "s", "", "Server URL (default: http://localhost:7432)")
	rootCmd.PersistentFlags().StringVarP(&workspaceID, "workspace", "w", "", "Workspace ID")
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "Output as JSON")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Config file path")

	// Add commands
	rootCmd.AddCommand(workspaceCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(closeCmd)
	rootCmd.AddCommand(readyCmd)
	rootCmd.AddCommand(blockedCmd)
	rootCmd.AddCommand(depCmd)
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(selfCmd)
}

// ============ Workspace Commands ============

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage workspaces",
}

func init() {
	workspaceCmd.AddCommand(workspaceListCmd)
	workspaceCmd.AddCommand(workspaceCreateCmd)
	workspaceCmd.AddCommand(workspaceDeleteCmd)
}

var whichCmd = &cobra.Command{
	Use:   "which",
	Short: "Show which workspace is active and how it was resolved",
	Long: `Display the currently active workspace and its resolution source.

This helps debug workspace resolution issues by showing:
- The active workspace ID and name
- Where the workspace was resolved from (flag, project config, or legacy .arc.json)
- The project config file path
- Any warnings about the configuration`,
	RunE: func(cmd *cobra.Command, args []string) error {
		wsID, source, warning, err := resolveWorkspace()
		if err != nil {
			return err
		}

		// Try to get workspace details
		c, clientErr := getClient()
		var wsName string
		if clientErr == nil {
			if ws, wsErr := c.GetWorkspace(wsID); wsErr == nil {
				wsName = ws.Name
			}
		}

		if outputJSON {
			result := map[string]string{
				"workspace_id": wsID,
				"source":       source.String(),
			}
			if wsName != "" {
				result["workspace_name"] = wsName
			}
			if warning != "" {
				result["warning"] = warning
			}
			outputResult(result)
			return nil
		}

		// Human-readable output
		if wsName != "" {
			fmt.Printf("Workspace: %s (%s)\n", wsName, wsID)
		} else {
			fmt.Printf("Workspace: %s\n", wsID)
		}
		fmt.Printf("Source: %s\n", source)

		if source == WorkspaceSourceProject || source == WorkspaceSourceLegacy {
			cwd, _ := os.Getwd()
			arcHome := project.DefaultArcHome()
			if root, err := project.FindProjectRootWithArcHome(cwd, arcHome); err == nil {
				fmt.Printf("Config: ~/.arc/projects/%s/config.json\n", project.PathToProjectDir(root))
			}
		}

		if warning != "" {
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, warning)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(whichCmd)
}

var workspaceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all workspaces",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		workspaces, err := c.ListWorkspaces()
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(workspaces)
			return nil
		}

		if len(workspaces) == 0 {
			fmt.Println("No workspaces found. Create one with: arc workspace create <name>")
			return nil
		}

		// Get current directory to mark the active workspace
		cwd, _ := os.Getwd()

		// Create a tabwriter for aligned columns
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "  \tNAME\tPREFIX\tID\tDESCRIPTION\tPATH")
		fmt.Fprintln(w, "  \t────\t──────\t──\t───────────\t────")

		for _, ws := range workspaces {
			marker := " "
			// Mark workspace if current directory is within its path
			if ws.Path != "" && cwd != "" && isSubdirectory(ws.Path, cwd) {
				marker = "*"
			}
			path := ws.Path
			if path == "" {
				path = "-"
			}
			desc := ws.Description
			if desc == "" {
				desc = "-"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", marker, ws.Name, ws.Prefix, ws.ID, desc, path)
		}
		w.Flush()
		return nil
	},
}

var workspaceCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new workspace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		path, _ := cmd.Flags().GetString("path")
		description, _ := cmd.Flags().GetString("description")

		// Generate prefix from path if provided, otherwise from workspace name
		var prefix string
		if path != "" {
			prefix, err = workspace.GeneratePrefix(path)
			if err != nil {
				return fmt.Errorf("generate prefix: %w", err)
			}
		} else {
			// No path - generate prefix from workspace name with hash
			prefix = workspace.GeneratePrefixFromName(args[0])
		}

		ws, err := c.CreateWorkspace(args[0], prefix, path, description)
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(ws)
			return nil
		}

		fmt.Printf("Created workspace: %s (%s)\n", ws.Name, ws.ID)
		return nil
	},
}

func init() {
	workspaceCreateCmd.Flags().String("path", "", "Associated directory path")
	workspaceCreateCmd.Flags().StringP("description", "d", "", "Workspace description")
}

var workspaceDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a workspace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		if err := c.DeleteWorkspace(args[0]); err != nil {
			return err
		}

		fmt.Printf("Deleted workspace: %s\n", args[0])
		return nil
	},
}

// ============ Issue Commands ============

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getWorkspaceID()
		if err != nil {
			return err
		}

		status, _ := cmd.Flags().GetString("status")
		issueType, _ := cmd.Flags().GetString("type")
		assignee, _ := cmd.Flags().GetString("assignee")
		query, _ := cmd.Flags().GetString("query")
		limit, _ := cmd.Flags().GetInt("limit")

		issues, err := c.ListIssues(wsID, client.ListIssuesOptions{
			Status:   status,
			Type:     issueType,
			Assignee: assignee,
			Query:    query,
			Limit:    limit,
		})
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(issues)
			return nil
		}

		for _, issue := range issues {
			fmt.Println(formatIssue(issue.ID, string(issue.Status), string(issue.IssueType),
				issue.Priority, issue.Title, issue.Labels))
		}
		return nil
	},
}

func init() {
	listCmd.Flags().String("status", "", "Filter by status")
	listCmd.Flags().String("type", "", "Filter by type")
	listCmd.Flags().String("assignee", "", "Filter by assignee")
	listCmd.Flags().StringP("query", "q", "", "Search query")
	listCmd.Flags().IntP("limit", "l", 50, "Max results")
}

var createCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a new issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getWorkspaceID()
		if err != nil {
			return err
		}

		priority, _ := cmd.Flags().GetInt("priority")
		issueType, _ := cmd.Flags().GetString("type")
		assignee, _ := cmd.Flags().GetString("assignee")
		description, _ := cmd.Flags().GetString("description")
		parentID, _ := cmd.Flags().GetString("parent")

		issue, err := c.CreateIssue(wsID, client.CreateIssueRequest{
			Title:       args[0],
			Description: description,
			Priority:    priority,
			IssueType:   issueType,
			Assignee:    assignee,
			ParentID:    parentID,
		})
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(issue)
			return nil
		}

		fmt.Printf("Created: %s\n", issue.ID)
		return nil
	},
}

func init() {
	createCmd.Flags().IntP("priority", "p", 2, "Priority (0-4)")
	createCmd.Flags().StringP("type", "t", "task", "Issue type")
	createCmd.Flags().StringP("assignee", "a", "", "Assignee")
	createCmd.Flags().StringP("description", "d", "", "Description")
	createCmd.Flags().String("parent", "", "Parent issue ID (creates child with .N suffix)")
}

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show issue details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getWorkspaceID()
		if err != nil {
			return err
		}

		details, err := c.GetIssueDetails(wsID, args[0])
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(details)
			return nil
		}

		fmt.Printf("ID:       %s\n", details.ID)
		fmt.Printf("Title:    %s\n", details.Title)
		fmt.Printf("Status:   %s\n", details.Status)
		fmt.Printf("Priority: P%d\n", details.Priority)
		fmt.Printf("Type:     %s\n", details.IssueType)
		if details.Assignee != "" {
			fmt.Printf("Assignee: %s\n", details.Assignee)
		}
		if details.Description != "" {
			fmt.Printf("\nDescription:\n%s\n", details.Description)
		}
		if len(details.Labels) > 0 {
			fmt.Printf("\nLabels: %s\n", strings.Join(details.Labels, ", "))
		}
		if len(details.Dependencies) > 0 {
			fmt.Printf("\nDepends on:\n")
			for _, dep := range details.Dependencies {
				fmt.Printf("  - %s (%s)\n", dep.DependsOnID, dep.Type)
			}
		}
		if len(details.Dependents) > 0 {
			fmt.Printf("\nBlocking:\n")
			for _, dep := range details.Dependents {
				fmt.Printf("  - %s (%s)\n", dep.IssueID, dep.Type)
			}
		}
		if len(details.Comments) > 0 {
			fmt.Printf("\nComments (%d):\n", len(details.Comments))
			for _, comment := range details.Comments {
				fmt.Printf("  [%s] %s: %s\n",
					comment.CreatedAt.Format("2006-01-02 15:04"),
					comment.Author, comment.Text)
			}
		}

		// Display plan context if available
		if details.PlanContext != nil && details.PlanContext.HasPlan() {
			pc := details.PlanContext
			fmt.Println()

			if pc.InlinePlan != nil {
				fmt.Printf("Plan:\n")
				// Indent the plan text
				for _, line := range strings.Split(pc.InlinePlan.Text, "\n") {
					fmt.Printf("  %s\n", line)
				}
			}

			if pc.ParentPlan != nil {
				fmt.Printf("Plan (from %s):\n", pc.ParentIssueID)
				for _, line := range strings.Split(pc.ParentPlan.Text, "\n") {
					fmt.Printf("  %s\n", line)
				}
			}

			if len(pc.SharedPlans) > 0 {
				fmt.Printf("Linked Plans:\n")
				for _, plan := range pc.SharedPlans {
					fmt.Printf("  - %s: %s\n", plan.ID, plan.Title)
				}
			}
		}

		return nil
	},
}

var updateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getWorkspaceID()
		if err != nil {
			return err
		}

		updates := make(map[string]interface{})

		if val, _ := cmd.Flags().GetString("status"); val != "" {
			updates["status"] = val
		}
		if val, _ := cmd.Flags().GetString("title"); val != "" {
			updates["title"] = val
		}
		if val, _ := cmd.Flags().GetString("assignee"); val != "" {
			updates["assignee"] = val
		}
		if cmd.Flags().Changed("priority") {
			val, _ := cmd.Flags().GetInt("priority")
			updates["priority"] = val
		}
		if val, _ := cmd.Flags().GetString("type"); val != "" {
			updates["issue_type"] = val
		}
		if val, _ := cmd.Flags().GetString("description"); val != "" {
			updates["description"] = val
		}

		if len(updates) == 0 {
			return fmt.Errorf("no updates specified")
		}

		issue, err := c.UpdateIssue(wsID, args[0], updates)
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(issue)
			return nil
		}

		fmt.Printf("Updated: %s\n", issue.ID)
		return nil
	},
}

func init() {
	updateCmd.Flags().String("status", "", "New status")
	updateCmd.Flags().String("title", "", "New title")
	updateCmd.Flags().StringP("assignee", "a", "", "New assignee")
	updateCmd.Flags().IntP("priority", "p", 0, "New priority")
	updateCmd.Flags().StringP("type", "t", "", "New type")
	updateCmd.Flags().StringP("description", "d", "", "New description")
}

var closeCmd = &cobra.Command{
	Use:   "close <id> [ids...]",
	Short: "Close one or more issues",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getWorkspaceID()
		if err != nil {
			return err
		}

		reason, _ := cmd.Flags().GetString("reason")

		for _, id := range args {
			issue, err := c.CloseIssue(wsID, id, reason)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to close %s: %v\n", id, err)
				continue
			}
			fmt.Printf("Closed: %s\n", issue.ID)
		}

		return nil
	},
}

func init() {
	closeCmd.Flags().StringP("reason", "r", "", "Close reason")
}

var readyCmd = &cobra.Command{
	Use:   "ready",
	Short: "Show issues ready to work on",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getWorkspaceID()
		if err != nil {
			return err
		}

		limit, _ := cmd.Flags().GetInt("limit")
		sortPolicy, _ := cmd.Flags().GetString("sort")

		issues, err := c.GetReadyWork(wsID, limit, sortPolicy)
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(issues)
			return nil
		}

		if len(issues) == 0 {
			fmt.Println("No ready issues")
			return nil
		}

		for _, issue := range issues {
			fmt.Println(formatIssue(issue.ID, string(issue.Status), string(issue.IssueType),
				issue.Priority, issue.Title, issue.Labels))
		}
		return nil
	},
}

func init() {
	readyCmd.Flags().IntP("limit", "l", 10, "Max results")
	readyCmd.Flags().String("sort", "hybrid", "Sort policy: hybrid (recent by priority, old by age), priority (always by priority), oldest (oldest first)")
}

var blockedCmd = &cobra.Command{
	Use:   "blocked",
	Short: "Show blocked issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getWorkspaceID()
		if err != nil {
			return err
		}

		limit, _ := cmd.Flags().GetInt("limit")

		issues, err := c.GetBlockedIssues(wsID, limit)
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(issues)
			return nil
		}

		if len(issues) == 0 {
			fmt.Println("No blocked issues")
			return nil
		}

		for _, issue := range issues {
			fmt.Println(formatBlockedIssue(issue.ID, string(issue.IssueType),
				issue.Priority, issue.Title, issue.Labels, issue.BlockedByCount))
		}
		return nil
	},
}

func init() {
	blockedCmd.Flags().IntP("limit", "l", 10, "Max results")
}

// ============ Dependency Commands ============

var depCmd = &cobra.Command{
	Use:   "dep",
	Short: "Manage dependencies",
}

func init() {
	depCmd.AddCommand(depAddCmd)
	depCmd.AddCommand(depRemoveCmd)
}

var depAddCmd = &cobra.Command{
	Use:   "add <issue> <depends-on>",
	Short: "Add dependency (issue depends on depends-on)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getWorkspaceID()
		if err != nil {
			return err
		}

		depType, _ := cmd.Flags().GetString("type")
		if depType == "" {
			depType = "blocks"
		}

		if err := c.AddDependency(wsID, args[0], args[1], depType); err != nil {
			return err
		}

		fmt.Printf("Added: %s depends on %s (%s)\n", args[0], args[1], depType)
		return nil
	},
}

func init() {
	depAddCmd.Flags().StringP("type", "t", "blocks", "Dependency type (blocks, parent-child, related, discovered-from)")
}

var depRemoveCmd = &cobra.Command{
	Use:   "remove <issue> <depends-on>",
	Short: "Remove dependency",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getWorkspaceID()
		if err != nil {
			return err
		}

		if err := c.RemoveDependency(wsID, args[0], args[1]); err != nil {
			return err
		}

		fmt.Printf("Removed: %s no longer depends on %s\n", args[0], args[1])
		return nil
	},
}

// ============ Stats Command ============

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show workspace statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getWorkspaceID()
		if err != nil {
			return err
		}

		stats, err := c.GetStatistics(wsID)
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(stats)
			return nil
		}

		fmt.Printf("Total:       %d\n", stats.TotalIssues)
		fmt.Printf("Open:        %d\n", stats.OpenIssues)
		fmt.Printf("In Progress: %d\n", stats.InProgressIssues)
		fmt.Printf("Blocked:     %d\n", stats.BlockedIssues)
		fmt.Printf("Deferred:    %d\n", stats.DeferredIssues)
		fmt.Printf("Ready:       %d\n", stats.ReadyIssues)
		fmt.Printf("Closed:      %d\n", stats.ClosedIssues)
		if stats.AvgLeadTimeHours > 0 {
			fmt.Printf("Avg Lead:    %.1f hours\n", stats.AvgLeadTimeHours)
		}

		return nil
	},
}

// Helper to parse priority (handles "P0", "P1", "0", "1", etc.)
func parsePriority(s string) (int, error) {
	s = strings.TrimPrefix(strings.ToUpper(s), "P")
	return strconv.Atoi(s)
}

// formatIssue returns a beads-style formatted issue line
func formatIssue(id, status, issueType string, priority int, title string, labels []string) string {
	// Status icon
	icon := "○" // open
	switch status {
	case "in_progress":
		icon = "◐"
	case "blocked":
		icon = "◌"
	case "closed":
		icon = "●"
	case "deferred":
		icon = "◇"
	}

	// Priority badge - filled for high priority (P0-P1), empty for lower
	priorityIcon := "○"
	if priority <= 1 {
		priorityIcon = "●"
	}

	// Labels as space-separated in brackets
	labelStr := ""
	if len(labels) > 0 {
		labelStr = " [" + strings.Join(labels, " ") + "]"
	}

	return fmt.Sprintf("%s %s [%s P%d] [%s]%s - %s",
		icon, id, priorityIcon, priority, issueType, labelStr, title)
}

// formatBlockedIssue returns a beads-style formatted blocked issue line
func formatBlockedIssue(id, issueType string, priority int, title string, labels []string, blockedByCount int) string {
	// Blocked issues always use blocked icon
	icon := "◌"

	// Priority badge
	priorityIcon := "○"
	if priority <= 1 {
		priorityIcon = "●"
	}

	// Labels
	labelStr := ""
	if len(labels) > 0 {
		labelStr = " [" + strings.Join(labels, " ") + "]"
	}

	return fmt.Sprintf("%s %s [%s P%d] [%s]%s - %s (blocked by %d)",
		icon, id, priorityIcon, priority, issueType, labelStr, title, blockedByCount)
}
