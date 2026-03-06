# Review Route Improvements

Design notes for `/<workspace_id>/review`: resizable file area and per-line comments.

---

## 1. Resize file window / horizontal scroll

### Current behavior

- **Layout**: Fixed left sidebar (`w-64`), main area `flex-1`, bottom submit bar.
- **Diff table**: Wrapped in `overflow-x-auto` in `FileSection.svelte`, so wide content can scroll horizontally.
- **Code cell**: Uses `whitespace-pre-wrap`, so long lines wrap instead of driving horizontal scroll.

### Recommendations

**A. Resizable sidebar (recommended)**

- Make the boundary between the file tree and the main content **draggable** so users can give more width to the diff.
- Store sidebar width in component state (e.g. `sidebarWidth` in the review page), with min/max (e.g. 200px–480px).
- Use a narrow drag handle (e.g. 4–6px) with a hover state; on `mousedown` set a global `mousemove`/`mouseup` listener to update width.
- No new dependencies required; alternatively use something like `svelte-resizable-panels` if you want persisted sizes or multiple splitters.

**B. Horizontal scroll for long lines**

- In `DiffLine.svelte`, change the code cell from `whitespace-pre-wrap` to `whitespace-pre` so long lines do not wrap.
- Ensure the table can grow: the parent already has `overflow-x-auto` in `FileSection.svelte`; give the `<table>` a sensible `min-width` (e.g. `min-w-full` or a fixed min) so the scrollable region is the table, not the viewport.
- Optionally add `overflow-x-auto` at the review page level on the main scroll area so the whole diff pane scrolls horizontally when the table is wide.

**C. Optional: full resizable layout**

- If you want both the file tree and the diff to be resizable, add a second split between the diff list and a future “comments” panel, or use a single horizontal split (tree | content) as in (A).

---

## 2. Per-line review comments

### Current behavior

- **API**: `submitReview` accepts `comment` (overall) and `file_comments: Record<string, string>` (one string per file).
- **Backend**: `reviewSession` stores `FileComments map[string]string`; no notion of “line” today.
- **UI**: One comment area per file at the bottom of each `FileSection`; no line-level UI.

### Option 1: Backend-supported line comments (recommended long-term)

- **API change**: Add a structured field for line comments, e.g.:
  - `line_comments?: Record<string, LineComment[]>` where key is file path and value is `{ line: number, side: 'old' | 'new', comment: string }[]`, or
  - Keep `file_comments` and add `line_comments` as a separate structure keyed by file, then by line (e.g. `"path/to/file"` → `{ "new:42": "comment" }`).
- **Backend**: Extend `reviewSession` and `submitReviewRequest` to store and return line comments; no change to diff or status response format beyond new fields.
- **Frontend**:
  - **State**: e.g. `lineComments: Map<fileKey, Map<lineKey, string>>` where `lineKey` is `"old:N"` or `"new:N"` (so you can comment on deleted lines via old line number and added lines via new line number).
  - **UI**: In `DiffLine.svelte` (or a wrapper), make each row able to open a comment:
    - **Hover**: Show a “comment” icon at the end of the line (or on the line number).
    - **Click**: Open a small popover or inline form (textarea + Save/Cancel) under or beside the line; on save, update `lineComments` for this file and line.
  - **Display**: For each line that has a comment, show a small comment thread below the row (or in a right-hand “comments” column/sidebar). Keep the existing per-file comment area for file-level notes.
- **Submit**: Send the new `line_comments` structure in the submit payload; backend persists it and returns it in status so you can later re-fetch and show comments (e.g. after submit or on reload if you add “load status” on page load).

### Option 2: Encode line comments in existing `file_comments` (no backend change)

- **Idea**: Keep `file_comments` as `Record<string, string>`, but for files that have line comments, store a JSON string, e.g.:
  - `{ "fileComment": "optional file-level text", "lineComments": { "new:42": "comment for new line 42", "old:10": "comment for old line 10" } }`
- **Convention**: If the string starts with `{`, parse as JSON and handle `lineComments` (and optional `fileComment`); otherwise treat as legacy single file comment.
- **Frontend**: Same as above for state and UI (per-line comment icon, popover, display under row). On submit, for each file build either a plain string (file comment only) or the JSON string (file + line comments). When loading comments (e.g. after submit or when you add “load review status” on page load), parse and populate `lineComments` from `file_comments`.
- **Limitation**: Backend and other clients don’t understand “lines”; they only see a string. Fine for a single UI that owns the convention; not ideal if you want line comments in API responses or other tools.

### Recommendation

- **Short term / minimal change**: Option 2 (encode in `file_comments`) so you can add per-line comments in the UI without touching the backend. Optionally call `getReviewStatus` after creating the session (or on load when `reviewId` is present) to restore comments if you later persist them.
- **Long term**: Option 1 (explicit `line_comments` in API and backend) for clarity, extensibility, and potential reuse (e.g. showing line comments in emails or other UIs).

### Line identity

- Use **`oldNumber`** for deleted/context lines (and when you want to refer to the “before” side).
- Use **`newNumber`** for inserted/context lines (and when you want to refer to the “after” side).
- Keys like `"old:42"` and `"new:42"` avoid collisions and make it clear which side the comment refers to; `DiffLine` already has `oldNumber` and `newNumber` from the parser.

---

## Summary

| Goal                         | Approach                                                                 |
|-----------------------------|---------------------------------------------------------------------------|
| Resize file/diff area       | Draggable boundary for sidebar width; optional `whitespace-pre` + min-width for true horizontal scroll. |
| Per-line comments (fast)    | Encode line comments in existing `file_comments` as JSON; same UI as below. |
| Per-line comments (proper)   | Add `line_comments` (or equivalent) in API and backend; UI: comment icon per line, popover, show thread under row. |
