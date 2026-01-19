# Security Review Report

**Date:** 2026-01-18
**Reviewer:** Claude Code Security Analysis
**Scope:** Full codebase review of arc issue tracking server

## Summary

**1 HIGH-CONFIDENCE vulnerability identified** in the arc codebase.

| Severity | Count |
|----------|-------|
| HIGH     | 1     |
| MEDIUM   | 0     |
| LOW      | 0     |

---

## Vuln 1: IDOR / Authorization Bypass - Cross-Workspace Access

**Status:** ✅ FIXED (2026-01-19)

**Files:**
- `internal/api/issues.go:125-237`
- `internal/api/comments.go:21-86`
- `internal/api/dependencies.go:17-60`
- `internal/api/labels.go:104-148`
- `internal/storage/sqlite/issues.sql:8-89`

**Severity:** HIGH

**Category:** `authorization_bypass` / `idor`

**Confidence:** 9/10

### Description

The API handlers for issues, comments, and dependencies do NOT validate that the requested resource belongs to the specified workspace in the URL path. The `workspaceID` parameter is extracted from the URL but never used for authorization checks. Database queries use only the issue ID without workspace filters, allowing access to any resource regardless of which workspace is specified in the URL.

### Affected Endpoints

- `GET/PUT/DELETE /api/v1/workspaces/:ws/issues/:id`
- `POST /api/v1/workspaces/:ws/issues/:id/close`
- `POST /api/v1/workspaces/:ws/issues/:id/reopen`
- `GET/POST /api/v1/workspaces/:ws/issues/:id/comments`
- `PUT/DELETE /api/v1/workspaces/:ws/issues/:id/comments/:cid`
- `GET/POST /api/v1/workspaces/:ws/issues/:id/deps`
- `DELETE /api/v1/workspaces/:ws/issues/:id/deps/:dep`

### Exploit Scenario

1. Workspace A (`ws-alpha`) contains sensitive issue `ALPHA-001`
2. Workspace B (`ws-beta`) is controlled by attacker with issue `BETA-001`
3. Attacker sends: `GET /api/v1/workspaces/ws-beta/issues/ALPHA-001`
4. Server returns `ALPHA-001` from workspace A despite URL specifying workspace B
5. Attacker can also modify/delete issues, add comments, and create dependencies across workspace boundaries using similar requests

### Root Cause

```go
// internal/api/issues.go:125-142
func (s *Server) getIssue(c echo.Context) error {
    id := c.Param("id")
    // workspaceID(c) is available but NEVER used for validation

    issue, err := s.store.GetIssue(c.Request().Context(), id)  // No workspace check!
    if err != nil {
        return errorJSON(c, http.StatusNotFound, err.Error())
    }
    return successJSON(c, issue)
}
```

### Recommendation

**Option 1: Add validation in API handlers**

```go
func (s *Server) getIssue(c echo.Context) error {
    id := c.Param("id")
    wsID := workspaceID(c)

    issue, err := s.store.GetIssue(c.Request().Context(), id)
    if err != nil {
        return errorJSON(c, http.StatusNotFound, err.Error())
    }

    // Add workspace membership validation
    if issue.WorkspaceID != wsID {
        return errorJSON(c, http.StatusForbidden, "access denied")
    }

    return successJSON(c, issue)
}
```

**Option 2: Modify storage layer queries to include workspace_id**

```sql
-- Change from:
SELECT * FROM issues WHERE id = ?;

-- To:
SELECT * FROM issues WHERE id = ? AND workspace_id = ?;
```

Option 2 is preferred as it enforces authorization at the data layer, preventing bypasses.

---

## Issues Examined But Not Reported

The following were examined but did not meet the reporting threshold:

| Category | Finding |
|----------|---------|
| SQL Injection | Codebase uses sqlc with parameterized queries throughout |
| XSS | Svelte frontend auto-escapes by default, no `{@html}` on user input |
| Command Injection | No shell execution with user-controlled input found |
| Path Traversal | No filesystem operations with user-controlled paths |
| Authentication | System intentionally uses optional "actor" header for audit purposes (documented design choice) |

---

## Remediation Priority

| Priority | Vulnerability | Effort |
|----------|--------------|--------|
| P0 - Critical | IDOR / Cross-Workspace Access | Medium |

The IDOR vulnerability should be fixed immediately as it completely undermines workspace isolation.

---

## Fix Applied

**Date:** 2026-01-19

**Approach:** Option 1 - Handler validation (API layer)

Added workspace validation to all issue-related API handlers. Each handler now validates that the issue belongs to the workspace specified in the URL before performing any operations.

**Changes:**

1. **`internal/api/server.go`** - Added helper functions:
   - `validateIssueWorkspace(c, issueID)` - Returns error if issue doesn't belong to workspace
   - `getIssueInWorkspace(c, issueID)` - Returns issue after validation
   - `errWorkspaceMismatch` - Sentinel error for workspace mismatch

2. **`internal/api/issues.go`** - Added validation to:
   - `getIssue`, `updateIssue`, `deleteIssue`, `closeIssue`, `reopenIssue`

3. **`internal/api/comments.go`** - Added validation to:
   - `getComments`, `addComment`, `updateComment`, `deleteComment`, `getEvents`

4. **`internal/api/dependencies.go`** - Added validation to:
   - `getDependencies`, `addDependency`, `removeDependency`

5. **`internal/api/labels.go`** - Added validation to:
   - `addLabelToIssue`, `removeLabelFromIssue`

**Security Response:**
- Returns HTTP 403 Forbidden with "access denied" when workspace mismatch detected
- Returns HTTP 404 Not Found when issue doesn't exist

**Verification:**
- All tests pass
- Build succeeds
