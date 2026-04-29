<script lang="ts">
	import { slugify } from '$lib/paste/anchor';
	import { marked } from 'marked';

	const { markdown, onSelection } = $props<{
		markdown: string;
		onSelection: (a: {
			lineStart: number;
			lineEnd: number;
			charStart?: number;
			charEnd?: number;
			quotedText: string;
			headingSlug?: string;
			contextBefore?: string;
			contextAfter?: string;
		}) => void;
	}>();

	const html = $derived(renderWithSourceLines(markdown));

	function renderWithSourceLines(md: string): string {
		// Simple v1: split by blank-line paragraphs, wrap each in a <div data-source-line>
		// and pipe content through marked. Tracks source lines well enough for paragraph
		// anchoring; refine later for sub-paragraph precision.
		const lines = md.split('\n');
		const blocks: { startLine: number; text: string }[] = [];
		let cur: { startLine: number; lines: string[] } | null = null;
		for (let i = 0; i < lines.length; i++) {
			const ln = lines[i];
			if (ln.trim() === '') {
				if (cur) {
					blocks.push({ startLine: cur.startLine, text: cur.lines.join('\n') });
					cur = null;
				}
			} else {
				if (!cur) cur = { startLine: i + 1, lines: [] };
				cur.lines.push(ln);
			}
		}
		if (cur) blocks.push({ startLine: cur.startLine, text: cur.lines.join('\n') });
		return blocks
			.map((b) => `<div data-source-line="${b.startLine}">${marked.parse(b.text)}</div>`)
			.join('\n');
	}

	function handleMouseUp() {
		const sel = window.getSelection();
		if (!sel || sel.rangeCount === 0 || sel.toString().length === 0) return;
		const range = sel.getRangeAt(0);
		const startEl = (range.startContainer.parentElement)?.closest('[data-source-line]') as HTMLElement | null;
		const endEl = (range.endContainer.parentElement)?.closest('[data-source-line]') as HTMLElement | null;
		if (!startEl || !endEl) return;
		const lineStart = Number(startEl.getAttribute('data-source-line'));
		const lineEnd = Number(endEl.getAttribute('data-source-line'));
		const quotedText = sel.toString();

		// Find nearest preceding heading element for slug.
		let headingSlug: string | undefined;
		let cursor: Element | null = startEl;
		while (cursor) {
			const h = cursor.querySelector('h1,h2,h3,h4,h5,h6');
			if (h) {
				headingSlug = slugify(h.textContent ?? '');
				break;
			}
			cursor = cursor.previousElementSibling;
		}
		onSelection({ lineStart, lineEnd, quotedText, headingSlug });
	}
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div onmouseup={handleMouseUp} class="prose" role="document">
	{@html html}
</div>
