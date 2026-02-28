# Plans

Plans provide structured context for how work should be executed. Arc supports three plan patterns, each suited to different coordination needs.

## Three Plan Patterns

| Pattern | When to Use | How It Works |
|---------|------------|--------------|
| **Inline Plan** | Single issue with clear steps | Plan stored directly on the issue as a special comment |
| **Parent Epic** | Epic with children that share a plan | Set plan on parent; children inherit via parent-child dependency |
| **Shared Plan** | Initiative spanning unrelated issues | Standalone plan object linked to multiple issues |

---

## Inline Plans

Attach a plan directly to an issue. The plan is stored as a comment with type `plan`, and only the latest version is shown (previous versions are kept as history).

### Commands

```bash
# Set a plan (provide text directly)
arc plan set <issue-id> "Step 1: Do X. Step 2: Do Y."

# Set a plan (open $EDITOR)
arc plan set <issue-id> --editor

# View the plan
arc plan show <issue-id>

# View plan version history
arc plan history <issue-id>
```

### When to Use

- Single-issue work with a clear sequence of steps
- Recording implementation decisions before starting work
- Updating the plan as understanding evolves (history is preserved)

---

## Parent Epic Pattern

Set an inline plan on a parent issue. Any child issue linked via `parent-child` dependency automatically inherits the parent's plan. This is displayed when viewing child issues with `arc show` or `arc plan show`.

### Commands

```bash
# Create the epic with a plan
arc create "Auth system overhaul" -t epic
arc plan set <epic-id> "Phase 1: JWT tokens. Phase 2: OAuth. Phase 3: RBAC."

# Create children — they inherit the plan automatically
arc create "Implement JWT tokens" -t task --parent <epic-id>
arc create "Add OAuth provider" -t task --parent <epic-id>

# Viewing a child shows the inherited plan
arc plan show <child-id>
```

### When to Use

- Epic with multiple subtasks that need shared context
- Work breakdown where children should see the bigger picture
- Plans that evolve as the epic progresses

**Note**: `parent-child` dependencies affect ready work — children are excluded from `arc ready` until the parent is closed.

---

## Shared Plans

Standalone plan objects that can be linked to multiple unrelated issues across the workspace. Useful for cross-cutting initiatives.

### Commands

```bash
# Create a shared plan
arc plan create "Q1 Performance Initiative"

# Create with $EDITOR for longer content
arc plan create "Q1 Performance Initiative" --editor

# List all shared plans
arc plan list

# Edit a shared plan
arc plan edit <plan-id>

# Link issues to the plan
arc plan link <plan-id> <issue-1> <issue-2> ...

# Unlink an issue
arc plan unlink <plan-id> <issue-id>

# Delete a shared plan (removes all linkages)
arc plan delete <plan-id>
```

### When to Use

- Cross-cutting initiatives that span multiple unrelated issues
- Coordination plans that aren't tied to a single epic hierarchy
- Shared context for issues that need to reference the same strategy

---

## Plan Context Aggregation

When you run `arc show <issue-id>`, arc aggregates all plan context for that issue:

1. **Inline plan** — set directly on this issue
2. **Parent plan** — inherited from parent issue (via parent-child dependency)
3. **Shared plans** — linked to this issue

All three sources are displayed together, giving a complete picture of the planning context.

`arc plan show <issue-id>` provides a more detailed view of the same information.

---

## Plan History

Inline plans are versioned automatically. Each time you run `arc plan set`, a new version is created. The latest version is displayed by default.

```bash
# View all versions
arc plan history <issue-id>
```

History shows each version with its timestamp and author, newest first.

---

## Choosing the Right Pattern

```
Single issue, clear steps?
  → Inline plan

Epic with subtasks sharing context?
  → Parent epic pattern (set plan on parent, children inherit)

Multiple unrelated issues, shared strategy?
  → Shared plan (link to each issue)
```

You can combine patterns — an issue can have an inline plan, inherit a parent plan, and be linked to shared plans simultaneously. All are shown together in `arc show`.
