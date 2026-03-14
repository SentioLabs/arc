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
  arc docs plans         - Plan workflow (create, review, approve)
  arc docs plugin        - Claude Code plugin and Codex CLI integration guide

## Quick Reference

  arc onboard           - Get project orientation
  arc ready             - Find available work
  arc create "title"    - Create new issue
  arc show <id>         - View issue details
  arc close <id>        - Complete an issue
  arc plan create --file <path> - Register plan for review
  arc plan show <plan-id>       - View plan content and status
  arc plan approve <plan-id>    - Approve plan
  arc plan reject <plan-id>     - Reject plan
  arc plan comments <plan-id>   - List review comments
  arc which             - Show active project
  arc paths             - Manage workspace path registrations
  arc project list      - List all projects
  arc project rename    - Rename current project
  arc project merge     - Merge projects together
  arc db backup         - Create database backup
  arc self update       - Update arc CLI

## More Help

  arc quickstart        - Quick start guide
  arc prime             - Full workflow context
  arc --help            - All commands
`
