# Plan Simplification Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the over-engineered plan system with a lightweight ephemeral review workflow — filesystem-backed content, minimal DB metadata, and a web planner UI.

**Architecture:** Drop the current plans table/API/CLI/web pages. Create a simplified plans + plan_comments schema. The server reads/writes markdown files from `docs/plans/` while DB stores only metadata and review comments. A new `/planner/:planId` web route provides the review UI.

**Tech Stack:** Go (Echo, sqlc), SQLite, SvelteKit (Svelte 5 runes), Tailwind CSS, marked (markdown rendering)

**Spec:** `docs/superpowers/specs/2026-03-14-plan-simplification-design.md`

---

## Chunk 1: Data Layer — Migration, Types, Storage, and Tests

### Task 1: Database Migration

**Files:**
- Create: `internal/storage/sqlite/migrations/014_simplify_plans.sql`
- Modify: `internal/storage/sqlite/db/schema.sql` (update canonical schema)

- [ ] **Step 1: Write migration file**

Create `internal/storage/sqlite/migrations/014_simplify_plans.sql`:

```sql
-- Migration 014: Simplify plans to ephemeral review artifacts
-- Drop the current plans infrastructure (from migrations 004 + 013)
DROP INDEX IF EXISTS idx_plans_project;
DROP INDEX IF EXISTS idx_plans_status;
DROP INDEX IF EXISTS idx_plans_issue;
DROP TABLE IF EXISTS plans;

-- New simplified plans table (content lives on filesystem)
CREATE TABLE plans (
    id TEXT PRIMARY KEY,
    file_path TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Plan review comments (line-level and overall feedback)
CREATE TABLE plan_comments (
    id TEXT PRIMARY KEY,
    plan_id TEXT NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    line_number INTEGER,
    content TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_plan_comments_plan ON plan_comments(plan_id);
```

- [ ] **Step 2: Update canonical schema**

In `internal/storage/sqlite/db/schema.sql`, replace the existing `plans` table definition (lines ~154-182) with the new simplified schema from step 1. Remove any indexes or comments related to the old plan model (project_id, issue_id, title, content columns).

- [ ] **Step 3: Verify migration applies cleanly**

Run: `make test`

If any tests fail due to schema changes, that's expected — the test fixes come in later tasks. The migration itself should apply without errors.

- [ ] **Step 4: Commit**

```bash
git add internal/storage/sqlite/migrations/014_simplify_plans.sql internal/storage/sqlite/db/schema.sql
git commit -m "feat: add migration 014 to simplify plans schema"
```

---

### Task 2: Update Types

**Files:**
- Modify: `internal/types/types.go`

- [ ] **Step 1: Remove old plan types and constants**

In `internal/types/types.go`, remove the following:
- `maxPlanTitleLength` constant (line 14)
- `PlanStatusDraft`, `PlanStatusApproved`, `PlanStatusRejected` constants (lines ~410-415)
- The current `Plan` struct (lines ~417-427) with fields: ID, ProjectID, Title, Content, Status, IssueID, CreatedAt, UpdatedAt
- The `Plan.Validate()` method (lines ~429-441)
- The `PlanContext` struct and `HasPlan()` method (lines ~443-465)
- The `PlansMoved` field in `MergeResult` (line ~352)
- If `internal/types/plan_test.go` exists, remove or rewrite it (it tests `PlanContext` and `Validate` which are being removed)

- [ ] **Step 1b: Update merge-related references**

The `PlansMoved` field removal requires updates in:
- `cmd/arc/merge.go`: Remove the line that prints `result.PlansMoved` (search for `PlansMoved`)
- `internal/storage/sqlite/merge.go`: Remove the plan-moving logic from the `MergeProjects` implementation (search for `plans` table references in the merge query)
- `internal/storage/sqlite/merge_test.go`: Remove assertions on `PlansMoved`

- [ ] **Step 2: Add new plan types**

Add the following types (near where the old plan types were removed):

```go
// Plan status constants
const (
	PlanStatusDraft    = "draft"
	PlanStatusInReview = "in_review"
	PlanStatusApproved = "approved"
	PlanStatusRejected = "rejected"
)

// Plan represents an ephemeral review artifact backed by a filesystem markdown file.
type Plan struct {
	ID        string    `json:"id"`
	FilePath  string    `json:"file_path"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PlanComment is a review comment on a plan, optionally anchored to a line number.
type PlanComment struct {
	ID         string    `json:"id"`
	PlanID     string    `json:"plan_id"`
	LineNumber *int      `json:"line_number,omitempty"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

// PlanWithContent combines plan metadata with the file content read from disk.
type PlanWithContent struct {
	Plan
	Content string `json:"content"`
}
```

- [ ] **Step 3: Verify types compile**

Run: `go build ./internal/types/...`

This will fail with downstream errors (storage, API, etc.) — that's expected. The types package itself should compile.

- [ ] **Step 4: Commit**

```bash
git add internal/types/types.go
git commit -m "feat: replace Plan types with simplified ephemeral plan model"
```

---

### Task 3: Update Storage Interface and sqlc Queries

**Files:**
- Modify: `internal/storage/storage.go`
- Rewrite: `internal/storage/sqlite/db/queries/plans.sql`

- [ ] **Step 1: Update storage interface**

In `internal/storage/storage.go`, replace all plan methods (lines 69-79) with:

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

- [ ] **Step 2: Rewrite sqlc queries**

Replace the entire contents of `internal/storage/sqlite/db/queries/plans.sql` with:

```sql
-- name: CreatePlan :exec
INSERT INTO plans (id, file_path, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?);

-- name: GetPlan :one
SELECT id, file_path, status, created_at, updated_at
FROM plans WHERE id = ?;

-- name: UpdatePlanStatus :exec
UPDATE plans SET status = ?, updated_at = ? WHERE id = ?;

-- name: DeletePlan :exec
DELETE FROM plans WHERE id = ?;

-- name: CreatePlanComment :exec
INSERT INTO plan_comments (id, plan_id, line_number, content, created_at)
VALUES (?, ?, ?, ?, ?);

-- name: ListPlanComments :many
SELECT id, plan_id, line_number, content, created_at
FROM plan_comments WHERE plan_id = ? ORDER BY created_at ASC;
```

- [ ] **Step 3: Regenerate sqlc**

Run: `make gen`

This regenerates `internal/storage/sqlite/db/plans.sql.go` with the new query functions. Verify the generated file has the expected functions: `CreatePlan`, `GetPlan`, `UpdatePlanStatus`, `DeletePlan`, `CreatePlanComment`, `ListPlanComments`.

- [ ] **Step 4: Commit**

```bash
git add internal/storage/storage.go internal/storage/sqlite/db/queries/plans.sql internal/storage/sqlite/db/plans.sql.go
git commit -m "feat: simplify plan storage interface and sqlc queries"
```

---

### Task 4: Rewrite Storage Implementation

**Files:**
- Rewrite: `internal/storage/sqlite/plans.go`

- [ ] **Step 1: Write failing tests**

Rewrite `internal/storage/sqlite/plans_test.go` with tests for the new interface. The test file follows the existing pattern in the project (see `issues_test.go` for reference). Each test creates a fresh DB via the test helper.

```go
func TestCreatePlan(t *testing.T) {
	// Create plan with valid fields
	// Verify it can be retrieved with GetPlan
	// Verify all fields match
}

func TestGetPlan_NotFound(t *testing.T) {
	// GetPlan with nonexistent ID returns error
}

func TestUpdatePlanStatus(t *testing.T) {
	// Create plan in draft status
	// Update to in_review, verify
	// Update to approved, verify
}

func TestDeletePlan(t *testing.T) {
	// Create plan, delete it, verify GetPlan returns error
}

func TestCreatePlanComment(t *testing.T) {
	// Create plan, add line-level comment, add overall comment (nil line_number)
	// List comments, verify both returned in order
}

func TestListPlanComments_Empty(t *testing.T) {
	// Create plan with no comments, list returns empty slice
}

func TestDeletePlan_CascadesComments(t *testing.T) {
	// Create plan, add comments, delete plan
	// Verify comments are also deleted (via ListPlanComments returning empty)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/storage/sqlite/ -run TestCreatePlan -v`
Expected: Compilation errors (methods don't exist yet)

- [ ] **Step 3: Implement storage methods**

Rewrite `internal/storage/sqlite/plans.go`. Replace the entire file with implementations of the 6 interface methods. Follow the existing pattern in the file — each method wraps the sqlc-generated query and converts between DB types and domain types.

Key implementation details:
- `CreatePlan`: Takes a `*types.Plan`, calls `s.queries.CreatePlan()` with the fields. ID is generated by the caller (using `project.GeneratePlanID`).
- `GetPlan`: Calls `s.queries.GetPlan()`, converts the DB row to `*types.Plan`. Return an error if not found.
- `UpdatePlanStatus`: Calls `s.queries.UpdatePlanStatus()` with the new status and `time.Now()` for updated_at.
- `DeletePlan`: Calls `s.queries.DeletePlan()`. CASCADE handles comments.
- `CreatePlanComment`: Takes a `*types.PlanComment`, calls `s.queries.CreatePlanComment()`.
- `ListPlanComments`: Calls `s.queries.ListPlanComments()`, converts each row.

For the conversion helper, create a `dbPlanToType` function that maps the sqlc-generated struct to `*types.Plan`, and a `dbPlanCommentToType` for comments. Handle nullable `line_number` using `sql.NullInt64` → `*int` conversion.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/storage/sqlite/ -run "TestCreatePlan|TestGetPlan|TestUpdatePlanStatus|TestDeletePlan|TestCreatePlanComment|TestListPlanComments" -v`
Expected: All PASS

- [ ] **Step 5: Clean up any remaining test references**

Search for `CountPlansByStatus` or other old plan method references in test files:

Run: `grep -r "CountPlansByStatus\|GetPlanByIssueID\|ListPlans\|ListAllPlans\|UpdatePlanContent\|UpdatePlanIssueID\|CreateOrUpdatePlan\|PlanContext\|PlansMoved\|SetPlan\|GetPendingPlan" --include="*.go" -l`

Note: Search the ENTIRE repo, not just `internal/storage/sqlite/`. Known files that need updating:
- `internal/storage/sqlite/show_ready_test.go` — references `CountPlansByStatus`
- `internal/storage/sqlite/merge_test.go` — asserts on `PlansMoved`
- `internal/api/workspace_paths_test.go` — mock store implements old plan interface methods (update mock to implement new 6-method interface)
- `internal/client/client_test.go` — references old client methods (`TestClientSetPlan`, etc.) — rewrite or delete plan-related tests
- `internal/types/plan_test.go` — tests old Plan type (`Title`, `ProjectID` fields) — delete or rewrite
- `tests/integration/plan_test.go` — integration tests reference old commands (`arc plan set`, etc.) — rewrite or delete
- Any other mock stores that implement the `Storage` interface

Fix all references: remove old method implementations from mocks, update to implement the new plan interface methods (stubs returning nil/empty are fine for mocks not testing plan functionality).

- [ ] **Step 6: Run full storage test suite**

Run: `go test ./internal/storage/sqlite/... -v`
Expected: All PASS

- [ ] **Step 7: Commit**

```bash
git add internal/storage/sqlite/plans.go internal/storage/sqlite/plans_test.go
git commit -m "feat: rewrite plan storage implementation for ephemeral model"
```

---

## Chunk 2: API Layer — OpenAPI Spec, Handlers, and Client

### Task 5: Update OpenAPI Spec

**Files:**
- Modify: `api/openapi.yaml`

- [ ] **Step 1: Remove old plan schemas and endpoints**

In `api/openapi.yaml`, remove:
- The `plans` tag description (line ~34-35)
- All plan endpoints under `/plans` (line ~1042-1044), `/projects/{projectId}/plans` (~1070), `/projects/{projectId}/plans/pending-count` (~1118), `/projects/{projectId}/plans/{planId}` (~1142), `/projects/{projectId}/plans/{planId}/status` (~1200)
- All issue-plan endpoints: `/projects/{projectId}/issues/{issueId}/plan` (POST and GET)
- Plan schemas: `PlanStatus`, `Plan`, `SetPlanRequest`, `UpdatePlanContentRequest`, `UpdatePlanStatusRequest`, `PlanCount`
- Plan references in `TeamContextIssue` and `TeamContextEpic` schemas (the `plan` field, lines ~1579 and ~1614)

- [ ] **Step 2: Add new plan schemas and endpoints**

Add the `plans` tag back:
```yaml
  - name: plans
    description: Ephemeral plan review management
```

Add new schemas:

```yaml
    PlanStatus:
      type: string
      enum: [draft, in_review, approved, rejected]

    Plan:
      type: object
      required: [id, file_path, status, created_at, updated_at]
      properties:
        id:
          type: string
          description: Unique plan ID (plan.xxxxx format)
        file_path:
          type: string
          description: Relative path to the plan markdown file
        status:
          $ref: "#/components/schemas/PlanStatus"
        created_at:
          type: string
          format: date-time
        updated_at:
          type: string
          format: date-time

    PlanWithContent:
      allOf:
        - $ref: "#/components/schemas/Plan"
        - type: object
          properties:
            content:
              type: string
              description: Markdown content read from the plan file

    PlanComment:
      type: object
      required: [id, plan_id, content, created_at]
      properties:
        id:
          type: string
        plan_id:
          type: string
        line_number:
          type: integer
          nullable: true
          description: Line number anchor (null for overall feedback)
        content:
          type: string
        created_at:
          type: string
          format: date-time

    CreatePlanRequest:
      type: object
      required: [file_path]
      properties:
        file_path:
          type: string

    UpdatePlanContentRequest:
      type: object
      required: [content]
      properties:
        content:
          type: string

    UpdatePlanStatusRequest:
      type: object
      required: [status]
      properties:
        status:
          $ref: "#/components/schemas/PlanStatus"

    CreatePlanCommentRequest:
      type: object
      required: [content]
      properties:
        line_number:
          type: integer
          nullable: true
        content:
          type: string
```

Add new endpoints (not project-scoped — top-level under `/plans`):

```yaml
  /plans:
    post:
      operationId: createPlan
      tags: [plans]
      summary: Register an ephemeral plan
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/CreatePlanRequest"
      responses:
        "201":
          description: Plan created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Plan"

  /plans/{planId}:
    parameters:
      - name: planId
        in: path
        required: true
        schema:
          type: string
    get:
      operationId: getPlan
      tags: [plans]
      summary: Get plan metadata and file content
      responses:
        "200":
          description: Plan with content
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/PlanWithContent"
    put:
      operationId: updatePlanContent
      tags: [plans]
      summary: Update plan file content
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/UpdatePlanContentRequest"
      responses:
        "200":
          description: Content updated
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/PlanWithContent"
    delete:
      operationId: deletePlan
      tags: [plans]
      summary: Delete plan and its comments
      responses:
        "204":
          description: Plan deleted

  /plans/{planId}/status:
    parameters:
      - name: planId
        in: path
        required: true
        schema:
          type: string
    patch:
      operationId: updatePlanStatus
      tags: [plans]
      summary: Update plan status
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/UpdatePlanStatusRequest"
      responses:
        "200":
          description: Status updated
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Plan"

  /plans/{planId}/comments:
    parameters:
      - name: planId
        in: path
        required: true
        schema:
          type: string
    get:
      operationId: listPlanComments
      tags: [plans]
      summary: List plan review comments
      responses:
        "200":
          description: Comments list
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/PlanComment"
    post:
      operationId: createPlanComment
      tags: [plans]
      summary: Add a review comment
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/CreatePlanCommentRequest"
      responses:
        "201":
          description: Comment created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/PlanComment"
```

- [ ] **Step 3: Regenerate OpenAPI types**

Run: `make gen`

This regenerates `internal/api/openapi.gen.go` and `web/src/lib/api/types.ts`.

- [ ] **Step 4: Commit**

```bash
git add api/openapi.yaml internal/api/openapi.gen.go web/src/lib/api/types.ts
git commit -m "feat: update OpenAPI spec for simplified plan endpoints"
```

---

### Task 6: Rewrite API Handlers

**Files:**
- Rewrite: `internal/api/plans.go`
- Rewrite: `internal/api/plans_test.go` (or delete if tests are covered by integration tests)
- Modify: `internal/api/server.go`
- Modify: `internal/api/teams.go`
- Modify: `cmd/arc/team.go`

- [ ] **Step 1: Rewrite plans.go**

Replace the entire contents of `internal/api/plans.go`. The new file implements 7 handlers:

```go
package api

// Request/response types
type createPlanRequest struct {
    FilePath string `json:"file_path" validate:"required"`
}

type updatePlanContentRequest struct {
    Content string `json:"content" validate:"required"`
}

type updatePlanStatusRequest struct {
    Status string `json:"status" validate:"required"`
}

type createPlanCommentRequest struct {
    LineNumber *int   `json:"line_number,omitempty"`
    Content    string `json:"content" validate:"required"`
}
```

Handlers to implement:

1. **`createPlan`** (POST /plans): Parse `createPlanRequest`. Validate `file_path` is within the working directory (use `filepath.Rel` to prevent path traversal — if the relative path starts with `..`, reject with 400). Generate plan ID via `project.GeneratePlanID(filepath.Base(req.FilePath))`. Create `types.Plan` with status `draft`. Call `s.store.CreatePlan()`. Return 201 with the plan.

2. **`getPlan`** (GET /plans/:planId): Get plan from DB via `s.store.GetPlan()`. Read file content from `plan.FilePath` using `os.ReadFile`. If file doesn't exist, return 404 with message "plan file not found". Return `types.PlanWithContent` with the plan metadata and content.

3. **`updatePlanContent`** (PUT /plans/:planId): Get plan from DB. Parse `updatePlanContentRequest`. Validate `plan.FilePath` is within working directory. Write content to file via `os.WriteFile`. Return the updated `PlanWithContent`.

4. **`updatePlanStatus`** (PATCH /plans/:planId/status): Parse `updatePlanStatusRequest`. Validate status is one of: draft, in_review, approved, rejected. Call `s.store.UpdatePlanStatus()`. Return the updated plan.

5. **`deletePlan`** (DELETE /plans/:planId): Call `s.store.DeletePlan()` (CASCADE handles comments). Return 204.

6. **`listPlanComments`** (GET /plans/:planId/comments): Call `s.store.ListPlanComments()`. Return the comments array.

7. **`createPlanComment`** (POST /plans/:planId/comments): Parse `createPlanCommentRequest`. Verify the plan exists via `s.store.GetPlan()`. Generate comment ID. Create `types.PlanComment`. Call `s.store.CreatePlanComment()`. Return 201 with the comment.

For path validation, add a helper:

```go
// validateFilePath checks that the path is within the server's working directory.
func (s *Server) validateFilePath(filePath string) error {
    absPath, err := filepath.Abs(filePath)
    if err != nil {
        return fmt.Errorf("invalid path: %w", err)
    }
    wd, err := os.Getwd()
    if err != nil {
        return fmt.Errorf("cannot determine working directory: %w", err)
    }
    rel, err := filepath.Rel(wd, absPath)
    if err != nil || strings.HasPrefix(rel, "..") {
        return fmt.Errorf("path must be within project directory")
    }
    return nil
}
```

- [ ] **Step 2: Update route registration in server.go**

In `internal/api/server.go`, replace the plan route registrations (lines 117-118 and 158-166):

Remove:
```go
// Global plans (cross-project)
v1.GET("/plans", s.listAllPlans)
```
and:
```go
// Plans (unified)
ws.POST("/issues/:id/plan", s.setIssuePlan)
ws.GET("/issues/:id/plan", s.getIssuePlan)
ws.GET("/plans", s.listPlans)
ws.GET("/plans/pending-count", s.getPendingCount)
ws.GET("/plans/:pid", s.getPlan)
ws.PUT("/plans/:pid", s.updatePlan)
ws.PATCH("/plans/:pid/status", s.updatePlanStatus)
ws.DELETE("/plans/:pid", s.deletePlan)
```

Replace with (outside the `ws` group, since plans are not project-scoped):
```go
// Plans (ephemeral review artifacts, not project-scoped)
v1.POST("/plans", s.createPlan)
v1.GET("/plans/:planId", s.getPlan)
v1.PUT("/plans/:planId", s.updatePlanContent)
v1.PATCH("/plans/:planId/status", s.updatePlanStatus)
v1.DELETE("/plans/:planId", s.deletePlan)
v1.GET("/plans/:planId/comments", s.listPlanComments)
v1.POST("/plans/:planId/comments", s.createPlanComment)
```

- [ ] **Step 3: Update teams.go and related files**

In `internal/api/teams.go`, the `buildTeamContextIssue` function (line ~166) calls `s.store.GetPlanByIssueID()` to attach plan content. Since plans are no longer issue-linked:
- Remove the `GetPlanByIssueID` call and the `Plan` field from the `TeamContextIssue` struct
- The issue's description now serves this purpose

In `cmd/arc/team.go`, remove plan-related code:
- Remove `Plan` fields from any local team context structs
- Remove `GetPlanByIssue` client calls (there are two — one for epic context, one for issue context)
- Remove any plan truncation logic

Also rewrite or delete `internal/api/plans_test.go` — it tests old handlers (`TestSetIssuePlanCreatesDraft`, etc.) that no longer exist. Either rewrite with tests for the new handlers or delete if covered by Task 4's storage tests + Task 15's integration test.

- [ ] **Step 4: Verify compilation**

Run: `go build ./internal/api/...`
Expected: Compiles without errors.

- [ ] **Step 5: Commit**

```bash
git add internal/api/plans.go internal/api/server.go internal/api/teams.go
git commit -m "feat: rewrite plan API handlers for ephemeral model"
```

---

### Task 7: Rewrite HTTP Client

**Files:**
- Modify: `internal/client/client.go`

- [ ] **Step 1: Remove old plan client methods**

In `internal/client/client.go`, remove all plan methods (lines ~460-592):
- `SetPlan`
- `GetPlanByIssue`
- `ListPlans`
- `GetPlan`
- `UpdatePlanStatus`
- `UpdatePlanContent`
- `DeletePlan`
- `GetPendingPlanCount`

- [ ] **Step 2: Add new plan client methods**

Add the following methods. Note: these endpoints are NOT project-scoped, so URLs use `/api/v1/plans/...` directly (no project ID prefix).

Follow the existing client patterns in the file (see `CreateIssue`, `GetIssue`, etc.) for HTTP calls, error handling, and JSON marshaling. Here's one complete method as reference — implement the rest following this pattern:

```go
func (c *Client) CreatePlan(filePath string) (*types.Plan, error) {
	body := map[string]string{"file_path": filePath}
	resp, err := c.doRequest("POST", "/api/v1/plans", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var plan types.Plan
	if err := json.NewDecoder(resp.Body).Decode(&plan); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &plan, nil
}
```

Remaining methods (same pattern, different HTTP method/URL/body/return type):
- `GetPlan(planID string) (*types.PlanWithContent, error)` — GET `/api/v1/plans/{planID}`
- `UpdatePlanContent(planID string, content string) error` — PUT `/api/v1/plans/{planID}`, body: `{"content": content}`
- `UpdatePlanStatus(planID string, status string) error` — PATCH `/api/v1/plans/{planID}/status`, body: `{"status": status}`
- `DeletePlan(planID string) error` — DELETE `/api/v1/plans/{planID}`
- `ListPlanComments(planID string) ([]*types.PlanComment, error)` — GET `/api/v1/plans/{planID}/comments`
- `CreatePlanComment(planID string, lineNumber *int, content string) (*types.PlanComment, error)` — POST `/api/v1/plans/{planID}/comments`, body: `{"content": content, "line_number": lineNumber}`

- [ ] **Step 3: Verify compilation**

Run: `go build ./internal/client/...`

This may fail if `cmd/arc/main.go` still references old client methods. That's expected — CLI fixes come next.

- [ ] **Step 4: Commit**

```bash
git add internal/client/client.go
git commit -m "feat: rewrite plan HTTP client for ephemeral model"
```

---

### Task 8: Rewrite CLI Commands

**Files:**
- Rewrite: `cmd/arc/plan.go`
- Modify: `cmd/arc/main.go`

- [ ] **Step 1: Rewrite plan.go**

Replace the entire contents of `cmd/arc/plan.go`. New commands:

```
arc plan create --file <path>    → POST /plans, prints plan ID
arc plan show <plan-id>          → GET /plans/:id, displays content + status + comments
arc plan approve <plan-id>       → PATCH /plans/:id/status {status: "approved"}
arc plan reject <plan-id>        → PATCH /plans/:id/status {status: "rejected"}
arc plan comments <plan-id>      → GET /plans/:id/comments, prints structured output
```

Key details:
- `create` requires `--file` flag. The file path should be relative to CWD.
- `show` displays: status, file path, then the full file content. If there are comments, display them after the content grouped by line number.
- `comments` outputs in a structured format the AI skill can parse: each comment on its own line with format `[L{line}] {content}` or `[overall] {content}`.
- Remove `editInEditor` helper and all `--editor`/`--stdin` flags.
- Remove `planListCmd` entirely.

- [ ] **Step 2: Update main.go**

In `cmd/arc/main.go`:
- Remove the `formatPlanInfo` function (lines ~1262-1274)
- Remove the `formatPendingPlanNotice` function (lines ~1277-1283)
- Remove the plan display block in the `show` command (lines ~799-807 where it calls `c.GetPlanByIssue`)
- Remove the pending plan count notice in the `ready` command (lines ~1036-1041 where it calls `c.GetPendingPlanCount`)

Also update `cmd/arc/show_ready_test.go` if it tests `formatPlanInfo` or `formatPendingPlanNotice` — remove those test cases.

- [ ] **Step 3: Verify full build**

Run: `make build-quick`
Expected: Compiles without errors.

- [ ] **Step 4: Manual smoke test**

```bash
# Start server
./bin/arc-server &

# Create a test plan file
echo "# Test Plan" > docs/plans/test-plan.md

# Test CLI commands — capture plan ID
PLAN_ID=$(./bin/arc plan create --file docs/plans/test-plan.md 2>&1 | grep -o 'plan\.[a-z0-9]*')
echo "Created: $PLAN_ID"

./bin/arc plan show "$PLAN_ID"
# Expected: shows status, path, content

./bin/arc plan approve "$PLAN_ID"
# Expected: status updated to approved

# Clean up
rm docs/plans/test-plan.md
kill %1
```

- [ ] **Step 5: Commit**

```bash
git add cmd/arc/plan.go cmd/arc/main.go
git commit -m "feat: rewrite plan CLI commands for ephemeral model"
```

---

## Chunk 3: Web UI — Remove Old Pages, Add Planner

### Task 9: Remove Old Plan Web UI

**Files:**
- Delete: `web/src/routes/[projectId]/plans/+page.svelte`
- Delete: `web/src/routes/[projectId]/plans/[planId]/+page.svelte`
- Delete: `web/src/routes/plans/+page.svelte`
- Delete: `web/src/routes/plans/[planId]/+page.svelte`
- Modify: `web/src/lib/api/index.ts`
- Modify: `web/src/lib/components/Sidebar.svelte`

- [ ] **Step 1: Delete old plan route files**

Delete the following files/directories:
- `web/src/routes/[projectId]/plans/` (entire directory)
- `web/src/routes/plans/` (entire directory)

- [ ] **Step 2: Remove old plan API methods from index.ts**

In `web/src/lib/api/index.ts`, remove all plan-related methods and types (lines ~412-486):
- `Plan` type alias
- `listAllPlans`
- `listProjectPlans`
- `getPlan`
- `updatePlan`
- `updatePlanStatus`
- `deletePlan`

- [ ] **Step 3: Update Sidebar**

In `web/src/lib/components/Sidebar.svelte`:
- Remove the `draftPlanCount` state variable and the `$effect` that fetches pending plan count (lines ~16-30)
- Remove "Plans" from the project navigation items array (line ~58)
- Remove the `plans` icon from the icons object (line ~72)
- Remove the draft plan count badge rendering (line ~188-190)
- Remove the global "Plans" nav links (lines ~213-217 and ~300-304)

- [ ] **Step 4: Verify frontend builds**

Run: `cd web && npm run build`
Expected: Builds without errors.

- [ ] **Step 5: Commit**

```bash
git add -A web/src/routes/plans/ web/src/routes/\[projectId\]/plans/ web/src/lib/api/index.ts web/src/lib/components/Sidebar.svelte
git commit -m "feat: remove old plan web UI pages and sidebar references"
```

---

### Task 10: Add New Plan API Client Methods

**Files:**
- Modify: `web/src/lib/api/index.ts`

- [ ] **Step 1: Add new plan types and API methods**

In `web/src/lib/api/index.ts`, add new plan API methods. These use the generated OpenAPI types from `types.ts` (regenerated in Task 5).

```typescript
export type Plan = components['schemas']['Plan'];
export type PlanWithContent = components['schemas']['PlanWithContent'];
export type PlanComment = components['schemas']['PlanComment'];

export async function createPlan(filePath: string): Promise<Plan> {
    const { data, error } = await api.POST('/plans', {
        body: { file_path: filePath }
    });
    if (error) throw handleError(error);
    return data!;
}

export async function getPlan(planId: string): Promise<PlanWithContent> {
    const { data, error } = await api.GET('/plans/{planId}', {
        params: { path: { planId } }
    });
    if (error) throw handleError(error);
    return data!;
}

export async function updatePlanContent(planId: string, content: string): Promise<PlanWithContent> {
    const { data, error } = await api.PUT('/plans/{planId}', {
        params: { path: { planId } },
        body: { content }
    });
    if (error) throw handleError(error);
    return data!;
}

export async function updatePlanStatus(planId: string, status: string): Promise<Plan> {
    const { data, error } = await api.PATCH('/plans/{planId}/status', {
        params: { path: { planId } },
        body: { status }
    });
    if (error) throw handleError(error);
    return data!;
}

export async function deletePlan(planId: string): Promise<void> {
    const { error } = await api.DELETE('/plans/{planId}', {
        params: { path: { planId } }
    });
    if (error) throw handleError(error);
}

export async function listPlanComments(planId: string): Promise<PlanComment[]> {
    const { data, error } = await api.GET('/plans/{planId}/comments', {
        params: { path: { planId } }
    });
    if (error) throw handleError(error);
    return data ?? [];
}

export async function createPlanComment(planId: string, content: string, lineNumber?: number): Promise<PlanComment> {
    const { data, error } = await api.POST('/plans/{planId}/comments', {
        params: { path: { planId } },
        body: { content, line_number: lineNumber ?? null }
    });
    if (error) throw handleError(error);
    return data!;
}
```

- [ ] **Step 2: Verify types match**

Run: `cd web && npx tsc --noEmit`
Expected: No type errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/lib/api/index.ts
git commit -m "feat: add plan API client methods for ephemeral model"
```

---

### Task 11: Build the Planner Web UI

**Files:**
- Create: `web/src/routes/planner/[planId]/+page.svelte`

This is the core review UI. It shows rendered markdown with line numbers, supports click-to-comment on lines, has an overall feedback box, and provides approve/submit review/reject actions.

- [ ] **Step 1: Create the planner page**

Create `web/src/routes/planner/[planId]/+page.svelte`.

The page follows existing detail page patterns (see `web/src/routes/[projectId]/issues/[issueId]/+page.svelte` for reference).

**V1 approach:** Two-panel layout — rendered markdown for reading, then a line reference view (raw lines with numbers) for commenting. This avoids the complexity of mapping rendered markdown back to source lines.

**Script section** — state, data loading, and handlers:

```svelte
<script lang="ts">
    import { page } from '$app/stores';
    import {
        getPlan, updatePlanContent, updatePlanStatus,
        listPlanComments, createPlanComment
    } from '$lib/api';
    import type { PlanWithContent, PlanComment } from '$lib/api';
    import Markdown from '$lib/components/Markdown.svelte';
    import { formatRelativeTime } from '$lib/utils';

    let planId = $derived($page.params.planId);

    let plan = $state<PlanWithContent | null>(null);
    let comments = $state<PlanComment[]>([]);
    let loading = $state(true);
    let error = $state<string | null>(null);

    // Edit mode
    let editing = $state(false);
    let editContent = $state('');

    // Comment state
    let activeCommentLine = $state<number | null>(null);
    let commentText = $state('');
    let overallFeedback = $state('');
    let submitting = $state(false);

    // Split content into lines for the line reference view
    let contentLines = $derived((plan?.content ?? '').split('\n'));

    // Group comments by line number for indicators
    let commentsByLine = $derived.by(() => {
        const map = new Map<number | null, PlanComment[]>();
        for (const c of comments) {
            const key = c.line_number ?? null;
            if (!map.has(key)) map.set(key, []);
            map.get(key)!.push(c);
        }
        return map;
    });

    let hasAnyComments = $derived(comments.length > 0 || overallFeedback.trim().length > 0);

    $effect(() => { if (planId) loadData(); });

    async function loadData() {
        loading = true;
        error = null;
        try {
            const [planData, commentsData] = await Promise.all([
                getPlan(planId),
                listPlanComments(planId)
            ]);
            plan = planData;
            comments = commentsData;
        } catch (err) {
            error = err instanceof Error ? err.message : 'Failed to load plan';
        } finally {
            loading = false;
        }
    }

    async function handleSaveEdit() {
        if (!plan) return;
        try {
            const updated = await updatePlanContent(planId, editContent);
            plan = updated;
            editing = false;
        } catch (err) {
            error = err instanceof Error ? err.message : 'Failed to save';
        }
    }

    async function handleAddComment(lineNumber: number | null) {
        const text = lineNumber === null ? overallFeedback : commentText;
        if (!text.trim()) return;
        try {
            const comment = await createPlanComment(planId, text, lineNumber ?? undefined);
            comments = [...comments, comment];
            commentText = '';
            if (lineNumber === null) overallFeedback = '';
            activeCommentLine = null;
        } catch (err) {
            error = err instanceof Error ? err.message : 'Failed to add comment';
        }
    }

    async function handleUpdateStatus(status: string) {
        submitting = true;
        try {
            // If submitting review, save overall feedback as a comment first
            if (status === 'in_review' && overallFeedback.trim()) {
                await handleAddComment(null);
            }
            const updated = await updatePlanStatus(planId, status);
            plan = { ...plan!, ...updated };
        } catch (err) {
            error = err instanceof Error ? err.message : 'Failed to update status';
        } finally {
            submitting = false;
        }
    }

    function startEdit() {
        editContent = plan?.content ?? '';
        editing = true;
    }

    function statusColor(status: string): string {
        switch (status) {
            case 'draft': return 'bg-surface-600 text-text-secondary';
            case 'in_review': return 'bg-yellow-900/30 text-yellow-400 border border-yellow-800';
            case 'approved': return 'bg-green-900/30 text-green-400 border border-green-800';
            case 'rejected': return 'bg-red-900/30 text-red-400 border border-red-800';
            default: return 'bg-surface-600 text-text-secondary';
        }
    }
</script>
```

**Template markup:**

```svelte
{#if loading}
    <div class="flex items-center justify-center py-20">
        <div class="text-text-muted animate-pulse">Loading plan...</div>
    </div>
{:else if error}
    <div class="flex items-center justify-center py-20">
        <div class="text-red-400">{error}</div>
    </div>
{:else if plan}
    <div class="max-w-5xl mx-auto p-6 space-y-6">
        <!-- Header -->
        <div class="flex items-center justify-between">
            <div>
                <h1 class="text-xl font-semibold text-text-primary">
                    {plan.file_path.split('/').pop()}
                </h1>
                <p class="text-sm text-text-muted mt-1">{plan.file_path}</p>
            </div>
            <span class="px-3 py-1 rounded-full text-xs font-medium {statusColor(plan.status)}">
                {plan.status}
            </span>
        </div>

        <!-- Rendered Markdown (read-only view) -->
        {#if !editing}
            <div class="card p-6">
                <Markdown content={plan.content} />
            </div>
        {/if}

        <!-- Raw Editor (edit mode) -->
        {#if editing}
            <div class="card p-4 space-y-3">
                <textarea
                    bind:value={editContent}
                    class="w-full h-96 bg-surface-700 text-text-primary font-mono text-sm p-4 rounded border border-surface-500 focus:border-primary-500 focus:outline-none resize-y"
                ></textarea>
                <div class="flex gap-2 justify-end">
                    <button onclick={() => editing = false}
                        class="px-3 py-1.5 text-sm text-text-secondary hover:text-text-primary">
                        Cancel
                    </button>
                    <button onclick={handleSaveEdit}
                        class="px-3 py-1.5 text-sm bg-primary-600 text-white rounded hover:bg-primary-500">
                        Save
                    </button>
                </div>
            </div>
        {/if}

        <!-- Line Reference View (for commenting) -->
        {#if !editing}
            <div class="card">
                <div class="px-4 py-2 border-b border-surface-600 text-xs text-text-muted uppercase tracking-wide">
                    Line Comments — click a line number to add a comment
                </div>
                <div class="font-mono text-sm">
                    {#each contentLines as line, i}
                        {@const lineNum = i + 1}
                        {@const lineComments = commentsByLine.get(lineNum) ?? []}
                        <div class="group">
                            <div class="flex hover:bg-surface-700/50">
                                <button
                                    onclick={() => activeCommentLine = activeCommentLine === lineNum ? null : lineNum}
                                    class="w-12 text-right pr-3 py-0.5 text-text-muted hover:text-primary-400 select-none shrink-0 cursor-pointer"
                                >
                                    {lineNum}
                                </button>
                                <div class="flex-1 py-0.5 pr-4 text-text-primary whitespace-pre-wrap break-all">
                                    {line || '\u00A0'}
                                </div>
                                {#if lineComments.length > 0}
                                    <span class="pr-3 py-0.5 text-xs text-yellow-400">
                                        💬 {lineComments.length}
                                    </span>
                                {/if}
                            </div>

                            <!-- Inline comments for this line -->
                            {#if lineComments.length > 0}
                                <div class="ml-12 pl-3 border-l-2 border-yellow-800 bg-yellow-900/10 py-2 space-y-1">
                                    {#each lineComments as comment}
                                        <div class="text-sm text-text-secondary">
                                            {comment.content}
                                            <span class="text-xs text-text-muted ml-2">
                                                {formatRelativeTime(comment.created_at)}
                                            </span>
                                        </div>
                                    {/each}
                                </div>
                            {/if}

                            <!-- Comment input for this line -->
                            {#if activeCommentLine === lineNum}
                                <div class="ml-12 pl-3 py-2 flex gap-2">
                                    <input
                                        type="text"
                                        bind:value={commentText}
                                        placeholder="Add a comment on line {lineNum}..."
                                        class="flex-1 bg-surface-700 text-text-primary text-sm px-3 py-1.5 rounded border border-surface-500 focus:border-primary-500 focus:outline-none"
                                        onkeydown={(e) => { if (e.key === 'Enter') handleAddComment(lineNum); }}
                                    />
                                    <button
                                        onclick={() => handleAddComment(lineNum)}
                                        class="px-3 py-1.5 text-sm bg-primary-600 text-white rounded hover:bg-primary-500"
                                    >
                                        Comment
                                    </button>
                                </div>
                            {/if}
                        </div>
                    {/each}
                </div>
            </div>
        {/if}

        <!-- Overall Feedback -->
        <div class="card p-4 space-y-3">
            <label class="text-xs text-text-muted uppercase tracking-wide">Overall Feedback</label>
            <!-- Show existing overall comments -->
            {#if (commentsByLine.get(null) ?? []).length > 0}
                <div class="space-y-2 mb-3">
                    {#each commentsByLine.get(null) ?? [] as comment}
                        <div class="text-sm text-text-secondary bg-surface-700 rounded p-3">
                            {comment.content}
                            <span class="text-xs text-text-muted ml-2">
                                {formatRelativeTime(comment.created_at)}
                            </span>
                        </div>
                    {/each}
                </div>
            {/if}
            <textarea
                bind:value={overallFeedback}
                placeholder="Overall feedback on this plan..."
                rows="3"
                class="w-full bg-surface-700 text-text-primary text-sm p-3 rounded border border-surface-500 focus:border-primary-500 focus:outline-none resize-y"
            ></textarea>
        </div>

        <!-- Action Buttons -->
        <div class="flex gap-3 justify-end">
            {#if !editing}
                <button onclick={startEdit}
                    class="px-4 py-2 text-sm text-text-secondary border border-surface-500 rounded hover:bg-surface-700">
                    Edit
                </button>
            {/if}
            <button onclick={() => handleUpdateStatus('rejected')}
                disabled={submitting}
                class="px-4 py-2 text-sm text-red-400 border border-red-800 rounded hover:bg-red-900/30 disabled:opacity-50">
                Reject
            </button>
            <button onclick={() => handleUpdateStatus('in_review')}
                disabled={submitting || !hasAnyComments}
                title={hasAnyComments ? '' : 'Add at least one comment before submitting review'}
                class="px-4 py-2 text-sm text-yellow-400 border border-yellow-800 rounded hover:bg-yellow-900/30 disabled:opacity-50">
                Submit Review
            </button>
            <button onclick={() => handleUpdateStatus('approved')}
                disabled={submitting}
                class="px-4 py-2 text-sm text-green-400 border border-green-800 rounded hover:bg-green-900/30 disabled:opacity-50">
                Approve
            </button>
        </div>
    </div>
{/if}
```

**Styling notes:** Follow existing Tailwind patterns. Use `bg-surface-*` for backgrounds, `text-text-*` for text colors. The `.card` class is used throughout the project for content containers. Status badge colors: draft=gray, in_review=yellow, approved=green, rejected=red (matches existing convention in `web/src/routes/plans/[planId]/+page.svelte` before deletion).

- [ ] **Step 2: Verify the page loads**

Run: `cd web && npm run dev`

Navigate to `http://localhost:5173/planner/<some-plan-id>` (create a plan via CLI first).

Expected: Page renders with plan content, line numbers, and action buttons.

- [ ] **Step 3: Test comment flow**

1. Click a line number → comment input appears
2. Type a comment → submit
3. Comment appears in the comments section with the line anchor
4. Add an overall feedback comment
5. Click "Submit Review" → status changes to `in_review`

- [ ] **Step 4: Test edit flow**

1. Click "Edit" → textarea with raw markdown appears
2. Make an edit → click "Save"
3. Content updates, rendered view reflects changes

- [ ] **Step 5: Test approve/reject**

1. Click "Approve" → status badge changes to approved
2. Create another plan, click "Reject" → status changes to rejected

- [ ] **Step 6: Verify build**

Run: `cd web && npm run build`
Expected: Builds without errors.

- [ ] **Step 7: Commit**

```bash
git add web/src/routes/planner/
git commit -m "feat: add planner web UI for ephemeral plan review"
```

---

## Chunk 4: Skill Updates and Documentation

### Task 12: Update Brainstorm Skill

**Files:**
- Modify: `claude-plugin/skills/brainstorm/SKILL.md`

- [ ] **Step 1: Update end-of-brainstorm flow**

In `claude-plugin/skills/brainstorm/SKILL.md`, find and update all plan-related sections:

1. Remove references to "saves approved designs as arc plans" (line ~3 in description)
2. Remove the instruction to never create `docs/plans/` markdown files (line ~175) — this is now the intended workflow
3. Replace `arc plan set <meta-epic-id> --stdin` (line ~88) and `arc plan set <epic-id> --stdin` (line ~136) with the new flow:
   - Write the design markdown to `docs/plans/YYYY-MM-DD-<topic>.md`
   - Run `arc plan create --file docs/plans/YYYY-MM-DD-<topic>.md` to register it
4. Remove epic/issue creation from the brainstorm flow — that moves to the plan skill
5. Add the review loop instructions:
   - Present planner URL and CLI options (approve / review submitted)
   - If approve: transition to `/arc:plan`
   - If review submitted: read comments via `arc plan comments <plan-id>`, revise, loop

- [ ] **Step 2: Verify skill is valid markdown**

Read the updated file and verify the markdown renders correctly and the instructions are clear and self-consistent.

- [ ] **Step 3: Commit**

```bash
git add claude-plugin/skills/brainstorm/SKILL.md
git commit -m "feat: update brainstorm skill for ephemeral plan review flow"
```

---

### Task 13: Update Plan Skill

**Files:**
- Modify: `claude-plugin/skills/plan/SKILL.md`

- [ ] **Step 1: Update plan reading**

In `claude-plugin/skills/plan/SKILL.md`:

1. Replace `arc plan show <epic-id>` (line ~25) with `arc plan show <plan-id>` — the plan is now standalone, not issue-linked
2. Replace `arc plan set <epic-id> --stdin` (line ~110) with: write the implementation breakdown into the epic's description (using `arc update <epic-id> --description`)
3. Remove references to "web UI split-pane review editor" (lines ~12, 115)
4. Remove `arc plan list --status draft` reference (line ~115)
5. Remove "Never create `docs/plans/` markdown files" rule (line ~182)
6. Update plan command references: `arc plan set`, `arc plan show` with issue IDs → `arc plan show <plan-id>` with plan IDs
7. Add: the plan skill now writes approved design content into the epic's description field when creating the epic

- [ ] **Step 2: Verify skill is valid markdown**

Read the updated file and verify the markdown renders correctly, instructions are self-consistent, and no old plan commands remain.

- [ ] **Step 3: Commit**

```bash
git add claude-plugin/skills/plan/SKILL.md
git commit -m "feat: update plan skill for ephemeral plan model"
```

---

### Task 14: Update Arc Skill, Prime, and Docs

**Files:**
- Modify: `claude-plugin/skills/arc/SKILL.md`
- Modify: `cmd/arc/prime.go`
- Modify: `AGENTS.md` (if plan references exist)

- [ ] **Step 1: Update arc skill documentation**

In `claude-plugin/skills/arc/SKILL.md`:
- Update the plan command reference table (lines ~150-152):
  - `arc plan create --file <path>` — Register an ephemeral plan, returns plan ID
  - `arc plan show <plan-id>` — Show plan content, status, and comments
  - `arc plan approve <plan-id>` — Approve the plan
  - `arc plan reject <plan-id>` — Reject the plan
  - `arc plan comments <plan-id>` — List review comments
- Remove the "Plans" pattern table entry about inline plans (line ~158)
- Update the plans description paragraph (line ~161) to reflect the new ephemeral model

- [ ] **Step 2: Update prime.go**

In `cmd/arc/prime.go`, update the plan command references (lines ~382-385):
- Replace the old commands with:
  ```
  `arc plan create --file <path>` - Register ephemeral plan for review
  `arc plan show <plan-id>` - Show plan content, status, and comments
  `arc plan approve <plan-id>` - Approve plan
  `arc plan reject <plan-id>` - Reject plan
  `arc plan comments <plan-id>` - List review comments
  ```

- [ ] **Step 3: Update AGENTS.md if needed**

Check `AGENTS.md` for any plan-specific references. Update if they reference old plan commands or the old plan workflow.

- [ ] **Step 4: Full build and test**

Run: `make build && make test`
Expected: All pass.

- [ ] **Step 5: Commit**

```bash
git add claude-plugin/skills/arc/SKILL.md cmd/arc/prime.go AGENTS.md
git commit -m "docs: update arc skill, prime output, and agents for plan simplification"
```

---

### Task 15: Final Verification

- [ ] **Step 1: Run full test suite**

Run: `make test`
Expected: All tests pass.

- [ ] **Step 2: Run full build**

Run: `make build`
Expected: Build succeeds (frontend + backend).

- [ ] **Step 3: End-to-end smoke test**

```bash
# Start server
./bin/arc-server &

# Create a plan file
cat > docs/plans/test-e2e.md << 'EOF'
# E2E Test Plan

## Overview
Testing the ephemeral plan workflow.

## Steps
1. Create the plan
2. Review it
3. Approve it
EOF

# Register it and capture the plan ID
PLAN_ID=$(./bin/arc plan create --file docs/plans/test-e2e.md 2>&1 | grep -o 'plan\.[a-z0-9]*')
echo "Created plan: $PLAN_ID"

# Show it
./bin/arc plan show "$PLAN_ID"

# Open planner in browser
echo "Open: http://localhost:7432/planner/$PLAN_ID"
# Verify: rendered markdown, line numbers, action buttons

# Approve via CLI
./bin/arc plan approve "$PLAN_ID"

# Clean up
rm docs/plans/test-e2e.md
kill %1
```

- [ ] **Step 4: Verify no references to old plan system remain**

Run: `grep -r "GetPlanByIssueID\|SetPlan\|ListAllPlans\|PlanContext\|PlansMoved\|CountPlansByStatus\|GetPendingPlanCount\|pending-count\|setPlanRequest\|setIssuePlan\|getIssuePlan" --include="*.go" --include="*.ts" --include="*.svelte" --include="*.yaml" -l`

Expected: No matches (or only in migration files / git history).

- [ ] **Step 5: Commit any remaining fixes**

```bash
git add -A
git commit -m "chore: final cleanup for plan simplification"
```
