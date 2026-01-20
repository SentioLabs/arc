package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/sentiolabs/arc/internal/docs"
	"github.com/spf13/cobra"
)

var validDocTopics = []string{"workflows", "dependencies", "boundaries", "resumability", "plugin"}

var docsCmd = &cobra.Command{
	Use:   "docs [topic]",
	Short: "Display documentation for arc",
	Long: `Display documentation about arc workflows and usage.

Available topics:
  workflows     - Step-by-step workflow checklists
  dependencies  - Dependency types and when to use each
  boundaries    - When to use arc vs TodoWrite
  resumability  - Writing notes that survive compaction
  plugin        - Claude Code plugin installation guide

Run without a topic to see an overview.`,
	Args:              cobra.MaximumNArgs(1),
	RunE:              runDocs,
	ValidArgsFunction: completeDocTopics,
}

func init() {
	rootCmd.AddCommand(docsCmd)
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
	case "plugin":
		content = docs.Plugin
	default:
		return fmt.Errorf("unknown topic: %s\nAvailable topics: %s",
			topic, strings.Join(validDocTopics, ", "))
	}

	fmt.Fprint(os.Stdout, content)
	return nil
}

func completeDocTopics(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return validDocTopics, cobra.ShellCompDirectiveNoFileComp
}
