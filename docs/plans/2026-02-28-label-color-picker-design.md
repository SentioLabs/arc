# Global Labels with Color Picker — Design

## Problem

Labels are currently workspace-scoped (`PRIMARY KEY (workspace_id, name)`), which is unnecessarily complex — labels like "bug", "feature", "urgent" should be shared across workspaces. Additionally, labels support a `color` field throughout the backend, but the web UI has no color picker for creating/editing labels, and issue cards display all labels in monochrome gray.

## Design

### Schema Migration

Drop `workspace_id` from the `labels` table. Deduplicate on collision (keep first, drop dupes).

```sql
CREATE TABLE labels_new (
    name TEXT PRIMARY KEY,
    color TEXT,
    description TEXT
);

INSERT OR IGNORE INTO labels_new (name, color, description)
    SELECT name, color, description FROM labels;

DROP TABLE labels;
ALTER TABLE labels_new RENAME TO labels;
```

`issue_labels` table unchanged — already references `(issue_id, label)` with no workspace_id.

### Backend Changes

**Types** (`internal/types/types.go`):
- Remove `WorkspaceID` from `Label` struct

**Storage interface** (`internal/storage/storage.go`):
- Drop `workspaceID` parameter from `GetLabel`, `ListLabels`, `DeleteLabel`

**SQLite** (`internal/storage/sqlite/labels.go`):
- Update queries to remove workspace_id WHERE clauses

**API** (`internal/api/labels.go`):
- Move label CRUD endpoints to top-level:
  - `GET /labels`, `POST /labels`, `PUT /labels/{labelName}`, `DELETE /labels/{labelName}`
- Issue label endpoints stay workspace-scoped (they're about issues)

**OpenAPI spec** (`api/openapi.yaml`):
- Update label schemas and paths, regenerate types

### Web UI

**Global labels page** — new route at `/labels` (outside workspace context):
- Grid of label cards (existing layout)
- Create/edit form with:
  - Name text input
  - Preset color palette (~12-16 curated dark-theme-friendly swatches) + custom hex input
  - Description text input
- Delete via existing ConfirmDialog

**Color picker component**:
- Preset palette grid (clickable swatches, selected state with ring/check)
- Optional hex text input for custom colors
- No full HSL/gradient picker — keep it simple

**Issue cards** (`IssueCard.svelte`):
- Replace monochrome `bg-surface-600` with label's actual color
- Pattern: low-opacity background + readable text (like StatusBadge: `bg-{color}/15 text-{color}`)
- Fetch `GET /labels` once, cache label→color map
- Fallback to gray when no color set

### What doesn't change

- `issue_labels` table (already workspace-independent in its reference pattern)
- Issue label add/remove endpoints (stay under `/workspaces/{id}/issues/{id}/labels`)
- CLI `arc label` commands (will need workspace flag removed, but that's a follow-up)
