package docs

import (
	_ "embed"
)

//go:embed WORKFLOWS.md
var Workflows string

//go:embed DEPENDENCIES.md
var Dependencies string

//go:embed BOUNDARIES.md
var Boundaries string

//go:embed RESUMABILITY.md
var Resumability string

//go:embed PLUGIN.md
var Plugin string

//go:embed PLANS.md
var Plans string

// Overview is generated, not embedded
var Overview = `# Arc Documentation

Arc is a central issue tracking system for AI-assisted coding workflows.

## Available Topics

  arc docs workflows     - Step-by-step workflow checklists
  arc docs dependencies  - Dependency types and when to use each
  arc docs boundaries    - When to use arc vs TodoWrite
  arc docs resumability  - Writing notes that survive compaction
  arc docs plans         - Plan patterns (inline, parent-epic, shared)
  arc docs plugin        - Claude Code plugin and Codex CLI integration guide

## Quick Reference

  arc onboard           - Get workspace orientation
  arc ready             - Find available work
  arc create "title"    - Create new issue
  arc show <id>         - View issue details
  arc close <id>        - Complete an issue
  arc plan set <id>     - Set inline plan on issue
  arc plan show <id>    - View all plans for issue
  arc which             - Show active workspace

## More Help

  arc quickstart        - Quick start guide
  arc prime             - Full workflow context
  arc --help            - All commands
`
