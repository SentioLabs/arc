/**
 * Apply inline annotation marks to already-rendered markdown.
 *
 * The hard problem: selection.toString() inserts \n separators at block
 * boundaries (between <li>s, between <p>s, after a heading) and at <br>
 * elements, but a TreeWalker over the same DOM yields just the text-node
 * data with no separators. Naively searching the concatenated text-node
 * string for the needle, or even a whitespace-normalized version of it,
 * fails when the gap between blocks is zero whitespace — normalization can
 * only collapse existing runs.
 *
 * Strategy: for each annotation, gather the [data-source-line] blocks in
 * [lineStart, lineEnd] — snapping lineStart back to the greatest tagged line
 * at or before it, since only block START lines carry the attribute and a
 * drifted resolution can point mid-block — then walk all their text nodes plus
 * <br>/<hr> elements in document order, and build TWO parallel strings:
 *
 *   - `acc`: raw concatenation of text-node data. Cumulative offsets into
 *     this string map directly into textNodes via length math — used by the
 *     wrap step.
 *   - `searchSpace`: same content but with a synthetic '\n' inserted at
 *     every block-level boundary AND every <br>/<hr> element. This mirrors
 *     what selection.toString() emits, so the needle can match.
 *     `searchToAcc[i]` maps every searchSpace index to its acc index, with
 *     -1 marking a synthetic boundary char.
 *
 * Search is two-tier on `searchSpace`: exact match, then whitespace-
 * normalized fallback for cases like extra trailing whitespace. Both tiers
 * select the nth match rather than the first. The hit's range gets mapped back
 * through `searchToAcc` (skipping synthetic chars) before reaching the wrap
 * step.
 *
 * Wrapping is also two-tier:
 *
 *   1. Range surroundContents on a single Range from start text node to end
 *      text node — clean for ranges within one inline-friendly element.
 *   2. Per-text-node wrap when (1) fails because the range crosses sibling
 *      block elements like <li>s. surroundContents throws in that case;
 *      falling back to extractContents would rip the list structure apart,
 *      so we instead wrap the matched slice of each individual text node
 *      (which is always inline-safe).
 *
 * A mark's `occurrence` is doc-wide (counted over the whole container's search
 * space, which is what capture and anchor resolution both count against), but
 * the wrap search runs over the narrowed block range — so the doc-wide index is
 * recomputed into a local one by subtracting the matches that start before the
 * range. This "before" count is computed once in raw space and once in
 * normalized space, since the two tiers search different haystacks and
 * whitespace differences between occurrences make the two counts diverge.
 *
 * The implementation rebuilds the marks every render rather than diffing.
 */

import type { InlineMark } from './types';

export function applyInlineAnnotations(
	container: HTMLElement,
	marks: InlineMark[],
	activeId?: string
): void {
	// Tear down any prior marks so we don't double-wrap on re-render.
	clearMarks(container);

	const tagged = Array.from(container.querySelectorAll<HTMLElement>('[data-source-line]'))
		.map((el) => ({ el, line: Number(el.dataset.sourceLine) }))
		.filter((b) => Number.isFinite(b.line));

	// One doc-wide index per apply — the space `occurrence` was counted against.
	const full = buildSearchIndex(tagged.map((b) => b.el));
	// Normalized once per apply too, so the whitespace-normalized tier can
	// recompute its own doc-wide "before" count in normalized space instead
	// of reusing the raw-space one (see wrapNeedleAcrossBlocks).
	const fullNorm = normalizeWithMap(full.searchSpace);

	for (const mark of marks) {
		const isActive = mark.id === activeId;
		// Snap: greatest tagged line <= lineStart (drifted resolutions can point
		// mid-block); fall back to the first tagged line >= lineStart.
		let startLine = -1;
		for (const b of tagged) {
			if (b.line <= mark.lineStart) startLine = b.line;
			else break;
		}
		if (startLine === -1) {
			const next = tagged.find((b) => b.line >= mark.lineStart);
			if (!next) continue;
			startLine = next.line;
		}
		const endLine = Math.max(mark.lineEnd, startLine);
		const startIdx = tagged.findIndex((b) => b.line >= startLine && b.line <= endLine);
		if (startIdx === -1) continue;
		const blocks = tagged.filter((b) => b.line >= startLine && b.line <= endLine).map((b) => b.el);
		if (blocks.length === 0) continue;
		// Doc-wide occurrence → range-local occurrence: drop every match that
		// STARTS before the narrowed range begins. Counted by filtering the
		// full-space matches rather than by re-searching a prefix slice, so a
		// match that straddles the range boundary still counts as "before".
		const rangeStart = full.blockStarts[startIdx];
		const before = findAll(full.searchSpace, mark.quotedText).filter((i) => i < rangeStart).length;
		// Same "before" count, but in normalized space — for the normalized
		// fallback tier, which searches a normalized haystack and must not be
		// indexed with a raw-space count (they diverge whenever whitespace
		// differs across occurrences).
		const needleNorm = normalizeWS(mark.quotedText);
		const normRangeStart = countBefore(fullNorm.rawPositions, rangeStart);
		const normBefore = needleNorm
			? findAll(fullNorm.normalized, needleNorm).filter((i) => i < normRangeStart).length
			: 0;
		wrapNeedleAcrossBlocks(
			blocks,
			mark.quotedText,
			mark,
			isActive,
			mark.occurrence - before,
			mark.occurrence - normBefore
		);
	}
}

export function clearMarks(container: HTMLElement): void {
	const marks = container.querySelectorAll<HTMLElement>('mark[data-anno-id]');
	for (const m of marks) {
		const parent = m.parentNode;
		if (!parent) continue;
		while (m.firstChild) parent.insertBefore(m.firstChild, m);
		parent.removeChild(m);
		// Merge adjacent text nodes that were split when we wrapped.
		parent.normalize();
	}
}

type SearchIndex = {
	/** acc with synthetic '\n' at every block/<br>/<hr> boundary. */
	searchSpace: string;
	/** searchSpace index → acc index, -1 for synthetic boundary chars. */
	searchToAcc: number[];
	textNodes: Text[];
	/** searchSpace offset at which each input block starts. */
	blockStarts: number[];
};

/**
 * Walk text nodes AND inline structural elements (<br>/<hr>) across all
 * blocks in document order. We build:
 *
 *   - `acc`: raw text-node concatenation (textNode position math basis).
 *   - `searchSpace`: acc with synthetic '\n' at:
 *       * block-level boundaries (different closest-block ancestor), and
 *       * <br>/<hr> elements (invisible to SHOW_TEXT but they produce a
 *         '\n' in Selection.toString()).
 *     Mirrors what selection.toString() emits so the needle can match.
 */
function buildSearchIndex(blocks: HTMLElement[]): SearchIndex {
	const textNodes: Text[] = [];
	const blockStarts: number[] = [];
	let acc = '';
	let searchSpace = '';
	const searchToAcc: number[] = [];
	let prevBlock: Element | null = null;
	const filter: NodeFilter = {
		acceptNode(node) {
			if (node.nodeType === Node.TEXT_NODE) return NodeFilter.FILTER_ACCEPT;
			if (node.nodeType === Node.ELEMENT_NODE && LINE_BREAK_TAGS.has((node as Element).tagName)) {
				return NodeFilter.FILTER_ACCEPT;
			}
			return NodeFilter.FILTER_SKIP;
		}
	};
	for (const block of blocks) {
		blockStarts.push(searchSpace.length);
		const walker = document.createTreeWalker(
			block,
			NodeFilter.SHOW_TEXT | NodeFilter.SHOW_ELEMENT,
			filter
		);
		while (walker.nextNode()) {
			const node = walker.currentNode;
			if (node.nodeType === Node.ELEMENT_NODE) {
				if (acc.length > 0) {
					searchSpace += '\n';
					searchToAcc.push(-1);
				}
				continue;
			}
			const t = node as Text;
			const curBlock = closestBlockAncestor(t);
			if (prevBlock && curBlock !== prevBlock) {
				searchSpace += '\n';
				searchToAcc.push(-1);
			}
			prevBlock = curBlock;
			textNodes.push(t);
			const baseAcc = acc.length;
			for (let i = 0; i < t.data.length; i++) {
				searchSpace += t.data[i];
				searchToAcc.push(baseAcc + i);
			}
			acc += t.data;
		}
	}
	return { searchSpace, searchToAcc, textNodes, blockStarts };
}

/**
 * Canonical matchable text of one rendered block — the same search space the
 * wrapper matches against. RenderedBlock.text MUST use this so that occurrence
 * counting agrees between capture, anchor resolution, and mark wrapping.
 */
export function blockSearchText(block: HTMLElement): string {
	return buildSearchIndex([block]).searchSpace;
}

/** Start offsets of every non-overlapping occurrence of `needle`. */
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
 * The normalized index corresponding to raw offset `r`: the count of
 * `rawPositions` entries strictly less than `r`. `rawPositions` is the
 * monotonically increasing normalized-index → raw-index map produced by
 * normalizeWithMap, so this is where `r` would fall if inserted into it.
 */
function countBefore(rawPositions: number[], r: number): number {
	let lo = 0;
	let hi = rawPositions.length;
	while (lo < hi) {
		const mid = (lo + hi) >>> 1;
		if (rawPositions[mid] < r) lo = mid + 1;
		else hi = mid;
	}
	return lo;
}

function wrapNeedleAcrossBlocks(
	blocks: HTMLElement[],
	needle: string,
	mark: InlineMark,
	isActive: boolean,
	localOccurrence: number,
	normalizedLocalOccurrence: number
): void {
	if (!needle) return;

	const { searchSpace, searchToAcc, textNodes } = buildSearchIndex(blocks);
	if (textNodes.length === 0) return;

	// Two-tier search against searchSpace (which mirrors selection.toString()).
	// Each tier picks the nth match (clamped — a stale occurrence falls back to
	// the last match rather than dropping the mark).
	let searchStart = -1;
	let searchEnd = -1;
	const matches = findAll(searchSpace, needle);
	let exactOffset = -1;
	if (matches.length > 0) {
		const local = Math.min(Math.max(localOccurrence, 0), matches.length - 1);
		exactOffset = matches[local];
	}
	if (exactOffset >= 0) {
		searchStart = exactOffset;
		searchEnd = exactOffset + needle.length;
	} else {
		const norm = normalizeWithMap(searchSpace);
		const needleNorm = normalizeWS(needle);
		if (!needleNorm) return;
		const normMatches = findAll(norm.normalized, needleNorm);
		if (normMatches.length === 0) return;
		const normOffset =
			normMatches[Math.min(Math.max(normalizedLocalOccurrence, 0), normMatches.length - 1)];
		searchStart = norm.rawPositions[normOffset];
		const lastNormIdx = normOffset + needleNorm.length - 1;
		if (lastNormIdx >= norm.rawPositions.length) return;
		searchEnd = norm.rawPositions[lastNormIdx] + 1;
	}

	// Map searchSpace [start, end) back to acc, skipping synthetic boundary
	// chars. The first real char at-or-after searchStart anchors `start`;
	// the last real char before searchEnd anchors `end`.
	let start = -1;
	for (let i = searchStart; i < searchToAcc.length; i++) {
		if (searchToAcc[i] >= 0) {
			start = searchToAcc[i];
			break;
		}
	}
	let end = -1;
	for (let i = searchEnd - 1; i >= 0; i--) {
		if (searchToAcc[i] >= 0) {
			end = searchToAcc[i] + 1;
			break;
		}
	}
	if (start < 0 || end <= start) return;

	// Wrap. Try the contiguous Range first; on failure (range crosses sibling
	// block elements like <li>s), fall back to per-text-node wrap.
	if (!tryRangeWrap(textNodes, start, end, mark, isActive)) {
		wrapPerTextNode(textNodes, start, end, mark, isActive);
	}
}

// Tags that produce a literal '\n' in Selection.toString() despite having no
// text-node children. We have to walk SHOW_ELEMENT to see them and translate
// each into a synthetic separator.
const LINE_BREAK_TAGS = new Set(['BR', 'HR']);

// Tags treated as inline for the purpose of "did selection.toString() insert
// a \n between these two text nodes?". Anything not in this set is treated
// as a block boundary (paragraphs, list items, headings, code blocks, table
// cells, etc.). Matches the HTML spec's "phrasing content" tags that the
// markdown renderer can plausibly emit.
const INLINE_TAGS = new Set([
	'A',
	'ABBR',
	'B',
	'BDI',
	'BDO',
	'BR',
	'CITE',
	'CODE',
	'DATA',
	'DEL',
	'DFN',
	'EM',
	'I',
	'INS',
	'KBD',
	'MARK',
	'Q',
	'S',
	'SAMP',
	'SMALL',
	'SPAN',
	'STRONG',
	'SUB',
	'SUP',
	'TIME',
	'U',
	'VAR',
	'WBR'
]);

function closestBlockAncestor(node: Node): Element | null {
	let cur: Node | null = node.parentNode;
	while (cur && cur.nodeType === 1) {
		if (!INLINE_TAGS.has((cur as Element).tagName)) return cur as Element;
		cur = cur.parentNode;
	}
	return null;
}

/**
 * Attempts to wrap [start, end) of the concatenated text-node string in a
 * single Range surroundContents. Works for ranges that don't cross sibling
 * block elements. Returns false on any failure so the caller can fall back.
 */
function tryRangeWrap(
	textNodes: Text[],
	start: number,
	end: number,
	mark: InlineMark,
	isActive: boolean
): boolean {
	let cum = 0;
	let startNode: Text | null = null;
	let startInner = 0;
	let endNode: Text | null = null;
	let endInner = 0;
	for (const t of textNodes) {
		const next = cum + t.data.length;
		if (startNode === null && start < next) {
			startNode = t;
			startInner = start - cum;
		}
		if (endNode === null && end <= next) {
			endNode = t;
			endInner = end - cum;
			break;
		}
		cum = next;
	}
	if (!startNode || !endNode) return false;

	try {
		const range = document.createRange();
		range.setStart(startNode, startInner);
		range.setEnd(endNode, endInner);
		const wrapper = createMarkWrapper(mark, isActive);
		range.surroundContents(wrapper);
		return true;
	} catch {
		// surroundContents throws when the range crosses element boundaries
		// it can't surround (e.g., from inside one <li> to inside another).
		// We deliberately do NOT call extractContents here — that would rip
		// the structure apart and put the list contents into one flat <mark>.
		return false;
	}
}

/**
 * Wraps the [start, end) slice of each text node that overlaps that range.
 * Each text-node range is its own atom: surroundContents on a single text
 * node is always safe regardless of the surrounding element structure, so
 * this preserves <li>/<p>/<code> boundaries when the contiguous Range path
 * couldn't wrap across them.
 */
function wrapPerTextNode(
	textNodes: Text[],
	start: number,
	end: number,
	mark: InlineMark,
	isActive: boolean
): void {
	let cum = 0;
	for (const t of textNodes) {
		const tStart = cum;
		const tEnd = cum + t.data.length;
		cum = tEnd;
		const overlapStart = Math.max(start, tStart);
		const overlapEnd = Math.min(end, tEnd);
		if (overlapStart >= overlapEnd) continue;
		const localStart = overlapStart - tStart;
		const localEnd = overlapEnd - tStart;
		// Skip slices that are pure whitespace (e.g., a "\n" between code-block
		// lines that contributed only to the separator). They shouldn't get
		// their own visible <mark>.
		if (!t.data.slice(localStart, localEnd).trim()) continue;
		if (!t.parentNode) continue;
		try {
			const range = document.createRange();
			range.setStart(t, localStart);
			range.setEnd(t, localEnd);
			const wrapper = createMarkWrapper(mark, isActive);
			range.surroundContents(wrapper);
		} catch {
			// A text-node range shouldn't fail surroundContents under normal DOM,
			// but if it does, skip rather than throw.
		}
	}
}

function createMarkWrapper(mark: InlineMark, isActive: boolean): HTMLElement {
	const wrapper = document.createElement('mark');
	wrapper.className = 'anno-comment';
	if (mark.resolved) wrapper.classList.add('is-resolved');
	if (mark.drifted) wrapper.classList.add('is-drifted');
	if (isActive) wrapper.classList.add('is-active');
	wrapper.dataset.annoId = mark.id;
	return wrapper;
}

/**
 * Collapses runs of whitespace in `s` to single spaces (trimming ends),
 * returning the normalized string and a map from each normalized index to
 * its corresponding index in the original string. Used to bridge the gap
 * between needle (with \n separators from selection.toString()) and the
 * flat text walked via TreeWalker (no separators).
 */
function normalizeWithMap(s: string): { normalized: string; rawPositions: number[] } {
	let normalized = '';
	const rawPositions: number[] = [];
	let prevWS = true; // skip leading whitespace
	for (let i = 0; i < s.length; i++) {
		const ch = s[i];
		if (isWS(ch)) {
			if (!prevWS) {
				normalized += ' ';
				rawPositions.push(i);
				prevWS = true;
			}
		} else {
			normalized += ch;
			rawPositions.push(i);
			prevWS = false;
		}
	}
	if (normalized.endsWith(' ')) {
		normalized = normalized.slice(0, -1);
		rawPositions.pop();
	}
	return { normalized, rawPositions };
}

function normalizeWS(s: string): string {
	return s.replace(/\s+/g, ' ').trim();
}

function isWS(ch: string): boolean {
	return ch === ' ' || ch === '\t' || ch === '\n' || ch === '\r' || ch === '\f' || ch === '\v';
}
