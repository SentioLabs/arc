# Reviewing a shared plan — how to use Accept / Resolve / Reject

When someone shares a plan with you via `arc share create`, reviewers leave annotations and you (the plan author) close them out using one of three actions: **Accept**, **Resolve**, or **Reject**. This page explains what each one means, when to use each, and how they affect the downstream LLM consumer.

## TL;DR

| Action | Meaning | Flows to `arc share pull` (agent's queue)? |
|---|---|---|
| **Accept** | "I'll apply this to the plan." | ✅ Yes — `--accepted-only` is the default |
| **Resolve** | "Acknowledged, but no plan change needed." | ❌ No — closes the thread without queueing |
| **Reject** | "I disagree. Here's why (optional reply)." | ❌ No — reply preserved for the audit trail |
| **Reopen** | "On second thought, this should be active again." | Resets to `open` |

Mental shortcut:

- **Accept** = "do this"
- **Resolve** = "no-op, conversation done"
- **Reject** = "no, and here's why"

The discriminator is whether the comment should *cause an edit downstream*. Accept is the only path that does. Resolve and Reject both close the thread; the difference is whether the disagreement is worth recording — Reject preserves a reply, Resolve doesn't.

## Concrete examples

### Accept

The comment proposes a real change you want made.

> Steve: "The Goal section should mention success criteria for 'validated.'"

→ Ben Accepts. → `arc share pull <id>` surfaces this. → Claude rewrites the Goal section.

This is the path that produces actual plan edits. Treat Accept as a commitment to the change — once accepted, the comment locks (it stops being editable, since the meaning has been "consumed").

### Resolve

The comment is valid but no plan edit is needed.

Cases:

- **Clarifying question with a satisfying answer.** Steve: "Isn't 'validated' already defined in the previous brainstorm?" → It is. The comment helped clarify; no plan change needed. → Resolve.
- **Already covered elsewhere.** Steve: "What about edge case X?" → You think about it, realize the existing design handles X via Y. The discussion is done; the plan doesn't need to change. → Resolve.
- **Off-topic but harmless.** A comment that's interesting but not actionable in this plan.

Resolve closes the thread without sending instructions downstream.

### Reject

The suggestion is wrong, out-of-scope, or contradicts a constraint, and you want the reasoning recorded.

> Steve: "Add a section about caching."
> Ben: caching is intentionally out of scope; we're tracking it in arc-1234.

→ Reject with reply *"Caching is intentionally out of scope; tracked in arc-1234."*

The reply is encrypted in the event log alongside the rejection. Two things happen:

1. If Steve refreshes the share, he sees the rationale.
2. The agent never tries to apply the change, but the reasoning is preserved if anyone (including Claude) re-reads the share later.

Use Reject — not Resolve — whenever you'd want a future reader to know *why* you didn't act. It's the audit-trail action.

## Why three states instead of two

You could collapse Resolve and Reject into a single "Decline" — GitHub roughly does (it's just "Resolve conversation"). This UI keeps them separate because:

- The consumer is often an **LLM agent** that may re-read the share later. A `resolved` comment is "we discussed this and moved on"; a `rejected` comment with a reply is "the author considered this and explicitly disagreed because X."
- If you later ask Claude "why didn't we do the caching thing?", the rejected comment + reply gives it the exact answer. A resolved one leaves the question dangling.

If in practice you find yourself never using one of these states, that's a signal we should simplify the UI. The current design errs on the side of preserving rationale, since "feedback that's helpful for an LLM" is the project's product goal.

## Reviewers can refine their own comments

A reviewer who left a thin comment ("expand this more") can revise it themselves before the author has acted on it. Look for the **✎ Edit** button on your own annotation cards — it's only visible while the comment is `open` or `reopened`.

Once a comment is Accepted, Rejected, or Resolved, the edit button disappears. The reasoning: the author has already decided based on the wording at that moment, and changing it after the fact would be misleading.

## What flows where

```
Reviewer posts annotation
        │
        ▼
Comment status = open
        │
        ├── Author: Accept   ──► status=accepted ──► arc share pull picks it up ──► agent applies edit
        ├── Author: Resolve  ──► status=resolved   (closed, no downstream effect)
        ├── Author: Reject   ──► status=rejected   (closed, with reply for audit)
        └── Author: Reopen   ──► status=open       (back in queue)
```

The CLI commands:

```bash
# Show all comments + statuses (for a human reading the discussion):
arc share comments <id>

# Show only accepted comments — the agent's actionable queue:
arc share pull <id>          # alias for --accepted-only
arc share comments <id> --accepted-only

# Machine-readable form, used by the brainstorm skill:
arc share comments <id> --json
```

## Related

- [`docs/runbooks/paste-server.md`](runbooks/paste-server.md) — manual test runbook (covers the full flow including reviewer self-edits)
- [`docs/plans/2026-04-29-shared-review.md`](plans/2026-04-29-shared-review.md) — full design doc with event schema, replay logic, and CRDT semantics
