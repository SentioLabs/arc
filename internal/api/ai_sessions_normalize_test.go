package api_test

import (
	"encoding/json"
	"testing"

	"github.com/sentiolabs/arc/internal/api"
)

func TestNormalizeTranscriptEntries(t *testing.T) {
	tests := []struct {
		name      string
		input     []string // raw JSONL entries
		wantCount int
		wantRoles []string // expected role values in order
		checkNil  []int    // indices where content should be non-nil
	}{
		{
			name: "promotes user message fields to top level",
			input: []string{
				`{"type":"user","message":{"role":"user","content":"hello world"},` +
					`"timestamp":"2026-03-24T00:00:00Z"}`,
			},
			wantCount: 1,
			wantRoles: []string{"user"},
			checkNil:  []int{0},
		},
		{
			name: "promotes assistant message fields to top level",
			input: []string{
				`{"type":"assistant","message":{"role":"assistant","type":"message",` +
					`"content":[{"type":"text","text":"I will help"}]},` +
					`"timestamp":"2026-03-24T00:00:01Z"}`,
			},
			wantCount: 1,
			wantRoles: []string{"assistant"},
			checkNil:  []int{0},
		},
		{
			name: "filters progress entries without message field",
			input: []string{
				`{"type":"progress","data":{"type":"hook_progress","hookEvent":"PreToolUse"},` +
					`"toolUseID":"toolu_123"}`,
			},
			wantCount: 0,
		},
		{
			name: "filters progress entries with empty message field",
			input: []string{
				`{"type":"progress","message":{},"data":{"type":"hook_progress",` +
					`"hookEvent":"PostToolUse"}}`,
			},
			wantCount: 0,
		},
		{
			name: "preserves type and timestamp in flattened output",
			input: []string{
				`{"type":"user","message":{"role":"user","content":"test"},` +
					`"timestamp":"2026-03-24T12:00:00Z"}`,
			},
			wantCount: 1,
			wantRoles: []string{"user"},
		},
		{
			name: "mixed conversation filters progress and promotes messages",
			input: []string{
				`{"type":"user","message":{"role":"user","content":"do something"}}`,
				`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"sure"}]}}`,
				`{"type":"progress","data":{"type":"hook_progress"}}`,
				`{"type":"progress","message":{},"data":{"type":"hook_progress"}}`,
				`{"type":"user","message":{"role":"user","content":"thanks"}}`,
			},
			wantCount: 3,
			wantRoles: []string{"user", "assistant", "user"},
		},
		{
			name: "passes through entries without message field (non-progress)",
			input: []string{
				`{"type":"unknown","data":"something"}`,
			},
			wantCount: 1,
		},
		{
			name: "handles malformed JSON gracefully",
			input: []string{
				`not valid json`,
			},
			wantCount: 1, // passed through as-is
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := make([]json.RawMessage, len(tt.input))
			for i, s := range tt.input {
				raw[i] = json.RawMessage(s)
			}

			result := api.NormalizeTranscriptEntries(raw)

			if len(result) != tt.wantCount {
				t.Fatalf("got %d entries, want %d", len(result), tt.wantCount)
			}

			assertRoles(t, result, tt.wantRoles)
			assertContentPresent(t, result, tt.checkNil)
		})
	}
}

func assertRoles(t *testing.T, result []json.RawMessage, wantRoles []string) {
	t.Helper()
	for i, wantRole := range wantRoles {
		if i >= len(result) {
			break
		}
		entry := unmarshalEntry(t, i, result[i])
		gotRole, _ := entry["role"].(string)
		if gotRole != wantRole {
			t.Errorf("entry %d: role = %q, want %q", i, gotRole, wantRole)
		}
		if _, hasMessage := entry["message"]; hasMessage {
			t.Errorf("entry %d: still has nested 'message' field after normalization", i)
		}
	}
}

func assertContentPresent(t *testing.T, result []json.RawMessage, indices []int) {
	t.Helper()
	for _, idx := range indices {
		if idx >= len(result) {
			continue
		}
		entry := unmarshalEntry(t, idx, result[idx])
		if _, hasContent := entry["content"]; !hasContent {
			t.Errorf("entry %d: expected content field to be present", idx)
		}
	}
}

func unmarshalEntry(t *testing.T, idx int, raw json.RawMessage) map[string]any {
	t.Helper()
	var entry map[string]any
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatalf("entry %d: failed to unmarshal: %v", idx, err)
	}
	return entry
}

func TestNormalizePreservesTimestamp(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(
			`{"type":"user","message":{"role":"user","content":"test"},` +
				`"timestamp":"2026-03-24T12:00:00Z"}`,
		),
	}

	result := api.NormalizeTranscriptEntries(raw)
	if len(result) != 1 {
		t.Fatalf("got %d entries, want 1", len(result))
	}

	var entry map[string]any
	if err := json.Unmarshal(result[0], &entry); err != nil {
		t.Fatal(err)
	}

	ts, ok := entry["timestamp"].(string)
	if !ok || ts != "2026-03-24T12:00:00Z" {
		t.Errorf("timestamp = %v, want 2026-03-24T12:00:00Z", entry["timestamp"])
	}
}
