# Plan Simplification Design

**Date:** 2026-03-14
**Status:** Approved

## Problem

The current plan system was built across two epics ("Plan Review — unified plan storage with web editor and approval workflow" and "Plans Web UI — list, detail viewer, and issue linking") and introduced significant complexity for little value:

- A dedicated `plans` table with 11 storage methods, 8 API endpoints, a full CLI command group, and two web UI pages (list + detail with split-pane editor)
- Plans as first-class entities scoped to projects and linked to issues
- Arc is 99% used by AI agents — the plan infrastructure is over-engineered for its actual use case

The only real need for plans is **review and approval** of a design before implementation begins. The current system treats plans as persistent, issue-linked entities when they should be lightweight, ephemeral review artifacts.

## Solution

Replace the current plan system with a simplified ephemeral plan review workflow:

1. **Remove** the current plan infrastructure entirely (table, API, CLI, web UI pages)
2. **Introduce** a lightweight ephemeral plan that exists only for the review/approval cycle
3. **Store plan content** on the filesystem (`docs/plans/`), with only metadata and comments in the DB
4. **Move design content's permanent home** back to the issue/epic `description` field (markdown)
5. **Add a web planner** — a single-page review UI with rendered markdown, line-level commenting, and approve/reject actions

## Design Decisions

- **Ephemeral, not persistent:** Plans exist only for the brainstorm → approval cycle. Once approved, the content is written to the issue description by the plan skill. The artifact can be cleaned up or left on disk — it's no longer authoritative.
- **Filesystem for content, DB for metadata:** The markdown file is the source of truth for content. The DB stores only status, file path reference, and review comments. This keeps the data model minimal and lets users edit the plan in any editor.
- **Decoupled from issues:** The ephemeral plan has no `issue_id` or project association. The brainstorm skill decides the right issue structure (epic, few tasks, single issue) *after* approval, preserving flexibility for both large and small scopes.
- **Single line number for comments:** A starting line number is sufficient for the AI to locate what a comment refers to. The comment text provides context. Line ranges would double UI complexity for minimal value. If a comment is too vague, the review loop catches it.
- **Reuse the `plans` table name:** "Ephemeral" is an implementation detail, not a naming concern. Drop and recreate with the simpler schema.
- **CLI as orchestrator, web as review surface:** Same pattern as Claude Code + IDE. The brainstorm skill presents options in the CLI; the user optionally reviews in the web UI; the user tells the CLI what happened. No websockets or polling needed.

## Data Layer

### Migration (next sequence number)

```sql
-- Drop the current plans infrastructure
DROP INDEX IF EXISTS idx_plans_project;
DROP INDEX IF EXISTS idx_plans_status;
DROP INDEX IF EXISTS idx_plans_issue;
DROP TABLE IF EXISTS plans;

-- New simplified plans table
CREATE TABLE plans (
    id TEXT PRIMARY KEY,
    file_path TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Plan comments (line-level and overall feedback)
CREATE TABLE plan_comments (
    id TEXT PRIMARY KEY,
    plan_id TEXT NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    line_number INTEGER,
    content TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_plan_comments_plan ON plan_comments(plan_id);
```

**Status values:** `draft`, `in_review`, `approved`, `rejected`

**`line_number`:** Nullable integer. `NULL` means overall feedback; a value anchors the comment to a specific line in the markdown file.

### Types (`internal/types/types.go`)

Remove: `Plan` (current), `PlanContext`, plan status constants.

Add:

```go
const (
    PlanStatusDraft    = "draft"
    PlanStatusInReview = "in_review"
    PlanStatusApproved = "approved"
    PlanStatusRejected = "rejected"
)

type Plan struct {
    ID        string    `json:"id"`
    FilePath  string    `json:"file_path"`
    Status    string    `json:"status"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

type PlanComment struct {
    ID         string    `json:"id"`
    PlanID     string    `json:"plan_id"`
    LineNumber *int      `json:"line_number,omitempty"`
    Content    string    `json:"content"`
    CreatedAt  time.Time `json:"created_at"`
}
```

### Storage Interface (`internal/storage/storage.go`)

Remove all 11 current plan methods. Replace with:

```go
// Plans
CreatePlan(ctx context.Context, plan *types.Plan) error
GetPlan(ctx context.Context, id string) (*types.Plan, error)
UpdatePlanStatus(ctx context.Context, id string, status string) error
DeletePlan(ctx context.Context, id string) error

// Plan Comments
CreatePlanComment(ctx context.Context, comment *types.PlanComment) error
ListPlanComments(ctx context.Context, planID string) ([]*types.PlanComment, error)
```

Six methods total, down from 11.

## API Layer

### Endpoints

Remove all current plan routes (8 endpoints across project-scoped, issue-scoped, and global). Replace with:

```
POST   /api/v1/plans                     → create plan (accepts file_path)
GET    /api/v1/plans/:planId             → get plan metadata + file content
PUT    /api/v1/plans/:planId             → update file content (web editor saves)
PATCH  /api/v1/plans/:planId/status      → update status
DELETE /api/v1/plans/:planId             → delete plan + comments
GET    /api/v1/plans/:planId/comments    → list comments
POST   /api/v1/plans/:planId/comments    → add comment
```

Seven endpoints, not scoped to projects or issues.

### Request/Response Types

```go
type createPlanRequest struct {
    FilePath string `json:"file_path"`
}

type updatePlanContentRequest struct {
    Content string `json:"content"`
}

type updatePlanStatusRequest struct {
    Status string `json:"status"`
}

type createPlanCommentRequest struct {
    LineNumber *int   `json:"line_number,omitempty"`
    Content    string `json:"content"`
}
```

**`GET /api/v1/plans/:planId`** returns the plan metadata plus the file content read from disk:

```go
type planResponse struct {
    types.Plan
    Content string `json:"content"`
}
```

**`PUT /api/v1/plans/:planId`** writes the content back to the file on disk.

## CLI Layer

### Commands (`cmd/arc/plan.go`)

Rewrite entirely. New commands:

```
arc plan create --file <path>        → registers plan in DB, returns plan ID
arc plan show <plan-id>              → displays file content + status + comments
arc plan approve <plan-id>           → sets status to approved
arc plan reject <plan-id>            → sets status to rejected
arc plan comments <plan-id>          → lists comments (structured for skill consumption)
```

Remove: `list`, `set` with inline text, `--editor`/`--stdin` flags, issue-linked operations.

### Client (`internal/client/client.go`)

Remove current plan client methods. Replace with:

```go
CreatePlan(filePath string) (*types.Plan, error)
GetPlan(planID string) (*types.PlanWithContent, error)
UpdatePlanContent(planID string, content string) error
UpdatePlanStatus(planID string, status string) error
DeletePlan(planID string) error
ListPlanComments(planID string) ([]*types.PlanComment, error)
CreatePlanComment(planID string, lineNumber *int, content string) (*types.PlanComment, error)
```

## Web UI Layer

### Removal

- Delete `web/src/routes/[projectId]/plans/+page.svelte` (plan list page)
- Delete `web/src/routes/[projectId]/plans/[planId]/+page.svelte` (plan detail page)
- Remove plan-related API client methods from `web/src/lib/api/index.ts`
- Remove plan types from generated OpenAPI types
- Remove any plan references from navigation, badges, or issue detail pages

### New Route: `/planner/:planId`

Single-page review UI:

```
┌─────────────────────────────────────────────────┐
│  Plan: 2026-03-14-auth-redesign.md    [status]  │
├─────────────────────────────────────────────────┤
│                                                  │
│  1 │ # Auth Redesign                             │
│  2 │                                             │
│  3 │ ## Approach                        💬 (1)   │
│  4 │ We'll use OAuth2 with PKCE...               │
│  5 │ ...                                         │
│                                                  │
│  (click any line number to add comment)          │
│                                                  │
├─────────────────────────────────────────────────┤
│  Overall Feedback:                               │
│  ┌───────────────────────────────────────────┐   │
│  │                                           │   │
│  └───────────────────────────────────────────┘   │
│                                                  │
│  [Edit] [Approve] [Submit Review] [Reject]       │
└─────────────────────────────────────────────────┘
```

**Behaviors:**

- **Default view:** Rendered markdown with line numbers. Comment indicators (icon + count) on lines that have comments.
- **Click line number:** Opens a small comment input anchored to that line.
- **Edit button:** Toggles to a raw markdown textarea for the full file. Save writes back via `PUT`.
- **Approve/Reject:** Updates status via `PATCH`. Immediate.
- **Submit Review:** Requires at least one comment (line-level or overall). Sets status to `in_review`.

**V1 scope — functional minimum:**
- No split-pane editor (toggle between rendered and raw)
- No comment threading or resolved/unresolved tracking
- No syntax highlighting in the raw editor
- Comments are a flat list per line

## Skill Updates

### Brainstorm Skill (`claude-plugin/skills/brainstorm/SKILL.md`)

**Current end-of-brainstorm flow:**
1. Design approved in conversation
2. Create epic via `arc-issue-tracker` agent
3. Write design to `arc plan set <epic-id>`
4. Transition to `/arc:plan`

**New flow:**
1. Design approved in conversation
2. Write markdown file to `docs/plans/YYYY-MM-DD-<topic>.md`
3. Run `arc plan create --file docs/plans/YYYY-MM-DD-<topic>.md` → get plan ID
4. Present user with planner URL and CLI options:
   > Plan ready for review: http://localhost:PORT/planner/<plan-id>
   >
   > You can:
   > 1. **Approve** — proceed directly to implementation planning
   > 2. **Review submitted** — I'll read your feedback and revise
5. **If approve:** Transition to `/arc:plan`
6. **If review submitted:** Read comments via `arc plan comments <plan-id>`, read updated file, revise, write updated file, loop back to step 4

**Key change:** No epic/issue creation during brainstorming. The plan is standalone. Issue structure decisions happen in the plan skill.

### Plan Skill (`claude-plugin/skills/plan/SKILL.md`)

**Changes:**
- Read approved design from `arc plan show <plan-id>` (the ephemeral plan file content)
- When creating the epic, write the plan content into the epic's `description` field
- Task descriptions remain self-contained as they are today (no change)
- Remove all references to `arc plan set/show` with issue IDs

### Other Updates

- Update `AGENTS.md` plan references
- Update `CLAUDE.md` if needed
- Update any agent prompts that reference `arc plan set/show`
- Update `arc prime` output if it mentions plans

## File Inventory

### Files to Rewrite
- `internal/types/types.go` — replace Plan/PlanContext types
- `internal/storage/storage.go` — replace plan interface methods
- `internal/storage/sqlite/plans.go` — rewrite implementation
- `internal/storage/sqlite/db/queries/plans.sql` — rewrite queries
- `internal/api/plans.go` — rewrite handlers
- `internal/api/server.go` — update route registration
- `internal/client/client.go` — replace plan client methods
- `cmd/arc/plan.go` — rewrite CLI commands

### Files to Delete
- `web/src/routes/[projectId]/plans/+page.svelte`
- `web/src/routes/[projectId]/plans/[planId]/+page.svelte`
- Any plan-related loader files in those directories

### Files to Create
- New migration SQL file
- `web/src/routes/planner/[planId]/+page.svelte` (planner UI)

### Files to Update
- `claude-plugin/skills/brainstorm/SKILL.md`
- `claude-plugin/skills/plan/SKILL.md`
- `AGENTS.md`
- `web/src/lib/api/index.ts` (remove old, add new plan API methods)
- OpenAPI spec (regenerated)
- Navigation components (remove plan links)
