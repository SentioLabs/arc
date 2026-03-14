# Custom Prefix Override Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `--prefix` flag to `arc init` so users can override the issue prefix basename, and increase the default basename length from 5 to 10 characters.

**Architecture:** The change flows through three layers: CLI flag parsing (`cmd/arc/init.go`), prefix generation logic (`internal/project/naming.go`), and validation (`internal/types/types.go`). A new `GeneratePrefixWithCustomName` function handles the custom basename case while reusing the existing path-based hash suffix for uniqueness.

**Tech Stack:** Go, Cobra CLI, SHA-256 hashing, base36 encoding

---

### Task 1: Increase prefix max length validation from 10 to 15

**Files:**
- Modify: `internal/types/types.go:32-33`
- Modify: `internal/types/types_test.go:493-500`

**Step 1: Update the failing test expectation**

In `internal/types/types_test.go`, update the "prefix too long" test case:

```go
{
    name: "prefix too long",
    ws: Project{
        Name:   "Test",
        Prefix: "thisprefixtoolong",
    },
    wantErr: true,
    errMsg:  "project prefix must be 15 characters or less",
},
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/types/ -run TestProjectValidate -v`
Expected: FAIL — error message still says "10 characters or less"

**Step 3: Update validation in types.go**

In `internal/types/types.go`, change the prefix length check:

```go
if len(w.Prefix) > 15 {
    return fmt.Errorf("project prefix must be 15 characters or less")
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/types/ -run TestProjectValidate -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/types/types.go internal/types/types_test.go
git commit -m "feat: increase workspace prefix max length from 10 to 15"
```

---

### Task 2: Increase default basename truncation from 5 to 10

**Files:**
- Modify: `internal/project/naming.go:126-130` (`GeneratePrefix`)
- Modify: `internal/project/naming.go:153-156` (`GeneratePrefixFromName`)
- Modify: `internal/project/naming_test.go`

**Step 1: Update test expectations for new truncation length**

In `internal/project/naming_test.go`, update `TestGeneratePrefixTruncation`:

```go
func TestGeneratePrefixTruncation(t *testing.T) {
	// Long basename should be truncated to 10 alphanumeric chars before hash
	// "my-very-long-project-name" -> "myverylongprojectname" -> "myverylongp" -> wait, 10 chars = "myverylong"
	prefix, err := GeneratePrefix("/tmp/my-very-long-project-name")
	if err != nil {
		t.Fatalf("GeneratePrefix failed: %v", err)
	}

	// Format: xxxxxxxxxx-yyyy (10 basename + 1 dash + 4 hash = 15 chars max)
	if len(prefix) > 15 {
		t.Errorf("Prefix should be max 15 chars, got %q (len %d)", prefix, len(prefix))
	}

	// Should start with truncated alphanumeric basename "myverylong-"
	if prefix[:11] != "myverylong-" {
		t.Errorf("Expected prefix to start with 'myverylong-', got %q", prefix)
	}
}
```

Update `TestGeneratePrefixNormalization` — the expected prefixes for truncated names change:

```go
{"hyphens removed", "/tmp/test-id-format", "testidform"},
{"underscores removed", "/tmp/my_cool_project", "mycoolproj"},
{"spaces removed", "/tmp/my project", "myproject"},
{"special chars removed", "/tmp/I was_here#yesterday!", "iwashereye"},
{"already clean", "/tmp/myapi", "myapi"},
{"short name", "/tmp/api", "api"},
```

Update `TestGeneratePrefixFromName`:

```go
func TestGeneratePrefixFromName(t *testing.T) {
	// "my-project" normalizes to "myproject", no truncation needed (9 chars < 10)
	prefix := GeneratePrefixFromName("my-project")

	// ... existing hash suffix checks ...

	// Test that basename portion is correct
	basename := prefix[:lastHyphen]
	if basename != "myproject" {
		t.Errorf("Expected basename 'myproject', got %q", basename)
	}

	// Test max length: 10 basename + 1 dash + 4 hash = 15 chars
	if len(prefix) > 15 {
		t.Errorf("Prefix should be max 15 chars, got %q (len %d)", prefix, len(prefix))
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/project/ -run "TestGeneratePrefix" -v`
Expected: FAIL — truncation is still at 5

**Step 3: Update truncation in GeneratePrefix and GeneratePrefixFromName**

In `internal/project/naming.go`, in `GeneratePrefix()` change:

```go
// Max 10 chars to fit in 15-char limit: 10 basename + 1 hyphen + 4 suffix = 15
basename := normalizeForPrefix(filepath.Base(evalPath))
if len(basename) > 10 {
    basename = basename[:10]
}
```

In `GeneratePrefixFromName()` change:

```go
// Max 10 chars to fit in 15-char limit: 10 basename + 1 hyphen + 4 suffix = 15
normalized := normalizeForPrefix(name)
if len(normalized) > 10 {
    normalized = normalized[:10]
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/project/ -run "TestGeneratePrefix" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/project/naming.go internal/project/naming_test.go
git commit -m "feat: increase prefix basename length from 5 to 10 characters"
```

---

### Task 3: Add GeneratePrefixWithCustomName function

**Files:**
- Modify: `internal/project/naming.go` (add new function after `GeneratePrefix`)
- Modify: `internal/project/naming_test.go` (add new test)

**Step 1: Write the failing test**

Add to `internal/project/naming_test.go`:

```go
func TestGeneratePrefixWithCustomName(t *testing.T) {
	// Custom name should be normalized and used as basename
	prefix, err := GeneratePrefixWithCustomName("/tmp/cortex-shell", "cxsh")
	if err != nil {
		t.Fatalf("GeneratePrefixWithCustomName failed: %v", err)
	}

	// Should start with custom basename
	lastHyphen := strings.LastIndex(prefix, "-")
	if lastHyphen == -1 {
		t.Fatalf("Prefix should contain a hyphen, got %q", prefix)
	}
	basename := prefix[:lastHyphen]
	if basename != "cxsh" {
		t.Errorf("Expected basename 'cxsh', got %q (full prefix: %q)", basename, prefix)
	}

	// Hash suffix should be 4 chars
	suffix := prefix[lastHyphen+1:]
	if len(suffix) != 4 {
		t.Errorf("Expected 4-char hash suffix, got %q (len %d)", suffix, len(suffix))
	}

	// Should be deterministic (same path = same hash)
	prefix2, err := GeneratePrefixWithCustomName("/tmp/cortex-shell", "cxsh")
	if err != nil {
		t.Fatalf("GeneratePrefixWithCustomName failed: %v", err)
	}
	if prefix != prefix2 {
		t.Errorf("Should be deterministic: %q != %q", prefix, prefix2)
	}

	// Different paths with same custom name should produce different prefixes
	prefix3, err := GeneratePrefixWithCustomName("/tmp/other-project", "cxsh")
	if err != nil {
		t.Fatalf("GeneratePrefixWithCustomName failed: %v", err)
	}
	if prefix == prefix3 {
		t.Errorf("Different paths should produce different prefixes: %q == %q", prefix, prefix3)
	}

	// Custom name should be normalized (special chars stripped)
	prefix4, err := GeneratePrefixWithCustomName("/tmp/test", "cx-sh!")
	if err != nil {
		t.Fatalf("GeneratePrefixWithCustomName failed: %v", err)
	}
	basename4 := prefix4[:strings.LastIndex(prefix4, "-")]
	if basename4 != "cxsh" {
		t.Errorf("Expected normalized basename 'cxsh', got %q", basename4)
	}

	// Long custom name should be truncated to 10
	prefix5, err := GeneratePrefixWithCustomName("/tmp/test", "verylongcustomname")
	if err != nil {
		t.Fatalf("GeneratePrefixWithCustomName failed: %v", err)
	}
	if len(prefix5) > 15 {
		t.Errorf("Prefix should be max 15 chars, got %q (len %d)", prefix5, len(prefix5))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/project/ -run TestGeneratePrefixWithCustomName -v`
Expected: FAIL — function does not exist

**Step 3: Implement GeneratePrefixWithCustomName**

Add to `internal/project/naming.go` after `GeneratePrefix`:

```go
// GeneratePrefixWithCustomName creates an issue prefix using a user-provided basename
// combined with a path-derived hash suffix for uniqueness.
// The custom name is normalized (lowercased, non-alphanumeric stripped) and truncated to 10 chars.
// Format: {normalized-custom-name}-{4-char-base36-hash}
func GeneratePrefixWithCustomName(dirPath, customName string) (string, error) {
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return "", err
	}

	evalPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		evalPath = absPath
	}

	normalized := filepath.ToSlash(evalPath)

	// Normalize and truncate custom name
	basename := normalizeForPrefix(customName)
	if len(basename) > 10 {
		basename = basename[:10]
	}

	// Generate deterministic hash from full path using base36
	hash := sha256.Sum256([]byte(normalized))
	suffix := Base36Encode(hash[:2])

	for len(suffix) < 4 {
		suffix = "0" + suffix
	}
	if len(suffix) > 4 {
		suffix = suffix[:4]
	}

	return basename + "-" + suffix, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/project/ -run TestGeneratePrefixWithCustomName -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/project/naming.go internal/project/naming_test.go
git commit -m "feat: add GeneratePrefixWithCustomName for custom prefix support"
```

---

### Task 4: Add --prefix flag to arc init command

**Files:**
- Modify: `cmd/arc/init.go:41-43` (add flag)
- Modify: `cmd/arc/init.go:46-74` (use flag in runInit)

**Step 1: Add the flag registration**

In `cmd/arc/init.go`, in the `init()` function, add:

```go
func init() {
	initCmd.Flags().StringP("description", "d", "", "Workspace description")
	initCmd.Flags().StringP("prefix", "p", "", "Custom issue prefix (alphanumeric, max 10 chars)")
	initCmd.Flags().BoolP("quiet", "q", false, "Suppress output")
	rootCmd.AddCommand(initCmd)
}
```

**Step 2: Update runInit to use the flag**

In `cmd/arc/init.go`, in `runInit`, after getting `cwd`, add the custom prefix flag reading and update the prefix generation:

```go
customPrefix, _ := cmd.Flags().GetString("prefix")

// Generate prefix with hash for guaranteed uniqueness
var prefix string
if customPrefix != "" {
    prefix, err = project.GeneratePrefixWithCustomName(cwd, customPrefix)
    if err != nil {
        return fmt.Errorf("generate prefix: %w", err)
    }
} else {
    prefix, err = project.GeneratePrefix(cwd)
    if err != nil {
        return fmt.Errorf("generate prefix: %w", err)
    }
}
```

**Step 3: Update the command's Long description**

Update the `Long` field and examples to mention `--prefix`:

```go
Long: `Initialize arc in the current directory by creating a workspace.

This command:
1. Creates a workspace on the server (or connects to existing)
2. Sets the workspace as default for this directory
3. Creates .arc.json with workspace configuration
4. Creates AGENTS.md with session completion instructions

For Claude Code users: Install the arc plugin for full integration
(hooks, skills, agents). The plugin's onboard skill will handle
workspace initialization automatically.

For Codex CLI users: Run arc setup codex to install the repo-scoped
arc skill bundle under .codex/skills.

Examples:
  arc init                    # Use directory name as workspace
  arc init my-project         # Use custom name
  arc init --prefix cxsh      # Custom issue prefix (e.g., cxsh-0b7w)`,
```

**Step 4: Build and verify**

Run: `make build-quick && ./bin/arc init --help`
Expected: Output shows `--prefix` / `-p` flag in help text

**Step 5: Commit**

```bash
git add cmd/arc/init.go
git commit -m "feat: add --prefix flag to arc init for custom issue prefixes"
```

---

### Task 5: Update arc docs for init command

**Files:**
- Modify: `claude-plugin/commands/init.md`

**Step 1: Update the docs**

Replace `claude-plugin/commands/init.md` with:

```markdown
---
description: Initialize arc in the current project
argument-hint: [workspace-name]
---

Initialize arc in the current directory.

```bash
arc init                        # Use directory name as workspace
arc init my-project             # Custom workspace name
arc init --prefix cxsh          # Custom issue prefix (e.g., cxsh-0b7w)
arc init my-project -p cxsh     # Both custom name and prefix
```

This command:
1. Creates a workspace on the arc server
2. Saves workspace config to `.arc.json`
3. Creates AGENTS.md with workflow instructions
4. Sets up Claude Code hooks (unless --skip-claude)

**Flags:**
- `--prefix`, `-p`: Custom issue prefix basename (alphanumeric, max 10 chars). Gets normalized (lowercased, special chars stripped) and combined with a hash suffix for uniqueness.
- `--description`, `-d`: Workspace description
- `--quiet`, `-q`: Suppress output

**Prerequisites:**
- Arc server must be running (`arc server start`)
```

**Step 2: Commit**

```bash
git add claude-plugin/commands/init.md
git commit -m "docs: update arc init docs with --prefix flag"
```

---

### Task 6: Final integration test

**Step 1: Run full test suite**

Run: `make test`
Expected: All tests pass

**Step 2: Manual smoke test**

Run:
```bash
make build-quick
cd /tmp && mkdir prefix-test && cd prefix-test
arc init --prefix mytest
```

Expected output includes:
```
Prefix: mytest-xxxx
Issues will be named: mytest-xxxx.<hash>
```

**Step 3: Commit any remaining fixes if needed**
