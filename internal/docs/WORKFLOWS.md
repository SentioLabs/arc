# Workflows and Checklists

Detailed step-by-step workflows for common arc usage patterns with checklists.

## Contents

- [Session Start Workflow](#session-start) - Check arc ready, establish context
- [Compaction Survival](#compaction-survival) - Recovering after compaction events
- [Discovery and Issue Creation](#discovery) - Proactive issue creation during work
- [Status Maintenance](#status-maintenance) - Keeping arc status current
- [Epic Planning](#epic-planning) - Structuring complex work with dependencies
- [Side Quest Handling](#side-quests) - Discovery during main task, assessing blocker vs deferrable, resuming
- [Multi-Session Resume](#resume) - Returning after days/weeks away
- [Session Handoff Workflow](#session-handoff) - Collaborative handoff between sessions
- [Unblocking Work](#unblocking) - Handling blocked issues
- [Integration with TodoWrite](#integration-with-todowrite) - Using both tools together
- [Common Workflow Patterns](#common-workflow-patterns)
- [Checklist Templates](#checklist-templates)
- [Decision Points](#decision-points)
- [Troubleshooting Workflows](#troubleshooting-workflows)

## Session Start Workflow {#session-start}

**Arc is available when**:
- Server is running (daemon or standalone)
- Workspace initialized with `arc init`

**Automatic checklist at session start:**

```
Session Start (when arc is available):
- [ ] Run arc ready
- [ ] Report: "X items ready to work on: [summary]"
- [ ] If none ready, check arc blocked
- [ ] Suggest next action based on findings
```

**Pattern**: Always run `arc ready` when starting work where arc is available. Report status immediately to establish shared context.

**Server connection**: Arc automatically connects to the server. If not running, start with `arc server` or let daemon auto-start.

---

## Compaction Survival {#compaction-survival}

**Critical**: After compaction events, conversation history is deleted but arc state persists on the server. Arc issues are your only memory.

**Post-compaction recovery checklist:**

```
After Compaction:
- [ ] Run arc list --status in_progress to see active work
- [ ] Run arc show <issue-id> for each in_progress issue
- [ ] Read notes field to understand: COMPLETED, IN PROGRESS, BLOCKERS, KEY DECISIONS
- [ ] Check dependencies: arc dep tree <issue-id> for context
- [ ] If notes insufficient, check arc list --status open for related issues
- [ ] Reconstruct TodoWrite list from notes if needed
```

**Pattern**: Well-written notes enable full context recovery even with zero conversation history.

**Writing notes for compaction survival:**

**Good note (enables recovery):**
```
arc update issue-42 --notes "COMPLETED: User authentication - added JWT token
generation with 1hr expiry, implemented refresh token endpoint using rotating
tokens pattern. IN PROGRESS: Password reset flow. Email service integration
working. NEXT: Need to add rate limiting to reset endpoint (currently unlimited
requests). KEY DECISION: Using bcrypt with 12 rounds after reviewing OWASP
recommendations, tech lead concerned about response time but benchmarks show <100ms."
```

**Bad note (insufficient for recovery):**
```
arc update issue-42 --notes "Working on auth feature. Made some progress.
More to do later."
```

The good note contains:
- Specific accomplishments (what was implemented/configured)
- Current state (which part is working, what's in progress)
- Next concrete step (not just "continue")
- Key context (team concerns, technical decisions with rationale)

**After compaction**: `arc show issue-42` reconstructs the full context needed to continue work.

---

## Discovery and Issue Creation {#discovery}

**When encountering new work during implementation:**

```
Discovery Workflow:
- [ ] Notice bug, improvement, or follow-up work
- [ ] Assess: Can defer or is blocker?
- [ ] Create issue with arc create "Issue title"
- [ ] Add discovered-from dependency: arc dep add current-id new-id --type discovered-from
- [ ] If blocker: pause and switch; if not: continue current work
- [ ] Issue persists for future sessions
```

**Pattern**: Proactively file issues as you discover work. Context captured immediately instead of lost when session ends.

**When to ask first**:
- Knowledge work with fuzzy scope
- User intent unclear
- Multiple valid approaches

**When to create directly**:
- Clear bug found
- Obvious follow-up work
- Technical debt with clear scope

---

## Status Maintenance {#status-maintenance}

**Throughout work on an issue:**

```
Issue Lifecycle:
- [ ] Start: Update status to in_progress
- [ ] During: Add notes as decisions made
- [ ] During: Update acceptance criteria if requirements clarify
- [ ] During: Add dependencies if blockers discovered
- [ ] Complete: Close with summary of what was done
- [ ] After: Check arc ready to see what unblocked
```

**Pattern**: Keep arc status current so project state is always accurate.

**Status transitions**:
- `open` → `in_progress` when starting work
- `in_progress` → `blocked` if blocker discovered
- `blocked` → `in_progress` when unblocked
- `in_progress` → `closed` when complete

---

## Epic Planning {#epic-planning}

**For complex multi-step features, think in Ready Fronts, not phases.**

### The Ready Front Model

A **Ready Front** is the set of issues with all dependencies satisfied - what can be worked on *right now*. As issues close, the front advances. The dependency DAG IS the execution plan.

```
Ready Front = Issues where all dependencies are closed
              (no blockers remaining)

Static view:  Natural topology in the DAG (sync points, bottlenecks)
Dynamic view: Current wavefront of in-progress work
```

**Why Ready Fronts, not Phases?**

"Phases" trigger temporal reasoning that inverts dependencies:

```
⚠️ COGNITIVE TRAP:
"Phase 1 before Phase 2" → brain thinks "Phase 1 blocks Phase 2"
                         → WRONG: arc dep add phase1 phase2

Correct: "Phase 2 needs Phase 1" → arc dep add phase2 phase1
```

**The fix**: Name issues by what they ARE, think about what they NEED.

### Epic Planning Workflow

```
Epic Planning with Ready Fronts:
- [ ] Create epic issue for high-level goal
- [ ] Walk backward from goal: "What does the end state need?"
- [ ] Create child issues named by WHAT, not WHEN
- [ ] Add deps using requirement language: "X needs Y" → arc dep add X Y
- [ ] Verify with arc blocked (tasks blocked BY prerequisites, not dependents)
- [ ] Use arc ready to work through in dependency order
```

### The Graph Walk Pattern

Walk **backward** from the goal to get correct dependencies:

```
Start: "What's the final deliverable?"
       ↓
       "Integration tests passing" → gt-integration
       ↓
"What does that need?"
       ↓
       "Streaming support" → gt-streaming
       "Header display" → gt-header
       ↓
"What do those need?"
       ↓
       "Message rendering" → gt-messages
       ↓
"What does that need?"
       ↓
       "Buffer layout" → gt-buffer (foundation, no deps)
```

This produces correct deps because you're asking "X needs Y", not "X before Y".

### Example: OAuth Integration

```bash
# Create epic (the goal)
arc create "OAuth integration" -t epic

# Walk backward: What does OAuth need?
arc create "Login/logout endpoints" -t task        # needs token storage
arc create "Token storage and refresh" -t task     # needs auth flow
arc create "Authorization code flow" -t task       # needs credentials
arc create "OAuth client credentials" -t task      # foundation

# Add deps using requirement language: "X needs Y"
arc dep add endpoints storage      # endpoints need storage
arc dep add storage flow           # storage needs flow
arc dep add flow credentials       # flow needs credentials
# credentials has no deps - it's Ready Front 1

# Verify: arc blocked should show sensible blocking
arc blocked
# endpoints blocked by storage ✓
# storage blocked by flow ✓
# flow blocked by credentials ✓
# credentials ready ✓
```

---

## Side Quest Handling {#side-quests}

**When discovering work that pauses main task:**

```
Side Quest Workflow:
- [ ] During main work, discover problem or opportunity
- [ ] Create issue for side quest
- [ ] Add discovered-from dependency linking to main work
- [ ] Assess: blocker or can defer?
- [ ] If blocker: mark main work blocked, switch to side quest
- [ ] If deferrable: note in issue, continue main work
- [ ] Update statuses to reflect current focus
```

**Example**: During feature implementation, discover architectural issue

```
Main task: Adding user profiles

Discovery: Notice auth system should use role-based access

Actions:
1. Create issue: "Implement role-based access control"
2. Link: discovered-from "user-profiles-feature"
3. Assess: Blocker for profiles feature
4. Mark profiles as blocked
5. Switch to RBAC implementation
6. Complete RBAC, unblocks profiles
7. Resume profiles work
```

---

## Multi-Session Resume {#resume}

**Starting work after days/weeks away:**

```
Resume Workflow:
- [ ] Run arc ready to see available work
- [ ] Run arc stats for project overview
- [ ] List recent closed issues for context
- [ ] Show details on issue to work on
- [ ] Review notes and acceptance criteria
- [ ] Update status to in_progress
- [ ] Begin work with full context
```

**Why this works**: Arc preserves notes, acceptance criteria, and dependency context on the server. No scrolling conversation history or reconstructing from markdown.

---

## Session Handoff Workflow {#session-handoff}

**Collaborative handoff between sessions using notes field:**

This workflow enables smooth work resumption by updating arc notes when stopping, then reading them when resuming. Works in conjunction with compaction survival - creates continuity even after conversation history is deleted.

### At Session Start (Claude's responsibility)

```
Session Start with in_progress issues:
- [ ] Run arc list --status in_progress
- [ ] For each in_progress issue: arc show <issue-id>
- [ ] Read notes field to understand: COMPLETED, IN PROGRESS, NEXT
- [ ] Report to user with context from notes field
- [ ] Example: "arc.a3f2 is in_progress. Last session:
       completed tidying. No code written yet. Next step: create
       markdown_to_docs.py. Should I continue with that?"
- [ ] Wait for user confirmation or direction
```

**Pattern**: Notes field is the "read me first" guide for resuming work.

### At Session End (Claude prompts user)

When wrapping up work on an issue:

```
Session End Handoff:
- [ ] Notice work reaching a stopping point
- [ ] Prompt user: "We just completed X and started Y on <issue-id>.
       Should I update the arc notes for next session?"
- [ ] If yes, suggest command:
       arc update <issue-id> --notes "COMPLETED: X. IN PROGRESS: Y. NEXT: Z"
- [ ] User reviews and confirms
- [ ] Claude executes the update
- [ ] Notes saved for next session's resumption
```

**Pattern**: Update notes at logical stopping points, not after every keystroke.

### Notes Format (Current State, Not Cumulative)

```
Good handoff note (current state):
COMPLETED: Parsed markdown into structured format
IN PROGRESS: Implementing Docs API insertion
NEXT: Debug batchUpdate call - getting 400 error on formatting
BLOCKER: None
KEY DECISION: Using two-phase approach (insert text, then apply formatting) based on reference implementation

Bad handoff note (not useful):
Updated some stuff. Will continue later.
```

**Rules for handoff notes:**
- Current state only (overwrite previous notes, not append)
- Specific accomplishments (not vague progress)
- Concrete next step (not "continue working")
- Optional: Blockers, key decisions, references
- Written for someone with zero conversation context

---

## Unblocking Work {#unblocking}

**When ready list is empty:**

```
Unblocking Workflow:
- [ ] Run arc blocked to see what's stuck
- [ ] Show details on blocked issues: arc show issue-id
- [ ] Identify blocker issues
- [ ] Choose: work on blocker, or reassess dependency
- [ ] If reassess: remove incorrect dependency
- [ ] If work on blocker: close blocker, check ready again
- [ ] Blocked issues automatically become ready when blockers close
```

**Pattern**: Arc automatically maintains ready state based on dependencies. Closing a blocker makes blocked work ready.

**Example**:

```
Situation: arc ready shows nothing

Actions:
1. arc blocked shows: "api-endpoint blocked by db-schema"
2. Show db-schema: "Create user table schema"
3. Work on db-schema issue
4. Close db-schema when done
5. arc ready now shows: "api-endpoint" (automatically unblocked)
```

---

## Integration with TodoWrite

**Using both tools in one session:**

```
Hybrid Workflow:
- [ ] Check arc for high-level context
- [ ] Choose arc issue to work on
- [ ] Mark arc issue in_progress
- [ ] Create TodoWrite from acceptance criteria for execution
- [ ] Work through TodoWrite items
- [ ] Update arc notes as you learn
- [ ] When TodoWrite complete, close arc issue
```

**Why hybrid**: Arc provides persistent structure, TodoWrite provides visible progress.

---

## Common Workflow Patterns

### Pattern: Systematic Exploration

Research or investigation work:

```
1. Create research issue with question to answer
2. Update notes field with findings as you go
3. Create new issues for discoveries
4. Link discoveries with discovered-from
5. Close research issue with conclusion
```

### Pattern: Bug Investigation

```
1. Create bug issue
2. Reproduce: note steps in description
3. Investigate: track hypotheses in notes field
4. Fix: implement solution
5. Test: verify in acceptance criteria
6. Close with explanation of root cause and fix
```

### Pattern: Refactoring with Dependencies

```
1. Create issues for each refactoring step
2. Add blocks dependencies for correct order
3. Work through in dependency order
4. arc ready automatically shows next step
5. Each completion unblocks next work
```

### Pattern: Spike Investigation

```
1. Create spike issue: "Investigate caching options"
2. Time-box exploration
3. Document findings in notes field
4. Create follow-up issues for chosen approach
5. Link follow-ups with discovered-from
6. Close spike with recommendation
```

---

## Checklist Templates

### Starting Any Work Session

```
- [ ] Verify arc server running
- [ ] Run arc ready
- [ ] Report status to user
- [ ] Get user input on what to work on
- [ ] Show issue details
- [ ] Update to in_progress
- [ ] Begin work
```

### Creating Issues During Work

```
- [ ] Notice new work needed
- [ ] Create issue with clear title
- [ ] Add context in description
- [ ] Link with discovered-from to current work
- [ ] Assess blocker vs deferrable
- [ ] Update statuses appropriately
```

### Completing Work

```
- [ ] Implementation done
- [ ] Tests passing
- [ ] Close issue with summary
- [ ] Check arc ready for unblocked work
- [ ] Report completion and next available work
```

### Planning Complex Features

```
- [ ] Create epic for overall goal
- [ ] Break into child tasks
- [ ] Create all child issues
- [ ] Link with parent-child dependencies
- [ ] Add blocks between children if order matters
- [ ] Work through in dependency order
```

---

## Decision Points

**Should I create an arc issue or use TodoWrite?**
→ Run `arc docs boundaries` for decision matrix

**Should I ask user before creating issue?**
→ Ask if scope unclear; create if obvious follow-up work

**Should I mark work as blocked or just note dependency?**
→ Blocked = can't proceed; dependency = need to track relationship

**Should I create epic or just tasks?**
→ Epic if 5+ related tasks; tasks if simpler structure

**Should I update status frequently or just at start/end?**
→ Start and end minimum; during work if significant changes

---

## Troubleshooting Workflows

**"I can't find any ready work"**
1. Run arc blocked
2. Identify what's blocking progress
3. Either work on blockers or create new work

**"I created an issue but it's not showing in ready"**
1. Run arc show on the issue
2. Check dependencies field
3. If blocked, resolve blocker first
4. If incorrectly blocked, remove dependency

**"Work is more complex than expected"**
1. Transition from TodoWrite to arc mid-session
2. Create arc issue with current context
3. Note: "Discovered complexity during implementation"
4. Add dependencies as discovered
5. Continue with arc tracking

**"I closed an issue but work isn't done"**
1. Reopen with `arc update <issue-id> --status open`
2. Or create new issue linking to closed one
3. Note what's still needed

**"Too many issues, can't find what matters"**
1. Use arc list with filters (priority, type)
2. Use arc ready to focus on unblocked work
3. Consider closing old issues that no longer matter
4. Use labels for organization
