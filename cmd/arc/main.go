// Command arc is the CLI for arc, a central issue tracking server for
// AI-assisted coding workflows. This package wires together all CLI
// commands using Cobra, handles project resolution, and provides
// human-readable and JSON output for every operation.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/sentiolabs/arc/internal/client"
	cfgpkg "github.com/sentiolabs/arc/internal/config"
	"github.com/sentiolabs/arc/internal/project"
	"github.com/sentiolabs/arc/internal/types"
	"github.com/sentiolabs/arc/internal/vcs"
	"github.com/sentiolabs/arc/internal/version"
	"github.com/spf13/cobra"
)

// CLI constants for default values and formatting.
const (
	// tabwriterPadding is the minimum padding between columns in tabwriter output.
	tabwriterPadding = 2

	// defaultListLimit is the default maximum number of issues returned by the list command.
	defaultListLimit = 50

	// defaultPriority is the default priority for new issues (medium).
	defaultPriority = 2

	// defaultReadyLimit is the default maximum number of issues shown by the ready command.
	defaultReadyLimit = 10

	// defaultBlockedLimit is the default maximum number of issues shown by the blocked command.
	defaultBlockedLimit = 10

	// milestoneSiblingsLimit bounds the sibling-milestone lookup used for
	// auto-sequencing. It must exceed the server's default list cap so a
	// release with many milestones doesn't silently lose sequencing
	// candidates past the cap.
	milestoneSiblingsLimit = 1000

	// depPairArgCount is the number of arguments for commands that take a pair of issue IDs.
	depPairArgCount = 2

	// statusIconOpen is the icon displayed for open issues.
	statusIconOpen = "\u25cb" // ○

	// statusIconClosed is the icon displayed for closed issues.
	statusIconClosed = "\u25cf" // ●

	// priorityNormal is the priority level for normal/default issues (P2).
	priorityNormal = 2
	// priorityLow is the priority level for low-importance issues (P3).
	priorityLow = 3

	// cmdList is the cobra Use string shared by all "list" sub-commands.
	cmdList = "list"
	// cmdStart is the cobra Use string for "start" sub-commands.
	cmdStart = "start"
	// useShowID is the cobra Use string for "show <id>" sub-commands.
	useShowID = "show <id>"
	// cmdProject is the cobra Use string for the "project" command.
	cmdProject = "project"
	// flagProject is the name of the persistent --project flag.
	flagProject = "project"
)

// Global CLI flags shared across all commands.
var (
	serverURL  string // --server flag override
	projectID  string // --project flag override
	outputJSON bool   // --json flag for machine-readable output
	configPath string // --config flag for alternate config file
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// ProjectSource indicates how the project was resolved
type ProjectSource int

const (
	ProjectSourceFlag    ProjectSource = iota
	ProjectSourceProject               // ~/.arc/projects/<path>/config.json
	ProjectSourceServer                // server path matching (containers/mounts)
)

func (s ProjectSource) String() string {
	switch s {
	case ProjectSourceFlag:
		return "command line flag (--project)"
	case ProjectSourceProject:
		return "~/.arc/projects/ (local)"
	case ProjectSourceServer:
		return "server path match"
	default:
		return "unknown"
	}
}

// loadConfig reads CLI configuration from disk, creating defaults on first use.
// It delegates to the internal/config package which handles TOML load, legacy
// JSON migration, and validation.
func loadConfig() (*cfgpkg.Config, error) {
	path := configPath
	if path == "" {
		path = cfgpkg.DefaultPath()
	}
	return cfgpkg.Load(path)
}

// saveConfig persists the CLI configuration to disk.
func saveConfig(cfg *cfgpkg.Config) error {
	path := configPath
	if path == "" {
		path = cfgpkg.DefaultPath()
	}
	return cfgpkg.Save(path, cfg)
}

// getClient returns an HTTP client configured for the current server URL.
func getClient() (*client.Client, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}

	url := serverURL
	if url == "" {
		if envURL := os.Getenv("ARC_SERVER"); envURL != "" {
			url = envURL
		}
	}
	if url == "" {
		url = cfg.CLI.Server
	}

	return client.New(url), nil
}

// getProjectID resolves the project ID using the following priority:
// 1. CLI flag (--project) - explicit override
// 2. Project config (~/.arc/projects/<path>/config.json)
//
// If none is available, an error is returned. There is no global fallback.
func getProjectID() (string, error) {
	wsID, _, _, err := resolveProject()
	return wsID, err
}

// resolveProject returns the project ID, source, and error.
// Resolution priority:
//  1. CLI flag (--project) - explicit override always works
//  2. Server path matching (delegates all resolution to server)
//  3. Legacy config fallback (~/.arc/projects/ configs from before server-side paths)
//
// If none is available, an error is returned. There is no global fallback
// to prevent accidentally operating in the wrong project.
func resolveProject() (wsID string, source ProjectSource, warning string, err error) {
	// Priority 1: CLI flag (explicit override)
	if projectID != "" {
		return projectID, ProjectSourceFlag, "", nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", 0, "", fmt.Errorf("get current directory: %w", err)
	}

	arcHome := project.DefaultArcHome()

	// Priority 2: Server path matching (checks workspace_paths table, handles symlinks)
	if serverWsID, serverErr := resolveFromServer(cwd); serverErr == nil {
		return serverWsID, ProjectSourceServer, "", nil
	}

	// Priority 3: Legacy config fallback (~/.arc/projects/ configs from before server-side paths)
	wsID, source, warning, resolveErr := resolveFromLegacyConfig(cwd, arcHome)
	if resolveErr != nil {
		return "", 0, "", resolveErr
	}
	if wsID != "" {
		return wsID, source, warning, nil
	}

	return "", 0, "", errors.New(
		"no project configured for this directory\n" +
			"  Run 'arc init' to set up a project, or use '--project <id>' to specify one")
}

// resolveFromServer asks the server to resolve the given cwd to a project ID.
// The server's resolver handles exact match, longest-prefix match against
// registered workspace paths, and linked-git-worktree detection.
func resolveFromServer(cwd string) (string, error) {
	c, err := getClient()
	if err != nil {
		return "", err
	}

	// Try the path exactly as the shell reported it. This is the form `arc init`
	// registers when cwd was reached through a symlink, and is the common case.
	if res, resolveErr := c.ResolveProjectByPath(cwd); resolveErr == nil && res.ProjectID != "" {
		return res.ProjectID, nil
	}

	// Retry with symlinks resolved, to cover the inbound direction: a project
	// initialized from its canonical path cannot know which symlinks point at it,
	// so that pairing can only be closed here. Resolution must happen client-side
	// — the server may be remote and cannot inspect this filesystem. Costs one
	// extra request, and only on a miss where the resolved form actually differs.
	if resolved := project.NormalizePath(cwd); resolved != cwd {
		if res, resolveErr := c.ResolveProjectByPath(resolved); resolveErr == nil && res.ProjectID != "" {
			return res.ProjectID, nil
		}
	}

	return "", errors.New(
		"no project configured for this directory\n" +
			"  Run 'arc init' to set up a project, or use '--project <id>' to specify one")
}

// resolveFromLegacyConfig attempts to resolve project from ~/.arc/projects/ config.
// Returns empty wsID if no config is found (without error).
// When a valid legacy config is found, auto-migrates it to server-side paths
// and cleans up the legacy config directory.
func resolveFromLegacyConfig(cwd, arcHome string) (wsID string, source ProjectSource, warning string, err error) {
	cfg, cfgErr := readLegacyConfig(arcHome, cwd)
	if cfgErr != nil {
		return "", 0, "", cfgErr
	}
	if cfg == nil || cfg.WorkspaceID == "" {
		return "", 0, "", nil
	}

	// Validate project exists on server
	c, clientErr := getClient()
	if clientErr != nil {
		return "", 0, "", nil //nolint:nilerr // server unreachable; skip validation
	}

	if _, wsErr := c.GetProject(cfg.WorkspaceID); wsErr != nil {
		return "", 0, "", fmt.Errorf(
			"project '%s' (%s) not found on server\n  Run 'arc init' to reconfigure this directory",
			cfg.WorkspaceName, cfg.WorkspaceID)
	}

	// Auto-migrate: register paths on server and clean up legacy config
	absPath, resolvedPath := project.NormalizePathPair(cwd)
	hostname, _ := os.Hostname()
	if regErr := registerPathPair(c, cfg.WorkspaceID, absPath, resolvedPath, hostname); regErr == nil {
		// Only clean up legacy config if registration succeeded
		_ = removeLegacyConfig(arcHome, cwd)
	}

	return cfg.WorkspaceID, ProjectSourceProject, "", nil
}

// outputResult writes data as indented JSON to stdout when --json is set.
func outputResult(data any) {
	if outputJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(data)
	} else {
		_, _ = fmt.Println(data)
	}
}

// rootCmd is the top-level Cobra command for the arc CLI.
var rootCmd = &cobra.Command{
	Use:     "arc",
	Short:   "arc CLI - central issue tracking",
	Long:    `arc is a central issue tracking server for AI-assisted coding workflows.`,
	Version: version.Info(),
}

func init() {
	rootCmd.PersistentFlags().StringVarP(
		&serverURL, "server", "s", "",
		"Server URL (env: ARC_SERVER, default: http://localhost:7432)")
	rootCmd.PersistentFlags().StringVar(&projectID, flagProject, "", "Project ID")
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "Output as JSON")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Config file path")

	// Add commands
	rootCmd.AddCommand(projectCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(closeCmd)
	rootCmd.AddCommand(readyCmd)
	rootCmd.AddCommand(roadmapCmd)
	rootCmd.AddCommand(blockedCmd)
	rootCmd.AddCommand(depCmd)
	rootCmd.AddCommand(labelCmd)
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(selfCmd)
}

// ============ Project Commands ============

// projectCmd is the parent command for project management.
var projectCmd = &cobra.Command{
	Use:   cmdProject,
	Short: "Manage projects",
}

func init() {
	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectCreateCmd)
	projectCmd.AddCommand(projectDeleteCmd)
}

// whichResult is the JSON shape returned by the which command.
type whichResult struct {
	ProjectID   string   `json:"project_id"`
	ProjectName string   `json:"project_name,omitempty"`
	Source      string   `json:"source"`
	VCS         []string `json:"vcs"` // always present; [] when none
	PlansDir    string   `json:"plans_dir,omitempty"`
	PlansType   string   `json:"plans_type,omitempty"`
	PlansSource string   `json:"plans_source,omitempty"`
	Warning     string   `json:"warning,omitempty"`
}

// whichCmd shows the active project and how it was resolved.
var whichCmd = &cobra.Command{
	Use:   "which",
	Short: "Show which project is active and how it was resolved",
	Long: `Display the currently active project and its resolution source.

This helps debug project resolution issues by showing:
- The active project ID and name
- Where the project was resolved from (flag or project config)
- The project config file path
- Any warnings about the configuration`,
	RunE: func(cmd *cobra.Command, args []string) error {
		wsID, source, warning, err := resolveProject()
		if err != nil {
			return err
		}

		// Try to get project details
		c, clientErr := getClient()
		var wsName, wsPrefix string
		if clientErr == nil {
			if proj, wsErr := c.GetProject(wsID); wsErr == nil {
				wsName = proj.Name
				wsPrefix = proj.Prefix
			}
		}

		cwd, _ := os.Getwd()
		systems := vcs.Detect(cwd)

		// Plans resolution is non-fatal: a failure leaves which's other
		// output intact, just without plans_dir/plans_type/plans_source. Record
		// the reason in warning so a misconfigured new CLI is distinguishable
		// from an old CLI that never emitted these fields.
		plansDir, plansType, plansSource, plansErr := resolvePlansForProject(wsID, wsName, wsPrefix)
		if plansErr != nil {
			plansDir, plansType, plansSource = "", "", ""
			note := fmt.Sprintf("plans resolution failed: %v", plansErr)
			if warning == "" {
				warning = note
			} else {
				warning += "\n" + note
			}
		}

		if outputJSON {
			result := whichResult{
				ProjectID:   wsID,
				ProjectName: wsName,
				Source:      source.String(),
				VCS:         systems,
				PlansDir:    plansDir,
				PlansType:   plansType,
				PlansSource: plansSource,
				Warning:     warning,
			}
			outputResult(result)
			return nil
		}

		// Human-readable output
		if wsName != "" {
			fmt.Printf("Project: %s (%s)\n", wsName, wsID)
		} else {
			fmt.Printf("Project: %s\n", wsID)
		}
		fmt.Printf("Source: %s\n", source)

		if len(systems) > 0 {
			fmt.Printf("VCS: %s\n", strings.Join(systems, ", "))
		}

		if plansDir != "" {
			fmt.Printf("Plans: %s (%s, from %s)\n", plansDir, plansType, plansSource)
		}

		if source == ProjectSourceProject {
			fmt.Printf("Config: legacy ~/.arc/projects/ config\n")
		}

		if warning != "" {
			_, _ = fmt.Fprintln(os.Stderr)
			_, _ = fmt.Fprintln(os.Stderr, warning)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(whichCmd)
}

// projectListCmd lists all projects on the server.
var projectListCmd = &cobra.Command{
	Use:   cmdList,
	Short: "List all projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		projects, err := c.ListProjects()
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(projects)
			return nil
		}

		if len(projects) == 0 {
			_, _ = fmt.Println("No projects found. Create one with: arc project create <name>")
			return nil
		}

		// Create a tabwriter for aligned columns
		w := tabwriter.NewWriter(os.Stdout, 0, 0, tabwriterPadding, ' ', 0)
		_, _ = fmt.Fprintln(w, "NAME\tPREFIX\tID\tDESCRIPTION")
		_, _ = fmt.Fprintln(w, "────\t──────\t──\t───────────")

		for _, p := range projects {
			desc := p.Description
			if desc == "" {
				desc = "-"
			}
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Name, p.Prefix, p.ID, desc)
		}
		_ = w.Flush()
		return nil
	},
}

// projectCreateCmd creates a new project on the server.
var projectCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		path, _ := cmd.Flags().GetString("path")
		description, _ := cmd.Flags().GetString("description")

		// Generate prefix from path if provided, otherwise from project name
		var prefix string
		if path != "" {
			prefix, err = project.GeneratePrefix(path)
			if err != nil {
				return fmt.Errorf("generate prefix: %w", err)
			}
		} else {
			// No path - generate prefix from project name with hash
			prefix = project.GeneratePrefixFromName(args[0])
		}

		proj, err := c.CreateProject(args[0], prefix, description)
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(proj)
			return nil
		}

		fmt.Printf("Created project: %s (%s)\n", proj.Name, proj.ID)
		return nil
	},
}

func init() {
	projectCreateCmd.Flags().String("path", "", "Associated directory path")
	projectCreateCmd.Flags().StringP("description", "d", "", "Project description")
}

// projectDeleteCmd deletes a project from the server.
var projectDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		if err := c.DeleteProject(args[0]); err != nil {
			return err
		}

		fmt.Printf("Deleted project: %s\n", args[0])
		return nil
	},
}

// ============ Issue Commands ============

// listCmd lists issues in the active project with optional filters.
var listCmd = &cobra.Command{
	Use:   cmdList,
	Short: "List issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getProjectID()
		if err != nil {
			return err
		}

		status, _ := cmd.Flags().GetString("status")
		issueType, _ := cmd.Flags().GetString("type")
		query, _ := cmd.Flags().GetString("query")
		limit, _ := cmd.Flags().GetInt("limit")
		parentID, _ := cmd.Flags().GetString("parent")

		issues, err := c.ListIssues(wsID, client.ListIssuesOptions{
			Status: status,
			Type:   issueType,
			Query:  query,
			Limit:  limit,
			Parent: parentID,
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
	listCmd.Flags().StringP("query", "q", "", "Search query")
	listCmd.Flags().IntP("limit", "l", defaultListLimit, "Max results")
	listCmd.Flags().String("parent", "", "Filter by parent issue ID")
}

// createCmd creates a new issue in the active project.
var createCmd = &cobra.Command{
	Use:   "create [title]",
	Short: "Create a new issue",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getProjectID()
		if err != nil {
			return err
		}

		priority, _ := cmd.Flags().GetInt("priority")
		issueType, _ := cmd.Flags().GetString("type")
		description, _ := cmd.Flags().GetString("description")
		useStdin, _ := cmd.Flags().GetBool("stdin")
		if useStdin && description == "" {
			content, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("reading stdin: %w", err)
			}
			description = strings.TrimSpace(string(content))
		}
		parentID, _ := cmd.Flags().GetString("parent")
		titleFlag, _ := cmd.Flags().GetString("title")

		afterID, _ := cmd.Flags().GetString("after")
		parallel, _ := cmd.Flags().GetBool("parallel")
		if err := validateSequencingFlags(issueType, afterID, parallel); err != nil {
			return err
		}

		title := titleFlag
		if len(args) > 0 {
			title = args[0]
		}
		if title == "" {
			return errors.New("title is required (positional arg or --title flag)")
		}

		issue, err := c.CreateIssue(wsID, client.CreateIssueRequest{
			Title:       title,
			Description: description,
			Priority:    priority,
			IssueType:   issueType,
			ParentID:    parentID,
		})
		if err != nil {
			return err
		}

		// Apply labels (warn on failure, don't fail the create)
		labels, _ := cmd.Flags().GetStringSlice("label")
		if parallel {
			labels = append(labels, "parallel")
		}
		for _, lbl := range labels {
			if labelErr := c.AddLabelToIssue(wsID, issue.ID, lbl); labelErr != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to add label %q: %v\n", lbl, labelErr)
			}
		}

		seqMsg := sequenceCreatedMilestone(c, wsID, issue.ID, createSequencingOpts{
			issueType: issueType,
			parentID:  parentID,
			afterID:   afterID,
			parallel:  parallel,
		})

		if outputJSON {
			if seqMsg != "" {
				_, _ = fmt.Fprintln(os.Stderr, seqMsg)
			}
			// Re-fetch with details so JSON includes labels and any
			// auto-sequenced blocks dependency.
			if seqMsg != "" || len(labels) > 0 {
				details, fetchErr := c.GetIssueDetails(wsID, issue.ID)
				if fetchErr == nil {
					outputResult(details)
					return nil
				}
				// Fall back to the original issue if re-fetch fails
			}
			outputResult(issue)
			return nil
		}

		fmt.Printf("Created: %s\n", issue.ID)
		if seqMsg != "" {
			fmt.Println(seqMsg)
		}
		return nil
	},
}

func init() {
	createCmd.Flags().String("title", "", "Issue title (alternative to positional arg)")
	createCmd.Flags().IntP("priority", "p", defaultPriority, "Priority (0-4)")
	createCmd.Flags().StringP("type", "t", "task", "Issue type")
	createCmd.Flags().StringP("description", "d", "", "Description")
	createCmd.Flags().Bool("stdin", false, "Read description from stdin")
	createCmd.Flags().String("parent", "", "Parent issue ID (creates child with .N suffix)")
	createCmd.Flags().StringSlice("label", nil, "Label to apply (repeatable)")
	createCmd.Flags().String("after", "", "Milestone only: sequence after this issue (blocks dep)")
	createCmd.Flags().Bool("parallel", false, "Milestone only: parallel track, no sequencing dependency")
}

// validateSequencingFlags checks that --after and --parallel are used
// correctly: they are mutually exclusive, and both apply only to milestones.
func validateSequencingFlags(issueType, afterID string, parallel bool) error {
	if afterID != "" && parallel {
		return errors.New("--after and --parallel are mutually exclusive")
	}
	if (afterID != "" || parallel) && issueType != string(types.TypeMilestone) {
		return errors.New("--after/--parallel only apply to --type=milestone")
	}
	return nil
}

// createSequencingOpts bundles the create-time flags relevant to milestone
// sequencing.
type createSequencingOpts struct {
	issueType string
	parentID  string
	afterID   string
	parallel  bool
}

// sequenceCreatedMilestone auto-wires a milestone's sequencing dependency
// after a successful create and returns a confirmation line describing what
// happened (empty if there's nothing to report). The caller prints this
// after the create output, or routes it to stderr in JSON mode, so stdout
// order and JSON purity stay the caller's concern. A parallel milestone opts
// out entirely; a sequencing failure is reported as a warning on stderr
// rather than failing the already-completed create.
func sequenceCreatedMilestone(c *client.Client, wsID, newID string, opts createSequencingOpts) string {
	if opts.parallel {
		return "Parallel track (no sequencing dependency)"
	}
	if opts.issueType != string(types.TypeMilestone) || opts.parentID == "" {
		return ""
	}
	msg, err := sequenceMilestone(c, wsID, newID, opts.parentID, opts.afterID)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr,
			"warning: issue created but auto-sequencing failed: %v\n"+
				"Add manually: arc dep add %s <previous-milestone>\n",
			err, newID)
		return ""
	}
	return msg
}

// sequenceMilestone adds a "blocks" dependency from the newly created
// milestone to its sequencing predecessor: the sibling named by afterID if
// given, otherwise the tail of the sequenced chain among sibling milestones.
// It returns a confirmation line for the caller to print (empty if no
// dependency was added).
//
// Known limitation: two clients creating milestones under the same parent
// concurrently can compute the same tail and fan in on one predecessor
// instead of chaining. Fixing this needs server-side sequencing; accepted
// for now (documented in the epic).
func sequenceMilestone(c *client.Client, wsID, newID, parentID, afterID string) (string, error) {
	if afterID != "" {
		if err := c.AddDependency(wsID, newID, afterID, string(types.DepBlocks)); err != nil {
			return "", err
		}
		return fmt.Sprintf(
			"Sequenced after %s (blocks dependency); use --parallel or 'arc dep remove' to change", afterID,
		), nil
	}

	siblings, err := c.ListIssues(wsID, client.ListIssuesOptions{
		Parent: parentID,
		Type:   string(types.TypeMilestone),
		Limit:  milestoneSiblingsLimit, // above the server's default cap, so large releases aren't truncated
	})
	if err != nil {
		return "", err
	}

	candidates, blocked, err := milestoneCandidates(c, wsID, siblings, newID)
	if err != nil {
		return "", err
	}

	tail := latestUnblockedCandidate(candidates, blocked)
	if tail == nil {
		return "", nil
	}

	if err := c.AddDependency(wsID, newID, tail.ID, string(types.DepBlocks)); err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"Sequenced after %s (blocks dependency); use --parallel or 'arc dep remove' to change", tail.ID,
	), nil
}

// milestoneCandidates fetches dependency and label details for every sibling
// milestone except newID, returning the sequenced-chain candidates (those
// without the "parallel" label) and the set of IDs that another sibling's
// "blocks" dependency already targets (i.e. are not the chain tail).
func milestoneCandidates(
	c *client.Client, wsID string, siblings []*types.Issue, newID string,
) ([]*types.IssueDetails, map[string]bool, error) {
	var candidates []*types.IssueDetails
	blocked := make(map[string]bool)

	for _, sibling := range siblings {
		if sibling.ID == newID {
			continue
		}
		details, err := c.GetIssueDetails(wsID, sibling.ID)
		if err != nil {
			return nil, nil, err
		}
		for _, dep := range details.Dependencies {
			if dep.Type == types.DepBlocks {
				blocked[dep.DependsOnID] = true
			}
		}
		if !hasLabel(details.Labels, "parallel") {
			candidates = append(candidates, details)
		}
	}

	return candidates, blocked, nil
}

// hasLabel reports whether target is present in labels.
func hasLabel(labels []string, target string) bool {
	for _, l := range labels {
		if l == target {
			return true
		}
	}
	return false
}

// latestUnblockedCandidate returns the candidate that no other candidate
// blocks on, breaking ties by the most recent CreatedAt. It returns nil if
// every candidate is already blocked on or there are no candidates.
func latestUnblockedCandidate(candidates []*types.IssueDetails, blocked map[string]bool) *types.IssueDetails {
	var tail *types.IssueDetails
	for _, candidate := range candidates {
		if blocked[candidate.ID] {
			continue
		}
		if tail == nil || candidate.CreatedAt.After(tail.CreatedAt) {
			tail = candidate
		}
	}
	return tail
}

// showCmd displays full details for a single issue.
var showCmd = &cobra.Command{
	Use:   useShowID,
	Short: "Show issue details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		details, err := c.GetIssueDetailsByID(args[0])
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
		if details.AISessionID != "" {
			fmt.Printf("AI Session: %s\n", details.AISessionID)
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

		return nil
	},
}

// updateCmd modifies fields on an existing issue.
var updateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		updates := make(map[string]any)

		if val, _ := cmd.Flags().GetString("status"); val != "" {
			updates["status"] = val
		}
		if val, _ := cmd.Flags().GetString("title"); val != "" {
			updates["title"] = val
		}
		if cmd.Flags().Changed("priority") {
			val, _ := cmd.Flags().GetInt("priority")
			updates["priority"] = val
		}
		if val, _ := cmd.Flags().GetString("type"); val != "" {
			updates["issue_type"] = val
		}
		description, _ := cmd.Flags().GetString("description")
		useStdin, _ := cmd.Flags().GetBool("stdin")
		if useStdin && description == "" {
			content, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("reading stdin: %w", err)
			}
			description = strings.TrimSpace(string(content))
		}
		if description != "" {
			updates["description"] = description
		}

		// Handle --take flag
		take, _ := cmd.Flags().GetBool("take")
		sessionID, _ := cmd.Flags().GetString("session-id")

		if sessionID != "" && !take {
			return errors.New("--session-id requires --take")
		}

		if take {
			// Resolve session ID: explicit flag > env var > error
			if sessionID == "" {
				sessionID = os.Getenv("ARC_SESSION_ID")
			}
			if sessionID == "" {
				return errors.New("no session ID available — set ARC_SESSION_ID or pass --session-id")
			}
			updates["ai_session_id"] = sessionID
			// Set status to in_progress unless user explicitly passed --status
			if !cmd.Flags().Changed("status") {
				updates["status"] = string(types.StatusInProgress)
			}
		}

		labelsAdd, _ := cmd.Flags().GetStringSlice("label-add")
		labelsRemove, _ := cmd.Flags().GetStringSlice("label-remove")

		if len(updates) == 0 && len(labelsAdd) == 0 && len(labelsRemove) == 0 {
			return errors.New("no updates specified")
		}

		// Apply field updates first (if any)
		var issue *types.Issue
		if len(updates) > 0 {
			issue, err = c.UpdateIssueByID(args[0], updates)
			if err != nil {
				return err
			}
		}

		// Apply label additions
		for _, lbl := range labelsAdd {
			if labelErr := c.AddLabelToIssueByID(args[0], lbl); labelErr != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to add label %q: %v\n", lbl, labelErr)
			}
		}

		// Apply label removals
		for _, lbl := range labelsRemove {
			if labelErr := c.RemoveLabelFromIssueByID(args[0], lbl); labelErr != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to remove label %q: %v\n", lbl, labelErr)
			}
		}

		if outputJSON {
			// If labels were changed or issue is nil, re-fetch with details
			if len(labelsAdd) > 0 || len(labelsRemove) > 0 || issue == nil {
				details, fetchErr := c.GetIssueDetailsByID(args[0])
				if fetchErr == nil {
					outputResult(details)
					return nil
				}
				// Fall back to the original issue if re-fetch fails
			}
			if issue != nil {
				outputResult(issue)
			}
			return nil
		}

		fmt.Printf("Updated: %s\n", args[0])
		return nil
	},
}

func init() {
	updateCmd.Flags().String("status", "", "New status")
	updateCmd.Flags().String("title", "", "New title")
	updateCmd.Flags().IntP("priority", "p", 0, "New priority")
	updateCmd.Flags().StringP("type", "t", "", "New type")
	updateCmd.Flags().StringP("description", "d", "", "New description")
	updateCmd.Flags().Bool("stdin", false, "Read description from stdin")
	updateCmd.Flags().Bool("take", false,
		"Take this issue for the current AI session (sets ai_session_id + status=in_progress)")
	updateCmd.Flags().String("session-id", "", "Explicit AI session ID (used with --take)")
	updateCmd.Flags().StringSlice("label-add", nil, "Label to add (repeatable)")
	updateCmd.Flags().StringSlice("label-remove", nil, "Label to remove (repeatable)")
}

// closeCmd marks one or more issues as closed.
var closeCmd = &cobra.Command{
	Use:   "close <id> [ids...]",
	Short: "Close one or more issues",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		reason, _ := cmd.Flags().GetString("reason")
		cascade, _ := cmd.Flags().GetBool("cascade")

		for _, id := range args {
			issue, err := c.CloseIssueByID(id, reason, cascade)
			if err != nil {
				var openChildrenErr *types.OpenChildrenError
				if errors.As(err, &openChildrenErr) {
					_, _ = fmt.Fprint(os.Stderr, formatOpenChildrenError(openChildrenErr))
				} else {
					_, _ = fmt.Fprintf(os.Stderr, "Failed to close %s: %v\n", id, err)
				}
				continue
			}
			fmt.Printf("Closed: %s\n", issue.ID)
		}

		return nil
	},
}

func init() {
	closeCmd.Flags().StringP("reason", "r", "", "Close reason")
	closeCmd.Flags().Bool("cascade", false, "Close all open child issues recursively")
}

// formatOpenChildrenError formats an OpenChildrenError into a human-readable
// multi-line message showing which children are open and how to resolve.
func formatOpenChildrenError(e *types.OpenChildrenError) string {
	var b strings.Builder

	// Header
	plural := "issues"
	if len(e.Children) == 1 {
		plural = "issue"
	}
	_, _ = fmt.Fprintf(&b, "Error: cannot close %s: %d open child %s must be closed first\n",
		e.IssueID, len(e.Children), plural)

	// Children list
	_, _ = b.WriteString("\n  Open children:\n")

	// Calculate max widths for alignment
	maxIDLen := 0
	maxTitleLen := 0
	for _, child := range e.Children {
		if len(child.ID) > maxIDLen {
			maxIDLen = len(child.ID)
		}
		if len(child.Title) > maxTitleLen {
			maxTitleLen = len(child.Title)
		}
	}

	for _, child := range e.Children {
		_, _ = fmt.Fprintf(&b, "    %-*s  %-*s  (%s)\n",
			maxIDLen, child.ID,
			maxTitleLen, child.Title,
			child.Status)
	}

	// Hint
	_, _ = b.WriteString("\n  Use --cascade to close all children, or close them individually first.\n")

	return b.String()
}

// readyCmd shows issues that are unblocked and available to work on.
var readyCmd = &cobra.Command{
	Use:   "ready",
	Short: "Show issues ready to work on",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getProjectID()
		if err != nil {
			return err
		}

		limit, _ := cmd.Flags().GetInt("limit")
		sortPolicy, _ := cmd.Flags().GetString("sort")
		under, _ := cmd.Flags().GetString("under")

		issues, err := c.GetReadyWork(wsID, limit, sortPolicy, under)
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
			fmt.Println(formatReadyIssue(issue))
		}

		return nil
	},
}

func init() {
	readyCmd.Flags().IntP("limit", "l", defaultReadyLimit, "Max results")
	readyCmd.Flags().String("sort", "hybrid",
		"Sort policy: hybrid (recent by priority, old by age), "+
			"priority (always by priority), oldest (oldest first)")
	readyCmd.Flags().String("under", "",
		"Only show ready work under this issue subtree (release/milestone/epic)")
}

// blockedCmd shows issues that are waiting on unresolved dependencies.
var blockedCmd = &cobra.Command{
	Use:   "blocked",
	Short: "Show blocked issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getProjectID()
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
	blockedCmd.Flags().IntP("limit", "l", defaultBlockedLimit, "Max results")
}

// ============ Dependency Commands ============

// depCmd is the parent command for dependency management.
var depCmd = &cobra.Command{
	Use:   "dep",
	Short: "Manage dependencies",
}

func init() {
	depCmd.AddCommand(depAddCmd)
	depCmd.AddCommand(depRemoveCmd)
}

// depAddCmd creates a dependency between two issues.
var depAddCmd = &cobra.Command{
	Use:   "add <issue> <depends-on>",
	Short: "Add dependency (issue depends on depends-on)",
	Args:  cobra.ExactArgs(depPairArgCount),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		depType, _ := cmd.Flags().GetString("type")
		if depType == "" {
			depType = "blocks"
		}

		if err := c.AddDependencyByID(args[0], args[1], depType); err != nil {
			return err
		}

		fmt.Printf("Added: %s depends on %s (%s)\n", args[0], args[1], depType)
		return nil
	},
}

func init() {
	depAddCmd.Flags().StringP("type", "t", "blocks", "Dependency type (blocks, parent-child, related, discovered-from)")
}

// depRemoveCmd removes a dependency between two issues.
var depRemoveCmd = &cobra.Command{
	Use:   "remove <issue> <depends-on>",
	Short: "Remove dependency",
	Args:  cobra.ExactArgs(depPairArgCount),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		if err := c.RemoveDependencyByID(args[0], args[1]); err != nil {
			return err
		}

		fmt.Printf("Removed: %s no longer depends on %s\n", args[0], args[1])
		return nil
	},
}

// ============ Stats Command ============

// statsCmd displays aggregate statistics for the active project.
var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show project statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getProjectID()
		if err != nil {
			return err
		}

		stats, err := c.GetProjectStats(wsID)
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

// formatIssue returns a beads-style formatted issue line.
//
//nolint:revive // argument-limit: all fields needed for single-line formatting
func formatIssue(id, status, issueType string, priority int, title string, labels []string) string {
	// Status icon
	icon := statusIconOpen
	switch status {
	case string(types.StatusInProgress):
		icon = "\u25d0" // ◐
	case "blocked":
		icon = "\u25cc" // ◌
	case "closed":
		icon = statusIconClosed
	case "deferred":
		icon = "\u25c7" // ◇
	}

	// Priority badge - color-coded by priority level
	priorityStr := colorPriority(priority)

	// Labels as space-separated in brackets
	labelStr := ""
	if len(labels) > 0 {
		labelStr = " [" + strings.Join(labels, " ") + "]"
	}

	return fmt.Sprintf("%s %s [%s] [%s]%s - %s",
		icon, id, priorityStr, issueType, labelStr, title)
}

// formatBlockedIssue returns a beads-style formatted blocked issue line.
//
//nolint:revive // argument-limit: all fields needed for single-line formatting
func formatBlockedIssue(id, issueType string, priority int, title string, labels []string, blockedByCount int) string {
	// Blocked issues always use blocked icon
	icon := "\u25cc" // ◌

	// Priority badge - color-coded
	priorityStr := colorPriority(priority)

	// Labels
	labelStr := ""
	if len(labels) > 0 {
		labelStr = " [" + strings.Join(labels, " ") + "]"
	}

	return fmt.Sprintf("%s %s [%s] [%s]%s - %s (blocked by %d)",
		icon, id, priorityStr, issueType, labelStr, title, blockedByCount)
}

// formatPlanInfo returns a formatted string describing a plan.
// Returns an empty string if the plan is nil.
func formatPlanInfo(plan *types.Plan) string {
	if plan == nil {
		return ""
	}
	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "Plan [%s]:\n", plan.Status)
	_, _ = fmt.Fprintf(&sb, "  %s\n", plan.FilePath)
	if plan.Status == "draft" {
		_, _ = sb.WriteString("  (pending review)\n")
	}
	return sb.String()
}

// formatPendingPlanNotice returns a notice string for pending plan reviews.
// Returns an empty string if count is zero.
func formatPendingPlanNotice(count int) string {
	if count == 0 {
		return ""
	}
	return fmt.Sprintf("⚠ %d plan(s) pending review", count)
}

// colorPriority returns a color-coded priority string.
// P0=red (critical), P1=yellow (high), P2=cyan (normal), P3=blue (low), P4=dim (minimal).
func colorPriority(priority int) string {
	label := fmt.Sprintf("P%d", priority)
	switch priority {
	case 0:
		return color.New(color.FgRed, color.Bold).Sprint(label)
	case 1:
		return color.New(color.FgYellow).Sprint(label)
	case priorityNormal:
		return color.New(color.FgCyan).Sprint(label)
	case priorityLow:
		return color.New(color.FgBlue).Sprint(label)
	default:
		return color.New(color.FgMagenta).Sprint(label)
	}
}
