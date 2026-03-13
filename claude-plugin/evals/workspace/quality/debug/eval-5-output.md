# Debug Eval 5: API returns 500 on /users

## Hypothesis Chosen

**The `/users` endpoint does not exist in the arc API.** None of the three suggested causes (database connection pool exhausted, query timeout, missing index on users table) are applicable because there is no `/users` route, no users table, and no user-related storage methods in the entire codebase.

## Investigation Steps (4-Phase Process)

### Phase 1: Investigate Root Cause

1. **Searched for `/users` in all Go files** - zero matches. The endpoint is not registered anywhere.
2. **Reviewed `internal/api/server.go` route registration** (`registerRoutes()` at line 90) - confirmed the complete list of API routes. No `/users` endpoint exists. The API serves: projects, workspaces, issues, dependencies, labels, comments, plans, events, health, and filesystem browsing.
3. **Reviewed `internal/storage/storage.go` interface** - no user-related storage methods. The only "user" concept is the `X-Actor` request header (a simple string, defaults to "anonymous").
4. **Searched OpenAPI spec** (`api/` directory) for any mention of "users" - no matches.
5. **Checked recent git history** (`git log --oneline -10`, `git diff --stat`) - no user-related changes.
6. **Ran full test suite** (`make test`) - all tests pass, confirming the codebase is healthy.

### Phase 2: Pattern Analysis

- All existing endpoints follow a consistent pattern: route registered in `registerRoutes()`, backed by a handler method on `Server`, calling through to a `Storage` interface method, implemented in SQLite.
- There is no users resource at any layer (types, storage, API, OpenAPI spec).
- Checked SPA fallback behavior: when built with `webui` tag, unmatched GET routes fall through to `web.RegisterSPA` which serves `index.html` (returns 200 with HTML, not 500). When built without `webui`, no fallback is registered (Echo returns 404/405).

### Phase 3: Hypothesis Testing

**Hypothesis**: The 500 is caused by requesting a nonexistent endpoint.

**Test**: Examined what happens when a request hits a route not in `registerRoutes()`:
- For GET requests: the `/*` SPA catch-all (line 58 of `web/embed_webui.go`) intercepts and serves `index.html` with a 200 status. This means GET `/users` would return HTML (the SPA), not a 500.
- For non-GET requests (POST, PUT, etc) to `/users`: Echo returns 405 Method Not Allowed.
- Without the webui build tag: Echo returns 404 Not Found.

**Conclusion**: A true HTTP 500 on `/users` should not occur under normal conditions. If a 500 is actually being returned, it could only happen if:
1. The SPA fallback panics (caught by `middleware.Recover()`, which returns 500)
2. The embedded filesystem is corrupted or the build is malformed
3. A reverse proxy or middleware upstream is generating the 500

### Phase 4: Implement Fix

**No fix is needed.** The `/users` endpoint does not exist and was never part of the arc API. The reported 500 error cannot be attributed to any of the three suggested causes (connection pool, query timeout, missing index) because there is no users table or query.

**Recommended action**: If a `/users` endpoint is actually desired, it would need to be designed and implemented end-to-end (types, storage interface, SQLite implementation, API handler, OpenAPI spec, route registration). If the 500 is being observed in practice, the investigation should focus on what is actually handling the request (reverse proxy, SPA fallback, middleware).

## Tools Called

| Tool | Purpose |
|------|---------|
| Grep (pattern: `/users`, glob: `*.go`) | Search for users endpoint in Go source |
| Bash (`git log --oneline -10`) | Check recent commits for relevant changes |
| Bash (`git diff --stat`) | Check uncommitted changes |
| Grep (pattern: `user`, path: `internal/api/`, glob: `*.go`) | Broader search for user-related code in API layer |
| Read (`internal/api/server.go`) | Review all registered routes |
| Read (`internal/storage/storage.go`) | Review storage interface for user methods |
| Grep (pattern: `users`, path: `api/`) | Check OpenAPI spec for users endpoint |
| Grep (pattern: `RegisterSPA`, glob: `*.go`) | Find SPA handler registration |
| Read (`web/embed_webui.go`) | Understand SPA fallback behavior |
| Read (`web/embed_stub.go`) | Understand non-webui build behavior |
| Bash (`make test`) | Verify codebase health and no regressions |
