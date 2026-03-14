# Agentic Teams Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Integrate arc's labels, plans, and dependency graph with Claude Code agent teams so that `teammate:*` labels drive team composition, plans serve as team briefs and acceptance criteria, and a web dashboard shows team activity.

**Architecture:** Arc serves as the strategic context layer (persistent issues, labels, plans, deps) while Claude Code's TaskList handles tactical real-time coordination. The team lead bridges the two: reading arc via `arc team context` to decompose work, spawning teammates with `ARC_TEAMMATE_ROLE` env vars so `arc prime` gives role-filtered context, and syncing results back after verification. A new web Team View renders the same grouped data.

**Tech Stack:** Go (Cobra CLI, Echo API, sqlc storage), SvelteKit 5 (Svelte runes, TypeScript), Claude Code plugin (skills/agents markdown)

**Skills:** Use `use-modern-go` for all Go code. Use `frontend-design` for all Svelte UI work. Use `svelte-autofixer` before finalizing any Svelte component.

---

### Task 1: Enhanced `arc prime` — Role Detection

**Files:**
- Modify: `cmd/arc/prime.go`
- Test: manual — `ARC_TEAMMATE_ROLE=frontend ./bin/arc prime`

**Step 1: Add role flag and env var detection to prime command**

In `cmd/arc/prime.go`, add a `--role` flag alongside the existing `--mcp` flag, and check the `ARC_TEAMMATE_ROLE` env var as fallback.

```go
var primeRole string // add alongside primeFullMode, primeMCPMode

// In init() or flag registration:
primeCmd.Flags().StringVar(&primeRole, "role", "", "Teammate role (lead, frontend, backend, etc.)")
```

In the `RunE` function, after the existing mode checks, add role detection:

```go
// Detect role from flag or env var
role := primeRole
if role == "" {
    role = os.Getenv("ARC_TEAMMATE_ROLE")
}
```

**Step 2: Add team lead output function**

Create `outputTeamLeadContext(w io.Writer)` that outputs:
- Full CLI reference (reuse `outputCLIContext` content)
- Team sync protocol section:
  - "After teammate completes work, verify before closing arc issues"
  - `arc close <id> --reason "completed by <teammate>"`
  - "Use `arc team context <epic-id>` to check progress"

**Step 3: Add teammate output function**

Create `outputTeammateContext(w io.Writer, role string)` that outputs:
- Role identification: "You are the {role} teammate"
- Instruction to focus on issues labeled `teammate:{role}`
- Session close protocol (git commit/push — same as default)
- Concise — no full CLI reference (teammates use TaskList, not arc CLI directly)

**Step 4: Wire role-based output into RunE**

After the existing mode selection (`if primeMCPMode {...} else if primeFullMode {...}`), add:

```go
if role == "lead" {
    return outputTeamLeadContext(os.Stdout)
} else if role != "" {
    return outputTeammateContext(os.Stdout, role)
}
```

This goes before the default output path so existing behavior is unchanged when no role is set.

**Step 5: Build and test**

Run: `make build-quick`

Test all three modes:
```bash
./bin/arc prime                              # default — unchanged
ARC_TEAMMATE_ROLE=lead ./bin/arc prime       # team lead
ARC_TEAMMATE_ROLE=frontend ./bin/arc prime   # teammate
./bin/arc prime --role=backend               # teammate via flag
```

Expected: Each mode outputs distinct context appropriate to the role.

**Step 6: Commit**

```bash
git add cmd/arc/prime.go
git commit -m "feat(cli): add role-aware output to arc prime

Detect ARC_TEAMMATE_ROLE env var or --role flag to output
team-specific context. Lead gets sync protocol, teammates
get role-filtered guidance."
```

---

### Task 2: `arc team context` CLI Command

**Files:**
- Create: `cmd/arc/team.go`
- Modify: `cmd/arc/main.go` (register command)
- Test: manual — `arc team context <epic-id> --json`

**Step 1: Create the team command group**

Create `cmd/arc/team.go` with a parent `teamCmd` and a `teamContextCmd` subcommand. Follow the pattern from `cmd/arc/plan.go`:

```go
var teamCmd = &cobra.Command{
    Use:   "team",
    Short: "Agent team operations",
}

func init() {
    rootCmd.AddCommand(teamCmd)
    teamCmd.AddCommand(teamContextCmd)
}
```

**Step 2: Implement `teamContextCmd`**

The command accepts an optional epic ID argument. It:
1. Resolves the workspace (same pattern as other commands)
2. If epic ID given: fetches the epic's child issues via `ListIssues` with parent filter
3. If no epic ID: fetches all issues with `teammate:*` labels via `ListIssues`
4. Fetches labels for all issues via `GetLabelsForIssues`
5. Groups issues by their `teammate:*` label
6. Fetches plans for the epic (shared) and each issue (inline)
7. Fetches dependencies for each issue
8. Outputs the grouped structure

```go
var teamContextCmd = &cobra.Command{
    Use:   "context [epic-id]",
    Short: "Output team context grouped by teammate roles",
    Args:  cobra.MaximumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        // Implementation
    },
}
```

**Step 3: Define the output struct**

```go
type TeamContext struct {
    Project    string                  `json:"project"`
    Epic       *TeamContextEpic        `json:"epic,omitempty"`
    Roles      map[string]*TeamRole    `json:"roles"`
    Unassigned []TeamContextIssue      `json:"unassigned"`
}

type TeamContextEpic struct {
    ID    string `json:"id"`
    Title string `json:"title"`
    Plan  string `json:"plan,omitempty"`
}

type TeamRole struct {
    Issues []TeamContextIssue `json:"issues"`
}

type TeamContextIssue struct {
    ID       string   `json:"id"`
    Title    string   `json:"title"`
    Priority int      `json:"priority"`
    Status   string   `json:"status"`
    Type     string   `json:"type"`
    Plan     string   `json:"plan,omitempty"`
    Deps     []string `json:"deps,omitempty"`
}
```

**Step 4: Implement grouping logic**

For each issue, check its labels. Labels starting with `teammate:` extract the role name (everything after the prefix). Issues with no `teammate:*` label go into `Unassigned`.

```go
for _, issue := range issues {
    labels := labelsMap[issue.ID]
    role := ""
    for _, l := range labels {
        if strings.HasPrefix(l, "teammate:") {
            role = strings.TrimPrefix(l, "teammate:")
            break
        }
    }
    // ... add to roles map or unassigned
}
```

**Step 5: Add human-readable output**

When `--json` is not set, output a formatted table using `tabwriter`:
```
ROLE         ISSUES  IDS
architect    1       PROJ-7
backend      2       PROJ-8, PROJ-11
frontend     3       PROJ-12, PROJ-14, PROJ-16
unassigned   1       PROJ-20
```

**Step 6: Register in main.go**

The `init()` in `team.go` handles this via `rootCmd.AddCommand(teamCmd)`. Verify it compiles.

**Step 7: Build and test**

Run: `make build-quick`

Test:
```bash
./bin/arc team context --json              # all teammate-labeled issues
./bin/arc team context PROJ-5 --json       # epic children grouped
./bin/arc team context                     # human-readable
```

**Step 8: Commit**

```bash
git add cmd/arc/team.go
git commit -m "feat(cli): add arc team context command

Outputs issues grouped by teammate:* labels with plans and
dependencies. Supports --json for machine consumption and
human-readable table output."
```

---

### Task 3: Team Context API Endpoint

**Files:**
- Create: `internal/api/teams.go`
- Modify: `internal/api/server.go` (register route)
- Test: `internal/api/teams_test.go`

**Step 1: Write the failing test**

Create `internal/api/teams_test.go`:

```go
func TestGetTeamContext(t *testing.T) {
    server, cleanup := testServer(t)
    defer cleanup()

    // Create workspace
    wsID := createTestWorkspace(t, server.echo)

    // Create teammate labels
    createLabel(t, server.echo, "teammate:frontend", "#3b82f6")
    createLabel(t, server.echo, "teammate:backend", "#22c55e")

    // Create epic
    epicID := createTestIssue(t, server.echo, wsID, "Auth System", "epic")

    // Create child issues with labels
    frontendID := createTestIssue(t, server.echo, wsID, "Login form", "task")
    addLabelToIssue(t, server.echo, wsID, frontendID, "teammate:frontend")

    backendID := createTestIssue(t, server.echo, wsID, "Auth API", "task")
    addLabelToIssue(t, server.echo, wsID, backendID, "teammate:backend")

    // Add parent-child deps
    addDependency(t, server.echo, wsID, frontendID, epicID, "parent-child")
    addDependency(t, server.echo, wsID, backendID, epicID, "parent-child")

    // Request team context
    req := httptest.NewRequest(http.MethodGet,
        fmt.Sprintf("/api/v1/projects/%s/team-context?epic_id=%s", wsID, epicID), nil)
    rec := httptest.NewRecorder()
    server.echo.ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
    }

    var ctx TeamContext
    json.Unmarshal(rec.Body.Bytes(), &ctx)

    if len(ctx.Roles["frontend"].Issues) != 1 {
        t.Errorf("expected 1 frontend issue, got %d", len(ctx.Roles["frontend"].Issues))
    }
    if len(ctx.Roles["backend"].Issues) != 1 {
        t.Errorf("expected 1 backend issue, got %d", len(ctx.Roles["backend"].Issues))
    }
}
```

**Step 2: Run test to verify it fails**

Run: `make test`
Expected: FAIL — `TestGetTeamContext` fails because endpoint doesn't exist.

**Step 3: Implement the handler**

Create `internal/api/teams.go` with:

```go
func (s *Server) getTeamContext(c echo.Context) error {
    wsID := c.Param("ws")
    epicID := c.QueryParam("epic_id")

    // Fetch issues (children of epic, or all with teammate:* labels)
    // Group by teammate:* labels
    // Fetch plans and dependencies
    // Return TeamContext struct
}
```

Reuse the same `TeamContext` struct from the CLI command (move it to `internal/types/` or define locally in the API package).

**Step 4: Register the route**

In `internal/api/server.go`, add to the workspace-scoped routes:

```go
ws.GET("/team-context", s.getTeamContext)
```

**Step 5: Run test to verify it passes**

Run: `make test`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/api/teams.go internal/api/teams_test.go internal/api/server.go
git commit -m "feat(api): add team-context endpoint

GET /api/v1/projects/:ws/team-context?epic_id=...
Returns issues grouped by teammate:* labels with plans and deps."
```

---

### Task 4: Team Orchestration Skill

**Files:**
- Create: `claude-plugin/skills/arc-team-deploy/SKILL.md`

**Step 1: Write the skill file**

Create `claude-plugin/skills/arc-team-deploy/SKILL.md` with:
- Frontmatter: name, description, tools needed
- When to invoke: user says "deploy team", "create agent team from arc", or `/arc team-deploy`
- Workflow steps (as documented in design):
  1. Run `arc team context <epic-id> --json`
  2. Parse output and present team composition proposal
  3. After approval, call TeamCreate, TaskCreate (per issue), set deps, spawn teammates
  4. Set `ARC_TEAMMATE_ROLE` env var on each teammate
  5. Assign tasks via TaskUpdate
  6. Define sync protocol for the team lead

**Step 2: Test by reading the skill**

Verify the skill loads correctly by checking Claude Code recognizes it:
```bash
cat claude-plugin/skills/arc-team-deploy/SKILL.md
```

**Step 3: Commit**

```bash
git add claude-plugin/skills/arc-team-deploy/SKILL.md
git commit -m "feat(plugin): add arc team-deploy orchestration skill

Skill for decomposing arc issues into Claude Code agent teams
based on teammate:* labels and shared plans."
```

---

### Task 5: Web Team View — API Client

**Files:**
- Modify: `web/src/lib/api.ts` (add team context fetch function)
- Modify: `web/src/lib/types.ts` (add TeamContext types, or verify auto-generated)

**Step 1: Add TypeScript types**

If not auto-generated from OpenAPI, add types in the appropriate location:

```typescript
interface TeamContext {
    workspace: string;
    epic?: { id: string; title: string; plan?: string };
    roles: Record<string, { issues: TeamContextIssue[] }>;
    unassigned: TeamContextIssue[];
}

interface TeamContextIssue {
    id: string;
    title: string;
    priority: number;
    status: string;
    type: string;
    plan?: string;
    deps?: string[];
}
```

**Step 2: Add API fetch function**

In the API client file, add:

```typescript
export async function getTeamContext(projectId: string, epicId?: string): Promise<TeamContext> {
    const params = epicId ? `?epic_id=${epicId}` : '';
    const response = await fetch(`${baseUrl}/api/v1/projects/${projectId}/team-context${params}`);
    return response.json();
}
```

**Step 3: Commit**

```bash
git add web/src/lib/api.ts
git commit -m "feat(web): add team context API client function"
```

---

### Task 6: Web Team View — Page Components

**Files:**
- Create: `web/src/routes/[workspaceId]/teams/+page.svelte`
- Create: `web/src/lib/components/RoleLane.svelte`
- Create: `web/src/lib/components/TeamIssueCard.svelte`

> **Note:** Use the `frontend-design` skill for UI implementation. Use `svelte-autofixer` before finalizing each component.

**Step 1: Create TeamIssueCard component**

Compact card showing: issue ID, title, priority indicator, type badge, status icon.
If issue has deps, show them as small linked badges.
Follow existing IssueCard patterns but more compact.

**Step 2: Create RoleLane component**

Props: `role: string`, `color: string`, `issues: TeamContextIssue[]`

Vertical column with:
- Header: role name (capitalized) with color accent stripe, issue count badge
- Stacked TeamIssueCards

**Step 3: Create the Teams page**

`web/src/routes/[workspaceId]/teams/+page.svelte`

Layout:
- Epic selector (dropdown of epics in workspace) or URL param
- Epic header bar with plan summary (expandable)
- Horizontal flex of RoleLane columns + Unassigned column
- Fetches data via `getTeamContext(workspaceId, epicId)`

Follow existing page patterns:
- `$state` for data
- `$effect` to reload when workspace/epic changes
- Error handling with existing patterns

**Step 4: Add navigation link**

Add a "Teams" link to the workspace sidebar/navigation (if one exists), following the pattern of the existing "Issues", "Ready", "Blocked", "Labels" nav items.

**Step 5: Run lint and format**

```bash
cd web && bun run lint && bun run format:check
```

Fix any issues.

**Step 6: Run svelte-autofixer on each component**

Use the `svelte-autofixer` MCP tool on each `.svelte` file. Fix any issues until clean.

**Step 7: Commit**

```bash
git add web/src/routes/[workspaceId]/teams/ web/src/lib/components/RoleLane.svelte web/src/lib/components/TeamIssueCard.svelte
git commit -m "feat(web): add Team View dashboard page

Displays issues grouped by teammate:* role labels in a
column layout. Shows epic plan summary and dependency links."
```

---

### Task 7: Update OpenAPI Spec

**Files:**
- Modify: `api/openapi.yaml` (add team-context endpoint)

**Step 1: Add the endpoint definition**

Add the `GET /projects/{ws}/team-context` endpoint with:
- Query param: `epic_id` (optional)
- Response schema: `TeamContext` object

**Step 2: Regenerate types**

```bash
make gen
```

**Step 3: Verify generated types match manually added types**

If web types were manually added in Task 5, verify they match the generated output. Remove manual types if auto-generation covers them.

**Step 4: Commit**

```bash
git add api/openapi.yaml
git commit -m "docs(api): add team-context endpoint to OpenAPI spec"
```

---

### Task 8: Brainstorming/Planning Skill Modifications

**Files:**
- This task documents what skill changes are needed. The actual skill files live in the superpowers plugin, which is external. The integration happens through arc CLI commands that the skills invoke.

**Step 1: Verify arc CLI supports the needed operations**

The skills will need these commands to work:
```bash
arc create "Topic" --type=epic --description="..."     # ✓ exists
arc plan set <epic-id> "plan content"                   # ✓ exists (but needs --stdin flag)
arc create "Step" --type=task --parent=<epic-id>        # ✓ exists (parent via dep add)
arc dep add <child> <parent>                            # ✓ exists
arc label add <id> teammate:frontend                    # needs arc label CLI (follow-up)
```

**Step 2: Add `--stdin` flag to `arc plan set`**

In `cmd/arc/plan.go`, add a `--stdin` flag to `planSetCmd` that reads plan content from stdin:

```go
planSetCmd.Flags().Bool("stdin", false, "Read plan content from stdin")
```

In the RunE, before the editor check:
```go
useStdin, _ := cmd.Flags().GetBool("stdin")
if useStdin {
    content, err := io.ReadAll(os.Stdin)
    if err != nil {
        return fmt.Errorf("reading stdin: %w", err)
    }
    planContent = string(content)
}
```

**Step 3: Test stdin flag**

```bash
echo "This is the plan content" | ./bin/arc plan set PROJ-5 --stdin
arc plan show PROJ-5  # verify content was set
```

**Step 4: Document skill integration points**

Create a brief guide in the plugin for how skills should use arc:

Add to `claude-plugin/commands/team.md`:
```markdown
# arc team

## Subcommands

### arc team context [epic-id]
Output team context grouped by teammate:* labels.
```

**Step 5: Commit**

```bash
git add cmd/arc/plan.go claude-plugin/commands/team.md
git commit -m "feat(cli): add --stdin flag to arc plan set

Allows programmatic plan content input from skills and scripts.
Also adds team command documentation for plugin."
```

---

## Task Dependencies

```
Task 1 (prime)  ──────────────────────┐
                                      ├── Task 4 (skill)
Task 2 (team context CLI) ───────────┤
         │                            │
         ├── Task 3 (API endpoint) ───┤
         │         │                  │
         │         ├── Task 5 (web API client)
         │         │         │
         │         │         ├── Task 6 (web components)
         │         │
         │         ├── Task 7 (OpenAPI)
         │
Task 8 (stdin flag) ── independent
```

Tasks 1, 2, and 8 can run in parallel.
Task 3 depends on Task 2 (shared types/logic).
Tasks 5-6 depend on Task 3.
Task 4 depends on Tasks 1 and 2.
Task 7 depends on Task 3.
