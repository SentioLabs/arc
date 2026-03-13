# Debug Eval 1: TestCreateIssue nil pointer at issues.go:42

## Task

"Tests are failing: TestCreateIssue panics with nil pointer at internal/storage/sqlite/issues.go:42. Fix it."

## Phase 1: Investigate Root Cause

### Tools Called & Files Read

1. **Read** `/Volumes/ExternalSamsung/devspace/personal/sentiolabs/arc/claude-plugin/skills/debug/SKILL.md` -- loaded skill instructions
2. **Read** `/Volumes/ExternalSamsung/devspace/personal/sentiolabs/arc/internal/storage/sqlite/issues.go` -- the file containing the alleged panic at line 42
3. **Bash** `go test ./internal/storage/sqlite/...` -- attempted to reproduce the failure
4. **Bash** `git log --oneline -10` -- checked recent commits
5. **Bash** `git diff HEAD~1 -- internal/storage/sqlite/issues.go` -- checked recent changes to the failing file
6. **Glob** `internal/storage/sqlite/*_test.go` -- found all test files
7. **Grep** `func TestCreateIssue` -- searched for the named test function
8. **Grep** `type Store struct` -- examined Store structure (fields: `db *sql.DB`, `queries *db.Queries`, `path string`)
9. **Read** `/Volumes/ExternalSamsung/devspace/personal/sentiolabs/arc/internal/storage/sqlite/store.go` -- examined Store constructor (`New()`)
10. **Read** `/Volumes/ExternalSamsung/devspace/personal/sentiolabs/arc/internal/storage/sqlite/plans_test.go` -- found test helpers (`setupTestStore`, `setupTestProject`, `setupTestIssue`)
11. **Read** `/Volumes/ExternalSamsung/devspace/personal/sentiolabs/arc/internal/storage/sqlite/workspaces_test.go` -- checked for issue-creation tests
12. **Grep** `func (s *Store) GetProject` -- examined GetProject to check for nil-return scenarios
13. **Grep** `func dbProjectToType` -- confirmed it always returns a non-nil pointer
14. **Grep** `func.*GetIssue` in `db/` -- examined sqlc-generated GetIssue function
15. **Grep** `type Issue struct` in `db/` -- confirmed db.Issue model structure

### Key Findings

1. **The test `TestCreateIssue` does not exist.** There is no function named `TestCreateIssue` anywhere in the codebase. A grep across the entire project returned zero matches. The test names that do exist are: `TestCreateWorkspace`, `TestCreateAndGetPlan`, tests in `search_test.go`, `close_cascade_test.go`, `issues_parent_test.go`, and `merge_test.go`.

2. **All tests pass.** Running `go test ./internal/storage/sqlite/...` completed successfully with status `ok` in 1.412s. No panics, no failures.

3. **Line 42 of issues.go is not a nil pointer risk in normal operation.** Line 42 is inside `getNextChildNumber()`, which calls `s.db.QueryRowContext(...)`. The `s.db` field is always set by the `New()` constructor (store.go:57-58), and `New()` is the only way to create a Store. A nil pointer at `s.db` would only happen if someone constructed a `Store{}` literal without going through `New()` -- but no test does this.

4. **No recent changes to issues.go.** `git diff HEAD~1` showed no changes to this file. The recent commits are about project rename, web lint fixes, and OpenAPI spec regeneration -- none touch issue storage logic.

5. **The `CreateIssue` method (line 80-149) is well-guarded.** It calls `GetProject` (which returns error if project not found), validates the issue, and all code paths check errors before proceeding. The `dbIssueToType` and `dbProjectToType` helper functions always return non-nil pointers (they construct a new struct literal).

## Phase 2: Pattern Analysis

- The test helpers in `plans_test.go` use `setupTestStore` -> `sqlite.New(dbPath)` which properly initializes all Store fields including `s.db`.
- `setupTestIssue` calls `store.CreateIssue(ctx, issue, "test-actor")` and this works correctly in existing tests (e.g., search_test.go creates multiple issues without issues).
- No code path in the existing codebase creates a Store with a nil `db` field.

## Conclusion

**The reported bug cannot be reproduced.** The test `TestCreateIssue` does not exist in this codebase, all existing tests pass, and the code at issues.go:42 has no nil pointer vulnerability under normal usage. The bug report appears to describe a scenario that does not match the current state of the code.

### Possible Explanations

- The test may have existed in a different branch or was removed.
- The error report may reference a different version of the code.
- The line numbers may have shifted after recent refactoring (the workspace-to-project rename).

### Next Steps (if this were a real bug)

Per the debug skill, Phase 3 (hypothesis testing) and Phase 4 (implement fix) are blocked because there is nothing to fix -- the failure cannot be reproduced and the referenced test does not exist. If the user can provide the actual test output or stack trace, investigation can resume from there.
