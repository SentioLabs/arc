# Eval 5: Failing Tests During Finish Protocol

## Scenario

The test suite was run as part of the finish skill's Phase 2 (Quality Gates), and 2 tests failed in `TestOAuthCallback`.

## Decision: Block and Fix — Do NOT Commit

The finish skill is explicit on this point. Step 7 states:

> **Hard gate**: If tests fail, fix them. Do NOT skip to commit. Invoke `debug` if needed.

This is a blocking requirement. I would NOT proceed to Phase 3 (Update Arc Issues) or Phase 4 (Commit and Push). The protocol halts at Phase 2 until the quality gate passes.

## What I Would Do

1. **Stay in Phase 2.** The quality gate has not passed, so no subsequent phase is reachable.
2. **Diagnose the 2 failing tests in `TestOAuthCallback`.** I would examine the test output to understand what assertions failed and why.
3. **Invoke the `debug` skill if needed**, as the finish skill explicitly suggests this for test failures.
4. **Fix the root cause** of the `TestOAuthCallback` failures in the production or test code.
5. **Re-run `make test`** to confirm all tests pass, including the previously failing ones.
6. **Continue through the remaining quality gates** (lint, build) if they haven't been run yet.
7. **Only after all quality gates pass**, proceed to Phase 3 (Update Arc Issues), then Phase 4 (Commit and Push), and finally Phase 5 (Verify and Hand Off).

## Key Principle

The finish skill treats quality gates as hard gates, not advisory checks. Failing tests are not something to note and move past — they are a stop sign. The entire purpose of Phase 2 is to ensure that what gets committed and pushed is working code. Committing with known test failures would violate the protocol's core rule: "Never skip Phase 2 (quality gates) when code has changed."
