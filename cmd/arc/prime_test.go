package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseHookInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"valid UUID", `{"session_id":"983a7cf7-bcb6-48fc-b485-129b4f1aaa45"}`, "983a7cf7-bcb6-48fc-b485-129b4f1aaa45"},
		{"invalid JSON", `not json`, ""},
		{"missing session_id", `{"cwd":"/tmp"}`, ""},
		{"invalid UUID format", `{"session_id":"not-a-uuid"}`, ""},
		{"empty input", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseHookInput(strings.NewReader(tt.input))
			if got != tt.want {
				t.Errorf("parseHookInput() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPersistSessionID(t *testing.T) {
	t.Run("writes to new file", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "env.sh")
		t.Setenv("CLAUDE_ENV_FILE", f)
		persistSessionID("abc-123")
		got, _ := os.ReadFile(f)
		if !strings.Contains(string(got), "export ARC_SESSION_ID=abc-123") {
			t.Errorf("expected ARC_SESSION_ID in file, got: %s", got)
		}
	})

	t.Run("replaces existing line", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "env.sh")
		if err := os.WriteFile(f, []byte("export ARC_SESSION_ID=old-id\nexport OTHER=val\n"), 0o600); err != nil {
			t.Fatalf("write test file: %v", err)
		}
		t.Setenv("CLAUDE_ENV_FILE", f)
		persistSessionID("new-id")
		got, _ := os.ReadFile(f)
		content := string(got)
		if strings.Contains(content, "old-id") {
			t.Error("old session ID should have been replaced")
		}
		if !strings.Contains(content, "export ARC_SESSION_ID=new-id") {
			t.Error("new session ID should be present")
		}
		if !strings.Contains(content, "export OTHER=val") {
			t.Error("other env vars should be preserved")
		}
	})
}
