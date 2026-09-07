# Arc agent operations investigation

Investigation: `arc-0d80.05y7th`, September 2026. This examines Claude Code, pi,
and Codex. It does not establish a production incident or authorize a new
orchestration framework. Detailed task contracts and scope boundaries remain
required in every runtime.

## Recommendations

| Workstream | Decision | Reason and follow-up |
| --- | --- | --- |
| Progress recording | Implement CLI access to existing comments; migrate all three adapters | Append-only comments preserve concurrent notes and specification edits. `arc-0d80.034wng`, then `arc-0d80.03c64b`. |
| Multi-field update consistency | Implement atomic storage update | Injected failure leaves half an update committed today. `arc-0d80.03smij`. |
| Exclusive acquisition | Retain assignment semantics; defer acquisition and leases | Two claims succeeding demonstrates current semantics, not duplicate implementation. Global dispatch/recovery requirements are unspecified. |
| Task-source preparation | Implement narrow runtime-neutral capture/validation with thin adapters | Three consumers justify sharing schema and exact serialization. `arc-0d80.01xfdz`, then `arc-0d80.02veqj`. |
| Concurrent specification editing | Defer conditional revision writes | Comments remove progress writes from this path. Multi-editor requirements remain unestablished. |
| Comment retry idempotency | Defer operation-key storage; prohibit blind automatic POST retries initially | Existing API has no idempotency key. Identical text is not proof of duplicate intent. |

Session resolution and registration are separate work in `arc-0d80.00tifl` and
`arc-0d80.01f3ua`. The concurrency fixtures use valid explicitly registered
sessions; identity fallback does not explain their results.

## Baselines and reproduction

- Arc: `93f5823cfe6c2dc54552b518a27fb2d72a283060`, including the root-session fix.
  The installed global CLI was `0.16.0-rc9`; the fixtures used a locally built
  binary from the inspected checkout.
- Agent marketplace: `f806cb7bdb47984b2d706d634240c8e29b02f4ae`. Inspected Codex
  plugin `0.13.0` and Claude plugin `0.19.0`; relevant installed files matched
  the inspected source.
- pi-nexus: `62c7a3550ddc3cb6e65ded609d78012821bf3cf9`, pi-arc `0.11.0`.
  Relevant installed extension, skills and agent templates matched source.
- Independent source review traced complete consumers, metadata, retry paths,
  and per-runtime differences. No implementation workers ran in the competing
  claim experiment.

The adjacent scripts are investigative fixtures, not future-behavior regression
tests: they deliberately assert the baseline failures. Run from an Arc checkout
containing the baseline binary; the second command accepts an agent-marketplace
checkout with the recorded commit available:

```sh
task build:quick
python3 docs/investigations/agent-operations/reproduce.py ./bin/arc
python3 docs/investigations/agent-operations/handoff-fixtures.py /path/to/agent-marketplace
```

`reproduce.py` needs Python's standard library, local sockets, and the Arc binary.
It creates and removes a temporary server/database, records synchronized writes,
injects failures only into that disposable database, and stops the server in
cleanup. `handoff-fixtures.py` needs Bash, jq and Git; it extracts exact recorded
Codex source lines using `git show` and synthetic API-shaped JSON. It neither
runs a model nor dispatches a worker. Results are retained in
`baseline-results.json` and `handoff-results.json`.

## A. Progress and specification preservation

### Reproduced behavior

| Fixture | Result |
| --- | --- |
| A reads D; B saves D+spec-B; A saves D+note-A | Final D+note-A; A's equality readback passes while spec-B is absent. |
| A's successful description response is discarded; C edits; A retries old body | C's intervening edit is overwritten. |
| Two note appenders synchronize their reads of D | Final D+note-B; note-A is lost. |
| Two comment POSTs and a specification edit run concurrently | Both distinct comments and D+spec-B survive. |
| Successful comment response discarded, identical POST repeated | Two comments with the same text are created. |

These are observed disposable-fixture outcomes. They do not show that a real
user's task was lost. A successful readback detects some write failures; it
cannot detect an overwritten edit absent from the writer's stale body.

The current API already exposes issue comments in
`internal/api/comments.go`, `internal/api/server.go`, and OpenAPI. Storage inserts
comments separately from the issue description. The missing part is ergonomic
CLI add/list input and an all-runtime caller migration. `arc show --json` already
contains comments; ordinary `arc show` prints them.

### Actual consumers differ

Paths below are relative to each runtime's Arc plugin/package root at the
recorded revision.

| Runtime | Progress writer | Progress reader |
| --- | --- | --- |
| Codex | `skills/finish/SKILL.md:14-31` merges a note into a copy, replaces description, and compares | Build sequential/parallel, review, and team paths project `.description` into source files and omit comments. |
| Claude Code | `skills/finish/SKILL.md:39,68` uses short CONTEXT/PROGRESS description replacements | Builder/reviewer templates paste ordinary `arc show`, which includes comments. |
| pi | `skills/arc-finish/SKILL.md:29,58` uses the same short description replacements | Builder/reviewer templates paste ordinary `arc show`, which includes comments. |

The Claude/pi PROGRESS recipe directly targets existing in-progress tasks and
therefore replaces their full specification under current update semantics.
This is a source-demonstrated destructive recipe, not an observed production
loss. The CONTEXT example may target a newly created issue with an empty body;
that does not make the existing-task PROGRESS example safe.

Moving notes to comments without updating consumers is insufficient. Codex needs
separate complete comment sources. Claude/pi already retrieve comments through
ordinary show, but need reliable retention and clear separation of approved
contracts from progress. Their evaluator templates also paste ordinary show
while requiring independent spec-based tests: automatically including builder
reports conflicts with that intended separation. Preserve full approved task
and design for evaluators; supply progress separately to authorized consumers.
Do not delete older notes embedded in descriptions or silently truncate decisions.

### Smallest alternatives and costs

1. Keep description appends: no rollout cost, but retains the demonstrated
   lost-update behavior and destructive Claude/pi examples. Not recommended.
2. CLI over existing comments: small core change, no schema migration required.
   Add stdin/file input, actor attribution, returned IDs and JSON output; change
   all finish/resume/build/review/debug callers and evaluator input plumbing.
   Publish a minimum CLI version with adapter rollout. Selected.
3. Add operation keys: enables safe lost-response retries but needs persistent
   uniqueness scoped to issue/key, replay versus changed-payload conflict rules,
   client key retention, and API/schema tests. Deferred. The first version must
   disclose ambiguous outcomes and avoid automatic duplicate POSTs; comparing
   text alone is not a safe substitute.
4. Conditional description writes: a real revision token plus expected-revision
   predicate could reject stale specification edits. It needs API/client support
   and deliberate conflict resolution. A local equality check or timestamp read
   followed by an unconditional write is not equivalent. Deferred pending need.

Required tests: CLI input/actor/JSON fidelity; concurrent comments and spec edit
all retained; documented lost-response behavior; all runtime consumers retain
complete specifications and relevant decisions; evaluator inputs remain
independent; no automatic rewriting of embedded history.

## B. Assignment, exclusivity, and atomicity

Both synchronized `arc update --take --session-id <registered-id>` commands
returned exit 0. The issue ended in_progress under one owner. The winner varies
with scheduling; the recorded run ended with registered-session-A. This is
assignment, not compare-and-set acquisition. It does not prove either worker
implemented anything.

Current orchestration coordinates within a deployment, without a global Arc
ownership exclusion:

- Codex build uses ready/selection followed by a separate take; team labels and
  task maps are procedural coordination.
- Claude team dispatch uses native team tasks and runtime ownership within one
  team. Other Claude sessions, pi, Codex and CLI clients can still assign the
  same Arc issue.
- pi parallel build uses optional pi-subagents with fresh contexts and worktree
  patches; arc_agent is its sequential fallback. Neither is a database lock.

The maintainer was asked whether Arc must arbitrate independent dispatchers.
No additional requirement was supplied during this investigation. The source
does not justify assuming a universal single dispatcher, but it also does not
establish a need for leases. Retain `--take` as assignment. If acquisition becomes
required, prefer a distinct atomic conditional operation with current-owner
conflicts and deliberate reassignment kept separate. Specify same-owner retries,
in-progress/closed/deferred state, blockers, cancellation and abandoned ownership
before implementation. Leases/heartbeats need separate recovery requirements.

There is a concrete consistency defect independent of that decision:
`internal/storage/sqlite/issues.go:341-401` updates fields individually. A trigger
that rejects the second status/session write, regardless of map order, produces
HTTP 500 while the first field remains changed. The recorded run retained
status=in_progress and no owner; another run retained an owner and status=open.

Select one transactional UpdateIssue follow-up. Preserve successful assignment
semantics and coordinate events/FTS with commit/rollback. The store has a single
connection, so a transaction must not call helpers that acquire the global
connection and deadlock. Test deterministic second-write failure, committed
events/search consistency, successful explicit status overrides, validation,
project isolation and repeated deliberate assignment. Do not describe session
registration plus issue update as one atomic transaction.

## C. Complete, deterministic task sources

The current Arc details fixture returns parent-child dependencies but omits
populated parent_id; it also omits labels for an unlabeled issue. All three
adapters' parent_id lookups therefore miss a real parent design.

Exact Codex shell fixtures additionally reproduce:

- Command substitution removes trailing LF bytes from descriptions; trailing
  CRLF leaves a stray CR. Full source text is not retained exactly.
- Its label validator rejects omitted labels even though that is valid API JSON.
- The conditional parent selector returns the whole object when parent_id is
  populated. Claude/pi have a simpler selector and do not share this extra bug.
- Projecting only description drops comments present in the response.

Claude/pi currently paste ordinary show and ask for relevant design excerpts;
they do not retain the Codex task-indexed immutable source manifest. Their
builder scope constraints still require complete Files, Scope Boundary, Design
Contracts, steps and acceptance requirements. An excerpt or summary is not a
replacement for the user's full-source requirement. Claude team dispatch starts
from compact arc team context and needs explicit per-task hydration.

Pi's existing materializer generates specialist agent definitions and model
metadata. It does not capture issue/parent sources. Its migration script imports
Claude skills with Pi adaptations and intentionally omits persistent Claude team
primitives. These distinctions must survive the rollout.

### Ownership alternatives

A shared portable plugin helper could avoid an Arc release and use existing
detail endpoints. It must be distributed and versioned in three integrations,
survive Pi source regeneration, and be packaged explicitly: pi-arc does not ship
its repository scripts directory. Three independent shell implementations would
retain the maintenance problem.

A narrow Arc CLI capture/export plus retained-source validation is selected:
one schema-aware implementation serves three real consumers, at the cost of a
stable command/manifest contract and minimum CLI rollout. Keep runtime adapters
small. Do not move role/model choice, tool syntax, dispatch, workspace lifecycle,
scope judgment or integration into core. No new API is needed unless atomic
cross-resource snapshots later become a requirement.

The core contract should retain raw task/parent JSON, exact full description
files, separate comments, resolved identities and dependency parent, normalized
absent labels, observed updated_at/capture timestamps, and hashes. Validate
returned IDs and types, use an explicit isolated destination, reject cross-task
reuse, and publish the manifest only after all required sources validate.

Adapter metadata retains the selected route, workspace/cwd, VCS, immutable code
base, plugin path, and verification commands. A retry validates retained source
identity/paths/hashes without refetching mutable Arc state. Add feedback
separately. Explicit source refresh creates a new capture. Digests establish
what was captured; existing separate API reads do not prove task, comments,
labels and parent existed at one atomic database revision.

Tests must exercise API-shaped omitted fields and dependencies, LF/CRLF/Unicode
and shell-sensitive bytes, wrong identities/types, missing parents, fetch/write
failures, destination collisions and tampering. Two tasks with distinct parents,
routes and workspaces must survive reverse-order retries after server mutation
with zero refetches. Thin-adapter tests cover Codex build/review/team, Claude
Agent/team, pi preferred subagent and arc_agent fallback. Pi regeneration/package
checks and existing runtime reference validators complement executable source
fidelity tests; static token matches alone cannot prove the contract.

## Implementation boundaries

The five linked issues contain the selected implementation contracts and
dependencies. This investigation changes no claim semantics, progress-writing
semantics, dispatch framework or task specification. Exclusive acquisition,
leases, conditional specification writes, operation-key storage and atomic
server snapshots remain deferred. The separate session-registration fix is
validated across actual Claude Code, pi and Codex runtime identities.
