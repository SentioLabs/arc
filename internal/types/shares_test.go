package types

import (
	"encoding/json"
	"strings"
	"testing"
)

// --- Contract assertions ---
// Verify Share JSON tag stability — wire format must stay backward-compatible
// with the legacy shares.json field names so the one-shot import is 1:1.
func TestShareJSONTags(t *testing.T) {
	b, _ := json.Marshal(Share{
		ID: "x", Kind: ShareKindLocal, URL: "u",
		KeyB64Url: "k", EditToken: "t", PlanFile: "p",
	})
	for _, want := range []string{
		`"id"`, `"kind"`, `"url"`, `"key_b64url"`,
		`"edit_token"`, `"plan_file"`, `"created_at"`,
	} {
		if !strings.Contains(string(b), want) {
			t.Errorf("missing JSON tag %s in %s", want, b)
		}
	}
}

func TestShareKindIsValid(t *testing.T) {
	cases := []struct {
		kind ShareKind
		want bool
	}{
		{ShareKindLocal, true},
		{ShareKindShared, true},
		{"", false},
		{"bogus", false},
	}
	for _, tc := range cases {
		if got := tc.kind.IsValid(); got != tc.want {
			t.Errorf("ShareKind(%q).IsValid() = %v, want %v", tc.kind, got, tc.want)
		}
	}
}

func TestShareValidate(t *testing.T) {
	valid := Share{ID: "id", Kind: ShareKindLocal, URL: "u", KeyB64Url: "k", EditToken: "t"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid share: unexpected error: %v", err)
	}
	cases := map[string]Share{
		"missing id":         {Kind: ShareKindLocal, URL: "u", KeyB64Url: "k", EditToken: "t"},
		"invalid kind":       {ID: "id", Kind: "x", URL: "u", KeyB64Url: "k", EditToken: "t"},
		"missing url":        {ID: "id", Kind: ShareKindLocal, KeyB64Url: "k", EditToken: "t"},
		"missing key_b64url": {ID: "id", Kind: ShareKindLocal, URL: "u", EditToken: "t"},
		"missing edit_token": {ID: "id", Kind: ShareKindLocal, URL: "u", KeyB64Url: "k"},
	}
	for name, s := range cases {
		if err := s.Validate(); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}
