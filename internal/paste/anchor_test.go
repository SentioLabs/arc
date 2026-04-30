package paste_test

import (
	"strings"
	"testing"

	"github.com/sentiolabs/arc/internal/paste"
)

const samplePlan = "# Title\n\nFirst paragraph.\nSecond paragraph.\n## Sub\nThird.\n"

func TestResolveAnchor_Ok(t *testing.T) {
	r := paste.ResolveAnchor(samplePlan, paste.Anchor{
		LineStart:  3,
		LineEnd:    3,
		QuotedText: "First paragraph.",
	})
	if r.Status != "ok" {
		t.Errorf("status = %q, want ok", r.Status)
	}
	if r.LineStart != 3 || r.LineEnd != 3 {
		t.Errorf("line range = (%d, %d), want (3, 3)", r.LineStart, r.LineEnd)
	}
}

func TestResolveAnchor_DriftedViaHeading(t *testing.T) {
	// Insert a prelude — the same paragraph now lives at line 5, not 3.
	// The heading_slug fallback must relocate it.
	edited := "PRELUDE\n# Title\n\nMore content.\nFirst paragraph.\n## Sub\nThird.\n"
	r := paste.ResolveAnchor(edited, paste.Anchor{
		LineStart:   3,
		LineEnd:     3,
		QuotedText:  "First paragraph.",
		HeadingSlug: "title",
	})
	if r.Status != paste.AnchorStatusDrifted {
		t.Errorf("status = %q, want drifted", r.Status)
	}
	if r.LineStart != 5 {
		t.Errorf("line_start = %d, want 5", r.LineStart)
	}
}

func TestResolveAnchor_DriftedViaContext(t *testing.T) {
	// Heading was renamed (slug doesn't match) but the surrounding context
	// is intact, so the fuzzy fallback should still find the location.
	edited := "# A different title\n\nFirst paragraph.\nSecond paragraph.\n"
	r := paste.ResolveAnchor(edited, paste.Anchor{
		LineStart:     5,
		LineEnd:       5,
		QuotedText:    "First paragraph.",
		HeadingSlug:   "old-slug",
		ContextBefore: "\n\n",
		ContextAfter:  "\nSecond",
	})
	if r.Status != paste.AnchorStatusDrifted {
		t.Errorf("status = %q, want drifted via fuzzy match", r.Status)
	}
}

func TestResolveAnchor_Orphaned(t *testing.T) {
	edited := "# Title\n\nDifferent stuff.\n"
	r := paste.ResolveAnchor(edited, paste.Anchor{
		LineStart:   3,
		LineEnd:     3,
		QuotedText:  "First paragraph.",
		HeadingSlug: "title",
	})
	if r.Status != "orphaned" {
		t.Errorf("status = %q, want orphaned", r.Status)
	}
	// Original coordinates preserved so callers can still display them.
	if r.LineStart != 3 || r.LineEnd != 3 {
		t.Errorf("orphaned should preserve original coords; got (%d, %d)", r.LineStart, r.LineEnd)
	}
}

func TestResolveAnchor_OutOfRangeFallsThrough(t *testing.T) {
	// Anchor refers to a line beyond the plan — must NOT panic, must fall
	// through to subsequent steps.
	short := "# Title\n\nOnly one paragraph.\n"
	r := paste.ResolveAnchor(short, paste.Anchor{
		LineStart:   500,
		LineEnd:     500,
		QuotedText:  "Only one paragraph.",
		HeadingSlug: "title",
	})
	if r.Status != paste.AnchorStatusDrifted {
		t.Errorf("status = %q, want drifted (fell through to heading match)", r.Status)
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Title":          "title",
		"Hello World":    "hello-world",
		"Multi  Spaces":  "multi-spaces",
		"With-Dashes":    "with-dashes",
		"Punctuation!?.": "punctuation",
		"Mixed Case 123": "mixed-case-123",
		"  Trim Me  ":    "trim-me",
		// Non-ASCII letters are stripped char-by-char (the regex is `[^a-z0-9\s-]`),
		// leaving only the ASCII letters embedded in the words. Matches what the
		// TS implementation produces.
		"Über Größe": "ber-gre",
	}
	for in, want := range cases {
		if got := paste.Slugify(in); got != want {
			t.Errorf("paste.Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSlugify_TSCompatibility(t *testing.T) {
	// Crucially this MUST match what web/src/lib/paste/anchor.ts produces,
	// since the SPA writes heading_slug values into the encrypted anchor.
	// If these diverge, drifted/heading-scoped resolution silently fails.
	if got := paste.Slugify("Goal"); got != "goal" {
		t.Errorf(`paste.Slugify("Goal") = %q, want "goal"`, got)
	}
	if got := paste.Slugify("Approach"); got != "approach" {
		t.Errorf(`paste.Slugify("Approach") = %q, want "approach"`, got)
	}
}

func TestSnippet(t *testing.T) {
	r := paste.AnchorResolution{LineStart: 3, LineEnd: 3, Status: "ok"}
	got := paste.Snippet(samplePlan, r)
	// Should include lines 1-5 (line 3 ± 2 padding).
	if !strings.Contains(got, "First paragraph.") {
		t.Errorf("snippet missing the anchor line; got: %q", got)
	}
	if !strings.Contains(got, "# Title") {
		t.Errorf("snippet missing leading context; got: %q", got)
	}
}

func TestSnippet_OrphanedReturnsEmpty(t *testing.T) {
	r := paste.AnchorResolution{LineStart: 99, LineEnd: 99, Status: "orphaned"}
	if got := paste.Snippet(samplePlan, r); got != "" {
		t.Errorf("orphaned should yield empty snippet; got %q", got)
	}
}
