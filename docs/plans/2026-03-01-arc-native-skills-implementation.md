# Arc-Native Skills — Implementation Plan

> **For Claude:** Implement this plan task-by-task. Each task creates one file. Read the design doc first.

**Goal:** Build 7 skills + 2 agents in arc's `claude-plugin/` that embed superpowers process disciplines into arc's issue lifecycle.

**Architecture:** Markdown skill/agent files in `claude-plugin/skills/<name>/SKILL.md` and `claude-plugin/agents/<name>.md`. Each skill is self-contained with frontmatter, workflow steps, iron laws, and arc CLI integration. No code compilation — these are prompt files.

**Required reading:** `docs/plans/2026-03-01-arc-native-skills-design.md`

**Existing patterns:** Follow the format of `claude-plugin/agents/arc-issue-tracker.md` (agent frontmatter) and `claude-plugin/skills/arc/SKILL.md` (skill structure).

---

## Arc Issue Tracking

Each task maps to an arc issue. Create the epic and tasks before starting implementation.

**Setup commands:**
```bash
# Create the epic
arc create "Arc-native workflow skills" --type=epic --priority=1

# Create tasks (use the epic ID as parent for each)
arc create "Write arc-implementer agent" --type=task --parent=<epic-id> --priority=1
arc create "Write arc-reviewer agent" --type=task --parent=<epic-id> --priority=1
arc create "Write brainstorm skill" --type=task --parent=<epic-id> --priority=1
arc create "Write plan skill" --type=task --parent=<epic-id> --priority=1
arc create "Write implement skill" --type=task --parent=<epic-id> --priority=1
arc create "Write debug skill" --type=task --parent=<epic-id> --priority=1
arc create "Write verify skill" --type=task --parent=<epic-id> --priority=1
arc create "Write review skill" --type=task --parent=<epic-id> --priority=1
arc create "Write finish skill" --type=task --parent=<epic-id> --priority=1
arc create "Update arc SKILL.md index" --type=task --parent=<epic-id> --priority=2

# Set dependencies
arc dep add <implement-id> <arc-implementer-id> --type=blocks
arc dep add <review-id> <arc-reviewer-id> --type=blocks
arc dep add <plan-id> <brainstorm-id> --type=blocks
arc dep add <implement-id> <plan-id> --type=blocks
arc dep add <finish-id> <verify-id> --type=blocks
arc dep add <update-index-id> <finish-id> --type=blocks
```

---

### Task 1: Write `arc-implementer` Agent

**Files:**
- Create: `claude-plugin/agents/arc-implementer.md`

**Step 1: Write the agent file**

The agent is dispatched by the `implement` skill. It receives a task description from `arc show <task-id>` and implements following TDD. It has a fresh context window — no prior conversation history.

**Content requirements:**
- **Frontmatter**: `description` field that triggers on task implementation dispatch. Tools: `Bash`, `Read`, `Write`, `Edit`, `Glob`, `Grep`.
- **Identity**: You are an implementation agent. You receive a single task, implement it using TDD, and report results.
- **TDD Iron Law**: "NO PRODUCTION CODE WITHOUT FAILING TEST FIRST." Include the full RED → GREEN → REFACTOR cycle with explicit steps.
- **Workflow**:
  1. Read task description (provided in dispatch prompt)
  2. Identify files to create/modify and test files
  3. RED: Write minimal failing test, run it, confirm it fails
  4. GREEN: Write simplest code to pass the test, run it, confirm it passes
  5. REFACTOR: Clean up while tests stay green
  6. Commit with conventional commit message
  7. Report: what was implemented, test results, files changed
- **Rationalizations table**: Counter "too simple to test" (simple code breaks, test takes 30 seconds), "I'll test after" (you won't, and you lose the design benefit), "this is just a config change" (config errors cause production outages)
- **Rules**: Never skip the failing test step. Never write implementation before seeing the test fail. Never use mocks when real code is available. Never touch files outside the task scope. Never interact with the user — report results back to the dispatching agent.

**Step 2: Verify**

- Frontmatter has correct `description` and `tools` fields
- TDD iron law is present with rationalizations table
- Workflow references no arc CLI commands (the dispatcher handles arc, not the implementer)
- Agent is self-contained — no references to other skills

**Step 3: Commit**

```bash
git add claude-plugin/agents/arc-implementer.md
git commit -m "feat(plugin): add arc-implementer agent for TDD task execution"
```

---

### Task 2: Write `arc-reviewer` Agent

**Files:**
- Create: `claude-plugin/agents/arc-reviewer.md`

**Step 1: Write the agent file**

The agent is dispatched by the `review` skill. It receives a git diff and task spec, reviews for issues, and reports findings categorized by severity.

**Content requirements:**
- **Frontmatter**: `description` field for code review dispatch. Tools: `Bash`, `Read`, `Glob`, `Grep`.
- **Identity**: You are a code review agent. You review changes against a task spec and project conventions.
- **Workflow**:
  1. Read the task spec (provided in dispatch prompt)
  2. Read the git diff (provided or retrieve via `git diff <base>..<head>`)
  3. Check spec compliance: Does the implementation match what was requested?
  4. Check code quality: Naming, structure, error handling, edge cases
  5. Check test quality: Coverage, assertions, edge cases tested
  6. Report findings in three categories
- **Output format**: Critical (must fix before proceeding), Important (should fix before proceeding), Minor (note for later). Each finding: file, line, issue, suggestion.
- **Discipline**: Technical evaluation, not performative agreement. No "Great work!" or "Looks good!" without specific evidence. If code is clean, say "No issues found" — not compliments.
- **Rules**: Never make code changes. Never close issues. Report only — the dispatching agent decides what to do with findings.

**Step 2: Verify**

- Frontmatter has correct fields
- Output format is specified (Critical/Important/Minor)
- No performative agreement language
- Agent is read-only — no edit/write tools

**Step 3: Commit**

```bash
git add claude-plugin/agents/arc-reviewer.md
git commit -m "feat(plugin): add arc-reviewer agent for code review dispatch"
```

---

### Task 3: Write `brainstorm` Skill

**Files:**
- Create: `claude-plugin/skills/brainstorm/SKILL.md`

**Step 1: Create directory and write the skill**

```bash
mkdir -p claude-plugin/skills/brainstorm
```

**Content requirements:**
- **Header**: Name, description ("Use when starting a new feature, project, or significant piece of work that needs design exploration before implementation")
- **Hard gate**: "Do NOT write any implementation code, scaffold any project, or take any implementation action until the design is approved."
- **Workflow** (as TodoWrite checklist):
  1. Explore project context — check files, docs, recent commits, existing arc issues
  2. Ask clarifying questions — one at a time, prefer multiple choice, understand purpose/constraints/success criteria
  3. Propose 2-3 approaches — with trade-offs and recommendation
  4. Present design — section by section, get user approval after each section
  5. Save to arc — create epic (or meta epic for large work) + save design as arc plan
  6. Transition — invoke the `plan` skill
- **Scale detection**:
  - Large (multi-phase): Create meta epic, phase epics as children, set dependencies between phases
  - Medium (single feature): Create single epic
  - Small (single task): Create single issue, skip to `implement`
- **Arc commands used**:
  - `arc create "Title" --type=epic` — create epic
  - `arc plan set <epic-id> "<design content>"` — save design as plan
  - `arc create "Phase N" --type=epic --parent=<meta-epic-id>` — create phase epics
  - `arc dep add <phase-2-id> <phase-1-id> --type=blocks` — set phase dependencies
- **YAGNI**: Remove unnecessary features from all designs
- **Terminal state**: The ONLY next skill is `plan`. Never invoke implementation skills from brainstorm.

**Step 2: Verify**

- Hard gate is present and explicit
- TodoWrite checklist has all 6 steps
- Scale detection covers all three levels
- Arc commands are correct (check against `arc --help`)
- No references to `docs/plans/` markdown files (arc plan is sole artifact)
- Terminal state clearly points to `plan` skill only

**Step 3: Commit**

```bash
git add claude-plugin/skills/brainstorm/SKILL.md
git commit -m "feat(plugin): add brainstorm skill for design discovery"
```

---

### Task 4: Write `plan` Skill

**Files:**
- Create: `claude-plugin/skills/plan/SKILL.md`

**Step 1: Create directory and write the skill**

```bash
mkdir -p claude-plugin/skills/plan
```

**Content requirements:**
- **Header**: Name, description ("Use after brainstorm to break an approved design into implementation tasks with exact file paths and steps")
- **Granularity rule**: Each task step is ONE action, 2-5 minutes. Assume the implementer has zero codebase context and questionable testing discipline.
- **Workflow** (as TodoWrite checklist):
  1. Read the design — `arc plan show <epic-id>` to load the approved design
  2. Identify tasks — break into self-contained units, each with exact file paths and complete code guidance
  3. Create arc issues — `arc create "Task title" --type=task --parent=<epic-id>` with full task description including files, steps, code
  4. Set dependencies — `arc dep add <task-B> <task-A> --type=blocks` where order matters
  5. Update epic plan — `arc plan set <epic-id> "<updated plan with task listing>"` to include the task breakdown
  6. Choose execution path — ask user: single-agent (invoke `implement`) or agentic team (add `teammate:*` labels, invoke `arc team-deploy`)
- **Task description format**: Each task's `--description` must be self-contained (~3-5k tokens):
  - Files to create/modify (exact paths)
  - Steps 1-5 (write failing test, see it fail, implement, see it pass, commit)
  - Complete code guidance (not "add validation" — actual code)
  - Expected test commands and outputs
- **Token efficiency**: The task description IS the implementation context. The implementer loads `arc show <task-id>` and nothing else. Don't reference external docs or the full plan — everything needed is in the description.
- **Team preparation**: If user chooses agentic team, prompt for `teammate:*` labels per task. Available roles: `teammate:frontend`, `teammate:backend`, `teammate:architect`, `teammate:tests`, `teammate:devops`, or custom.
- **Arc commands used**:
  - `arc plan show <epic-id>` — read design
  - `arc create "Title" --type=task --parent=<epic-id> --description="..."` — create tasks
  - `arc dep add <id> <id> --type=blocks` — set dependencies
  - `arc plan set <epic-id> "<content>"` — update plan with task listing
  - `arc label add <id> teammate:<role>` — optional team labels

**Step 2: Verify**

- Granularity rule is explicit
- Task description format matches the design (self-contained, ~3-5k)
- Arc plan is updated with task listing
- Team preparation path is optional, not required
- No references to writing plan markdown to `docs/plans/`

**Step 3: Commit**

```bash
git add claude-plugin/skills/plan/SKILL.md
git commit -m "feat(plugin): add plan skill for implementation task breakdown"
```

---

### Task 5: Write `implement` Skill

**Files:**
- Create: `claude-plugin/skills/implement/SKILL.md`

**Step 1: Create directory and write the skill**

```bash
mkdir -p claude-plugin/skills/implement
```

**Content requirements:**
- **Header**: Name, description ("Use to execute implementation tasks from a plan. Dispatches fresh subagents per task for context-efficient TDD execution")
- **Core rule**: The main agent NEVER writes implementation code. It orchestrates, dispatches, and reviews.
- **Workflow** (as TodoWrite checklist — this is the orchestration loop):
  1. Find next task — `arc ready` or `arc list --parent=<epic-id> --status=open`
  2. Claim task — `arc update <task-id> --status in_progress`
  3. Dispatch implementer — use Agent tool to spawn `arc-implementer` subagent with: task description from `arc show <task-id>`, any relevant project context
  4. Review result — when subagent reports back, check: did tests pass? was the approach correct?
  5. If issues — dispatch `debug` skill or re-dispatch implementer with feedback
  6. If clean — dispatch `arc-reviewer` subagent (invoke `review` skill)
  7. Process review — fix critical/important issues (re-dispatch implementer), note minor for later
  8. Close task — `arc close <task-id> -m "Implemented: <summary>"`
  9. Repeat — go to step 1 for next task
- **Subagent dispatch template**: Include the exact prompt format for spawning the `arc-implementer`:
  ```
  Implement this task following TDD (RED → GREEN → REFACTOR).

  ## Task
  <output of arc show <task-id>>

  ## Project Test Command
  <project's test command, e.g., make test, go test ./...>

  Commit your work when tests pass.
  ```
- **When to invoke debug**: If the subagent reports test failures it can't resolve, or if 3+ implementation attempts fail on the same issue.
- **Arc commands used**:
  - `arc ready` — find next task
  - `arc update <id> --status in_progress` — claim task
  - `arc show <id>` — get task description for subagent
  - `arc close <id> -m "reason"` — close completed task

**Step 2: Verify**

- Core rule is explicit: main agent never writes implementation code
- Subagent dispatch template is complete
- Orchestration loop covers the full cycle (find → claim → dispatch → review → close)
- Debug escalation path is defined
- Arc commands are correct

**Step 3: Commit**

```bash
git add claude-plugin/skills/implement/SKILL.md
git commit -m "feat(plugin): add implement skill for subagent-driven TDD execution"
```

---

### Task 6: Write `debug` Skill

**Files:**
- Create: `claude-plugin/skills/debug/SKILL.md`

**Step 1: Create directory and write the skill**

```bash
mkdir -p claude-plugin/skills/debug
```

**Content requirements:**
- **Header**: Name, description ("Use when encountering bugs, test failures, or unexpected behavior during implementation. Requires root cause investigation before any fix attempt")
- **Iron Law**: "NO FIXES WITHOUT ROOT CAUSE INVESTIGATION FIRST"
- **4 phases** (as TodoWrite checklist):
  1. **Investigate root cause** — Read error messages carefully. Reproduce the failure consistently. Check recent changes (`git diff`, `git log`). Gather evidence: stack traces, logs, test output. In multi-component systems, trace the data flow.
  2. **Pattern analysis** — Find working examples of similar code. Compare against references. Identify what's different between working and broken code.
  3. **Hypothesis testing** — Form a single hypothesis. Test it minimally (one change, one test). If wrong, revert and form new hypothesis. Don't stack fixes.
  4. **Implement fix** — Write a failing test that demonstrates the bug. Fix the root cause (not the symptom). Verify the fix makes the test pass. Run full test suite to check for regressions.
- **3-fix rule**: If you've tried 3 fixes and none worked, STOP. You don't understand the problem yet. Go back to phase 1 and investigate more deeply. Consider: are you fixing the right thing? Is the architecture wrong? Do you need to question your assumptions?
- **Arc integration**: If the bug is bigger than expected (not a quick fix within the current task), create a new arc issue: `arc create "Bug: <description>" --type=bug --priority=<severity>`
- **Red flags**: Fixing symptoms instead of causes. Applying fixes without understanding why they work. Copying code from Stack Overflow without understanding it. Making multiple changes at once.

**Step 2: Verify**

- Iron law is present and prominent
- All 4 phases are explicit with concrete actions
- 3-fix rule is present
- Arc integration for unexpected bugs
- No shortcut paths that skip investigation

**Step 3: Commit**

```bash
git add claude-plugin/skills/debug/SKILL.md
git commit -m "feat(plugin): add debug skill for systematic root cause investigation"
```

---

### Task 7: Write `verify` Skill

**Files:**
- Create: `claude-plugin/skills/verify/SKILL.md`

**Step 1: Create directory and write the skill**

```bash
mkdir -p claude-plugin/skills/verify
```

**Content requirements:**
- **Header**: Name, description ("Use before claiming any work is complete, any test passes, or any fix works. Requires fresh verification evidence before any completion claim")
- **Iron Law**: "NO COMPLETION CLAIMS WITHOUT FRESH VERIFICATION EVIDENCE"
- **Gate sequence** (as TodoWrite checklist):
  1. **IDENTIFY** — What command proves this claim? (e.g., `make test`, `go test ./...`, `arc show <id>`)
  2. **RUN** — Execute the full command. Not a subset. Not from memory. Fresh, complete execution.
  3. **READ** — Read the FULL output. Check exit code. Count failures. Don't skim.
  4. **VERIFY** — Does the output actually confirm the claim? "0 failures" not "tests ran".
  5. **ONLY THEN** — Make the claim. Reference the evidence.
- **Red flags that indicate skipped verification**:
  - Using "should work", "probably passes", "seems fine"
  - Expressing satisfaction before running the proof command
  - Running a subset of tests instead of the full suite
  - Trusting a subagent's report without checking
  - Saying "tests pass" without showing output
  - Claiming "no regressions" without running full suite
- **Arc integration**: Only `arc close <id>` AFTER verification passes. If verification fails, do NOT close — go back to `implement` or `debug`.

**Step 2: Verify**

- Iron law is present and prominent
- Gate sequence has all 5 steps
- Red flags list is present
- Arc close is gated behind verification
- No shortcut paths

**Step 3: Commit**

```bash
git add claude-plugin/skills/verify/SKILL.md
git commit -m "feat(plugin): add verify skill for evidence-based completion gates"
```

---

### Task 8: Write `review` Skill

**Files:**
- Create: `claude-plugin/skills/review/SKILL.md`

**Step 1: Create directory and write the skill**

```bash
mkdir -p claude-plugin/skills/review
```

**Content requirements:**
- **Header**: Name, description ("Use after implementing a task to get code review. Dispatches the arc-reviewer agent and triages feedback")
- **Workflow** (as TodoWrite checklist):
  1. Get git SHAs — `BASE_SHA=$(git rev-parse HEAD~<n>)` where n = number of commits for this task. `HEAD_SHA=$(git rev-parse HEAD)`.
  2. Dispatch reviewer — use Agent tool to spawn `arc-reviewer` subagent with: git diff (`git diff <base>..<head>`), task spec (from `arc show <task-id>`), project conventions (from CLAUDE.md)
  3. Triage feedback — Critical: fix immediately before proceeding. Important: fix before moving to next task. Minor: note in arc issue comment for later.
  4. If fixes needed — re-dispatch `arc-implementer` with the specific fixes required. Then re-review.
  5. If clean — proceed to next task or `finish`
- **Dispatch template**: Include exact prompt for the `arc-reviewer`:
  ```
  Review these changes against the task spec and project conventions.

  ## Task Spec
  <output of arc show <task-id>>

  ## Changes
  <output of git diff BASE..HEAD>

  Report findings as: Critical (must fix), Important (should fix), Minor (note for later).
  ```
- **Response discipline**: When receiving review feedback, evaluate technically. Don't agree performatively. If a finding is wrong, explain why with evidence. If it's right, fix it.
- **Works in both contexts**: Single-agent (dispatch `arc-reviewer` subagent) and team mode (team lead dispatches QA teammate or subagent).

**Step 2: Verify**

- Dispatch template is complete
- Triage levels are defined (Critical/Important/Minor)
- Response discipline is explicit
- Both single-agent and team contexts noted
- Re-review loop is defined for fixes

**Step 3: Commit**

```bash
git add claude-plugin/skills/review/SKILL.md
git commit -m "feat(plugin): add review skill for code review dispatch"
```

---

### Task 9: Write `finish` Skill

**Files:**
- Create: `claude-plugin/skills/finish/SKILL.md`

**Step 1: Create directory and write the skill**

```bash
mkdir -p claude-plugin/skills/finish
```

**Content requirements:**
- **Header**: Name, description ("Use at the end of a session to capture remaining work, run quality gates, update arc issues, and commit/push all changes. Replaces both 'Landing the Plane' and 'finishing-a-development-branch'")
- **Iron Law**: "Work is NOT done until `git push` succeeds. No exceptions."
- **Protocol** (as TodoWrite checklist):

  **Phase 1: Capture Remaining Work**
  1. Review what was planned vs what was completed
  2. `arc create` for any unfinished work or newly discovered tasks
  3. Add context notes to new issues so the next session can pick up

  **Phase 2: Quality Gates** (if code was changed)
  4. Run project test suite (`make test`, `go test ./...`, etc.)
  5. Run linter/formatter if configured
  6. Run build if applicable
  7. **Hard gate**: If tests fail, fix them. Do NOT skip to commit.

  **Phase 3: Update Arc Issues**
  8. `arc close <id> -m "reason"` for completed work
  9. `arc update <id> --description "progress notes"` for in-progress work
  10. Verify issue states match reality

  **Phase 4: Commit and Push**
  11. `git add` changed files (specific files, not `-A`)
  12. `git commit` with conventional commit message
  13. `git push`
  14. `git status` — must show "up to date with origin"
  15. If push fails → resolve → retry → succeed

  **Phase 5: Verify and Hand Off**
  16. `git log -1` — confirm latest commit visible
  17. `arc prime` — output context for next session

- **Context-aware behavior**:
  - Single-agent session: Full protocol
  - Team lead session: Also verify teammate work + team cleanup before commit
  - Teammate session: Commit + push only (team lead handles arc close)
- **What's NOT in this protocol**: `git stash clear`, `git remote prune origin` (housekeeping, not gates). Worktree cleanup (orthogonal). The 4-option merge/PR/keep/discard choice (arc workflow always commits and pushes).

**Step 2: Verify**

- Iron law about push is present
- All 5 phases are explicit
- Quality gate hard gate is present (no skipping tests)
- Context-aware behavior covers all three session types
- Arc commands are correct
- `git push` is mandatory, not optional

**Step 3: Commit**

```bash
git add claude-plugin/skills/finish/SKILL.md
git commit -m "feat(plugin): add finish skill for unified session completion"
```

---

### Task 10: Update `skills/arc/SKILL.md` Index

**Files:**
- Modify: `claude-plugin/skills/arc/SKILL.md`

**Step 1: Add workflow skills section**

After the existing "When to Use Arc vs TodoWrite" section, add a new section:

```markdown
## Workflow Skills

Arc includes workflow skills that guide you through the development lifecycle with built-in process discipline.

| Skill | Purpose | Invoke when |
|-------|---------|-------------|
| `brainstorm` | Design discovery through Socratic dialogue | Starting new features or significant work |
| `plan` | Break design into implementation tasks | After brainstorm approves a design |
| `implement` | TDD execution via fresh subagents per task | Ready to implement planned tasks |
| `debug` | 4-phase root cause investigation | Encountering bugs or test failures |
| `verify` | Evidence-based completion gates | Before claiming any work is done |
| `review` | Code review dispatch and triage | After implementing a task |
| `finish` | Session completion protocol | Ending a work session |

### Pipeline

```
brainstorm → plan → implement (per task) → review → finish
                        ↕          ↕
                      debug      verify
```

### Execution Paths

After `plan`, choose:
- **Single-agent + subagents**: Invoke `implement`. Main agent orchestrates, subagents do TDD. Best for sequential tasks.
- **Agentic team**: Add `teammate:*` labels, invoke `arc team-deploy`. Best for parallel multi-role work.
```

**Step 2: Verify**

- New section doesn't break existing content
- All 7 skills are listed
- Pipeline diagram matches the design
- Both execution paths mentioned

**Step 3: Commit**

```bash
git add claude-plugin/skills/arc/SKILL.md
git commit -m "docs(plugin): add workflow skills index to arc SKILL.md"
```

---

## Execution

**Plan complete.** Two execution options:

**1. Subagent-Driven (this session)** — Dispatch fresh subagent per task, review between tasks, fast iteration.

**2. Parallel Session (separate)** — Open new session, batch execution with checkpoints.

Which approach?
