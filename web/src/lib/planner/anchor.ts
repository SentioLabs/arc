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

/** Collapse every run of whitespace to a single space. */
function normalizeWs(text: string): string {
	return text.replace(/\s+/g, ' ').trim();
}

/**
 * Concatenate block texts into one searchable string, remembering where each
 * block begins. `normalize` builds the whitespace-collapsed variant: blocks are
 * normalized individually and joined with a single space, so block boundaries
 * (and therefore line spans) stay resolvable in the collapsed space too.
 */
function buildIndex(blocks: RenderedBlock[], normalize = false): DocIndex {
	const sep = normalize ? ' ' : '\n';
	const starts: number[] = [];
	let text = '';
	for (const b of blocks) {
		if (text.length > 0) text += sep;
		starts.push(text.length);
		text += normalize ? normalizeWs(b.text) : b.text;
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
 *      If there are none, retry in a whitespace-normalized copy of the document
 *      (browsers emit `\n\n` at paragraph boundaries in selection.toString()
 *      while the index joins blocks with a single `\n`, so a legitimate
 *      cross-paragraph quote can miss exactly). Normalized hits are never 'ok'.
 *   2. Still no matches → orphaned (return stored lines unchanged).
 *   3. Pick the match: the stored occurrence when it still exists and its
 *      stored context (if any) still surrounds it; otherwise the candidate
 *      nearest to the stored line_start among those qualified by
 *      context_before/after, else by heading_slug, else among all matches.
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

	let normalized = false;
	let index = buildIndex(blocks);
	let needle = anchor.quoted_text;
	let matches = findAll(index.text, needle);

	if (matches.length === 0) {
		normalized = true;
		index = buildIndex(blocks, true);
		needle = normalizeWs(anchor.quoted_text);
		matches = findAll(index.text, needle);
	}
	if (matches.length === 0) return orphaned;

	const chosen = selectMatch(blocks, index, matches, anchor, needle, normalized);

	const startBlock = blocks[blockIndexAt(index, matches[chosen])];
	const endBlock = blocks[blockIndexAt(index, matches[chosen] + needle.length - 1)];
	const resolution: AnchorResolution = {
		lineStart: startBlock.lineStart,
		lineEnd: endBlock.lineEnd,
		occurrence: chosen,
		status: 'drifted'
	};
	if (
		!normalized &&
		chosen === anchor.occurrence &&
		resolution.lineStart === anchor.line_start &&
		resolution.lineEnd === anchor.line_end
	) {
		resolution.status = 'ok';
	}
	return resolution;
}

/**
 * Choose which occurrence the anchor refers to, as an index into `matches`.
 *
 * The stored occurrence wins outright when it still exists and its context
 * still checks out. Every repair path below it is a *tier* — context-qualified,
 * heading-qualified, then everything — and within whichever tier first yields
 * candidates the winner is the one whose block starts nearest to the stored
 * line_start (ties go to the earlier match). Picking blindly in document order
 * resolves to the wrong copy whenever a document repeats content.
 *
 * On the normalized tier the stored occurrence index is not comparable (the
 * collapsed text can merge or split matches), so it gets no fast path.
 */
function selectMatch(
	blocks: RenderedBlock[],
	index: DocIndex,
	matches: number[],
	anchor: PlanCommentAnchor,
	needle: string,
	normalized: boolean
): number {
	const before = normalized
		? normalizeWs(anchor.context_before ?? '')
		: (anchor.context_before ?? '');
	const after = normalized ? normalizeWs(anchor.context_after ?? '') : (anchor.context_after ?? '');
	const hasContext = Boolean(before || after);

	if (!normalized && anchor.occurrence < matches.length) {
		if (
			!hasContext ||
			contextMatches(index.text, matches[anchor.occurrence], needle, before, after)
		) {
			return anchor.occurrence;
		}
	}

	const all = matches.map((_, i) => i);

	if (hasContext) {
		const byContext = all.filter((i) =>
			contextMatches(index.text, matches[i], needle, before, after)
		);
		if (byContext.length > 0) return nearestToStoredLine(blocks, index, matches, byContext, anchor);
	}

	if (anchor.heading_slug) {
		const byHeading = all.filter(
			(i) => nearestHeadingSlug(blocks, index, matches[i]) === anchor.heading_slug
		);
		if (byHeading.length > 0) return nearestToStoredLine(blocks, index, matches, byHeading, anchor);
	}

	return nearestToStoredLine(blocks, index, matches, all, anchor);
}

/**
 * Of `candidates` (indices into `matches`), the one whose block begins closest
 * to the anchor's stored line_start. Ties resolve to the earlier match because
 * the scan runs in document order and only strictly-closer wins.
 */
function nearestToStoredLine(
	blocks: RenderedBlock[],
	index: DocIndex,
	matches: number[],
	candidates: number[],
	anchor: PlanCommentAnchor
): number {
	let best = candidates[0];
	let bestDistance = Infinity;
	for (const c of candidates) {
		const lineStart = blocks[blockIndexAt(index, matches[c])].lineStart;
		const distance = Math.abs(lineStart - anchor.line_start);
		if (distance < bestDistance) {
			bestDistance = distance;
			best = c;
		}
	}
	return best;
}

/** True when both stored context strings (if present) surround the match. */
function contextMatches(
	text: string,
	matchIdx: number,
	needle: string,
	before: string,
	after: string
): boolean {
	if (!before && !after) return false; // no context stored → cannot confirm
	const beforeOk =
		!before || text.slice(Math.max(0, matchIdx - before.length), matchIdx) === before;
	const afterEnd = matchIdx + needle.length;
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
