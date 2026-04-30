/**
 * Apply inline annotation marks to already-rendered markdown.
 *
 * Strategy: for each annotation, gather the [data-source-line] blocks in
 * [lineStart, lineEnd], walk all their text nodes in document order, and find
 * the anchor's quoted_text. The needle from selection.toString() may contain
 * \n separators between block-level elements (paragraphs, <li>s, etc.) while
 * the flat text walked via TreeWalker has none — so the search is two-tier:
 *
 *   1. Exact substring match (most annotations land here).
 *   2. Whitespace-normalized match (collapses runs of whitespace, including
 *      block-boundary newlines) with a position map back to raw offsets.
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
 * The implementation rebuilds the marks every render rather than diffing.
 */

export type InlineMark = {
	id: string;
	kind: 'comment' | 'delete';
	lineStart: number;
	lineEnd: number;
	quotedText: string;
};

const CLASS_BY_KIND: Record<InlineMark['kind'], string> = {
	comment: 'anno-comment',
	delete: 'anno-delete'
};

export function applyInlineAnnotations(
	container: HTMLElement,
	marks: InlineMark[],
	activeId?: string
): void {
	// Tear down any prior marks so we don't double-wrap on re-render.
	clearMarks(container);

	for (const mark of marks) {
		const isActive = mark.id === activeId;
		const blocks: HTMLElement[] = [];
		for (let line = mark.lineStart; line <= mark.lineEnd; line++) {
			const b = container.querySelector<HTMLElement>(`[data-source-line="${line}"]`);
			if (b) blocks.push(b);
		}
		if (blocks.length === 0) continue;
		wrapNeedleAcrossBlocks(blocks, mark.quotedText, mark, isActive);
	}
}

function clearMarks(container: HTMLElement): void {
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

function wrapNeedleAcrossBlocks(
	blocks: HTMLElement[],
	needle: string,
	mark: InlineMark,
	isActive: boolean
): void {
	if (!needle) return;

	// Walk text nodes across all blocks in DOM order.
	const textNodes: Text[] = [];
	let acc = '';
	for (const block of blocks) {
		const walker = document.createTreeWalker(block, NodeFilter.SHOW_TEXT);
		while (walker.nextNode()) {
			const t = walker.currentNode as Text;
			textNodes.push(t);
			acc += t.data;
		}
	}
	if (textNodes.length === 0) return;

	// Two-tier search.
	let start = -1;
	let end = -1;
	const exactOffset = acc.indexOf(needle);
	if (exactOffset >= 0) {
		start = exactOffset;
		end = exactOffset + needle.length;
	} else {
		const norm = normalizeWithMap(acc);
		const needleNorm = normalizeWS(needle);
		if (!needleNorm) return;
		const normOffset = norm.normalized.indexOf(needleNorm);
		if (normOffset < 0) return; // anchor lost
		start = norm.rawPositions[normOffset];
		const lastNormIdx = normOffset + needleNorm.length - 1;
		if (lastNormIdx >= norm.rawPositions.length) return;
		end = norm.rawPositions[lastNormIdx] + 1;
	}

	// Wrap. Try the contiguous Range first; on failure (range crosses sibling
	// block elements like <li>s), fall back to per-text-node wrap.
	if (!tryRangeWrap(textNodes, start, end, mark, isActive)) {
		wrapPerTextNode(textNodes, start, end, mark, isActive);
	}
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
	wrapper.className = CLASS_BY_KIND[mark.kind] + (isActive ? ' is-active' : '');
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
