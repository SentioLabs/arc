<script lang="ts">
	import { tick } from 'svelte';
	import { marked } from 'marked';
	import { slugify } from '$lib/paste/anchor';
	import { renderMarkdown } from '$lib/markdown';
	import { applyInlineAnnotations, type InlineMark } from './inline-annotations';

	type SelectionPayload = {
		lineStart: number;
		lineEnd: number;
		quotedText: string;
		headingSlug?: string;
		contextBefore?: string;
		contextAfter?: string;
		rect: DOMRect;
	};

	const {
		markdown,
		marks = [],
		onSelection,
		onMarkClick,
		activeMarkId
	}: {
		markdown: string;
		marks?: InlineMark[];
		onSelection?: (sel: SelectionPayload | null) => void;
		onMarkClick?: (id: string) => void;
		activeMarkId?: string;
	} = $props();

	let container: HTMLElement | undefined = $state();
	let html = $state('');

	/**
	 * Render markdown to HTML, attaching `data-source-line` to each
	 * top-level element so the selection→anchor pipeline can recover
	 * line numbers.
	 *
	 * Why this is custom and not just `renderMarkdown(md)`:
	 *   The selection toolbar needs to know which source lines a user's
	 *   selection covers. We do that by tagging every top-level rendered
	 *   element with the markdown line where its source starts, then
	 *   `closest('[data-source-line]')` from the selection range gives
	 *   us start/end lines.
	 *
	 * Why we don't just split the markdown on blank lines (the previous
	 * approach):
	 *   Fenced code blocks legally contain blank lines (separating SQL
	 *   statements, TS declarations, comment blocks). Splitting on those
	 *   destroys the fence boundary — `marked` then sees an unclosed
	 *   ```` ``` ```` and renders the rest as paragraphs/headings. That's
	 *   the bug that made SQL render as flowing prose and `#` comment
	 *   lines render as <h1>.
	 *
	 * The right approach: use `marked.lexer()` to get top-level tokens
	 * (each fenced code block is ONE token regardless of internal blank
	 * lines), compute each token's source-line position from `token.raw`'s
	 * offset in the original document, render the whole thing once via
	 * the shared markdown pipeline (shiki-highlighted + sanitized), then
	 * inject `data-source-line` onto the corresponding top-level elements.
	 */
	async function renderWithSourceLines(md: string): Promise<string> {
		// 1. Tokenize to compute line numbers per top-level block.
		const tokens = marked.lexer(md);
		const lineNumbers: number[] = [];
		let cursor = 0;
		for (const t of tokens) {
			if (t.type === 'space') {
				// `space` tokens are blank-line gaps with no rendered HTML.
				// Advance the cursor but don't claim a line slot.
				cursor += t.raw.length;
				continue;
			}
			const startIdx = md.indexOf(t.raw, cursor);
			if (startIdx === -1) {
				// Defensive — shouldn't happen since `raw` is a literal
				// substring of the source. Skip rather than crash.
				cursor += t.raw.length;
				continue;
			}
			lineNumbers.push(md.slice(0, startIdx).split('\n').length);
			cursor = startIdx + t.raw.length;
		}

		// 2. Render the WHOLE document through the shared pipeline.
		// This preserves fenced code blocks correctly and adds shiki
		// syntax highlighting via the existing markdown.ts wiring.
		const rendered = await renderMarkdown(md);

		// 3. Walk the rendered top-level elements and tag each with the
		// corresponding source line number. Direct attribute attach
		// avoids wrapper divs that would muddy the DOM tree and shift
		// the document's spacing rhythm.
		const parser = new DOMParser();
		const doc = parser.parseFromString(`<div id="r">${rendered}</div>`, 'text/html');
		const root = doc.getElementById('r');
		if (!root) return rendered;
		Array.from(root.children).forEach((child, i) => {
			if (i < lineNumbers.length) {
				child.setAttribute('data-source-line', String(lineNumbers[i]));
			}
		});
		return root.innerHTML;
	}

	$effect(() => {
		// Re-render whenever the markdown source changes. The cancelled
		// flag protects against late promise resolutions overwriting a
		// newer render — same pattern as a debounced fetch.
		const src = markdown;
		if (!src) {
			html = '';
			return;
		}
		let cancelled = false;
		void renderWithSourceLines(src).then((rendered) => {
			if (!cancelled) html = rendered;
		});
		return () => {
			cancelled = true;
		};
	});

	$effect(() => {
		void html;
		void marks;
		void activeMarkId;
		if (!container) return;
		void tick().then(() => {
			if (container) applyInlineAnnotations(container, marks, activeMarkId);
		});
	});

	function handleMouseUp(e: MouseEvent) {
		const target = e.target as Element | null;
		const existingMark = target?.closest('mark[data-anno-id]') as HTMLElement | null;
		if (existingMark) {
			onMarkClick?.(existingMark.dataset.annoId!);
			return;
		}

		const sel = window.getSelection();
		if (!sel || sel.rangeCount === 0 || sel.toString().trim().length === 0) {
			onSelection?.(null);
			return;
		}
		const range = sel.getRangeAt(0);
		const startEl = (range.startContainer.parentElement as Element | null)?.closest(
			'[data-source-line]'
		) as HTMLElement | null;
		const endEl = (range.endContainer.parentElement as Element | null)?.closest(
			'[data-source-line]'
		) as HTMLElement | null;
		if (!startEl || !endEl) {
			onSelection?.(null);
			return;
		}

		const lineStart = Number(startEl.getAttribute('data-source-line'));
		const lineEnd = Number(endEl.getAttribute('data-source-line'));
		const quotedText = sel.toString();
		const rect = range.getBoundingClientRect();

		let headingSlug: string | undefined;
		let cursor: Element | null = startEl;
		while (cursor) {
			const h = cursor.querySelector('h1, h2, h3, h4, h5, h6');
			if (h) {
				headingSlug = slugify(h.textContent ?? '');
				break;
			}
			cursor = cursor.previousElementSibling;
		}

		const blockText = startEl.textContent ?? '';
		const idx = blockText.indexOf(quotedText);
		const contextBefore = idx > 0 ? blockText.slice(Math.max(0, idx - 40), idx) : undefined;
		const contextAfter =
			idx >= 0 ? blockText.slice(idx + quotedText.length, idx + quotedText.length + 40) : undefined;

		onSelection?.({
			lineStart,
			lineEnd,
			quotedText,
			headingSlug,
			contextBefore,
			contextAfter,
			rect
		});
	}
</script>

<!-- The mouseup handler captures text selections; there's no keyboard equivalent
     to "select text and click a floating toolbar button", so the listener is
     mouse-only by design. -->
<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<article bind:this={container} class="doc" onmouseup={handleMouseUp}>
	<!-- eslint-disable-next-line svelte/no-at-html-tags -->
	{@html html}
</article>
