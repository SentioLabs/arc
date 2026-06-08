<!-- arc-review: kind=legacy id=plan.09tl5s -->
# Remove arc-paste / share stack — revert to legacy planner

## Summary

The arc-paste UI + standalone service + backend (the encrypted `arc share`
review surface) has not been well received. The previous setup — the legacy
planner (`arc plan` → `/planner/[planId]`) — was much better received and is
still fully intact in the tree. This plan removes the entire share/paste stack
via **surgical forward-removal**, restoring the legacy planner as the sole
design-review surface.

## Key findings from exploration

- **The two stacks coexist.** The legacy planner was never removed when
  arc-paste landed (~v0.14.0, foundation commit `cfcbce9`). This is a "remove
  the additive layer" problem, not a "restore deleted code" problem.
- **arc-paste is additive and well-isolated.** It never altered planner tables
  or code; migration `017_shares.sql` only *creates* a `shares` table. It
  couples to the rest of the system only at narrow seams.
- **The API `ServerConfig.DB *sql.DB` field exists solely** to feed paste route
  registration — no other consumer — so it can be removed too.
- **A `git revert` range is impractical:** arc-paste's ~50 commits are
  interleaved with unrelated work to keep (config-expand-and-ui #50, CI fixes,
  workspace-paths). Forward-removal sidesteps this.

## Decisions (locked)

| Decision | Choice | Rationale |
|---|---|---|
| Mechanism | Surgical forward-removal | Easiest + best practice; never rewrite pushed/released history (PRs #38–#49, tags v0.14/v0.15) |
| Scope | Full stack | Half-removed features are dead weight; legacy planner fully covers the use case |
| DB/data | Add drop migration `018` | Forward-only migrations; never edit/delete a released migration (`017`) |
| Structure | Phased, build-green each step | Reviewable, bisectable; each commit compiles and tests pass |

## Section 1 — Deletion inventory

Whole directories / files removed:

```
arc-paste/                                  # standalone service: binary, Caddyfile, compose.yaml, Dockerfile, main.go
internal/paste/                             # paste engine: crypto, anchor, handlers, storage, sqlite, types, cmd
internal/sharesconfig/                      # HTTP shim
internal/client/shares.go (+_test)          # CLI HTTP client methods for /shares  ← found in grill
internal/api/paste_routes.go (+_test)       # /api/paste/* mount
internal/api/shares.go                      # /shares handlers
internal/api/shares_import.go (+_test)      # legacy shares.json import
internal/storage/shares.go (+_test)         # shares store adapter
internal/storage/sqlite/db/queries/shares.sql   # sqlc query source       ← found in grill
internal/storage/sqlite/db/shares.sql.go        # generated shares queries ← found in grill
internal/types/shares_test.go               # share-type tests             ← found in grill
cmd/arc/share.go (+_test)                   # arc share CLI
web/src/routes/share/                       # /share/[id] UI + components
web/src/lib/paste/                          # paste client/crypto/identity
docs/runbooks/paste-server.md               # 100% paste feature           ← found in grill
docs/runbooks/review_howto.md               # 100% share Accept/Resolve/Reject (0 planner refs) ← found in grill
```

## Section 2 — Unwiring the seams (surgical edits, not deletions)

| File | Edit |
|---|---|
| `internal/api/server.go` | Remove `DB *sql.DB` field (~L37-39), the `if cfg.DB != nil { registerPasteRoutes }` block (~L72-75), `RegisterShareRoutes` method (~L98-106), and its call (~L166-167) |
| `internal/server/server.go` | Remove `sharesconfig` import (~L15), `DB: store.DB()` (~L71), legacy shares.json import block (~L74-83) |
| `cmd/arc/main.go` | Remove `sharesconfig` import (~L20) + client factory wiring (~L265-268) |
| `internal/storage/sqlite/store.go` | Remove `pastesqlite` import (~L13) + paste migration apply (~L116-118) and its doc comment (~L126) |
| `Makefile` | Remove `build-paste` target (~L107-110) + any `webui` paste reference |
| `web/vite.config.ts` | Revert proxy target to `http://localhost:7432`, drop `ARC_PASTE_BACKEND` env override |
| `web/src/routes/+layout.svelte` | Remove `isShareRoute` derivation (~L27), the projects-fetch skip (~L31), and the `{#if isShareRoute}…{:else}…{/if}` branch (~L55+) — collapse to the normal app shell since `/share/` no longer exists |
| `web/src/routes/settings/+page.svelte` | Remove the `<SettingsSection title="Share">` block (author + server fields, ~L84-90). These bind to `working.share.*` and reference `errors['share.author'\|'share.server']`, both removed with `ShareConfig` |
| `internal/storage/storage.go` | Remove `ErrShareNotFound` (~L11-12) and the 5 interface methods: `UpsertShare`, `UpsertShares`, `GetShare`, `ListShares`, `DeleteShare` (~L94-99). *Found in grill — without this, deleting the adapter breaks the interface contract* |
| `internal/types/types.go` | Remove `ShareKind` + consts + `IsValid`, `AllShareKinds`, and the `Share` struct (~L348-368). *Found in grill* |
| `internal/api/workspace_paths_test.go` | Remove the 5 `mockWPStore` share stub methods (~L321-337) — they exist only to satisfy the `Storage` interface and reference `types.Share`. *Found in grill — kept test file* |
| `cmd/arc/config.go` | Remove share from the legacy-key map (~L30-31), valid-keys list (~L40-41), the `[share]` print block (~L143-145), and the get/set switch cases (~L327-330, L351-354). *Found in grill — `arc config` command* |
| `internal/storage/sqlite/db/schema.sql` | Remove the `shares` table DDL + `idx_shares_created_at` index (~L202-212) so sqlc regen is consistent. *Found in grill* |

> **Discovered during grill:** the share/paste feature is referenced by two
> *kept* web files (the global layout and the #50 settings page). `make gen`
> handles the generated `web/src/lib/api/types.ts` (it currently carries the
> `/shares` operation types). Verify the settings page still loads/saves all
> remaining config fields after the Share section is removed.

## Section 3 — Config schema change

`internal/config/config.go` / `validate.go` / `migrate.go`:

- Remove `ShareConfig` struct + the `Share ShareConfig` field + its default
  (`https://arcplanner.sentiolabs.io`)
- Remove `share.server` URL validation
- Remove legacy `share_author` / `share_server` → `share.*` migration mapping

## Section 4 — Database migration (forward-only)

The paste engine maintains its **own** schema and migration tracker
(`paste_migrations` table) under `internal/paste/sqlite/migrations/001_init.sql`,
creating `paste_shares` and `paste_events`. Migration `018` (in arc's main
migration system) drops all of it in one shot:

```sql
-- internal/storage/sqlite/migrations/018_drop_shares.sql
DROP TABLE IF EXISTS paste_events;
DROP TABLE IF EXISTS paste_shares;
DROP TABLE IF EXISTS paste_migrations;
DROP TABLE IF EXISTS shares;
```

This is a full teardown — stored shared-review content in `paste_shares` /
`paste_events` is intentionally deleted (the feature is being removed).
**`017_shares.sql` is left untouched** — already shipped in v0.14/v0.15; never
edit a released migration.

## Section 5 — OpenAPI, generated artifacts, docs

- `api/openapi.yaml`: remove `/shares` and `/shares/{shareId}` paths + their
  schemas.
- **Generated artifacts — never hand-edited; regenerated by `make gen`** after
  the `openapi.yaml` + `schema.sql` source edits land:
  - `internal/api/openapi.gen.go` (oapi-codegen)
  - `internal/storage/sqlite/db/models.go` (sqlc — drops the `Share` model)
  - `web/src/lib/api/types.ts` (openapi-typescript — drops `/shares` ops)
- Runbooks (both 100% about the removed feature) are **deleted** in Section 1:
  `docs/runbooks/paste-server.md`, `docs/runbooks/review_howto.md`.

### Corrected scope (resolved in grill)

- **Plugin skills are OUT OF SCOPE.** `claude-plugin/` does **not** exist in this
  repo — the arc plugin/skills (`brainstorm`, `plan`) that reference `arc share`
  live in a separate marketplace repo. They need a follow-up change *there* so
  they stop pointing users at the removed surface, but that is not part of this
  repo's removal. **Track as a separate follow-up issue.**
- **`CHANGELOG.md` is left untouched.** It is the historical record of shipped
  releases (v0.14/v0.15 did ship `arc share`); rewriting it would falsify
  history. The removal earns its own changelog entry via release-please when it
  ships.
- **Historical design docs left untouched.** `docs/plans/2026-04-29-shared-review.md`
  (and any other dated plan docs) are timestamped records, treated like git
  history — not edited or deleted.

## Section 6 — Phasing & verification

Removal is **dependency-ordered**: consumers must be unwired before packages are
deleted. Phases 1 (web) and 2 (service) are independent and may run in parallel;
phases 3→4→5→6 are strictly sequential.

1. Web removal
2. Service removal
3. Unwire Go seams (server.go, main.go, store.go, internal/server)
4. Delete Go packages (paste, sharesconfig, api shares/paste, storage shares)
5. Config (drop ShareConfig)
6. Migration `018` drop shares
7. OpenAPI regen + docs/skills cleanup

**Verification gate after each phase:** `make build-quick` + `make test` stay
green. **Final gate:** full `make build` (frontend embeds) + a grep sweep
confirming zero `paste` / `share` / `sharesconfig` references remain outside git
history.

## Out of scope / kept intact

- Legacy planner: `cmd/arc/plan.go`, `internal/api/plans.go`,
  `internal/storage/plans.go`, `web/src/routes/planner/`, migrations
  `004/013/014`.
- Unrelated post-paste work: config-expand-and-ui (#50), CI fixes,
  workspace-paths.
