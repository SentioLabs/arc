---
name: debug
description: Use when encountering bugs, test failures, or unexpected behavior during implementation. Requires root cause investigation before any fix attempt. Invoked standalone or from the implement skill when things break.
---

# Debug — Systematic Root Cause Investigation

Investigate bugs methodically before attempting fixes. No guessing, no shotgunning, no Stack Overflow copypasta.

## Iron Law

**NO FIXES WITHOUT ROOT CAUSE INVESTIGATION FIRST.**

If you don't understand why something is broken, you cannot fix it. A "fix" without understanding is a coincidence.

## 4-Phase Process

Create a TodoWrite checklist with these phases and work through them:

### Phase 1: Investigate Root Cause

- Read error messages **carefully** — they often tell you exactly what's wrong
- Reproduce the failure consistently — if you can't reproduce it, you can't verify a fix
- Check recent changes: `git diff`, `git log --oneline -10`
- Gather evidence: stack traces, logs, test output, error codes
- In multi-component systems, trace the data flow end-to-end
- **Do not propose fixes yet.** You are gathering evidence.

### Phase 2: Pattern Analysis

- Find working examples of similar code in the codebase
- Compare working code against broken code — what's different?
- Check if this is a known pattern (dependency version, config issue, API change)
- Look for similar past issues: `arc list --type=bug -w <workspace>`

### Phase 3: Hypothesis Testing

- Form a **single** hypothesis about the root cause
- Design a minimal test to confirm or reject it — one change, one test
- If the hypothesis is wrong, **revert** the test change and form a new hypothesis
- Do NOT stack fixes — each hypothesis gets tested in isolation
- Document what you've tried and what you've learned

### Phase 4: Implement Fix

- Write a failing test that **demonstrates the bug** (the test should fail before the fix and pass after)
- Fix the **root cause**, not the symptom
- Verify the fix makes the bug test pass
- Run the **full test suite** to check for regressions
- If the fix introduces new failures, you fixed the wrong thing — go back to Phase 1

## The 3-Fix Rule

If you've tried 3 fixes and none worked, **STOP**.

You don't understand the problem yet. Going for fix #4 is insanity.

Instead:
- Go back to Phase 1 and investigate more deeply
- Question your assumptions — are you fixing the right thing?
- Consider whether the architecture is wrong, not just the code
- Read the error message again — you probably skimmed it the first time

## Arc Integration

If the bug turns out to be bigger than expected (not a quick fix within the current task):

```bash
arc create "Bug: <description>" --type=bug --priority=<severity> -w <workspace>
```

Then decide: fix it now (if it blocks current work) or defer it (if current work can continue without it).

## Red Flags

You're doing it wrong if you:
- Fix symptoms instead of causes
- Apply fixes without understanding why they work
- Copy code from the internet without understanding it
- Make multiple changes at once ("let me try this AND this AND this")
- Skip the failing test that demonstrates the bug
- Say "it works now" without understanding what changed

## Rules

- Always investigate before fixing — Phase 1 is not optional
- Always write a bug-demonstrating test before the fix
- Always run the full test suite after fixing
- Revert failed fix attempts cleanly — don't leave debris
- After debugging, return to wherever you were (implement, verify, etc.)
- Format all arc content (descriptions, plans, comments) per `skills/arc/_formatting.md`
