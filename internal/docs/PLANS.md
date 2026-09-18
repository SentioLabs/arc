# Durable plans

Plans are server-owned, immutable Markdown revisions. The CLI reads a local draft
and uploads its exact UTF-8 bytes (up to 10 MiB); the server never opens a client
path. Removing the draft or restarting the server does not remove retained
content. An explicit save creates a new draft revision, even for identical bytes.
Upload is not synchronization. Markdown frontmatter is content, not approval or
identity authority.

Client `plans.dir` and `plans.type` select where authors keep drafts (for example,
`docs/plans/YYYY-MM-DD-topic.md` or an Obsidian vault). The independent
`server.plans_dir` setting selects the server's absolute content root, defaults to
`~/.arc/plans`, and requires restart when changed. Keep the database and its SQLite
sidecars outside that root. The operator owns the root; do not edit published files.

## Upload and exact review

Run client commands in the intended project, or pass `--project PROJECT_ID`.

```bash
arc plan create design.md --title "Architecture" --idempotency-key create-design
# Returns plan ID and revision; --no-frontmatter avoids local provenance enrichment.
arc plan show PLAN --revision 1 --review-context-output submit.json
arc plan submit PLAN --revision 1 --review-context submit.json
arc plan show PLAN --revision 1 --review-context-output approval.json
# Review these exact bytes and feedback before using this captured snapshot.
arc plan approve PLAN --revision 1 --review-context approval.json
arc plan wait PLAN --revision 1 --timeout 30m
arc plan update PLAN revised.md --expected-revision 1 --idempotency-key save-design-2
arc plan history PLAN
arc plan show PLAN --revision 1
```

Replace the uppercase identifiers with returned IDs. Context outputs refuse to
overwrite files; choose a new filename for every deliberate review. Review
commands also accept all three explicit preconditions: `--expected-head`,
`--expected-review-version`, and `--expected-feedback-version`. Do not fetch fresh
versions just to force a stale decision through. Missing required preconditions
return 428; stale ones return 409. A terminal approval/rejection belongs to its
exact revision. Saving revision 2 retains revision 1's bytes, decision and comments;
revision 2 needs its own review. `wait` observes only its selected revision: an
undecided revision overtaken by a new head returns `superseded` and a nonzero exit,
while historical terminal decisions remain readable.

Browser URLs include project, plan and revision: `/PROJECT/plans/PLAN/1`. Review the
revision number and full content before approval. A stale approval or feedback
snapshot conflicts and requires deliberate refresh. An old `/planner/PLAN` URL
leads to migration guidance or the imported project-aware plan.

## Feedback and retained discussion

```bash
arc plan comments PLAN --revision 2
arc plan comment create PLAN --revision 2 --content "Explain recovery" --line 3
arc plan feedback PLAN --revision 2 --comment COMMENT \
  --expected-comment-version 1 --expected-feedback-version 1 \
  --disposition addressed --reason "Recovery is specified in section 4"
arc plan dispositions PLAN --revision 2
```

Use the actual captured comment and feedback versions, not the illustrative `1`s.
Comments retain original revision, anchors and version history. Current and earlier
outstanding feedback must receive an `addressed` or `deferred` disposition with a
reason before approval. Explain the change/evidence for addressed items; explain
why deferral is acceptable. Deferral applies only to its target revision and must
be reconsidered later. Comment edits/reopening invalidate old assessments; deletion
leaves a tombstone and does not bypass review. Concurrent feedback invalidates the
approval snapshot. `--include-prior=false` limits comment reads to the selected
revision. Disposition history remains available.

## Layered designs and captured work

A milestone can pin an approved architecture plan and its epic can pin an approved
tactical plan. A task inherits the closest pinned container as its primary design
and all pinned higher-level ancestors as ordered context. Read the complete chain;
tactical details must respect the higher-level design. Epic/task prose supplies the
task contract but cannot replace or contradict these approved revisions. Unpinned
containers pass inheritance through. Multiple paths to the same chain deduplicate;
incomparable sources or cycles fail explicitly. Standalone tasks have no direct
plan link; epics and milestones can pin only approved same-project revisions.

```bash
arc plan resolve TASK
arc show TASK
arc update TASK --take --context-output work.json
arc evidence TASK --phase build --context work.json --stdin <<'EVIDENCE'
Implemented the captured task contract; focused tests pass.
EVIDENCE
arc evidence TASK --phase review --context work.json --stdin <<'EVIDENCE'
Reviewed implementation against every pinned design revision.
EVIDENCE
arc evidence TASK --phase verify --context work.json --stdin <<'EVIDENCE'
Required checks pass at the recorded code revision.
EVIDENCE
arc close TASK --context work.json --reason "Verified"
# Equivalent guarded completion: arc update TASK --status closed --context work.json
```

The capture comes from the atomic claim/start response and contains the task's
contract version, primary container/reference and ordered higher-level context.
Unlinked captures explicitly contain `governing: null`. Keep the original artifact
through build/review/verify/completion; never replace it with a fresh read at the
end. A changed task or design chain causes conflict without recording stale work.
The HTTP update route is PUT (not PATCH); close and evidence are POST. Governed
cascade close is rejected: finish each task with its own capture first. Governed
containers also require a capture from the start of their own completion/verification
work; verify with that artifact and use it to close the container.

`ARC_SESSION_ID` supplies caller session provenance; `--take --session-id ID` can
select a claim session. CLI actor is `cli`; these fields are attribution, not
authenticated identity. Review, adoption and execution records retain actor/session.
A successful keyed replay returns the original result and its original provenance.

## Paused, atomic reconciliation

Stop affected workers, then move affected in-progress tasks to a non-running status.
Changing status alone does not terminate an agent. No acknowledgement handshake or
container freeze is provided. A claim or task/membership change invalidates a staged
proposal. First attachment with tasks, changed pins, detach, and governance-changing
hierarchy/type edits all require reconciliation.

```bash
arc plan adopt MILESTONE PLAN --revision 2 --dry-run > preview.json
jq '.request' preview.json > reconciliation.json
# Edit reconciliation.json: supply every task disposition/reason and staged edits.
arc plan adopt MILESTONE PLAN --revision 2 \
  --reconciliation reconciliation.json --dry-run > validated.json
# Inspect errors; apply only a complete, compatible proposal with no errors.
arc plan adopt MILESTONE PLAN --revision 2 \
  --reconciliation reconciliation.json --idempotency-key adopt-architecture-2
```

Preview `tasks` can be null for an empty set. Populate `request.tasks` from the
before snapshots with the exact `expected` chain/version and `unchanged`, `updated`,
or `follow_up` plus a nonempty reason. Stage title/description changes here, not in
live task edits. Include follow-up definitions/edges using `new:KEY` endpoints;
follow-up `parent_id` must be an existing issue. Preserve required nullable
`expected_pin` and `target_pin` fields; detach uses `arc plan adopt CONTAINER -`.

Higher-level changes must account for affected tactical descendant pins. Preview
errors identify missing `container_pins`; read each affected container's exact pin
and contract version. Add `compatible` with a reason and unchanged target, or
`updated` with an approved tactical target. Preview does not invent semantic reasons
or promise a complete compatibility list. Revalidate the complete manifest. Known
contradictions require a new approved revision and staged reconciliation, not a
claim of compatibility. The server checks structure and concurrency; reviewers
judge the rationale.

Apply commits tasks, follow-ups, dependencies and all pins atomically. Any stale
version, changed membership, running task, missing coverage or damaged/unapproved
content rejects the proposal. A failed proposal leaves no partial edits. Retrying
the same successful idempotency key and identical payload returns the original
result without duplicating follow-ups, even after later changes. Never reuse a key
with changed input. Closed task evidence retains its original revisions. Resume work
with a new claim/capture only after adoption succeeds.

## Export, archive and ownership

```bash
arc plan export PLAN --revision 1 --output retained.md
arc plan export PLAN --revision 1 --output retained.md --force
arc plan export PLAN --revision 1 --output annotated.md --with-frontmatter
arc plan archive PLAN --expected-version VERSION
arc plan restore PLAN --expected-version VERSION
```

Default export is byte-identical and refuses an existing destination. `--force`
permits atomic replacement. `--with-frontmatter` explicitly produces an annotated
export, not byte-identical evidence. Use current metadata versions for archive and
restore. Archive blocks new review/edit/adoption but retains bytes, approval,
discussion, pins and evidence; restore preserves approval. Closing an epic also
retains this history. There is no permanent plan deletion in this release.

Projects owning durable plans, including archived ones, cannot be deleted. Source
project merges with retained plans or governed provenance are blocked and report
blocking IDs. Archive does not permit project deletion. A plan-free source may
still merge into a destination with plans.

## Operator runbook: staging and coordinated upgrade

Upgrade CLI, server and both authoritative Arc plugin variants together. Old
`file_path` creation requests receive 400 with upgrade guidance and are never
opened. The source-compatible command `arc plan create FILE` now uploads bytes;
replace obsolete `create --file FILE` examples. Approval now requires an exact
revision and captured versions. There is no silent path-based compatibility mode.

The following exercise uses only a disposable staging directory. Set `ARC_NEW` to
the absolute new binary path and `PREUPGRADE_DB` to a **consistent, offline copy**
of the old database. Keep the original offline DB and complete content root as a
paired pre-upgrade backup before upgrading. Do not run an old binary on a migrated
DB. Before rollback, export any post-upgrade revisions that must survive.

```bash
STAGE=$(mktemp -d)
mkdir -p "$STAGE/home" "$STAGE/plans" "$STAGE/sources" "$STAGE/reports"
cp "$PREUPGRADE_DB" "$STAGE/data.db"
cat > "$STAGE/config.toml" <<CFG
[server]
db_path = "$STAGE/data.db"
plans_dir = "$STAGE/plans"
[plans]
dir = "$STAGE/sources"
type = "markdown"
CFG
# Start the matching new server ON THE COPIED DB to upgrade its schema.
# TEST_PORT must be a selected free staging port, never the live default.
HOME="$STAGE/home" "$ARC_NEW" --config "$STAGE/config.toml" server start \
  --foreground --port "$TEST_PORT"
# After it reports ready, stop this foreground process with Ctrl-C.
# Operator commands refuse an unmigrated schema; startup preserves legacy inventory
# without reading its source paths.
# Copy selected legacy sources into sources/, without modifying the originals.
cat > "$STAGE/import.json" <<JSON
{"entries":[{"legacy_id":"plan.LEGACY","project_id":"PROJECT_ID","source_file":"$STAGE/sources/design.md"}]}
JSON
HOME="$STAGE/home" "$ARC_NEW" --config "$STAGE/config.toml" server plans migrate \
  --manifest "$STAGE/import.json" --dry-run > "$STAGE/reports/preview.json"
HOME="$STAGE/home" "$ARC_NEW" --config "$STAGE/config.toml" server plans migrate \
  --manifest "$STAGE/import.json" --apply > "$STAGE/reports/apply.json"
# Repeat after fixing failed entries; successful imports are idempotent.
HOME="$STAGE/home" "$ARC_NEW" --config "$STAGE/config.toml" server plans migrate \
  --manifest "$STAGE/import.json" --apply > "$STAGE/reports/retry.json"
```

Replace legacy/project IDs with inventory IDs and explicitly choose ownership;
never infer it from paths. Read legacy inventory via `GET /api/v1/plans/legacy`
with bounded `limit`/`offset` from the upgraded staging server before stopping it.
On pre-upgrade servers, use their existing plan listing. Migration preserves plan IDs,
comment IDs and original anchors; imported comments are legacy discussion with
unknown revision. Imported bytes become revision 1 in draft, even if the old plan
was approved and later edited. `legacy_status_unverified` preserves the old status
as provenance, never approval. Sources remain unchanged.

Missing/unreadable/invalid sources stay pending with per-entry errors. A partial
apply exits nonzero while still writing a JSON report and retaining successful
imports. Capture stdout and exit status separately; inspect every entry before
continuing. Dry-run reads do not migrate schema or change content, IDs or counters;
SQLite may create transient lock/index sidecars. Missing legacy content cannot be
linked or reviewed until explicitly imported and reviewed.

Run this maintenance sequence **offline with all servers/publishers stopped**.
Migration should also be staged offline during the coordinated upgrade. Never
use the live root for rehearsals. Integrity reports missing, tampered, unsafe and
unreferenced files; damaged content returns nonzero and fails reads/review/adoption.
There is no `integrity --dry-run` flag: inspection itself is read-only.

```bash
HOME="$STAGE/home" "$ARC_NEW" --config "$STAGE/config.toml" server plans integrity \
  > "$STAGE/reports/integrity.json"
# Select only orphans you intend to remove; retain the original report.
jq '{dry_run: true, files: [.files[] | select(.status == "orphan")]}' \
  "$STAGE/reports/integrity.json" > "$STAGE/reports/selected.json"
HOME="$STAGE/home" "$ARC_NEW" --config "$STAGE/config.toml" server plans cleanup \
  --report "$STAGE/reports/selected.json" --dry-run
HOME="$STAGE/home" "$ARC_NEW" --config "$STAGE/config.toml" server plans cleanup \
  --report "$STAGE/reports/selected.json" --apply
HOME="$STAGE/home" "$ARC_NEW" --config "$STAGE/config.toml" server plans backup \
  --output "$STAGE/backup" > "$STAGE/reports/backup.json"
HOME="$STAGE/home" "$ARC_NEW" --config "$STAGE/config.toml" server plans verify-backup \
  --directory "$STAGE/backup"
```

Cleanup rechecks selected regular orphan files under exclusion; never delete files
by guessing from names or during ordinary request handling. A crash can leave an
orphan without violating the rule that bytes are durable before committed metadata.
Backup requires a new destination outside the content root with an existing parent.
It captures SQLite-consistent `data.db`, the complete `plans/` tree (including
orphans), legacy metadata and a `manifest.json` of hashes, then verifies the pair.
Copying only a live SQLite main file can omit WAL data and is not a backup.

Restore into a new staging location, verify the copied pair **before starting any
server**, then point a staging config at both restored paths. Use an explicitly
selected free test port. The example requires `TEST_PORT` to have been selected
and checked by the operator; never substitute the live default port for a rehearsal.

```bash
cp -R "$STAGE/backup" "$STAGE/restored"
HOME="$STAGE/home" "$ARC_NEW" --config "$STAGE/config.toml" server plans verify-backup \
  --directory "$STAGE/restored"
cat > "$STAGE/restored.toml" <<CFG
[server]
db_path = "$STAGE/restored/data.db"
plans_dir = "$STAGE/restored/plans"
CFG
HOME="$STAGE/home" "$ARC_NEW" --config "$STAGE/restored.toml" server plans integrity
HOME="$STAGE/home" "$ARC_NEW" --config "$STAGE/restored.toml" server start \
  --foreground --port "$TEST_PORT"
```

Stop that foreground process before further offline operations. For rollback,
restore the untouched **pre-upgrade** DB/root pair into another new location,
check SQLite integrity and the saved root hash inventory, and only then start the
old binary with its explicit temporary HOME/config/DB/port. Never pair an old DB
with new files or assume a post-upgrade backup can be opened by an old binary.
Actual installation, live cutover and permanent erasure require separate operator
work; these commands rehearse migration and recovery without touching live state.
