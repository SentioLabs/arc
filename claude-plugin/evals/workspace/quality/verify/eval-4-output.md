# Eval 4 — Partial Test Failure: What I Would Do

## Situation

The test suite ran with results: 45 passed, 2 failed. The failures are in `TestSessionExpiry` and `TestTokenRefresh`.

## Decision: Do NOT Close the Arc Issue

The verify skill's Gate Sequence is unambiguous on this point.

### Step 3 (READ) requires:

> "Failure count (must be 0, not 'some passed')"

The failure count here is 2, not 0. Forty-five passing tests is irrelevant when two are failing. The skill explicitly warns against treating "some passed" as evidence of success.

### Step 4 (VERIFY) requires:

> "'0 failures' confirms 'tests pass' — 'tests ran' does not"

The output does not confirm the claim "tests pass." It confirms "tests ran" — which is insufficient.

### Therefore, Step 5 (ONLY THEN) is not reached.

The completion claim cannot be made. The arc issue must NOT be closed.

## What I Would Do Instead

Per the skill's Arc Integration section:

> If verification **fails**, do NOT close the issue. Instead:
> - Return to `implement` to fix the failure
> - Or invoke `debug` if the failure is unexpected

1. **Investigate the 2 failures.** I would examine the test output for `TestSessionExpiry` and `TestTokenRefresh` to understand why they failed — looking at expected vs. actual values, stack traces, and whether the failures relate to the changes made or are pre-existing.
2. **Fix or debug.** If the failures are caused by the current changes, return to `implement` to fix them. If they are unexpected or unrelated, invoke `debug` to understand the root cause.
3. **Re-run the full test suite** after any fix — fresh, complete, now. Not a subset.
4. **Only close the arc issue** when the full suite shows 0 failures.

## Summary

Two test failures block issue closure regardless of how many tests pass. The verify skill's Iron Law is clear: no completion claims without fresh verification evidence showing 0 failures. Passing 45 out of 47 tests is not passing.
