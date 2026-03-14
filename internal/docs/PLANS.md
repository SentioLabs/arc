# Plans

Plans are ephemeral review artifacts backed by filesystem markdown files. They support a review workflow with approval, rejection, and comments.

## Workflow

1. Write your design to a markdown file in `docs/plans/`
2. Register it with `arc plan create --file <path>` — returns a plan ID
3. Review via web UI or CLI (add comments, discuss)
4. Approve or reject the plan
5. If approved, write the design content into the epic's description

## Commands

```bash
# Register a plan for review
arc plan create --file docs/plans/2026-03-14-auth-system.md

# View plan content, status, and comments
arc plan show <plan-id>

# Approve the plan
arc plan approve <plan-id>

# Reject the plan
arc plan reject <plan-id>

# List review comments
arc plan comments <plan-id>
```

## File Naming Convention

Use `YYYY-MM-DD-<topic>.md` for plan files:

```
docs/plans/2026-03-14-auth-system.md
docs/plans/2026-03-15-api-redesign.md
```

## Review Cycle

Plans go through a review cycle:

```
create → review (with comments) → approve or reject
                ↑                       |
                └── revise and re-register ←─┘ (if rejected)
```

After registration, the plan is visible in the web planner UI where reviewers can add comments. Use `arc plan comments <plan-id>` to read feedback, revise the file, and re-register if needed.

## Integration with Epics

Once a plan is approved, the design content is written into the epic's description field:

```bash
arc update <epic-id> --stdin <<'EOF'
<approved design content with task breakdown>
EOF
```

This keeps the implementation context directly on the epic for reference during execution.
