---
name: verify
description: Use before claiming any work is complete, any test passes, or any fix works. Requires fresh verification evidence before any completion claim. Called by the implement skill before closing tasks, and by finish before session end. Invoke this before arc close or before telling the user something is done.
---

# Verify — Evidence-Based Completion Gates

Run proof commands and read their output before making any completion claim.

## Iron Law

**NO COMPLETION CLAIMS WITHOUT FRESH VERIFICATION EVIDENCE.**

"It should work" is not evidence. "Tests pass" without output is not evidence. Satisfaction expressed before running the proof command is a red flag.

## Gate Sequence

Create a TodoWrite checklist with these steps:

### 1. IDENTIFY

What command proves the claim? Examples:
- "Tests pass" → `make test` or `go test ./...`
- "Build succeeds" → `make build`
- "Issue is resolved" → `arc show <id>` (check status)
- "No regressions" → full test suite, not a subset

### 2. RUN

Execute the **full** command. At minimum, run all tests affected by the change; running the full suite is safer. Not a subset from memory. Not "I ran it earlier."

Fresh. Complete. Now.

### 3. READ

Read the **FULL** output. Not just the last line. Check:
- Exit code (0 = success)
- Failure count (must be 0, not "some passed")
- Warning count (investigate, don't ignore)
- Any skipped tests (why were they skipped?)

### 4. VERIFY

Does the output **actually confirm** the claim?
- "0 failures" confirms "tests pass" — "tests ran" does not
- "exit 0" confirms "build succeeds" — "compiling..." does not
- "status: closed" confirms "issue resolved" — "status: in_progress" does not

### 5. ONLY THEN

Make the claim. Reference the evidence:
- "Tests pass: `go test ./...` shows 47 passed, 0 failed"
- "Build succeeds: `make build` exits 0, binary at `./bin/arc`"

## Red Flags

You are skipping verification if you:
- Use "should work", "probably passes", "seems fine"
- Express satisfaction before running the proof command
- Run a subset of tests instead of the full suite
- Trust a subagent's report without running the proof command yourself
- Say "tests pass" without showing the output
- Claim "no regressions" without running the full suite
- Close an arc issue before verification

## Arc Integration

```bash
# ONLY after verification passes:
arc close <id> -r "Verified: <evidence summary>"
```

If verification **fails**, do NOT close the issue. Instead:
- Return to `implement` to fix the failure
- Or invoke `debug` if the failure is unexpected

## Rules

- Never close an arc issue without fresh verification evidence
- Never claim completion without running the proof command
- Never trust cached or remembered results — run it fresh
- After verification, proceed to `finish` (session end) or back to `implement` (next task)
- Format all arc content (descriptions, plans, comments) per `skills/arc/_formatting.md`
