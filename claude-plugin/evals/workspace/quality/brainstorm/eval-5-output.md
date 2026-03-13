# Brainstorm Eval 5: Notification System Design

## Task

Design a notification system with email, Slack, and webhook channels where each channel will be implemented by a different agent in parallel.

## Tools Called

1. **Read** `claude-plugin/skills/brainstorm/SKILL.md` -- loaded skill instructions
2. **Read** `internal/types/types.go` (multiple ranges) -- understood existing types, Event/EventType, and project patterns
3. **Read** `internal/storage/storage.go` -- understood the Storage interface pattern
4. **Read** `internal/server/server.go` -- understood server config wiring
5. **Read** `claude-plugin/skills/arc/_formatting.md` -- loaded formatting rules
6. **Bash** `ls internal/` and `ls internal/api/` and `ls internal/types/` -- explored project structure

## Questions Asked

1. **What events should trigger notifications?** -- asked which issue lifecycle events matter (created, status changed, closed, comments, assignments, dependencies, or all). In eval context, proceeded with reasonable defaults (use existing `EventType` constants).

## Contracts Identified

### Foundation Layer (must be implemented before parallel work)

**Package:** `internal/notify/`

| Contract | File | Description |
|----------|------|-------------|
| `Notifier` interface | `notifier.go` | `Send(ctx, Notification) error` + `Type() ChannelType` -- the core contract all channels implement |
| `Notification` struct | `notifier.go` | Channel-agnostic payload: EventType, IssueID, ProjectID, Actor, Summary, Details map |
| `ChannelType` type | `notifier.go` | String enum: `email`, `slack`, `webhook` |
| `Config` struct | `config.go` | Top-level config holding optional `EmailConfig`, `SlackConfig`, `WebhookConfig` |
| `EmailConfig` struct | `config.go` | SMTP host/port, from address, recipients |
| `SlackConfig` struct | `config.go` | Webhook URL, optional channel override |
| `WebhookConfig` struct | `config.go` | Target URL, optional HMAC signing secret |
| `Dispatcher` struct | `dispatcher.go` | Fans out `Notification` to all registered `Notifier` instances via goroutines |

### Parallel Tasks (independent, each implements `Notifier`)

| Task | Package | Agent Work |
|------|---------|------------|
| Email channel | `internal/notify/email/` | SMTP send using `EmailConfig` |
| Slack channel | `internal/notify/slack/` | HTTP POST to Slack webhook using `SlackConfig` |
| Webhook channel | `internal/notify/webhook/` | HTTP POST with JSON + optional HMAC using `WebhookConfig` |

### Integration Task (after parallel work completes)

- Wire `Dispatcher` into API handlers in `internal/api/` where events are recorded
- Add `notify.Config` to `server.Config`

## Design Decisions

- **Approach chosen:** Event-driven with goroutine fan-out (simplest, YAGNI)
- **Rejected:** Queue-based persistent delivery (too complex for initial implementation), plugin/hook-based (poor DX)
- **Scale:** Medium -- single epic with foundation task, three parallel channel tasks, one integration task
- **Key insight:** Arc already has `Event` and `EventType` in `internal/types/types.go` covering all relevant lifecycle events, so the notification system subscribes to existing events rather than creating new ones
