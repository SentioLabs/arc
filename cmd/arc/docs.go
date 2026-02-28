// Package main provides the docs commands for the arc CLI, allowing users to
// browse and search embedded documentation about arc workflows, dependencies,
// boundaries, resumability, and plugin configuration.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/sentiolabs/arc/internal/docs"
	"github.com/sentiolabs/arc/internal/docsearch"
	"github.com/spf13/cobra"
)

// defaultSearchLimit is the default maximum number of documentation search results.
const defaultSearchLimit = 5

// validDocTopics lists the available documentation topic names.
var validDocTopics = []string{"workflows", "dependencies", "boundaries", "resumability", "plans", "plugin"}

// searchLimit controls the maximum number of search results to display.
// searchExact disables fuzzy matching when true.
// searchVerbose shows relevance scores when true.
var (
	searchLimit   int
	searchExact   bool
	searchVerbose bool
)

var docsCmd = &cobra.Command{
	Use:   "docs [topic]",
	Short: "Display documentation for arc",
	Long: `Display documentation about arc workflows and usage.

Available topics:
  workflows     - Step-by-step workflow checklists
  dependencies  - Dependency types and when to use each
  boundaries    - When to use arc vs TodoWrite
  resumability  - Writing notes that survive compaction
  plans         - Plan patterns (inline, parent-epic, shared)
  plugin        - Claude Code plugin and Codex CLI integration guide

Run without a topic to see an overview.`,
	Args:              cobra.MaximumNArgs(1),
	RunE:              runDocs,
	ValidArgsFunction: completeDocTopics,
}

var docsSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search documentation with fuzzy matching",
	Long: `Search arc documentation using BM25 ranking with fuzzy matching.

Fuzzy matching handles typos automatically:
  arc docs search "dependncy"     # finds "dependency" docs
  arc docs search "workflo"       # finds "workflow" docs

Use --exact for precise matching only.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runDocsSearch,
}

func init() {
	rootCmd.AddCommand(docsCmd)
	docsCmd.AddCommand(docsSearchCmd)

	// Search command flags
	docsSearchCmd.Flags().IntVarP(&searchLimit, "limit", "n", defaultSearchLimit, "Maximum number of results")
	docsSearchCmd.Flags().BoolVar(&searchExact, "exact", false, "Disable fuzzy matching")
	docsSearchCmd.Flags().BoolVarP(&searchVerbose, "verbose", "v", false, "Show relevance scores")
}

func runDocs(cmd *cobra.Command, args []string) error {
	topic := ""
	if len(args) > 0 {
		topic = strings.ToLower(args[0])
	}

	var content string
	switch topic {
	case "":
		content = docs.Overview
	case "workflows":
		content = docs.Workflows
	case "dependencies":
		content = docs.Dependencies
	case "boundaries":
		content = docs.Boundaries
	case "resumability":
		content = docs.Resumability
	case "plans":
		content = docs.Plans
	case "plugin":
		content = docs.Plugin
	default:
		return fmt.Errorf("unknown topic: %s\nAvailable topics: %s",
			topic, strings.Join(validDocTopics, ", "))
	}

	fmt.Fprint(os.Stdout, content) //nolint:revive // stdout write errors are not actionable in CLI
	return nil
}

func completeDocTopics(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return validDocTopics, cobra.ShellCompDirectiveNoFileComp
}

func runDocsSearch(cmd *cobra.Command, args []string) error {
	query := strings.Join(args, " ")

	// Build index from embedded docs
	chunks := docsearch.ChunkAllDocs()
	searcher, err := docsearch.NewSearcher(chunks)
	if err != nil {
		return fmt.Errorf("failed to create searcher: %w", err)
	}
	defer searcher.Close()

	// Perform search
	results, err := searcher.Search(query, searchLimit, searchExact)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		_, _ = fmt.Fprintf(os.Stderr, "No results found for %q\n", query)
		_, _ = fmt.Fprintf(os.Stderr, "\nTry:\n")
		_, _ = fmt.Fprintf(os.Stderr, "  - Different keywords\n")
		_, _ = fmt.Fprintf(os.Stderr, "  - arc docs <topic> to browse topics\n")
		return nil
	}

	// Display results
	fmt.Printf("Results for %q:\n\n", query)

	for i, result := range results {
		if searchVerbose {
			fmt.Printf("%d. [%s] %s (%.2f)\n",
				i+1, result.Chunk.Topic, result.Chunk.Heading, result.Score)
		} else {
			fmt.Printf("%d. [%s] %s\n",
				i+1, result.Chunk.Topic, result.Chunk.Heading)
		}
		fmt.Printf("   %s\n\n", result.Chunk.Preview)
	}

	return nil
}
