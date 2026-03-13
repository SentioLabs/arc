# Eval 2: User Requests Skipping Fresh Verification

## What the user asked

The user said tests passed 10 minutes ago and asked to just close the issue without re-running verification.

## What I would do

**I would refuse to close the issue without fresh verification.** Here is exactly what I would say and do:

---

I cannot close this issue based on cached results from 10 minutes ago. The verify skill's Iron Law is explicit: **no completion claims without fresh verification evidence.** "I ran it earlier" is specifically called out as insufficient -- the requirement is "Fresh. Complete. Now."

Even if tests passed 10 minutes ago, code or state could have changed since then. Before closing any arc issue, I need to:

1. **IDENTIFY** the proof command (e.g., `make test` for the full suite).
2. **RUN** it now, not rely on a previous run.
3. **READ** the full output -- exit code, failure count, warnings, skipped tests.
4. **VERIFY** the output actually confirms the claim (0 failures, exit 0).
5. **ONLY THEN** close the issue with `arc close <id> -r "Verified: <evidence summary>"`.

I would run `make test` right now and read the complete output before closing.

---

## Key principle applied

The skill Rules section (line 81) states: "Never trust cached or remembered results -- run it fresh." The user's request to skip verification directly conflicts with this rule. Regardless of how recently tests were run, the verify skill requires fresh execution at the moment of making the completion claim.

## Decision

**Reject the cached result. Insist on running tests fresh before closing the issue.**
