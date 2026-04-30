# Manual test runbook — paste server (local + shared review)

This runbook walks through end-to-end manual testing of the encrypted paste service that backs `arc share` and the `/share/[id]` SvelteKit UI. Use it after rebuilding the binaries or before cutting a release.

For background on the architecture, see [`docs/plans/2026-04-29-shared-review.md`](../plans/2026-04-29-shared-review.md).

## Prerequisites

- `bun` and `go` installed
- A clean checkout on `feat/add-shared-review` (or whatever branch contains `internal/paste/`, `arc-paste/`, and `cmd/arc/share.go`)

## 1. Build everything

arc uses Go build tags to gate the embedded SPA. Files in `web/` have `//go:build webui` (real embed) vs `//go:build !webui` (stub no-op `RegisterSPA`). Without the `webui` tag, **both binaries will return JSON 404 for `/share/<id>`** — they have the API but no SPA.

```bash
# From the worktree root

# arc-server WITH embedded SPA (this is the one you want for manual testing)
make build           # depends on `web-build` + `build-bin --webui`

# arc-paste with embedded SPA
make build-paste     # builds web/ then `go build -tags webui ./arc-paste`

ls -la bin/          # expect: arc, arc-paste  (the unified `arc` binary serves the API; `arc server start` boots the daemon)
```

> **Do NOT use `make build-quick`** for manual UI testing — that target produces a CLI-only binary with the stub `RegisterSPA` (no-op), and `/share/<id>` will 404 with `{"message":"Not Found"}`. `build-quick` is fine for CLI-only flows like `arc share comments` but won't render the UI.

If you skip the SPA build step, `/share/[id]` will return 404 even with the right tag because the embedded filesystem will be empty.

## 2. Local review

Local mode hosts the paste API and SPA on the same `arc-server` binary that already serves arc's issues/projects. The encryption key is auto-generated, persisted to `~/.arc/shares.json`, and embedded into the URL fragment so the browser can decrypt.

### Start the server

```bash
# Terminal 1
./bin/arc server start --foreground
# listens on :7432; paste handlers mounted at /api/paste/*
# (drop --foreground to run as a daemon; use `arc server logs` to tail it)
```

### Create a test plan and share it

```bash
# Terminal 2
cat > /tmp/test-plan.md <<'EOF'
# Test Plan

## Goal
Validate the shared review feature.

## Approach
- Selection-based annotation
- Conventional labels
- Resolve / accept / reject
EOF

./bin/arc share create /tmp/test-plan.md --local
# → Share URL:  http://localhost:7432/share/<id>#k=<key>
# → Edit token: <token> (saved in ~/.arc/shares.json)

./bin/arc share list                # see all known shares
ls -la ~/.arc/shares.json           # verify file mode 0600
```

The author identity embedded in the plan is resolved in this order (highest to lowest priority):

1. `--author "Name"` flag on `arc share create`
2. `share_author` field in `~/.arc/cli-config.json` (set once, applies to every share you create)
3. `$ARC_SHARE_AUTHOR` env var
4. `git config user.name`

**Note the name that gets used** — you'll need to type it back in the UI to claim the author role. If none of these produce a value, `arc share create` prints a warning and Accept/Resolve/Reject controls won't appear for anyone.

To set a persistent default once:

```bash
# Edit ~/.arc/cli-config.json and add:
#   "share_author": "Ben Firestone"
# or, if you prefer:
echo '{"share_author":"Ben Firestone"}' | jq -s '.[0] * .[1]' ~/.arc/cli-config.json - \
  > ~/.arc/cli-config.json.tmp && mv ~/.arc/cli-config.json.tmp ~/.arc/cli-config.json
```

### Exercise the UI

Open the printed URL in Chrome/Firefox.

1. **Name prompt** appears on first comment — type the **same name** that was embedded as the author at create time (i.e. your `git config user.name` output, or whatever you passed to `--author`). The reviewer-name chip in the header will read `<name> · author` once the names match.
2. **Highlight a paragraph** in the rendered plan → floating annotation toolbar appears
3. **Pick a label** (`praise` / `issue` / `suggestion` / `question` / `nit`)
4. **Type a comment**, optionally toggle "Suggest replacement text"
5. **Post** — the comment appears in the sidebar
6. As the author (your localStorage name matches `plan.author_name`), you'll see **Accept / Resolve / Reject** controls. Try Accept on one comment, Reject (with reply) on another, and Resolve on a third. If the controls don't appear, double-check that the name in the header chip matches the value in `git config user.name` (or whatever you passed to `--author`) — the comparison is case- and whitespace-sensitive.
7. As a reviewer (your localStorage name does NOT match the plan's author), find your own annotation in the sidebar. You should see an **✎ Edit** button on it. Click it; the body becomes a textarea pre-filled with your existing comment. Refine the wording — e.g. expand "expand this more" into a fully-formed suggestion — and **⌘/Ctrl-⏎** (or click Save). The card re-renders with `· edited Nm` next to the timestamp. Confirm `arc share comments <id>` prints the new body, not the original.

### Pull comments back to the CLI

```bash
./bin/arc share comments <id>             # all comments + statuses
./bin/arc share pull <id>                 # accepted-only (the brainstorm-flow form)
./bin/arc share comments <id> --json      # machine-readable
```

### Simulate a second reviewer

Without spinning up another machine, open the same URL in an **incognito window** (or a different browser entirely). Incognito gets a fresh `localStorage`, so:

1. The name prompt fires again — type any name **other than** the embedded author name (e.g. "Reviewer-2"). The chip in the header should NOT show `· author`.
2. Post a few comments
3. Refresh the author's window → new comments replay in via `replayEvents()`

The author's resolution events still apply (their `author_name` matches the plan's). Reviewer-2's comments cannot be marked as `accepted` by Reviewer-2 itself, even if they tried — the client filters out resolution events whose `author_name` doesn't match `plan.author_name`.

## 3. Shared / remote review

Remote mode runs the standalone `arc-paste` binary (a thin wrapper around the same `internal/paste/` package). It owns its own SQLite, has CORS enabled, and can be deployed anywhere reachable.

### Start arc-paste on a separate port

```bash
# Terminal 1
ARC_PASTE_ADDR=:7433 ARC_PASTE_DB=/tmp/arc-paste.db ./bin/arc-paste
```

### Create a shared paste pointed at the standalone server

```bash
# Terminal 2
./bin/arc share create /tmp/test-plan.md --share --server http://localhost:7433
# → Share URL:  http://localhost:7433/share/<id>#k=<key>
```

That URL is what you'd send to a real reviewer (Slack, email, etc.). It contains everything they need: the share id and the decryption key in the fragment.

### Pull the comments back

```bash
./bin/arc share pull <id> --accepted-only
```

The CLI looks up the share id in `~/.arc/shares.json` to find the server URL and decryption key — no need to paste the full URL again.

### Simulate a public deploy

To exercise the actual "remote" code paths (cross-origin, no shared filesystem), run `arc-paste` on a different host or behind a tunnel:

```bash
# On a VPS, or via cloudflared/ngrok:
./bin/arc-paste

# From your laptop:
./bin/arc share create plan.md --share --server https://share.example.com
```

For a persistent default, set `share_server` in `~/.arc/cli-config.json`:

```json
{
  "server_url": "http://localhost:7432",
  "share_author": "Ben Firestone",
  "share_server": "https://share.example.com"
}
```

Then `arc share create plan.md --share` will pick it up without any flag or env var. The full precedence is `--server flag → share_server in cli-config.json → $ARC_SHARE_SERVER → https://arcplanner.sentiolabs.io`.

## 4. End-to-end via the brainstorm skill

In a new Claude Code session, the agent-nexus brainstorm skill update can be exercised directly:

```
/arc:brainstorm let's design a small feature
```

When the skill reaches step 6, it should now offer three options via `AskUserQuestion`:

- **Local review** → invokes `arc share create --local`
- **Shared review** → invokes `arc share create --share`
- **Save for later** → no server registration

Step 7 (review loop) uses `arc share approve` and `arc share pull` instead of the legacy `arc plan *` commands.

## 5. Gotchas

| Symptom | Cause | Fix |
|---|---|---|
| Accept / Resolve / Reject controls never appear, even when typing the "right" name | Plan was created without an author name (`arc share create` was run pre-fix, or `git config user.name` was empty and no `--author` flag passed). `plan.author_name` is empty, so `isAuthor` is `false` for every reviewer | Recreate the share with `--author "Your Name"` (or set `git config user.name` first), then enter that exact name in the SPA prompt |
| `/share/<id>` returns `{"message":"Not Found"}` (Echo's default 404 JSON) | Binary built without the `webui` build tag — `web.RegisterSPA` is the no-op stub | Rebuild with `make build` (not `make build-quick`); for arc-paste use `go build -tags webui -o ./bin/arc-paste ./arc-paste` |
| `/share/<id>` returns blank HTML / cannot find static assets | `web/build/` not present at compile time, even with the `webui` tag | Re-run `bun run build` in `web/`, then rebuild the binary |
| SPA console says `missing #k=<key> in URL` | URL was pasted without its fragment | Use the full URL printed by `arc share create` — fragments are dropped by some chat apps; copy carefully |
| `arc share comments <id>` errors with "unknown share id" | Looking up an id you didn't create on this machine | Use the full URL: `arc share comments 'http://host/share/<id>#k=<key>'` |
| Comments from another reviewer don't appear after refresh | Browser is caching `GET /api/paste/:id` | Hard reload (Cmd-Shift-R / Ctrl-Shift-R); the `arc-paste` server doesn't currently set Cache-Control headers |
| Lost `~/.arc/shares.json` | The only copy of edit_tokens + keys lives there | There is no recovery — same trade-off plannotator makes. Back up `~/.arc/` before destructive testing |
| CORS error in browser console (shared mode) | Talking to an arc-paste instance without `middleware.CORS()` | Verify you're running the binary built from this branch (`./bin/arc-paste --help` should exist; if not, rebuild via `make build-paste`) |

## 6. Quick reset

To wipe local state and start fresh:

```bash
# Stop the servers first (Ctrl-C)
rm ~/.arc/shares.json                 # clears CLI registry
rm /tmp/arc-paste.db                  # arc-paste's blob DB (if used in step 3)

# arc-server's paste tables live in arc.db alongside issues — to clear just paste state:
sqlite3 ~/.arc/arc.db 'DELETE FROM paste_events; DELETE FROM paste_shares;'
```

## See also

- [`docs/plans/2026-04-29-shared-review.md`](../plans/2026-04-29-shared-review.md) — design doc with full architecture, data model, and phasing
- `internal/paste/` — Go package shared by `arc-server` and `arc-paste`
- `arc-paste/` — standalone binary
- `web/src/routes/share/[id]/` — SvelteKit UI
- `cmd/arc/share.go` — CLI subcommands
- `internal/sharesconfig/` — `~/.arc/shares.json` registry
