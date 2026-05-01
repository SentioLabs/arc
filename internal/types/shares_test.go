package types_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sentiolabs/arc/internal/types"
)

// --- Contract assertions ---
// Verify Share JSON tag stability — wire format must stay backward-compatible
// with the legacy shares.json field names so the one-shot import is 1:1.
func TestShareJSONTags(t *testing.T) {
	b, _ := json.Marshal(types.Share{
		ID: "x", Kind: types.ShareKindLocal, URL: "u",
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
		kind types.ShareKind
		want bool
	}{
		{types.ShareKindLocal, true},
		{types.ShareKindShared, true},
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
	valid := types.Share{ID: "id", Kind: types.ShareKindLocal, URL: "u", KeyB64Url: "k", EditToken: "t"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid share: unexpected error: %v", err)
	}
	cases := map[string]types.Share{
		"missing id":         {Kind: types.ShareKindLocal, URL: "u", KeyB64Url: "k", EditToken: "t"},
		"invalid kind":       {ID: "id", Kind: "x", URL: "u", KeyB64Url: "k", EditToken: "t"},
		"missing url":        {ID: "id", Kind: types.ShareKindLocal, KeyB64Url: "k", EditToken: "t"},
		"missing key_b64url": {ID: "id", Kind: types.ShareKindLocal, URL: "u", EditToken: "t"},
		"missing edit_token": {ID: "id", Kind: types.ShareKindLocal, URL: "u", KeyB64Url: "k"},
	}
	for name, s := range cases {
		if err := s.Validate(); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}

func TestAllShareKinds(t *testing.T) {
	kinds := types.AllShareKinds()
	if len(kinds) != 2 {
		t.Fatalf("expected 2 kinds, got %d", len(kinds))
	}
	found := map[types.ShareKind]bool{}
	for _, k := range kinds {
		found[k] = true
	}
	for _, want := range []types.ShareKind{types.ShareKindLocal, types.ShareKindShared} {
		if !found[want] {
			t.Errorf("AllShareKinds missing %q", want)
		}
	}
}
