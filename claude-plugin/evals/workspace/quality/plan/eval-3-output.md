# Eval 3: File Ownership Overlap in Parallel Tasks

## Prompt

> Break this feature into tasks that can run in parallel. Task A modifies `internal/api/server.go` and Task B also needs to modify `internal/api/server.go`.

## Answer

**No, two parallel tasks cannot own the same file.** The plan skill has an explicit rule:

> No two parallelizable tasks may own the same file — resolve overlaps via foundation task, merging, or serialization.

Since both Task A and Task B need to modify `internal/api/server.go`, they have a file ownership conflict. This must be resolved before the tasks can be parallelized. There are exactly three resolution strategies the skill permits:

### Resolution Options

1. **Merge into one task** — Combine Task A and Task B into a single task that owns `internal/api/server.go`. This is simplest when the changes are small or tightly coupled.

2. **Serialize with a dependency** — Keep them as separate tasks but add `Task B blocked by Task A`. Task A runs first and owns `server.go`; Task B runs after and also lists `server.go` in its Files section. They are no longer parallel, so the single-owner rule is satisfied.

3. **Extract to a foundation task (T0)** — If both tasks need to add shared contracts (types, interfaces, route registrations) to `server.go`, create a T0 Foundation task that makes all the shared changes to `server.go` first. Then Task A and Task B each own only their own new files and are blocked by T0. Neither touches `server.go` directly.

### Recommended Approach

The best choice depends on the nature of the changes:

- If both tasks add independent route handlers, **extract route registration to T0** and let each task own only its handler file (e.g., `internal/api/handler_a.go` and `internal/api/handler_b.go`).
- If the changes to `server.go` are interleaved or both tasks modify the same function, **merge the tasks**.
- If one task's changes are a prerequisite for the other, **serialize with a dependency**.

### Key Principle

The file ownership rule exists because parallel subagents work from the same HEAD. If two agents independently edit the same file, their changes will conflict on merge. The plan skill prevents this at planning time rather than dealing with merge conflicts at implementation time.
