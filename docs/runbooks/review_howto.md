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

## URLs you'll receive

`arc share create` prints exactly one URL — the **Author URL** (or **Preview URL** for `--local` shares). It contains both the decryption key and the `&t=<edit_token>` that grants Accept / Resolve / Reject. Treat it like a write password: don't paste it into tickets, screenshots, or shared chat threads. The first time you open it, the page detects your role from the URL itself; you don't sign in or pick a name.

| URL form | What it is | Who gets it | Source |
|---|---|---|---|
| **Author URL** (`#k=…&t=…`) | Read + comment + Accept / Resolve / Reject | Plan author only | Printed by `arc share create` |
| **Reviewer URL** (`#k=…` only, no `&t=`) | Read + comment | Reviewers | Click the in-page **Share link** button on the share page header |

To send a reviewer link: open the Author URL in your browser, click the **Share link** button in the page header, and paste the resulting URL into your message. The button strips `&t=` so the URL you share grants reviewer-only access — copy-pasting the bare Author URL would hand the recipient your edit token.

Lost the URL? Run `arc share show <id> --author-url` to reprint it (uses the `edit_token` saved to `~/.arc/shares.json`).

Reviewers see no sign-in screen. The first time a reviewer leaves a comment, a small modal asks for their name. The name is stored in this browser only.

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

## Editing annotations

Two roles can edit an annotation while it's `open` or `reopened`:

1. **The original commenter** can refine their own wording.
2. **The plan author** can sharpen any reviewer's comment — useful for turning a thin "expand this more" into a fully-formed instruction the LLM can act on, without waiting on the reviewer.

Either way, **`comment.author_name` doesn't change** — Steve's comment is still attributed to Steve even after Ben rewrites the body. Only the *body* (and `suggested_text`, `comment_type`) changes. The underlying edit event records who actually edited, so the audit trail is preserved if you ever want to inspect it.

Once a comment is Accepted, Rejected, or Resolved, the edit button disappears for everyone. The reasoning: the meaning has been "consumed" by the resolution decision, and changing it after the fact would invalidate that decision.

## JSON output for LLM consumers

`arc share comments <id> --json` emits a single JSON object structured for direct LLM consumption.

**Local case** — share is registered in `~/.arc/shares.json` and the file is readable:

```json
{
  "plan": {
    "id": "abc123",
    "title": "Test Plan",
    "author_name": "Ben",
    "file": "/abs/path/to/docs/plans/foo.md"
  },
  "comments": [
    {
      "comment": {
        "kind": "comment",
        "id": "c-abc",
        "author_name": "Steve",
        "comment_type": "issue",
        "action": "comment",
        "body": "Goal section should mention success criteria",
        "anchor": { "line_start": 5, "line_end": 5, "quoted_text": "...", "heading_slug": "goal" },
        "created_at": "..."
      },
      "status": "accepted",
      "resolved_anchor": {
        "status": "ok",
        "line_start": 5,
        "line_end": 5,
        "snippet": "## Goal\n\nValidate the shared review feature."
      }
    }
  ]
}
```

The agent reads `plan.file` directly — the markdown content isn't included to avoid bloating every CLI call with content the agent can read in one tool call.

**Remote case** — share isn't registered locally (e.g. an agent consuming a shared URL it didn't create). The `file` field is omitted; `markdown_b64` carries the plan content base64-encoded:

```json
{
  "plan": {
    "id": "abc123",
    "markdown_b64": "IyBUZXN0IFBsYW4KCiMjIEdvYWwK..."
  },
  "comments": [...]
}
```

Base64 sidesteps the JSON-escape penalty for markdown (every `\n` and `\"` doubles the size and destroys readability when piped to `cat`). Decode with any standard base64 implementation.

Key fields for an agent applying feedback:

- **`plan.file`** *(local case)* — absolute path the agent should `Edit` directly.
- **`plan.markdown_b64`** *(remote case)* — base64-encoded plan content. Decode and write to disk if the agent needs to operate on a file.
- **`comment.action`** — `"comment"` (default) or `"delete"`. Delete annotations request removal of `quoted_text`; the body may be empty since the strikethrough IS the action.
- **`comment.suggested_text`** — when present, this is a literal find-and-replace candidate.
- **`resolved_anchor.status`** — `"ok"` if line numbers match the current content; `"drifted"` if the comment was relocated via the heading or fuzzy fallback (use the new line numbers); `"orphaned"` if the quoted text isn't in the current content (the agent should grep or skip).
- **`resolved_anchor.snippet`** — a few lines of context around the anchor, for orientation.

`arc share pull <id> --json` is the same shape filtered to `status === "accepted"` — typical agent input.

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
