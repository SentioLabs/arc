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
# Output:
#   Share URL  (send to reviewers):
#     http://localhost:7432/share/<id>#k=<key>
#
#   Author URL (keep private — gives you Accept/Resolve):
#     http://localhost:7432/share/<id>#k=<key>&t=<token>
#
#   Edit token saved to ~/.arc/shares.json

./bin/arc share list                       # see all known shares
ls -la ~/.arc/shares.json                  # verify file mode 0600
./bin/arc share show <id> --author-url     # reprint the author URL on demand
```

The author identity embedded in the plan is resolved in this order (highest to lowest priority):

1. `--author "Name"` flag on `arc share create`
2. `share_author` field in `~/.arc/cli-config.json` (set once, applies to every share you create)
3. `$ARC_SHARE_AUTHOR` env var
4. `git config user.name`

**Note the name that gets used** — it is embedded in the share bundle and displayed automatically when the Author URL is opened. If none of these produce a value, `arc share create` prints a warning and Accept/Resolve/Reject controls won't appear for anyone.

To set a persistent default once:

```bash
# Edit ~/.arc/cli-config.json and add:
#   "share_author": "Ben Firestone"
# or, if you prefer:
echo '{"share_author":"Ben Firestone"}' | jq -s '.[0] * .[1]' ~/.arc/cli-config.json - \
  > ~/.arc/cli-config.json.tmp && mv ~/.arc/cli-config.json.tmp ~/.arc/cli-config.json
```

### Exercise the UI

Open the **Author URL** (the one with `&t=`) in Chrome/Firefox.

1. **Open the Author URL.** The header chip immediately reads `<author_name> · author` (no modal). Author detection is now driven by the `&t=<token>` fragment param, not a name-string match — so even if your localStorage holds a different name, the page knows you're the author and renders Accept/Resolve/Reject controls. (Remember the URL printed by `arc share create`; if you've lost it, run `arc share show <id> --author-url`.)
2. **Highlight a paragraph** in the rendered plan → floating annotation toolbar appears.
3. **Pick a label** (`praise` / `issue` / `suggestion` / `question` / `nit`).
4. **Type a comment**, optionally toggle "Suggest replacement text".
5. **Post** — the first time you do this, a small modal asks for your name. (As the author you should already have a name from step 1; the modal won't fire.) The comment appears in the sidebar.
6. As the author, you'll see **Accept / Resolve / Reject** controls on every comment. Try Accept on one, Reject (with reply) on another, and Resolve on a third. If the controls don't appear, you opened the Share URL (no `&t=`) instead of the Author URL — close the tab and reopen via the Author URL.
7. As a reviewer (a fresh browser profile or incognito window with NO `&t=` in the URL), find your own annotation in the sidebar. You should see an **✎ Edit** button on it. Click it; the body becomes a textarea pre-filled with your existing comment. Refine the wording — e.g. expand "expand this more" into a fully-formed suggestion — and **⌘/Ctrl-⏎** (or click Save). The card re-renders with `· edited Nm` next to the timestamp. Confirm `arc share comments <id>` prints the new body, not the original.
8. Switch back to the author window. The **✎ Edit** button should also appear on every reviewer's annotation (not just your own). Use it to sharpen a thin reviewer comment — the displayed `author_name` stays as the reviewer, only the body changes. Run `arc share comments <id>` again and confirm the refined body shows up.
9. **Click the chip** in the header — the name prompt opens prefilled with your current name (text selected). Edit and save → chip updates. This is the rename affordance.

### Pull comments back to the CLI

```bash
./bin/arc share comments <id>             # all comments + statuses
./bin/arc share pull <id>                 # accepted-only (the brainstorm-flow form)
./bin/arc share comments <id> --json      # machine-readable
```

### Simulate a second reviewer

Without spinning up another machine, open the **Share URL (no `&t=`)** in an **incognito window** (or a different browser entirely). Incognito gets a fresh `localStorage`, so:

1. **No chip** appears in the header. There's no "Sign in" button anywhere — identity is captured lazily.
2. Highlight a paragraph and click Comment in the toolbar. A name prompt fires; type any name (e.g. "Reviewer-2"). The comment posts; the chip now reads `Reviewer-2` (no `· author`, because the URL has no `&t=`).
3. Post a few more comments (the prompt won't fire again — name is sticky in localStorage).
4. Refresh the author's window → new comments replay in via `replayEvents()`.

The author's resolution events still apply: their UI is gated on `authorToken` (in-memory, parsed from `&t=` in the fragment), not a name match. Reviewer-2 cannot resolve comments because Reviewer-2 doesn't have the token — the Accept/Resolve/Reject buttons are never rendered for that profile.

### Frictionless auth scenarios

Walk these in order to confirm the new auth UX end-to-end:

1. **Create a share.** `arc share create <plan.md> --local` — confirm output prints both Share URL and Author URL, no raw `Edit token: <hex>` line, and a "saved to ~/.arc/shares.json" pointer.
2. **Reprint the author URL.** `arc share show <id> --author-url` — single line, contains `&t=`.
3. **Author URL flow.** Open the Author URL in a fresh browser profile. Header chip shows `<author_name> · author` immediately, no modal. Accept / Resolve buttons visible on existing comments.
4. **Reviewer URL flow.** Open the Share URL (without `&t=`) in a different fresh profile. No chip in the header, no "Sign in" button. Select text → click Comment in the toolbar → name modal opens → save a name → comment posts → chip now shows the name.
5. **Rename via chip.** Click the chip on the reviewer side. Modal opens prefilled with the saved name (text selected). Edit, save → chip updates.
6. **Author opens the bare share URL.** Open the share URL (without `&t=`) on the author's browser. They are now in reviewer mode (no Accept buttons). Reopen via the Author URL → author mode restored.

## 3. Shared / remote review

Remote mode runs the standalone `arc-paste` binary (a thin wrapper around the same `internal/paste/` package). It owns its own SQLite, has CORS enabled, and can be deployed anywhere reachable.

### Start arc-paste on a separate port

```bash
# Terminal 1
ARC_PASTE_ADDR=:7433 ARC_PASTE_DB=/tmp/arc-paste.db ./bin/arc-paste
```

Or via Docker for the production-style HTTPS stack (uses the `arc-paste/Dockerfile` scratch image, a named SQLite volume, and Caddy for `https://arcpaste.company.com`):

```bash
docker compose -f arc-paste/compose.yaml up -d --build
docker compose -f arc-paste/compose.yaml logs -f
```

The compose file publishes Caddy on host ports 80/443 (including UDP 443 for HTTP/3), exposes `arc-paste` only on the internal Docker network, persists SQLite to the `arc-paste-data` named volume, persists Caddy certificate state to named volumes, and sets `restart: unless-stopped`. Caddy is configured via `arc-paste/Caddyfile` (mounted read-only) — edit the site address there to change the public hostname, or `email` in the global block to change the ACME contact. After editing, reload without downtime:

```bash
docker compose -f arc-paste/compose.yaml exec caddy caddy reload --config /etc/caddy/Caddyfile
```

Make sure DNS for `arcpaste.company.com` points to the host and ports 80/443 are reachable for Let's Encrypt issuance and renewal. Note: the runtime image is `scratch`, so there's no in-container healthcheck — pair with an external probe (Cloudflare health check, Uptime Kuma, etc.) against the HTTPS endpoint for production.

### Create a shared paste pointed at the remote server

```bash
# Terminal 2 (Docker/Caddy stack)
./bin/arc share create /tmp/test-plan.md --share --server https://arcpaste.company.com
# → Share URL:  https://arcpaste.company.com/share/<id>#k=<key>

# If you launched the standalone binary directly instead, use:
# ./bin/arc share create /tmp/test-plan.md --share --server http://localhost:7433
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
| Accept / Resolve / Reject controls never appear | You opened the Share URL (no `&t=`) instead of the Author URL — author detection is token-based, not name-based. Or the share was created without an author name (`git config user.name` was empty and no `--author` flag passed), so `plan.author_name` is empty and the chip never shows `· author` | Close the tab and reopen via the Author URL (run `arc share show <id> --author-url` to retrieve it). If the share has no author name, recreate it with `--author "Your Name"` |
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
