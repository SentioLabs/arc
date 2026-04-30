// Anchor resolution for replaying reviewer annotations against a possibly-
// edited plan. Mirror of web/src/lib/paste/anchor.ts; the two sides MUST
// stay in sync since both consume the same encrypted Anchor payloads.
//
// 4-step fallback (same as the TS):
//  1. Exact: line_start..line_end still contain quoted_text → status=ok
//  2. Heading-scoped: search the 50 lines after heading_slug → status=drifted
//  3. Fuzzy: context_before + quoted_text + context_after appears in the plan
//     → status=drifted
//  4. None of the above: status=orphaned, original line numbers preserved
//     for display purposes.
package paste

import (
	"regexp"
	"strings"
)

// headingWindowLines bounds the heading-scoped fuzzy search at step 2 of
// ResolveAnchor — large enough to span typical sections, small enough that an
// unrelated re-occurrence later in the plan can't accidentally match.
const headingWindowLines = 50

// Anchor mirrors the TS Anchor type (web/src/lib/paste/types.ts). Encoded as
// JSON inside encrypted comment events; decoded here only when the CLI needs
// to relocate a comment against a current plan.
type Anchor struct {
	LineStart     int    `json:"line_start"`
	LineEnd       int    `json:"line_end"`
	CharStart     *int   `json:"char_start,omitempty"`
	CharEnd       *int   `json:"char_end,omitempty"`
	QuotedText    string `json:"quoted_text"`
	ContextBefore string `json:"context_before,omitempty"`
	ContextAfter  string `json:"context_after,omitempty"`
	HeadingSlug   string `json:"heading_slug,omitempty"`
}

// AnchorStatus values for AnchorResolution.Status. Mirrored on the SPA side;
// adding/renaming a status here breaks the cross-language contract.
const (
	// AnchorStatusOK means the original line numbers still contain the quoted
	// text — no relocation was needed.
	AnchorStatusOK = "ok"
	// AnchorStatusDrifted means the anchor was relocated via heading scope
	// or fuzzy context match; the new line numbers are best-effort.
	AnchorStatusDrifted = "drifted"
	// AnchorStatusOrphaned means we couldn't relocate the anchor at all; the
	// original line numbers are preserved for display only.
	AnchorStatusOrphaned = "orphaned"
)

// AnchorResolution is the result of running ResolveAnchor against a plan.
// Status disambiguates "found at original location" from "relocated" from
// "couldn't find at all" — agents care about this for deciding whether to
// trust the line numbers or fall back to a Grep on QuotedText.
type AnchorResolution struct {
	LineStart int    `json:"line_start"`
	LineEnd   int    `json:"line_end"`
	Status    string `json:"status"` // one of AnchorStatus* constants
}

// ResolveAnchor finds the current location of an anchor in plan markdown.
// Returns the original line numbers + status="orphaned" if every fallback
// fails — never returns an error.
func ResolveAnchor(plan string, a Anchor) AnchorResolution {
	lines := strings.Split(plan, "\n")

	// Step 1: exact location still holds.
	if a.LineStart >= 1 && a.LineEnd <= len(lines) && a.LineStart <= a.LineEnd {
		slice := strings.Join(lines[a.LineStart-1:a.LineEnd], "\n")
		if strings.Contains(slice, a.QuotedText) {
			return AnchorResolution{
				LineStart: a.LineStart,
				LineEnd:   a.LineEnd,
				Status:    AnchorStatusOK,
			}
		}
	}

	// Step 2: heading-scoped — look 50 lines past the matching heading.
	if a.HeadingSlug != "" {
		if hi := findHeadingIndex(lines, a.HeadingSlug); hi >= 0 {
			end := min(hi+headingWindowLines, len(lines))
			window := strings.Join(lines[hi:end], "\n")
			if off := strings.Index(window, a.QuotedText); off >= 0 {
				lineNum := hi + 1 + countNewlinesBefore(window, off)
				return AnchorResolution{
					LineStart: lineNum,
					LineEnd:   lineNum + countNewlinesBefore(a.QuotedText, len(a.QuotedText)),
					Status:    AnchorStatusDrifted,
				}
			}
		}
	}

	// Step 3: fuzzy — match the surrounding window verbatim.
	if a.ContextBefore != "" && a.ContextAfter != "" {
		needle := a.ContextBefore + a.QuotedText + a.ContextAfter
		if idx := strings.Index(plan, needle); idx >= 0 {
			startOff := idx + len(a.ContextBefore)
			lineNum := countNewlinesBefore(plan, startOff) + 1
			return AnchorResolution{
				LineStart: lineNum,
				LineEnd:   lineNum + countNewlinesBefore(a.QuotedText, len(a.QuotedText)),
				Status:    AnchorStatusDrifted,
			}
		}
	}

	// Step 4: orphaned. Preserve original coords so the UI can still display
	// "this comment used to be at line X" rather than rendering nothing.
	return AnchorResolution{
		LineStart: a.LineStart,
		LineEnd:   a.LineEnd,
		Status:    AnchorStatusOrphaned,
	}
}

// Snippet returns up to ~5 lines around the resolved anchor — handy for
// LLM consumers that want a small chunk of context without re-reading the
// whole plan. Returns "" if the resolution is orphaned (no reliable
// location to extract from).
func Snippet(plan string, r AnchorResolution) string {
	if r.Status == AnchorStatusOrphaned {
		return ""
	}
	lines := strings.Split(plan, "\n")
	const padding = 2
	start := max(r.LineStart-1-padding, 0)
	end := min(r.LineEnd+padding, len(lines))
	if start >= end {
		return ""
	}
	return strings.Join(lines[start:end], "\n")
}

func findHeadingIndex(lines []string, slug string) int {
	re := regexp.MustCompile(`^#+\s+(.*)$`)
	for i, line := range lines {
		m := re.FindStringSubmatch(line)
		if len(m) == 2 && Slugify(m[1]) == slug {
			return i
		}
	}
	return -1
}

// Slugify mirrors web/src/lib/paste/anchor.ts:slugify so heading_slug values
// produced by the SPA match what we recompute here. Lowercase ASCII letters
// + digits + hyphens; whitespace becomes '-'.
func Slugify(text string) string {
	lowered := strings.ToLower(text)
	// Replace anything that isn't [a-z0-9 -] with empty string.
	stripped := nonSlugChars.ReplaceAllString(lowered, "")
	trimmed := strings.TrimSpace(stripped)
	return whitespaceRun.ReplaceAllString(trimmed, "-")
}

var (
	nonSlugChars  = regexp.MustCompile(`[^a-z0-9\s-]`)
	whitespaceRun = regexp.MustCompile(`\s+`)
)

func countNewlinesBefore(s string, idx int) int {
	if idx > len(s) {
		idx = len(s)
	}
	return strings.Count(s[:idx], "\n")
}
