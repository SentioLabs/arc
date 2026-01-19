# Making Issues Resumable Across Sessions

## When Resumability Matters

**Use enhanced documentation for:**
- Multi-session technical features with API integration
- Complex algorithms requiring code examples
- Features with specific output format requirements
- Work with "occult" APIs (undocumented capabilities)

**Skip for:**
- Simple bug fixes with clear scope
- Well-understood patterns (CRUD operations, etc.)
- Single-session tasks
- Work with obvious acceptance criteria

**The test:** Would a fresh Claude instance (or you after 2 weeks) struggle to resume this work from the description alone? If yes, add implementation details.

## Anatomy of a Resumable Issue

### Minimal (Always Include)
```markdown
Description: What needs to be built and why
Acceptance Criteria: Concrete, testable outcomes (WHAT not HOW)
```

### Enhanced (Complex Technical Work)
```markdown
Notes Field - IMPLEMENTATION GUIDE:

WORKING CODE:
```python
# Tested code that queries the API
service = build('drive', 'v3', credentials=creds)
result = service.about().get(fields='importFormats').execute()
# Returns: {'text/markdown': ['application/vnd.google-apps.document'], ...}
```

API RESPONSE SAMPLE:
Shows actual data structure (not docs description)

DESIRED OUTPUT FORMAT:
```markdown
# Example of what the output should look like
Not just "return markdown" but actual structure
```

RESEARCH CONTEXT:
Why this approach? What alternatives were considered?
Key discoveries that informed the design.
```

## Real Example: Before vs After

### ❌ Not Resumable
```
Title: Add dynamic capabilities resources
Description: Query Google APIs for capabilities and return as resources
Acceptance: Resources return capability info
```

**Problem:** Future Claude doesn't know:
- Which API endpoints to call
- What the responses look like
- What format to return

### ✅ Resumable
```
Title: Add dynamic capabilities resources
Description: Query Google APIs for system capabilities (import formats,
themes, quotas) that aren't in static docs. Makes server self-documenting.

Notes: IMPLEMENTATION GUIDE

WORKING CODE (tested):
```python
from workspace_mcp.tools.drive import get_credentials
from googleapiclient.discovery import build

creds = get_credentials()
service = build('drive', 'v3', credentials=creds)
about = service.about().get(
    fields='importFormats,exportFormats,folderColorPalette'
).execute()

# Returns:
# - importFormats: dict, 49 entries like {'text/markdown': [...]}
# - exportFormats: dict, 10 entries
# - folderColorPalette: list, 24 hex strings
```

OUTPUT FORMAT EXAMPLE:
```markdown
# Drive Import Formats

Google Drive supports 49 import formats:

## Text Formats
- **text/markdown** → Google Docs ✨ (NEW July 2024)
- text/plain → Google Docs
...
```

RESEARCH CONTEXT:
text/markdown support announced July 2024 but NOT in static Google docs.
Google's workspace-developer MCP server doesn't expose this.
This is why dynamic resources matter.

Acceptance Criteria:
- User queries workspace://capabilities/drive/import-formats
- Response shows all 49 formats including text/markdown
- Output is readable markdown, not raw JSON
- Queries live API (not static data)
```

**Result:** Fresh Claude instance can:
1. See working API query code
2. Understand response structure
3. Know desired output format
4. Implement with context

## Notes Template for Complex Issues

Copy this into notes field for complex technical features:

```markdown
IMPLEMENTATION GUIDE FOR FUTURE SESSIONS:

WORKING CODE (tested):
```language
# Paste actual code that works
# Include imports and setup
# Show what it returns
```

API RESPONSE SAMPLE:
```json
{
  "actualField": "actualValue",
  "structure": "as returned by API"
}
```

DESIRED OUTPUT FORMAT:
```
Show what the final output should look like
Not just "markdown" but actual structure/style
```

RESEARCH CONTEXT:
- Why this approach?
- What alternatives considered?
- Key discoveries?
- Links to relevant docs/examples?
```

## Anti-Patterns

### ❌ Over-Documenting Simple Work
```markdown
Title: Fix typo in README
Notes: IMPLEMENTATION GUIDE
WORKING CODE: Open README.md, change "teh" to "the"...
```
**Problem:** Wastes tokens on obvious work.

### ❌ Design Details in Acceptance Criteria
```markdown
Acceptance:
- [ ] Use batchUpdate approach
- [ ] Call API with fields parameter
- [ ] Format as markdown with ## headers
```
**Problem:** Locks implementation. Should be in notes, not acceptance criteria.

### ❌ Raw JSON Dumps
```markdown
API RESPONSE:
{giant unformatted JSON blob spanning 100 lines}
```
**Problem:** Hard to read. Extract relevant parts, show structure.

### ✅ Right Balance
```markdown
API RESPONSE SAMPLE:
Returns dict with 49 entries. Example entries:
- 'text/markdown': ['application/vnd.google-apps.document']
- 'text/plain': ['application/vnd.google-apps.document']
- 'application/pdf': ['application/vnd.google-apps.document']
```

## When to Add This Detail

**During issue creation:**
- Already have working code from research? Include it.
- Clear output format in mind? Show example.

**During work (update notes):**
- Just got API query working? Add to notes.
- Discovered important context? Document it.
- Made key decision? Explain rationale.

**Session end:**
- If resuming will be hard, add implementation guide.
- If obvious, skip it.

**The principle:** Help your future self (or next Claude) resume without rediscovering everything.

## Notes Format: Current State, Not Cumulative

Arc issue notes should represent **current state**, not a cumulative log.

### ✅ Good (Current State)
```
COMPLETED: Parsed markdown into structured format
IN PROGRESS: Implementing Docs API insertion
NEXT: Debug batchUpdate call - getting 400 error on formatting
BLOCKER: None
KEY DECISION: Using two-phase approach (insert text, then apply formatting)
```

### ❌ Bad (Cumulative Log)
```
2024-01-15: Started work
2024-01-15: Got markdown parsing done
2024-01-16: Started API work
2024-01-16: Hit error
...
```

**Why current state wins:**
- Faster to parse on resume
- Clear immediate next step
- No scrolling through history
- Smaller, more focused

**Update pattern**: Overwrite notes at session end with current state, not append.

## Compaction Survival Checklist

Before a session ends (especially if compaction might occur):

```
- [ ] Is the issue status current? (open/in_progress/blocked)
- [ ] Are notes written for someone with zero context?
- [ ] Is the next step clearly stated?
- [ ] Are key decisions documented with rationale?
- [ ] If blocked, is the blocker clearly identified?
- [ ] If working code exists, is it captured?
```

**Test**: Could a fresh Claude instance pick this up and continue productive work?

## Common Resumability Questions

**Q: How detailed should notes be?**
A: Detailed enough for a stranger to continue. Simple tasks need less; complex technical work needs more.

**Q: Should I update notes after every action?**
A: No. Update at logical stopping points: end of session, major milestone, before potential compaction.

**Q: What if the issue evolves significantly?**
A: Overwrite notes with current state. Previous state doesn't help resumption.

**Q: Should code samples be complete or abbreviated?**
A: Complete enough to be runnable. Include imports and setup. Abbreviate obvious boilerplate.

**Q: What about sensitive information?**
A: Don't store credentials or secrets. Reference config files or environment variables instead.

## The Two-Week Test

For any issue, ask: **"Could I resume this work after 2 weeks away?"**

If no:
- Add implementation guide to notes
- Document key decisions
- Capture working code
- Show expected output format

If yes:
- Keep notes minimal
- Focus on current status and next step

This test helps calibrate how much detail to include without over-documenting simple work.
