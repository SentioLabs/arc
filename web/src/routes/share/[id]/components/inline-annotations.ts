/**
 * Apply inline annotation marks to already-rendered markdown.
 *
 * Strategy: walk text nodes inside each block (data-source-line) referenced by
 * the anchor, find the first occurrence of the anchor's quoted_text, and wrap
 * it in a <mark> with the right CSS class. This is similar to plannotator's
 * web-highlighter but tailored to our line-anchored model.
 *
 * The implementation rebuilds the marks every render rather than diffing; this
 * is fine for the volume we expect (tens of annotations per plan).
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
		// Single-block selection: existing fast path. Most annotations land here.
		if (mark.lineStart === mark.lineEnd) {
			const block = container.querySelector<HTMLElement>(`[data-source-line="${mark.lineStart}"]`);
			if (block) wrapNeedleInBlock(block, mark.quotedText, mark, isActive);
			continue;
		}
		// Multi-block selection. `selection.toString()` inserts \n between
		// block-level elements, so the needle won't be found inside any
		// single block's textContent.
		const segments = mark.quotedText
			.split(/\n+/)
			.map((s) => s.trim())
			.filter((s) => s.length > 0);
		if (segments.length === 0) continue;
		const blocks: HTMLElement[] = [];
		for (let line = mark.lineStart; line <= mark.lineEnd; line++) {
			const b = container.querySelector<HTMLElement>(`[data-source-line="${line}"]`);
			if (b) blocks.push(b);
		}
		if (blocks.length === 0) continue;

		// Two strategies:
		//   1. Strict pairing: segment count == block count, e.g. paragraph +
		//      paragraph. Each segment goes in its corresponding block.
		//   2. Fallback: counts diverge, which happens when a block has
		//      internal multi-line structure — a <ul> with several <li>s, or
		//      a <pre><code> with many lines. The block has ONE source line
		//      but the needle has many \n separators within. We wrap every
		//      text node across the blocks in range.
		// The fallback can over-highlight when the user partially selected
		// the first or last block in range, but that's strictly better than
		// the no-highlight failure mode it replaces.
		if (segments.length === blocks.length) {
			for (let i = 0; i < blocks.length; i++) {
				wrapNeedleInBlock(blocks[i], segments[i], mark, isActive);
			}
		} else {
			wrapAllTextNodesInBlocks(blocks, mark, isActive);
		}
	}
}

/**
 * Wrap every non-empty text node inside the given blocks in a fresh <mark>.
 *
 * Used as the multi-block fallback when segment count and block count diverge
 * — typically when a block has internal multi-line structure (lists, code
 * blocks). Wrapping per text node (rather than spanning a Range across
 * sibling block elements) avoids breaking the structure: ranges that cross
 * <li> boundaries would either fail surroundContents or, on extractContents
 * fallback, rip the list apart. Each text node is its own atom — wrapping it
 * inline is always safe.
 *
 * Trade-off: if the original selection was partial inside the first or last
 * block in range, this over-highlights those blocks. That's an acceptable
 * regression of "exact selection visible" in exchange for restoring "any
 * highlight at all" for the multi-line section / code-block cases.
 */
function wrapAllTextNodesInBlocks(
	blocks: HTMLElement[],
	mark: InlineMark,
	isActive: boolean
): void {
	const className = CLASS_BY_KIND[mark.kind] + (isActive ? ' is-active' : '');
	for (const block of blocks) {
		// Snapshot text nodes before mutating; otherwise the wrapping inserts
		// new <mark> nodes and the live walker would visit them too.
		const walker = document.createTreeWalker(block, NodeFilter.SHOW_TEXT);
		const nodes: Text[] = [];
		while (walker.nextNode()) nodes.push(walker.currentNode as Text);

		for (const node of nodes) {
			if (!node.data.trim()) continue; // skip whitespace-only nodes
			if (!node.parentNode) continue;
			const wrapper = document.createElement('mark');
			wrapper.className = className;
			wrapper.dataset.annoId = mark.id;
			node.parentNode.insertBefore(wrapper, node);
			wrapper.appendChild(node);
		}
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

function wrapNeedleInBlock(
	block: HTMLElement,
	needle: string,
	mark: InlineMark,
	isActive: boolean
): void {
	if (!needle) return;

	// Walk text nodes in document order; track running offset so we can find the
	// first occurrence even when the needle spans multiple text nodes.
	const walker = document.createTreeWalker(block, NodeFilter.SHOW_TEXT);
	const textNodes: Text[] = [];
	let acc = '';
	while (walker.nextNode()) {
		const t = walker.currentNode as Text;
		textNodes.push(t);
		acc += t.data;
	}

	const offset = acc.indexOf(needle);
	if (offset < 0) return; // anchor lost; caller already showed a drift badge

	const start = offset;
	const end = offset + needle.length;

	// Find the start text node + offset within it.
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
	if (!startNode || !endNode) return;

	try {
		const range = document.createRange();
		range.setStart(startNode, startInner);
		range.setEnd(endNode, endInner);

		const wrapper = document.createElement('mark');
		wrapper.className = CLASS_BY_KIND[mark.kind] + (isActive ? ' is-active' : '');
		wrapper.dataset.annoId = mark.id;

		// `surroundContents` works for ranges that don't cross element boundaries.
		// For multi-element ranges we extract+wrap+reinsert which works for inline
		// content (text + simple inline elements like <code>, <strong>).
		try {
			range.surroundContents(wrapper);
		} catch {
			const frag = range.extractContents();
			wrapper.appendChild(frag);
			range.insertNode(wrapper);
		}
	} catch {
		// Range manipulation can fail on edge cases (e.g., needle spans across
		// block elements). Skip silently — the right-rail card still shows the
		// annotation so the reviewer's intent isn't lost.
	}
}
