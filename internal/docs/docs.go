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

## Quick Reference

  arc onboard           - Get project orientation
  arc ready             - Find available work
  arc create "title"    - Create new issue
  arc show <id>         - View issue details
  arc close <id>        - Complete an issue
  arc plan create FILE               - Upload durable Markdown revision 1
  arc plan show PLAN --revision N     - Read exact retained revision
  arc plan show PLAN --revision N --review-context-output review.json
  arc plan submit PLAN --revision N --review-context review.json
  arc plan approve PLAN --revision N --review-context approval.json
  arc plan comments PLAN --revision N - Read current and prior feedback
  arc plan resolve ISSUE              - Inspect complete pinned design chain
  arc docs plans                      - Capture review/work context and reconcile
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
