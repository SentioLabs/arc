# Custom Prefix Override for `arc init`

## Problem

When running `arc init` in a directory like `cortex-shell`, the auto-generated prefix truncates the basename to 5 alphanumeric characters: `corte-0b7w`. This is often unreadable and doesn't convey the project name well. Users want to:

1. Override the basename portion with a custom short name (e.g., `cxsh`)
2. Have longer default basenames (10 chars instead of 5) for better readability

## Design

### New `--prefix` / `-p` flag on `arc init`

```bash
arc init --prefix cxsh        # produces prefix: cxsh-0b7w
arc init -p cxsh              # same thing
arc init                      # auto-generates with 10-char basename (existing behavior, longer)
```

The flag value is silently normalized (lowercased, non-alphanumeric chars stripped) and truncated to 10 characters. The 4-char base36 hash suffix (derived from the full directory path) is always appended for uniqueness.

### Prefix format change

```
Before:  corte-0b7w    (5 basename + 1 hyphen + 4 hash = 10 max)
After:   cortexshel-0b7w  (10 basename + 1 hyphen + 4 hash = 15 max)
```

### Changes

| File | Change |
|------|--------|
| `internal/project/naming.go` | Increase basename truncation from 5 → 10 in `GeneratePrefix()` and `GeneratePrefixFromName()`. Add `GeneratePrefixWithCustomName(dirPath, customName)` that uses the custom name as basename with the path-derived hash. |
| `internal/types/types.go` | Update `Validate()` prefix max length from 10 → 15 |
| `cmd/arc/init.go` | Add `--prefix` / `-p` string flag. Route to `GeneratePrefixWithCustomName()` when provided. |
| `internal/project/naming_test.go` | Update truncation and max-length expectations. Add tests for custom name function. |
| `internal/types/types_test.go` | Update validation tests for new 15-char limit. |

### What doesn't change

- 4-char base36 hash suffix (still path-derived, deterministic)
- Issue ID format (`prefix.hash`)
- Web UI (sidebar uses first char of prefix)
- API contract (prefix is still a string field)
- Existing workspaces (shorter prefixes remain valid)

### Normalization rules

Custom prefix values go through `normalizeForPrefix()`:
- Lowercased
- Non-alphanumeric characters stripped
- Empty result falls back to "ws"
- Truncated to 10 characters
