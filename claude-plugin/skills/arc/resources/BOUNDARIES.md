# Boundaries: When to Use Arc vs TodoWrite

This reference provides detailed decision criteria for choosing between arc issue tracking and TodoWrite for task management.

## Contents

- [The Core Question](#the-core-question)
- [Decision Matrix](#decision-matrix)
  - [Use arc for](#use-arc-for): Multi-Session Work, Complex Dependencies, Knowledge Work, Side Quests, Team Memory
  - [Use TodoWrite for](#use-todowrite-for): Single-Session Tasks, Linear Execution, Immediate Context, Simple Tracking
- [Detailed Comparison](#detailed-comparison)
- [Integration Patterns](#integration-patterns)
  - Pattern 1: Arc as Strategic, TodoWrite as Tactical
  - Pattern 2: TodoWrite as Working Copy of Arc Issue
  - Pattern 3: Transition Mid-Session
- [Real-World Examples](#real-world-examples)
- [Common Mistakes](#common-mistakes)
- [The Transition Point](#the-transition-point)
- [Summary Heuristics](#summary-heuristics)

## The Core Question

**"Could I resume this work after 2 weeks away?"**

- If arc would help you resume → **use arc**
- If markdown skim would suffice → **TodoWrite is fine**

This heuristic captures the essential difference: arc provides structured context that persists across long gaps and context compactions, while TodoWrite excels at immediate session tracking.

## Decision Matrix

### Use arc for:

#### Multi-Session Work
Work spanning multiple compaction cycles or days where context needs to persist.

**Examples:**
- Strategic document development requiring research across multiple sessions
- Feature implementation split across several coding sessions
- Bug investigation requiring experimentation over time
- Architecture design evolving through multiple iterations

**Why arc wins**: Issues capture context that survives compaction. Return weeks later and see full history, design decisions, and current status via `arc show`.

#### Complex Dependencies
Work with blockers, prerequisites, or hierarchical structure.

**Examples:**
- OAuth integration requiring database setup, endpoint creation, and frontend changes
- Research project with multiple parallel investigation threads
- Refactoring with dependencies between different code areas
- Migration requiring sequential steps in specific order

**Why arc wins**: Dependency graph shows what's blocking what. `arc ready` automatically surfaces unblocked work. No manual tracking required.

#### Knowledge Work
Tasks with fuzzy boundaries, exploration, or strategic thinking.

**Examples:**
- Architecture decision requiring research into frameworks and trade-offs
- API design requiring research into multiple options
- Performance optimization requiring measurement and experimentation
- Documentation requiring understanding system architecture

**Why arc wins**: Issue fields capture evolving understanding. Issues can be refined as exploration reveals more information.

#### Side Quests
Exploratory work that might pause the main task.

**Examples:**
- During feature work, discover a better pattern worth exploring
- While debugging, notice related architectural issue
- During code review, identify potential improvement
- While writing tests, find edge case requiring research

**Why arc wins**: Create issue with `discovered-from` dependency, pause main work safely. Context preserved for both tracks. Resume either one later.

#### Team Memory
Need to resume work after significant time with full context.

**Examples:**
- Open source contributions across months
- Part-time projects with irregular schedule
- Complex features split across sprints
- Research projects with long investigation periods

**Why arc wins**: Central server persists indefinitely. All context, decisions, and history available on resume via `arc show`. No relying on conversation scrollback or markdown files.

---

### Use TodoWrite for:

#### Single-Session Tasks
Work that completes within current conversation.

**Examples:**
- Implementing a single function based on clear spec
- Fixing a bug with known root cause
- Adding unit tests for existing code
- Updating documentation for recent changes

**Why TodoWrite wins**: Simple checklist is perfect for linear execution. No need for persistence or dependencies. Clear completion within session.

#### Linear Execution
Straightforward step-by-step tasks with no branching.

**Examples:**
- Database migration with clear sequence
- Deployment checklist
- Code style cleanup across files
- Dependency updates following upgrade guide

**Why TodoWrite wins**: Steps are predetermined and sequential. No discovery, no blockers, no side quests. Just execute top to bottom.

#### Immediate Context
All information already in conversation.

**Examples:**
- User provides complete spec and asks for implementation
- Bug report with reproduction steps and fix approach
- Refactoring request with clear before/after vision
- Config changes based on user preferences

**Why TodoWrite wins**: No external context to track. Everything needed is in current conversation. TodoWrite provides user visibility, nothing more needed.

#### Simple Tracking
Just need a checklist to show progress to user.

**Examples:**
- Breaking down implementation into visible steps
- Showing validation workflow progress
- Demonstrating systematic approach
- Providing reassurance work is proceeding

**Why TodoWrite wins**: User wants to see thinking and progress. TodoWrite is visible in conversation. Arc is invisible background structure.

---

## Detailed Comparison

| Aspect | arc | TodoWrite |
|--------|-----|-----------|
| **Persistence** | Central server, survives compaction | Session-only, lost after conversation |
| **Dependencies** | Graph-based, automatic ready detection | Manual, no automatic tracking |
| **Discoverability** | `arc ready` surfaces work | Scroll conversation for todos |
| **Complexity** | Handles nested epics, blockers | Flat list only |
| **Visibility** | Background structure, not in conversation | Visible to user in chat |
| **Setup** | Requires `arc init` in project | Always available |
| **Best for** | Complex, multi-session, explorative | Simple, single-session, linear |
| **Context capture** | Notes, acceptance criteria, labels | Just task description |
| **Evolution** | Issues can be updated, refined over time | Static once written |
| **Audit trail** | Full history via server | Only visible in conversation |

## Integration Patterns

Arc and TodoWrite can coexist effectively in a session. Use both strategically.

### Pattern 1: Arc as Strategic, TodoWrite as Tactical

**Setup:**
- Arc tracks high-level issues and dependencies
- TodoWrite tracks current session's execution steps

**Example:**
```
Arc issue: "Implement user authentication" (epic)
  ├─ Child issue: "Create login endpoint"
  ├─ Child issue: "Add JWT token validation"  ← Currently working on this
  └─ Child issue: "Implement logout"

TodoWrite (for JWT validation):
- [ ] Install JWT library
- [ ] Create token validation middleware
- [ ] Add tests for token expiry
- [ ] Update API documentation
```

**When to use:**
- Complex features with clear implementation steps
- User wants to see current progress but larger context exists
- Multi-session work currently in single-session execution phase

### Pattern 2: TodoWrite as Working Copy of Arc Issue

**Setup:**
- Start with arc issue containing full context
- Create TodoWrite checklist from arc issue's acceptance criteria
- Update arc as TodoWrite items complete

**Example:**
```
Session start:
- Check arc: issue "arc-a3f2: Add JWT token validation" is ready
- Extract acceptance criteria into TodoWrite
- Mark arc issue as in_progress
- Work through TodoWrite items
- Update arc notes as you learn
- When TodoWrite completes, close arc issue
```

**When to use:**
- Arc issue is ready but execution is straightforward
- User wants visible progress tracking
- Need structured approach to larger issue

### Pattern 3: Transition Mid-Session

**From TodoWrite to arc:**

Recognize mid-execution that work is more complex than anticipated.

**Trigger signals:**
- Discovering blockers or dependencies
- Realizing work won't complete this session
- Finding side quests or related issues
- Needing to pause and resume later

**How to transition:**
```
1. Create arc issue with current TodoWrite content
2. Note: "Discovered this is multi-session work during implementation"
3. Add dependencies as discovered
4. Keep TodoWrite for current session
5. Update arc issue before session ends
6. Next session: resume from arc, create new TodoWrite if needed
```

**From arc to TodoWrite:**

Rare, but happens when arc issue turns out simpler than expected.

**Trigger signals:**
- All context already clear
- No dependencies discovered
- Can complete within session
- User wants execution visibility

**How to transition:**
```
1. Keep arc issue for historical record
2. Create TodoWrite from issue description
3. Execute via TodoWrite
4. Close arc issue when done
5. Note: "Completed in single session, simpler than expected"
```

## Real-World Examples

### Example 1: Database Migration Planning

**Scenario**: Planning migration from MySQL to PostgreSQL for production application.

**Why arc**:
- Multi-session work across days/weeks
- Fuzzy boundaries - scope emerges through investigation
- Side quests - discover schema incompatibilities requiring refactoring
- Dependencies - can't migrate data until schema validated
- Team memory - need to resume after interruptions

**Arc structure**:
```
db-epic: "Migrate production database to PostgreSQL" (epic)
  ├─ db-1: "Audit current MySQL schema and queries"
  ├─ db-2: "Research PostgreSQL equivalents for MySQL features" (blocks schema design)
  ├─ db-3: "Design PostgreSQL schema with type mappings"
  └─ db-4: "Create migration scripts and test data integrity" (blocked by db-3)
```

**TodoWrite role**: None initially. Might use TodoWrite for single-session testing sprints once migration scripts ready.

### Example 2: Simple Feature Implementation

**Scenario**: Add logging to existing endpoint based on clear specification.

**Why TodoWrite**:
- Single session work
- Linear execution - add import, call logger, add test
- All context in user message
- Completes within conversation

**TodoWrite**:
```
- [ ] Import logging library
- [ ] Add log statements to endpoint
- [ ] Add test for log output
- [ ] Run tests
```

**Arc role**: None. Overkill for straightforward task.

### Example 3: Bug Investigation

**Initial assessment**: Seems simple, try TodoWrite first.

**TodoWrite**:
```
- [ ] Reproduce bug
- [ ] Identify root cause
- [ ] Implement fix
- [ ] Add regression test
```

**What actually happens**: Reproducing bug reveals it's intermittent. Root cause investigation shows multiple potential issues. Needs time to investigate.

**Transition to arc**:
```
Create arc issue: "Fix intermittent auth failure in production"
  - Notes: Initially seemed simple but reproduction shows complex race condition
  - Created issues for each hypothesis with discovered-from dependency

Pause for day, resume next session from arc context
```

## Common Mistakes

### Mistake 1: Using TodoWrite for Multi-Session Work

**What happens**:
- Next session, forget what was done
- Scroll conversation history to reconstruct
- Lose design decisions made during implementation
- Start over or duplicate work

**Solution**: Create arc issue instead. Persist context across sessions.

### Mistake 2: Using Arc for Simple Linear Tasks

**What happens**:
- Overhead of creating issue not justified
- User can't see progress in conversation
- Extra tool use for no benefit

**Solution**: Use TodoWrite. It's designed for exactly this case.

### Mistake 3: Not Transitioning When Complexity Emerges

**What happens**:
- Start with TodoWrite for "simple" task
- Discover blockers and dependencies mid-way
- Keep using TodoWrite despite poor fit
- Lose context when conversation ends

**Solution**: Transition to arc when complexity signal appears. Not too late mid-session.

### Mistake 4: Creating Too Many Arc Issues

**What happens**:
- Every tiny task gets an issue
- Server cluttered with trivial items
- Hard to find meaningful work in `arc ready`

**Solution**: Reserve arc for work that actually benefits from persistence. Use "2 week test" - would arc help resume after 2 weeks? If no, skip it.

### Mistake 5: Never Using Arc Because TodoWrite is Familiar

**What happens**:
- Multi-session projects become markdown swamps
- Lose track of dependencies and blockers
- Can't resume work effectively
- Rotten half-implemented plans

**Solution**: Force yourself to use arc for next multi-session project. Experience the difference in organization and resumability.

## The Transition Point

Most work starts with an implicit mental model:

**"This looks straightforward"** → TodoWrite

**As work progresses:**

✅ **Stays straightforward** → Continue with TodoWrite, complete in session

⚠️ **Complexity emerges** → Transition to arc, preserve context

The skill is recognizing the transition point:

**Transition signals:**
- "This is taking longer than expected"
- "I've discovered a blocker"
- "This needs more research"
- "I should pause this and investigate X first"
- "The user might not be available to continue today"
- "I found three related issues while working on this"

**When you notice these signals**: Create arc issue, preserve context, work from structured foundation.

## Summary Heuristics

Quick decision guides:

**Time horizon:**
- Same session → TodoWrite
- Multiple sessions → arc

**Dependency structure:**
- Linear steps → TodoWrite
- Blockers/prerequisites → arc

**Scope clarity:**
- Well-defined → TodoWrite
- Exploratory → arc

**Context complexity:**
- Conversation has everything → TodoWrite
- External context needed → arc

**User interaction:**
- User watching progress → TodoWrite visible in chat
- Background work → arc invisible structure

**Resume difficulty:**
- Easy from markdown → TodoWrite
- Need structured history → arc

When in doubt: **Use the 2-week test**. If you'd struggle to resume this work after 2 weeks without arc, use arc.
