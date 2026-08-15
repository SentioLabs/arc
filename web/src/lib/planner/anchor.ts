import type { AnchorResolution, PlanCommentAnchor, RenderedBlock } from './types';

/** Number of rendered characters captured before/after a selection for drift recovery. */
export const CONTEXT_CHARS = 64;

/** slugify mirrors the heading-slug derivation used at capture time. */
export function slugify(text: string): string {
	return text
		.toLowerCase()
		.replace(/[^a-z0-9\s-]/g, '')
		.trim()
		.replace(/\s+/g, '-');
}

type DocIndex = {
	text: string;
	/** char offset where each block's text begins in `text` */
	starts: number[];
};

function buildIndex(blocks: RenderedBlock[]): DocIndex {
	const starts: number[] = [];
	let text = '';
	for (const b of blocks) {
		if (text.length > 0) text += '\n';
		starts.push(text.length);
		text += b.text;
	}
	return { text, starts };
}

function blockIndexAt(index: DocIndex, charIdx: number): number {
	let lo = 0;
	for (let i = 0; i < index.starts.length; i++) {
		if (index.starts[i] <= charIdx) lo = i;
		else break;
	}
	return lo;
}

function findAll(haystack: string, needle: string): number[] {
	const out: number[] = [];
	if (!needle) return out;
	let i = haystack.indexOf(needle);
	while (i !== -1) {
		out.push(i);
		i = haystack.indexOf(needle, i + needle.length);
	}
	return out;
}

/**
 * Resolve a stored anchor against the current rendered blocks.
 *
 * Strategy (all matching against rendered text — the same space quoted_text
 * was captured from):
 *   1. Find every occurrence of quoted_text in the concatenated block text.
 *   2. No matches → orphaned (return stored lines unchanged).
 *   3. Pick the match: stored occurrence if still valid; otherwise
 *      context_before/after disambiguation; otherwise the heading_slug
 *      section; otherwise clamp to the last match.
 *   4. status is 'ok' only when the chosen match IS the stored occurrence
 *      AND its block lines equal the stored lines; anything else 'drifted'.
 */
export function resolveAnchor(
	blocks: RenderedBlock[],
	anchor: PlanCommentAnchor
): AnchorResolution {
	const orphaned: AnchorResolution = {
		lineStart: anchor.line_start,
		lineEnd: anchor.line_end,
		occurrence: anchor.occurrence,
		status: 'orphaned'
	};
	if (!anchor.quoted_text) return orphaned;

	const index = buildIndex(blocks);
	const matches = findAll(index.text, anchor.quoted_text);
	if (matches.length === 0) return orphaned;

	let chosen = -1;

	// Stored occurrence still valid?
	if (anchor.occurrence < matches.length) {
		chosen = anchor.occurrence;
		// If context disagrees, fall through to context-based repair.
		if (!contextMatches(index.text, matches[chosen], anchor)) {
			const byContext = matches.findIndex((m) => contextMatches(index.text, m, anchor));
			if (byContext !== -1) chosen = byContext;
		}
	} else {
		const byContext = matches.findIndex((m) => contextMatches(index.text, m, anchor));
		if (byContext !== -1) {
			chosen = byContext;
		} else if (anchor.heading_slug) {
			const byHeading = matches.findIndex(
				(m) => nearestHeadingSlug(blocks, index, m) === anchor.heading_slug
			);
			chosen = byHeading !== -1 ? byHeading : matches.length - 1;
		} else {
			chosen = matches.length - 1;
		}
	}

	const startBlock = blocks[blockIndexAt(index, matches[chosen])];
	const endBlock = blocks[blockIndexAt(index, matches[chosen] + anchor.quoted_text.length - 1)];
	const resolution: AnchorResolution = {
		lineStart: startBlock.lineStart,
		lineEnd: endBlock.lineEnd,
		occurrence: chosen,
		status: 'drifted'
	};
	if (
		chosen === anchor.occurrence &&
		resolution.lineStart === anchor.line_start &&
		resolution.lineEnd === anchor.line_end
	) {
		resolution.status = 'ok';
	}
	return resolution;
}

/** True when both stored context strings (if present) surround the match. */
function contextMatches(text: string, matchIdx: number, anchor: PlanCommentAnchor): boolean {
	const before = anchor.context_before ?? '';
	const after = anchor.context_after ?? '';
	if (!before && !after) return false; // no context stored → cannot confirm
	const beforeOk =
		!before || text.slice(Math.max(0, matchIdx - before.length), matchIdx) === before;
	const afterEnd = matchIdx + anchor.quoted_text.length;
	const afterOk = !after || text.slice(afterEnd, afterEnd + after.length) === after;
	return beforeOk && afterOk;
}

/** Slug of the nearest heading block at or above the block containing charIdx. */
function nearestHeadingSlug(
	blocks: RenderedBlock[],
	index: DocIndex,
	charIdx: number
): string | undefined {
	for (let i = blockIndexAt(index, charIdx); i >= 0; i--) {
		if (blocks[i].headingSlug) return blocks[i].headingSlug;
	}
	return undefined;
}
