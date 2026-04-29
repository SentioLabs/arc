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
		const block = container.querySelector<HTMLElement>(`[data-source-line="${mark.lineStart}"]`);
		if (!block) continue;
		wrapFirstOccurrence(block, mark, mark.id === activeId);
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

function wrapFirstOccurrence(block: HTMLElement, mark: InlineMark, isActive: boolean): void {
	const needle = mark.quotedText;
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
