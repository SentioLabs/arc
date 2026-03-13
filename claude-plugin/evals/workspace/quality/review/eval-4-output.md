# Eval 4: Reviewer Finding — Pointer vs Value Return for GetUser

## Reviewer Finding

> Critical — "The function GetUser should return (User, error) not (*User, error) because User is small enough to copy."

## Evaluation: Reject this finding

### Reason: The finding contradicts an established, consistent codebase convention

The reviewer's suggestion is **technically defensible in isolation** — returning small structs by value avoids a heap allocation and can be marginally faster. However, applying it here would be wrong for this codebase. Here is the evidence:

**1. There is no `GetUser` function in this codebase.** A grep for `GetUser` across the entire repository returns zero hits in actual Go source files. The reviewer appears to be flagging a function that does not exist, which is itself a red flag on the finding's validity.

**2. Every storage getter returns a pointer.** The `Storage` interface in `internal/storage/storage.go` defines 20+ getter methods. Every single one that returns a domain struct returns a pointer:

- `GetProject(ctx, id) (*types.Project, error)`
- `GetIssue(ctx, id) (*types.Issue, error)`
- `GetLabel(ctx, name) (*types.Label, error)`
- `GetWorkspace(ctx, id) (*types.Workspace, error)`
- `GetPlan(ctx, id) (*types.Plan, error)`
- `GetIssueDetails(ctx, id) (*types.IssueDetails, error)`
- `GetStatistics(ctx, projectID) (*types.Statistics, error)`
- `GetPlanContext(ctx, issueID) (*types.PlanContext, error)`
- `AddComment(ctx, ...) (*types.Comment, error)`

There are zero exceptions. This is a deliberate, uniform convention.

**3. Even small structs like `Label` (3 string fields) use pointer returns.** If the "small enough to copy" argument were the governing principle, `Label` and `Dependency` would already be value returns. They are not. The codebase chose consistency over micro-optimization, which is the correct trade-off for a project of this nature.

**4. Pointer returns enable nil-as-not-found semantics.** Returning `*T` lets callers distinguish "not found" (nil pointer, no error) from "found" (non-nil pointer). Switching a single method to value return would break this pattern and force a different not-found signaling mechanism for one method only.

### Verdict

This is not a valid Critical finding. It is a style preference that conflicts with the project's established convention. Consistency across the interface is more valuable than saving one heap allocation on a function that (a) does not exist and (b) would be called infrequently even if it did.

**Action taken: None. Finding rejected with evidence.**
