# Eval 3: Circuit Breaker on Repeated Review/Fix Cycles

## Scenario

3rd review/fix cycle for ARC-5.2. The reviewer keeps flagging the same error handling pattern as Critical, but the implementer keeps writing it the same way.

## Decision

**STOP. Do NOT dispatch a 4th review/fix cycle. Escalate to the user.**

## Reasoning

The review skill (SKILL.md, step 4) defines a circuit breaker:

> If 3 review/fix cycles on the same task haven't resolved all findings, STOP. Escalate to the user with a summary of what keeps recurring -- the reviewer and implementer may disagree on the approach, or the task spec may be ambiguous.

This is exactly that situation. Three cycles have completed with the same Critical finding recurring each time. The reviewer and implementer are not converging on a solution. Dispatching a 4th cycle would produce the same result.

## What I Would Do

1. **Do NOT** re-dispatch `arc-implementer` for another fix attempt.
2. **Do NOT** re-dispatch `arc-reviewer` for another review.
3. **Escalate to the user** with:
   - The specific error handling pattern the reviewer flags as Critical
   - The pattern the implementer keeps using instead
   - A request for the user to decide: clarify the task spec, choose an approach, or downgrade the finding
4. Wait for the user's guidance before proceeding.

## Key Principle

The circuit breaker exists to prevent infinite loops when subagents disagree. The orchestrator's job is to detect non-convergence and involve the human, not to blindly retry the same failing cycle.
