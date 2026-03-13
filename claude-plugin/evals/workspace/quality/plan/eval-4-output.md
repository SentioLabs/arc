# Notification System — Task Breakdown

## Shared Contracts Identified

The design specifies three shared contracts referenced by all channel tasks:

- **`Notifier` interface** — common send/validate contract for all channels
- **`NotificationPayload` type** — shared data structure passed to every notifier
- **Config keys** — shared configuration key constants for channel registration

Because the email, Slack, and webhook channel tasks will run **in parallel**, these shared definitions must exist in HEAD before any parallel task begins. This triggers the **T0: Foundation** pattern from step 2 of the plan skill.

## Task Structure

### T0: Foundation — Shared Notification Contracts

**Type:** task
**Parent:** `<epic-id>`

**Files:**
- Create: `internal/notify/notify.go`
- Create: `internal/notify/notify_test.go`
- Modify: `internal/types/types.go` (add `NotificationPayload` type)

**Scope Boundary:**
Do NOT create or modify any files outside the Files section above.
Channel-specific implementations (email, Slack, webhook) belong to their own tasks.

**Steps:**
1. Write a failing test in `internal/notify/notify_test.go` that asserts a `Notifier` interface exists with `Send(ctx context.Context, payload NotificationPayload) error` and `Validate() error` methods
2. Run `go test ./internal/notify/...` — confirm it fails (undefined types)
3. Define `NotificationPayload` in `internal/types/types.go` with fields: `Event string`, `IssueID string`, `ProjectID string`, `Summary string`, `Timestamp time.Time`, `Metadata map[string]string`
4. Create `internal/notify/notify.go` with the `Notifier` interface and config key constants (`ConfigKeyEmail`, `ConfigKeySlack`, `ConfigKeyWebhook`)
5. Run `go test ./internal/notify/...` — confirm it passes
6. Commit: `feat(notify): add Notifier interface, NotificationPayload, and config keys`

**Test Command:** `go test ./internal/notify/...`

**Expected Outcome:** `Notifier` interface, `NotificationPayload` type, and config key constants are defined and importable by downstream channel tasks.

---

### T1: Email Channel Notifier

**Type:** task
**Parent:** `<epic-id>`
**Blocked by:** T0

**Files:**
- Create: `internal/notify/email.go`
- Create: `internal/notify/email_test.go`

**Scope Boundary:**
Do NOT create or modify any files outside the Files section above.
If you need the `Notifier` interface or `NotificationPayload` type, import them — they already exist from T0.

**Steps:**
1. Write a failing test in `internal/notify/email_test.go` that creates an `EmailNotifier` and asserts it satisfies the `Notifier` interface via `var _ Notifier = (*EmailNotifier)(nil)`
2. Write a test for `Validate()` that checks required config (SMTP host, port, from address) returns error when missing
3. Write a test for `Send()` that uses a mock SMTP connection and verifies the payload is formatted into an email with correct subject/body
4. Run `go test ./internal/notify/...` — confirm tests fail
5. Implement `EmailNotifier` struct in `internal/notify/email.go` with fields for SMTP config, a `NewEmailNotifier(cfg map[string]string) (*EmailNotifier, error)` constructor, `Validate() error`, and `Send(ctx, payload) error`
6. Run `go test ./internal/notify/...` — confirm all email tests pass
7. Commit: `feat(notify): add email channel notifier`

**Test Command:** `go test ./internal/notify/...`

**Expected Outcome:** `EmailNotifier` implements `Notifier`, validates SMTP config, and formats+sends email notifications.

---

### T2: Slack Channel Notifier

**Type:** task
**Parent:** `<epic-id>`
**Blocked by:** T0

**Files:**
- Create: `internal/notify/slack.go`
- Create: `internal/notify/slack_test.go`

**Scope Boundary:**
Do NOT create or modify any files outside the Files section above.
If you need the `Notifier` interface or `NotificationPayload` type, import them — they already exist from T0.

**Steps:**
1. Write a failing test in `internal/notify/slack_test.go` that creates a `SlackNotifier` and asserts it satisfies the `Notifier` interface via `var _ Notifier = (*SlackNotifier)(nil)`
2. Write a test for `Validate()` that checks required config (webhook URL, channel) returns error when missing
3. Write a test for `Send()` that uses an `httptest.Server` to capture the outbound POST and verifies the JSON payload contains the correct Slack message structure
4. Run `go test ./internal/notify/...` — confirm tests fail
5. Implement `SlackNotifier` struct in `internal/notify/slack.go` with fields for webhook URL and channel, a `NewSlackNotifier(cfg map[string]string) (*SlackNotifier, error)` constructor, `Validate() error`, and `Send(ctx, payload) error` that POSTs a Slack-formatted JSON message
6. Run `go test ./internal/notify/...` — confirm all Slack tests pass
7. Commit: `feat(notify): add Slack channel notifier`

**Test Command:** `go test ./internal/notify/...`

**Expected Outcome:** `SlackNotifier` implements `Notifier`, validates webhook config, and sends Slack-formatted notifications via HTTP POST.

---

### T3: Webhook Channel Notifier

**Type:** task
**Parent:** `<epic-id>`
**Blocked by:** T0

**Files:**
- Create: `internal/notify/webhook.go`
- Create: `internal/notify/webhook_test.go`

**Scope Boundary:**
Do NOT create or modify any files outside the Files section above.
If you need the `Notifier` interface or `NotificationPayload` type, import them — they already exist from T0.

**Steps:**
1. Write a failing test in `internal/notify/webhook_test.go` that creates a `WebhookNotifier` and asserts it satisfies the `Notifier` interface via `var _ Notifier = (*WebhookNotifier)(nil)`
2. Write a test for `Validate()` that checks required config (target URL) returns error when missing and that URL is valid
3. Write a test for `Send()` that uses an `httptest.Server` to capture the outbound POST and verifies the raw `NotificationPayload` is sent as JSON with correct `Content-Type` header and optional HMAC signature header
4. Run `go test ./internal/notify/...` — confirm tests fail
5. Implement `WebhookNotifier` struct in `internal/notify/webhook.go` with fields for target URL and optional signing secret, a `NewWebhookNotifier(cfg map[string]string) (*WebhookNotifier, error)` constructor, `Validate() error`, and `Send(ctx, payload) error` that POSTs the JSON payload with optional HMAC-SHA256 signature
6. Run `go test ./internal/notify/...` — confirm all webhook tests pass
7. Commit: `feat(notify): add webhook channel notifier`

**Test Command:** `go test ./internal/notify/...`

**Expected Outcome:** `WebhookNotifier` implements `Notifier`, validates URL config, and sends JSON payloads with optional HMAC signing.

---

## Dependencies

```
T1 (Email)   blocked by T0 (Foundation)
T2 (Slack)   blocked by T0 (Foundation)
T3 (Webhook) blocked by T0 (Foundation)
```

## Execution Graph

```
T0: Foundation (sequential — runs first)
    |
    +---> T1: Email    \
    +---> T2: Slack     } parallel batch
    +---> T3: Webhook  /
```

## File Ownership (No Overlaps)

| File | Owner |
|------|-------|
| `internal/notify/notify.go` | T0 |
| `internal/notify/notify_test.go` | T0 |
| `internal/types/types.go` (NotificationPayload) | T0 |
| `internal/notify/email.go` | T1 |
| `internal/notify/email_test.go` | T1 |
| `internal/notify/slack.go` | T2 |
| `internal/notify/slack_test.go` | T2 |
| `internal/notify/webhook.go` | T3 |
| `internal/notify/webhook_test.go` | T3 |

No two parallel tasks share a file. All shared definitions live in T0, which completes before the parallel batch begins.
