# Global Labels with Color Picker — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Use frontend-design skill for Task 9-11 (UI components).

**Goal:** Make labels global (drop workspace_id), add color picker UI for label create/edit, and render colored label badges on issue cards.

**Architecture:** Migration drops workspace_id from labels table (deduplicating on collision). API endpoints move from workspace-scoped to top-level `/api/v1/labels`. Frontend gets a new `/labels` route with color picker and CRUD forms. IssueCard renders label colors.

**Tech Stack:** Go (Echo, sqlc, goose migrations), SvelteKit 5 (runes), Tailwind CSS, openapi-fetch

---

### Task 1: Schema migration — drop workspace_id from labels

**Files:**
- Create: `internal/storage/sqlite/migrations/005_global_labels.sql`

**Step 1: Write the migration**

```sql
-- +goose Up
CREATE TABLE labels_new (
    name TEXT PRIMARY KEY,
    color TEXT,
    description TEXT
);

INSERT OR IGNORE INTO labels_new (name, color, description)
    SELECT name, color, description FROM labels;

DROP TABLE labels;

ALTER TABLE labels_new RENAME TO labels;

-- +goose Down
CREATE TABLE labels_new (
    workspace_id TEXT NOT NULL,
    name TEXT NOT NULL,
    color TEXT,
    description TEXT,
    PRIMARY KEY (workspace_id, name),
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
);

INSERT INTO labels_new (workspace_id, name, color, description)
    SELECT '', name, color, description FROM labels;

DROP TABLE labels;

ALTER TABLE labels_new RENAME TO labels;
```

**Step 2: Update the schema file**

In `internal/storage/sqlite/db/schema.sql`, update the labels table definition:

```sql
CREATE TABLE labels (
    name TEXT PRIMARY KEY,
    color TEXT,
    description TEXT
);
```

Remove the `workspace_id` and `FOREIGN KEY` from the labels table. Keep `issue_labels` unchanged.

**Step 3: Verify migration applies**

Run: `go test ./internal/storage/sqlite/ -run TestMigration -v` (if exists), otherwise: `make build-quick && ./bin/arc-server` to verify it starts without errors.

**Step 4: Commit**

```bash
git add internal/storage/sqlite/migrations/005_global_labels.sql internal/storage/sqlite/db/schema.sql
git commit -m "feat: migration to make labels global (drop workspace_id)"
```

---

### Task 2: Update sqlc queries for global labels

**Files:**
- Modify: `internal/storage/sqlite/db/queries/labels.sql`

**Step 1: Update the queries to remove workspace_id**

Replace the full file with:

```sql
-- name: CreateLabel :exec
INSERT INTO labels (name, color, description)
VALUES (?, ?, ?)
ON CONFLICT(name) DO UPDATE SET
    color = excluded.color,
    description = excluded.description;

-- name: GetLabel :one
SELECT * FROM labels WHERE name = ?;

-- name: ListLabels :many
SELECT * FROM labels ORDER BY name;

-- name: UpdateLabel :exec
UPDATE labels SET color = ?, description = ?
WHERE name = ?;

-- name: DeleteLabel :exec
DELETE FROM labels WHERE name = ?;

-- name: AddLabelToIssue :exec
INSERT INTO issue_labels (issue_id, label)
VALUES (?, ?)
ON CONFLICT(issue_id, label) DO NOTHING;

-- name: RemoveLabelFromIssue :exec
DELETE FROM issue_labels WHERE issue_id = ? AND label = ?;

-- name: GetIssueLabels :many
SELECT label FROM issue_labels WHERE issue_id = ? ORDER BY label;

-- name: GetIssuesByLabel :many
SELECT i.* FROM issues i
JOIN issue_labels il ON i.id = il.issue_id
WHERE il.label = ?
ORDER BY i.priority ASC, i.updated_at DESC;

-- name: GetLabelsForIssues :many
SELECT issue_id, label FROM issue_labels
WHERE issue_id IN (sqlc.slice('issue_ids'))
ORDER BY issue_id, label;

-- name: DeleteIssueLabels :exec
DELETE FROM issue_labels WHERE issue_id = ?;
```

**Step 2: Regenerate sqlc**

Run: `make gen`

This regenerates `internal/storage/sqlite/db/models.go` and `internal/storage/sqlite/db/labels.sql.go`. The `Label` model will lose `WorkspaceID`, and query params will lose workspace_id fields.

**Step 3: Commit**

```bash
git add internal/storage/sqlite/db/
git commit -m "feat: update sqlc queries for global labels"
```

---

### Task 3: Update Go types and storage interface

**Files:**
- Modify: `internal/types/types.go:233-239`
- Modify: `internal/storage/storage.go:43-52`

**Step 1: Update Label struct**

In `internal/types/types.go`, replace the Label struct (lines 233-239):

```go
// Label represents a global tag that can be applied to issues.
type Label struct {
	Name        string `json:"name"`
	Color       string `json:"color,omitempty"`
	Description string `json:"description,omitempty"`
}
```

**Step 2: Update storage interface**

In `internal/storage/storage.go`, replace label methods (lines 43-52):

```go
	// Labels (global)
	CreateLabel(ctx context.Context, label *types.Label) error
	GetLabel(ctx context.Context, name string) (*types.Label, error)
	ListLabels(ctx context.Context) ([]*types.Label, error)
	UpdateLabel(ctx context.Context, label *types.Label) error
	DeleteLabel(ctx context.Context, name string) error
	AddLabelToIssue(ctx context.Context, issueID, label, actor string) error
	RemoveLabelFromIssue(ctx context.Context, issueID, label, actor string) error
	GetIssueLabels(ctx context.Context, issueID string) ([]string, error)
	GetLabelsForIssues(ctx context.Context, issueIDs []string) (map[string][]string, error)
```

**Step 3: Commit**

```bash
git add internal/types/types.go internal/storage/storage.go
git commit -m "feat: update Label type and storage interface for global labels"
```

---

### Task 4: Update SQLite label storage implementation

**Files:**
- Modify: `internal/storage/sqlite/labels.go`

**Step 1: Update all label methods to remove workspace_id**

Key changes throughout the file:

- `CreateLabel`: Remove `WorkspaceID` from params, use new `db.CreateLabelParams{Name, Color, Description}`
- `GetLabel(ctx, name)`: Drop `workspaceID` param, call `s.queries.GetLabel(ctx, name)`
- `ListLabels(ctx)`: Drop `workspaceID` param, call `s.queries.ListLabels(ctx)`
- `UpdateLabel`: Remove workspace_id from `db.UpdateLabelParams`
- `DeleteLabel(ctx, name)`: Drop `workspaceID` param
- `dbLabelToType`: Remove `WorkspaceID` field from conversion
- `GetLabelsForIssues`: Update raw SQL — no workspace_id join needed (query is already workspace-independent since it queries `issue_labels`)

**Step 2: Run tests**

Run: `go test ./internal/storage/sqlite/ -v`
Expected: Compilation errors first (fix any remaining workspace_id references), then PASS

**Step 3: Commit**

```bash
git add internal/storage/sqlite/labels.go
git commit -m "feat: update SQLite label storage for global labels"
```

---

### Task 5: Update API handlers and routes

**Files:**
- Modify: `internal/api/labels.go`
- Modify: `internal/api/server.go:111-117`

**Step 1: Update label handlers to remove workspace_id dependency**

In `internal/api/labels.go`:
- `listLabels`: Call `s.store.ListLabels(ctx)` (no workspace param)
- `createLabel`: Build `types.Label` without `WorkspaceID`, call `s.store.CreateLabel`
- `updateLabel`: Call `s.store.GetLabel(ctx, name)` (no workspace), then `s.store.UpdateLabel`
- `deleteLabel`: Call `s.store.DeleteLabel(ctx, name)` (no workspace)
- `addLabelToIssue` and `removeLabelFromIssue`: Keep reading workspaceId from path (for issue validation), but label operations are global

**Step 2: Move label CRUD routes to top-level group**

In `internal/api/server.go`, move the 4 label CRUD routes out of the `ws` group into the `v1` group:

```go
// Labels (global)
v1.GET("/labels", s.listLabels)
v1.POST("/labels", s.createLabel)
v1.PUT("/labels/:name", s.updateLabel)
v1.DELETE("/labels/:name", s.deleteLabel)
```

Keep issue-label routes under `ws`:
```go
ws.POST("/issues/:id/labels", s.addLabelToIssue)
ws.DELETE("/issues/:id/labels/:label", s.removeLabelFromIssue)
```

**Step 3: Run tests**

Run: `go test ./internal/api/ -v`
Expected: PASS (update any test fixtures that reference workspace-scoped label endpoints)

**Step 4: Commit**

```bash
git add internal/api/labels.go internal/api/server.go
git commit -m "feat: move label CRUD endpoints to global scope"
```

---

### Task 6: Update OpenAPI spec and regenerate types

**Files:**
- Modify: `api/openapi.yaml`

**Step 1: Update OpenAPI spec**

1. Remove `workspace_id` from the `Label` schema required fields and properties
2. Move label CRUD paths from `/workspaces/{workspaceId}/labels` to `/labels`:
   - `GET /labels` — listLabels
   - `POST /labels` — createLabel
   - `PUT /labels/{labelName}` — updateLabel
   - `DELETE /labels/{labelName}` — deleteLabel
3. Keep issue-label paths under workspaces (unchanged)

**Step 2: Regenerate**

Run: `make gen`

This updates `internal/api/openapi.gen.go` and `web/src/lib/api/types.ts`.

**Step 3: Verify build**

Run: `make build-quick`
Expected: Builds successfully

**Step 4: Commit**

```bash
git add api/openapi.yaml internal/api/openapi.gen.go web/src/lib/api/types.ts
git commit -m "feat: update OpenAPI spec for global labels"
```

---

### Task 7: Update frontend API client

**Files:**
- Modify: `web/src/lib/api/index.ts`

**Step 1: Update listLabels and add CRUD functions**

Replace the existing `listLabels` function and add new ones:

```typescript
// Label APIs (global)
export async function listLabels(): Promise<Label[]> {
	const { data, error } = await api.GET('/labels');
	if (error) handleError(error);
	return data ?? [];
}

export async function createLabel(
	name: string,
	color?: string,
	description?: string
): Promise<Label> {
	const { data, error } = await api.POST('/labels', {
		body: { name, color, description }
	});
	if (error) handleError(error);
	return data!;
}

export async function updateLabel(
	name: string,
	color?: string,
	description?: string
): Promise<Label> {
	const { data, error } = await api.PUT('/labels/{labelName}', {
		params: { path: { labelName: name } },
		body: { color, description }
	});
	if (error) handleError(error);
	return data!;
}

export async function deleteLabel(name: string): Promise<void> {
	const { error } = await api.DELETE('/labels/{labelName}', {
		params: { path: { labelName: name } }
	});
	if (error) handleError(error);
}
```

**Step 2: Fix any callers of listLabels that pass workspaceId**

Search for `listLabels(` in the web/ directory. Update any calls to remove the workspaceId argument.

**Step 3: Commit**

```bash
git add web/src/lib/api/index.ts
git commit -m "feat: add global label CRUD functions to frontend API client"
```

---

### Task 8: Create ColorPicker component

> **Use frontend-design skill for this task.**

**Files:**
- Create: `web/src/lib/components/ColorPicker.svelte`

**Step 1: Build the color picker**

A preset palette grid + hex input. Props: `value` (bound color string), `onchange` callback.

Curated palette (~16 colors that work on the dark theme):

```typescript
const palette = [
	'#ef4444', '#f97316', '#f59e0b', '#eab308',
	'#84cc16', '#22c55e', '#14b8a6', '#06b6d4',
	'#3b82f6', '#6366f1', '#8b5cf6', '#a855f7',
	'#d946ef', '#ec4899', '#f43f5e', '#6b7280',
];
```

Component structure:
- Grid of clickable color swatches (4x4)
- Selected swatch shows a check mark
- Below: hex text input for custom colors (validates `#xxxxxx` format)
- Svelte 5 runes for state (`$state`, `$derived`)

**Step 2: Commit**

```bash
git add web/src/lib/components/ColorPicker.svelte
git commit -m "feat: add ColorPicker component with preset palette"
```

---

### Task 9: Create global labels management page

> **Use frontend-design skill for this task.**

**Files:**
- Create: `web/src/routes/labels/+page.svelte`
- Modify: `web/src/lib/components/Sidebar.svelte` (add Labels nav link outside workspace)

**Step 1: Build the labels management page**

Page layout:
- Header: "Labels" title + "New Label" button
- Grid of label cards (existing card pattern from current labels page)
- Each card: color swatch, name, description, edit/delete buttons
- Create/edit form (inline or modal):
  - Name input (required, disabled during edit)
  - ColorPicker component
  - Description textarea
  - Save/Cancel buttons
- Delete: uses existing ConfirmDialog component

Follow existing patterns from the workspace labels page (`web/src/routes/[workspaceId]/labels/+page.svelte`) but:
- No workspace context needed
- Add full CRUD functionality
- Use the new global API functions from Task 7

**Step 2: Add sidebar link**

In `Sidebar.svelte`, add a "Labels" link in the global navigation section (outside the workspace list), linking to `/labels`.

**Step 3: Remove or redirect old workspace labels page**

The old `web/src/routes/[workspaceId]/labels/+page.svelte` should either be removed or redirect to `/labels`.

**Step 4: Commit**

```bash
git add web/src/routes/labels/ web/src/lib/components/Sidebar.svelte
git rm web/src/routes/\[workspaceId\]/labels/+page.svelte  # if removing
git commit -m "feat: add global labels management page with color picker"
```

---

### Task 10: Render colored label badges on issue cards

> **Use frontend-design skill for this task.**

**Files:**
- Modify: `web/src/lib/components/IssueCard.svelte:71-87`

**Step 1: Pass label color map to IssueCard**

The parent page that renders IssueCards needs to fetch `listLabels()` and build a `Map<string, Label>` for color lookups. Pass this map as a prop to IssueCard.

**Step 2: Update label rendering in IssueCard**

Replace the monochrome `bg-surface-600` pills (lines 71-87) with colored badges. Use the label's hex color with low opacity for background, similar to StatusBadge pattern:

```svelte
{#each issue.labels.slice(0, 3) as label (label)}
    {@const color = labelMap?.get(label)?.color}
    <span
        class="px-1.5 py-0.5 text-[10px] font-medium rounded border"
        style={color
            ? `background-color: ${color}20; color: ${color}; border-color: ${color}40`
            : ''}
        class:bg-surface-600={!color}
        class:text-text-secondary={!color}
        class:border-transparent={!color}
    >
        {label}
    </span>
{/each}
```

The `20` and `40` hex suffixes give ~12% and ~25% opacity respectively — visible but not overwhelming on the dark theme.

**Step 3: Commit**

```bash
git add web/src/lib/components/IssueCard.svelte
git commit -m "feat: render colored label badges on issue cards"
```

---

### Task 11: Update CLI label commands (if any reference workspace)

**Files:**
- Search: `cmd/arc/` for label-related commands

**Step 1: Check for CLI label commands**

Search `cmd/arc/` for any files handling `arc label` commands. If they pass workspace_id to the API, update them to use the new global `/labels` endpoints.

**Step 2: Update client functions**

In `internal/client/client.go`, update any label-related client methods to use the new global paths (no workspace prefix).

**Step 3: Run full tests**

Run: `make test`
Expected: All tests pass

**Step 4: Commit**

```bash
git add cmd/arc/ internal/client/
git commit -m "feat: update CLI label commands for global labels"
```

---

### Task 12: Final verification

**Step 1: Full build**

Run: `make build`
Expected: Builds successfully (frontend + binaries)

**Step 2: Run all tests**

Run: `make test`
Expected: All pass

**Step 3: Manual smoke test**

1. Start server: `./bin/arc-server`
2. Open web UI
3. Navigate to global Labels page
4. Create a label with color picker
5. Verify it appears in the list with correct color
6. Edit the label's color
7. Create an issue and add the label
8. Verify IssueCard shows colored badge
9. Delete the label

**Step 4: Final commit if any fixes needed**
