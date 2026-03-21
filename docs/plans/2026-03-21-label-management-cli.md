# Label Management CLI Commands

## Summary

Add CLI commands for managing global labels (`arc label`) and integrate label assignment into `arc create` and `arc update`. The storage layer, API endpoints, and OpenAPI spec already exist — this design covers only the missing CLI and HTTP client layers.

## Scale

**Small** — additive CLI commands + client methods on top of an already-complete backend.

## Design

### 1. HTTP Client Methods

New methods on `client.Client` in a new file `internal/client/labels.go`:

```go
// Global label CRUD
ListLabels() ([]*types.Label, error)
CreateLabel(name, color, description string) (*types.Label, error)
UpdateLabel(name, color, description string) (*types.Label, error)
DeleteLabel(name string) error

// Issue-label associations (project-scoped)
AddLabelToIssue(projectID, issueID, label string) error
RemoveLabelFromIssue(projectID, issueID, label string) error
```

These map 1:1 to the existing API endpoints:

- `GET /api/v1/labels`
- `POST /api/v1/labels`
- `PUT /api/v1/labels/{name}`
- `DELETE /api/v1/labels/{name}`
- `POST /api/v1/projects/{ws}/issues/{id}/labels`
- `DELETE /api/v1/projects/{ws}/issues/{id}/labels/{label}`

### 2. `arc label` Command Group

New file `cmd/arc/label.go` with subcommands:

```bash
arc label list                                          # List all labels
arc label create <name> [--color=#hex] [--description="..."]  # Create a label
arc label update <name> [--color=#hex] [--description="..."]  # Update metadata
arc label delete <name>                                 # Delete a label
```

#### `arc label list`

- Calls `client.ListLabels()`
- Table output: `NAME  COLOR  DESCRIPTION`
- Supports `--json` via the global `outputJSON` flag

#### `arc label create <name>`

- Name is a required positional arg
- `--color` and `--description` are optional flags
- Calls `client.CreateLabel(name, color, description)`
- Prints `Created label: <name>`

#### `arc label update <name>`

- Name is a required positional arg
- `--color` and `--description` are optional flags
- At least one flag must be provided
- Calls `client.UpdateLabel(name, color, description)`
- Prints `Updated label: <name>`

#### `arc label delete <name>`

- Name is a required positional arg
- Calls `client.DeleteLabel(name)`
- Prints `Deleted label: <name>`

### 3. `arc create` Integration

Add a repeatable `--label` flag to `createCmd`:

```bash
arc create "Fix login bug" --label=bug --label=urgent
```

After `CreateIssue` succeeds, loop over the `--label` values and call `AddLabelToIssue` for each. If any label-add fails, print a warning but don't fail the overall create (the issue was already created).

### 4. `arc update` Integration

Add repeatable `--label-add` and `--label-remove` flags to `updateCmd`:

```bash
arc update ABC-1 --label-add=critical --label-remove=stale
```

- If field updates exist (status, title, etc.), call `UpdateIssue` first
- Then apply label additions via `AddLabelToIssue`
- Then apply label removals via `RemoveLabelFromIssue`
- Label-only updates (no field changes) are valid — skip the `UpdateIssue` call
- Adjust the "no updates specified" check to also consider label flags

### 5. Update to `arc prime` Context

Add `arc label` commands to the prime output so AI agents know about them:

```
arc label list                    # List all global labels
arc label create <name>           # Create a label (--color, --description)
arc label delete <name>           # Delete a label
arc create "title" --label=foo    # Create issue with labels
arc update <id> --label-add=foo    # Add label to issue
arc update <id> --label-remove=x  # Remove label from issue
```

## Files to Create/Modify

- **Create**: `internal/client/labels.go` — client methods for label API
- **Create**: `internal/client/labels_test.go` — client tests
- **Create**: `cmd/arc/label.go` — `arc label` command group
- **Create**: `cmd/arc/label_test.go` — command tests
- **Modify**: `cmd/arc/main.go` — add `--label` to `createCmd`, `--label-add`/`--label-remove` to `updateCmd`, register `labelCmd`
- **Modify**: `cmd/arc/prime.go` — add label commands to prime output

## Not in Scope

- No changes to storage, API, or OpenAPI spec (already complete)
- No label auto-creation on `--label` flag (label must exist; API returns error if not)
- No `arc label rename` (would require cascading updates — defer to future)
