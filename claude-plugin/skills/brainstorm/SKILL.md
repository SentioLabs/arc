---
name: brainstorm
description: You MUST use this skill for any design exploration, architecture decision, or trade-off analysis before implementation begins — especially when the user says "brainstorm", "explore the design", "think through", "what approach should we take", or describes a feature with multiple valid strategies. This is the arc-native brainstorming skill that writes designs to docs/plans/ and registers them for review via arc plan. Always prefer this over generic brainstorming when the project uses arc issue tracking.
---

# Brainstorm — Design Discovery

Explore requirements through Socratic dialogue before any implementation begins.

## Hard Gate

**Do NOT write any implementation code, scaffold any project, or take any implementation action until the design is approved.** Brainstorming produces a design document — not code.

## Workflow

Create a TodoWrite checklist with these steps and work through them:

### 1. Explore Project Context

- Check existing files, docs, recent commits
- Review existing arc issues (`arc list`)
- Understand what already exists and what constraints are in play

### 2. Ask Clarifying Questions

- Ask questions **one at a time** — don't dump a list
- **Use the AskUserQuestion tool** for multiple-choice decisions (2-4 options)
- Use open-ended text questions only when you need freeform feedback
- Understand: purpose, constraints, success criteria, target users
- Continue until you have enough to propose approaches

**Example AskUserQuestion usage:**
```
Question: "How should we handle session persistence?"
Options:
  - "In-memory only" (simplest, lost on restart)
  - "SQLite" (persistent, single-node, matches existing storage)
  - "Redis" (distributed, adds infrastructure dependency)
```

### 3. Propose 2-3 Approaches

- Each approach: summary, trade-offs, estimated complexity
- Include a recommendation with reasoning
- **Use the AskUserQuestion tool** to present approaches as structured choices
- Apply YAGNI — remove features from all designs that aren't explicitly required

**Example AskUserQuestion usage:**
```
Question: "Which approach should we go with?"
Options:
  - "Approach A: ..." (recommended — trade-offs...)
  - "Approach B: ..." (trade-offs...)
  - "Approach C: ..." (trade-offs...)
```

### 4. Present Design Section by Section

- Break the design into logical sections (data model, API, UI, etc.)
- Present each section and get user approval before moving to the next
- Iterate on sections as needed based on feedback

### 5. Identify Shared Contracts (Parallel Readiness)

If the design will produce multiple implementation tasks that could run in parallel, explicitly identify the **shared contracts** — types, interfaces, config keys, constants, and function signatures that multiple tasks will reference.

Present these to the user as a "foundation layer":

```
Shared contracts (referenced by multiple tasks):
- Type: `SessionConfig` in `internal/types/config.go`
- Config key: `user.session.timeout`
- Interface method: `Storage.GetSession(id string) (*Session, error)`
```

These contracts become a **foundation task** during planning — implemented sequentially before any parallel work begins. This prevents parallel agents from independently inventing conflicting names or duplicating shared types.

**Skip this step** if the design maps to a single task or purely sequential work.

### 6. Save Design and Register for Review

Write the design document to `docs/plans/` and register it as an ephemeral plan for review:

```bash
# Write the design markdown file
# Use YYYY-MM-DD-<topic>.md naming convention
cat > docs/plans/YYYY-MM-DD-<topic>.md <<'EOF'
<design content>
EOF

# Register the plan for review (returns a plan ID)
arc plan create --file docs/plans/YYYY-MM-DD-<topic>.md
```

The `arc plan create` command returns a plan ID. Use the plan ID to construct the planner URL in the next step.

### 7. Review Loop

After `arc plan create` returns the plan ID, present the user with the **planner URL** for web-based review. Determine the server URL from the arc config (default: `http://localhost:7432`):

```
Plan ready for review: http://localhost:7432/planner/<plan-id>
```

Then use the **AskUserQuestion tool:**
```
Question: "Plan registered for review at the URL above. How would you like to proceed?"
Options:
  - "Approve it" (approve and proceed to /arc:plan for implementation breakdown)
  - "I've submitted review comments in the planner" (read comments, revise, re-register)
  - "Reject it" (reject and start over)
```

**If user approves:**
```bash
arc plan approve <plan-id>
```
Then proceed to the `plan` skill.

**If user says "review submitted":**
```bash
# Read review comments
arc plan comments <plan-id>
# Revise the design file based on feedback, then re-register
arc plan create --file docs/plans/YYYY-MM-DD-<topic>.md
# Loop back to present review options
```

### 8. Transition

After the plan is approved:

- For large/medium work: invoke the `plan` skill to break the design into implementation tasks
- For small work: invoke the `implement` skill directly

**Example AskUserQuestion usage:**
```
Question: "Design is approved. What's next?"
Options:
  - "Move to /arc:plan to break this into trackable tasks"
  - "Move to /arc:implement to start building directly"
  - "I want to revise the design first"
```

## Scale Detection

| Indicator | Scale | Structure |
|-----------|-------|-----------|
| Multiple phases, weeks of work, cross-cutting concerns | Large | Meta epic → phase epics → tasks |
| Single feature, days of work, contained scope | Medium | Epic → tasks |
| One task, hours of work, obvious approach | Small | Single issue |

## Rules

- The ONLY next skill after brainstorm is `plan` (or `implement` for small work)
- Never invoke implementation skills from brainstorm
- Design documents go in `docs/plans/` and are registered via `arc plan create --file`
- Arc issues track persistent work; TodoWrite tracks your brainstorming checklist steps
- YAGNI: if the user didn't ask for it, don't design it
- Format all arc content (descriptions, plans, comments) per `skills/arc/_formatting.md`
