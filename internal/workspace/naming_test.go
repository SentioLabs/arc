package workspace //nolint:testpackage // tests verify internal normalization helpers

import (
	"strings"
	"testing"
)

func TestSanitizeBasename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple lowercase", "myproject", "myproject"},
		{"uppercase to lowercase", "MyProject", "myproject"},
		{"spaces to hyphens", "my project", "my-project"},
		{"underscores to hyphens", "my_project", "my-project"},
		{"special characters removed", "my@project!", "myproject"},
		{"mixed special chars", "My Cool_Project!", "my-cool-project"},
		{"multiple hyphens collapsed", "my--project", "my-project"},
		{"leading/trailing hyphens trimmed", "-my-project-", "my-project"},
		{"long name truncated", "this-is-a-very-long-project-name", "this-is-a-very-long-"},
		{"empty string fallback", "", "workspace"},
		{"only special chars fallback", "!!!@@@", "workspace"},
		{"numbers preserved", "project123", "project123"},
		{"hyphens preserved", "my-cool-project", "my-cool-project"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := SanitizeBasename(tc.input)
			if result != tc.expected {
				t.Errorf("SanitizeBasename(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestGenerateName(t *testing.T) {
	// Test that the same path produces the same hash (deterministic)
	name1, err := GenerateName("/tmp/test-project")
	if err != nil {
		t.Fatalf("GenerateName failed: %v", err)
	}

	name2, err := GenerateName("/tmp/test-project")
	if err != nil {
		t.Fatalf("GenerateName failed: %v", err)
	}

	if name1 != name2 {
		t.Errorf("GenerateName should be deterministic: %q != %q", name1, name2)
	}

	// Test format: basename-xxxxxx (6 hex chars)
	if len(name1) < 7 || name1[len(name1)-7] != '-' {
		t.Errorf("GenerateName should produce format 'basename-xxxxxx', got %q", name1)
	}

	// Test that different paths produce different hashes
	name3, err := GenerateName("/tmp/other-project")
	if err != nil {
		t.Fatalf("GenerateName failed: %v", err)
	}

	// The hash suffix should be different
	suffix1 := name1[len(name1)-6:]
	suffix3 := name3[len(name3)-6:]
	if suffix1 == suffix3 {
		t.Errorf("Different paths should produce different hashes: %q vs %q", name1, name3)
	}
}

func TestBase36Encode(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{"zero", []byte{0}, "0"},
		{"single byte", []byte{36}, "10"},        // 36 in base36 is "10"
		{"two bytes", []byte{0, 255}, "73"},      // 255 in base36
		{"three bytes", []byte{1, 0, 0}, "1ekg"}, // 65536 in base36
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := Base36Encode(tc.input)
			if result != tc.expected {
				t.Errorf("Base36Encode(%v) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestGenerateIssueID(t *testing.T) {
	// Test that IDs follow the format prefix.xxxxxx (6 base36 chars)
	id1 := GenerateIssueID("arc", "Test issue")

	if len(id1) < 10 || id1[3] != '.' {
		t.Errorf("GenerateIssueID should produce format 'arc.xxxxxx', got %q", id1)
	}

	// Verify the prefix
	if id1[:4] != "arc." {
		t.Errorf("Expected ID to start with 'arc.', got %q", id1)
	}

	// Verify the suffix is 6 characters (base36)
	suffix := id1[4:]
	if len(suffix) != 6 {
		t.Errorf("Expected 6-char base36 suffix, got %q (len %d)", suffix, len(suffix))
	}

	// Verify the suffix contains only base36 chars
	for _, c := range suffix {
		if (c < '0' || c > '9') && (c < 'a' || c > 'z') {
			t.Errorf("Suffix %q contains invalid base36 char: %c", suffix, c)
		}
	}
}

func TestGenerateNameSameBaseDifferentPath(t *testing.T) {
	// Two directories with the same basename but different paths
	name1, err := GenerateName("/home/user/projects/my-app")
	if err != nil {
		t.Fatalf("GenerateName failed: %v", err)
	}

	name2, err := GenerateName("/home/user/work/my-app")
	if err != nil {
		t.Fatalf("GenerateName failed: %v", err)
	}

	// Both should start with "my-app-"
	if name1[:7] != "my-app-" {
		t.Errorf("Expected name to start with 'my-app-', got %q", name1)
	}
	if name2[:7] != "my-app-" {
		t.Errorf("Expected name to start with 'my-app-', got %q", name2)
	}

	// But the full names should be different (different hash suffix)
	if name1 == name2 {
		t.Errorf("Same basename in different paths should produce different names: %q == %q", name1, name2)
	}
}

func TestGeneratePrefix(t *testing.T) {
	// Test determinism: same path produces same prefix
	prefix1, err := GeneratePrefix("/tmp/test-project")
	if err != nil {
		t.Fatalf("GeneratePrefix failed: %v", err)
	}

	prefix2, err := GeneratePrefix("/tmp/test-project")
	if err != nil {
		t.Fatalf("GeneratePrefix failed: %v", err)
	}

	if prefix1 != prefix2 {
		t.Errorf("GeneratePrefix should be deterministic: %q != %q", prefix1, prefix2)
	}

	// Test format: should be basename-xxxx (4-char base36 hash)
	// Find the last hyphen (before the hash suffix)
	lastHyphen := -1
	for i := len(prefix1) - 1; i >= 0; i-- {
		if prefix1[i] == '-' {
			lastHyphen = i
			break
		}
	}
	if lastHyphen == -1 {
		t.Errorf("GeneratePrefix should contain a hyphen, got %q", prefix1)
	} else {
		suffix := prefix1[lastHyphen+1:]
		if len(suffix) != 4 {
			t.Errorf("Expected 4-char hash suffix, got %q (len %d)", suffix, len(suffix))
		}
		// Verify suffix contains only base36 chars
		for _, c := range suffix {
			if (c < '0' || c > '9') && (c < 'a' || c > 'z') {
				t.Errorf("Suffix %q contains invalid base36 char: %c", suffix, c)
			}
		}
	}
}

func TestGeneratePrefixUniqueness(t *testing.T) {
	// Different paths should produce different prefixes
	prefix1, err := GeneratePrefix("/home/user/projects/api")
	if err != nil {
		t.Fatalf("GeneratePrefix failed: %v", err)
	}

	prefix2, err := GeneratePrefix("/home/user/work/api")
	if err != nil {
		t.Fatalf("GeneratePrefix failed: %v", err)
	}

	// Both should start with "api-" (same basename)
	if prefix1[:4] != "api-" {
		t.Errorf("Expected prefix to start with 'api-', got %q", prefix1)
	}
	if prefix2[:4] != "api-" {
		t.Errorf("Expected prefix to start with 'api-', got %q", prefix2)
	}

	// But the full prefixes should be different
	if prefix1 == prefix2 {
		t.Errorf("Same basename in different paths should produce different prefixes: %q == %q", prefix1, prefix2)
	}
}

func TestGeneratePrefixTruncation(t *testing.T) {
	// Long basename should be truncated to 10 alphanumeric chars before hash
	// "my-very-long-project-name" -> "myverylongprojectname" -> "myverylong"
	prefix, err := GeneratePrefix("/tmp/my-very-long-project-name")
	if err != nil {
		t.Fatalf("GeneratePrefix failed: %v", err)
	}

	// Format: xxxxxxxxxx-yyyy (10 basename + 1 dash + 4 hash = 15 chars max)
	if len(prefix) > MaxPrefixLength {
		t.Errorf("Prefix should be max %d chars, got %q (len %d)", MaxPrefixLength, prefix, len(prefix))
	}

	// Should start with truncated alphanumeric basename "myverylong-"
	if prefix[:11] != "myverylong-" {
		t.Errorf("Expected prefix to start with 'myverylong-', got %q", prefix)
	}
}

func TestGeneratePrefixNormalization(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		expectedPrefix string // Just the basename part before the hash
	}{
		{"hyphens removed", "/tmp/test-id-format", "testidform"},
		{"underscores removed", "/tmp/my_cool_project", "mycoolproj"},
		{"spaces removed", "/tmp/my project", "myproject"},
		{"special chars removed", "/tmp/I was_here#yesterday!", "iwashereye"},
		{"already clean", "/tmp/myapi", "myapi"},
		{"short name", "/tmp/api", "api"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prefix, err := GeneratePrefix(tc.path)
			if err != nil {
				t.Fatalf("GeneratePrefix failed: %v", err)
			}

			// Extract basename part (before the last hyphen)
			lastHyphen := strings.LastIndex(prefix, "-")
			if lastHyphen == -1 {
				t.Fatalf("Prefix should contain a hyphen, got %q", prefix)
			}
			basename := prefix[:lastHyphen]

			if basename != tc.expectedPrefix {
				t.Errorf("Expected basename %q, got %q (full prefix: %q)", tc.expectedPrefix, basename, prefix)
			}

			// Should never have double hyphens
			if strings.Contains(prefix, "--") {
				t.Errorf("Prefix should not contain double hyphens, got %q", prefix)
			}
		})
	}
}

func TestGeneratePlanID(t *testing.T) {
	// Test that IDs follow the format plan.xxxxxx (6 base36 chars)
	id1 := GeneratePlanID("Test plan")

	if len(id1) < 11 || id1[4] != '.' {
		t.Errorf("GeneratePlanID should produce format 'plan.xxxxxx', got %q", id1)
	}

	// Verify the prefix
	if id1[:5] != "plan." {
		t.Errorf("Expected ID to start with 'plan.', got %q", id1)
	}

	// Verify the suffix is 6 characters (base36)
	suffix := id1[5:]
	if len(suffix) != 6 {
		t.Errorf("Expected 6-char base36 suffix, got %q (len %d)", suffix, len(suffix))
	}

	// Verify the suffix contains only base36 chars
	for _, c := range suffix {
		if (c < '0' || c > '9') && (c < 'a' || c > 'z') {
			t.Errorf("Suffix %q contains invalid base36 char: %c", suffix, c)
		}
	}

	// Each call should produce a different ID (contains timestamp)
	id2 := GeneratePlanID("Test plan")
	if id1 == id2 {
		t.Errorf("GeneratePlanID should produce unique IDs, but got same: %q", id1)
	}
}

func TestGeneratePrefixFromName(t *testing.T) {
	// "my-project" normalizes to "myproject", no truncation needed (9 chars < 10)
	prefix := GeneratePrefixFromName("my-project")

	// Test format: should be basename-xxxx (4-char base36 hash)
	lastHyphen := strings.LastIndex(prefix, "-")
	if lastHyphen == -1 {
		t.Fatalf("GeneratePrefixFromName should contain a hyphen, got %q", prefix)
	}

	suffix := prefix[lastHyphen+1:]
	if len(suffix) != 4 {
		t.Errorf("Expected 4-char hash suffix, got %q (len %d)", suffix, len(suffix))
	}
	// Verify suffix contains only base36 chars
	for _, c := range suffix {
		if (c < '0' || c > '9') && (c < 'a' || c > 'z') {
			t.Errorf("Suffix %q contains invalid base36 char: %c", suffix, c)
		}
	}

	// Test that basename portion is correct
	basename := prefix[:lastHyphen]
	if basename != "myproject" {
		t.Errorf("Expected basename 'myproject', got %q", basename)
	}

	// Test max length: 10 basename + 1 dash + 4 hash = 15 chars
	if len(prefix) > MaxPrefixLength {
		t.Errorf("Prefix should be max %d chars, got %q (len %d)", MaxPrefixLength, prefix, len(prefix))
	}
}

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
	if len(suffix) != PrefixHashSuffixLength {
		t.Errorf("Expected %d-char hash suffix, got %q (len %d)", PrefixHashSuffixLength, suffix, len(suffix))
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

	// Long custom name should be truncated to MaxBasenameTruncation
	prefix5, err := GeneratePrefixWithCustomName("/tmp/test", "verylongcustomname")
	if err != nil {
		t.Fatalf("GeneratePrefixWithCustomName failed: %v", err)
	}
	if len(prefix5) > MaxPrefixLength {
		t.Errorf("Prefix should be max %d chars, got %q (len %d)", MaxPrefixLength, prefix5, len(prefix5))
	}
}
