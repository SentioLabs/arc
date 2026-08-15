<script lang="ts">
	import { tick } from 'svelte';
	import { lexMarkdown, renderMarkdown } from '$lib/markdown';
	import { CONTEXT_CHARS, slugify } from './anchor';
	import { applyInlineAnnotations, blockSearchText } from './inline-annotations';
	import type { InlineMark, RenderedBlock, SelectionPayload } from './types';

	let {
		markdown,
		marks = [],
		activeMarkId,
		onSelection,
		onMarkClick,
		onBlocks
	}: {
		markdown: string;
		marks?: InlineMark[];
		activeMarkId?: string;
		onSelection?: (sel: SelectionPayload | null) => void;
		onMarkClick?: (id: string) => void;
		onBlocks?: (blocks: RenderedBlock[]) => void;
	} = $props();

	let container: HTMLElement | undefined = $state();
	let html = $state('');

	// `selectionchange` fires per character while a drag or shift+arrow run is
	// in progress; settle before publishing so the toolbar doesn't thrash.
	const SELECTION_DEBOUNCE_MS = 150;

	/**
	 * Render markdown to HTML, attaching `data-source-line` /
	 * `data-source-line-end` to each top-level element so the
	 * selection→anchor pipeline can recover line numbers.
	 *
	 * Why this is custom and not just `renderMarkdown(md)`:
	 *   The selection toolbar needs to know which source lines a user's
	 *   selection covers. We do that by tagging every top-level rendered
	 *   element with the markdown line span its source occupies, then
	 *   `closest('[data-source-line]')` from the selection range gives
	 *   us start/end lines. Spans, not just start lines, because a
	 *   selection that ends inside a multi-line block (a list, a fenced
	 *   code block) must report that block's LAST line as `lineEnd`.
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
	 * The right approach: use `lexMarkdown()` to get top-level tokens from
	 * the SAME configured Marked instance the renderer uses (each fenced
	 * code block is ONE token regardless of internal blank lines), compute
	 * each token's source-line span from `token.raw`'s offset in the
	 * original document, render the whole thing once via the shared
	 * markdown pipeline (shiki-highlighted + sanitized), then inject the
	 * attributes onto the corresponding top-level elements.
	 */
	async function renderWithSourceLines(raw: string): Promise<string> {
		// Normalize line endings BEFORE both lexing and rendering: a CRLF
		// document would otherwise have `\r` counted into `raw` offsets and
		// stripped by the renderer, drifting every line number after it.
		const md = raw.replace(/\r\n?/g, '\n');

		// 1. Tokenize to compute the line span of each top-level block.
		const tokens = await lexMarkdown(md);
		// Tokens that render no top-level element must not claim a slot:
		// 'space' (blank-line gaps), 'def' (link reference definitions), and
		// pure HTML comments (stripped by DOMPurify).
		const spans: { start: number; end: number }[] = [];
		let cursor = 0;
		for (const t of tokens) {
			const startIdx = md.indexOf(t.raw, cursor);
			if (startIdx === -1) {
				// Defensive — shouldn't happen since `raw` is a literal
				// substring of the source. Skip rather than crash.
				cursor += t.raw.length;
				continue;
			}
			const skip =
				t.type === 'space' ||
				t.type === 'def' ||
				(t.type === 'html' && t.raw.trim().startsWith('<!--') && t.raw.trim().endsWith('-->'));
			if (!skip) {
				const start = md.slice(0, startIdx).split('\n').length;
				const rawTrimmed = t.raw.replace(/\n+$/, '');
				const end = start + (rawTrimmed ? rawTrimmed.split('\n').length - 1 : 0);
				spans.push({ start, end });
			}
			cursor = startIdx + t.raw.length;
		}

		// 2. Render the WHOLE document through the shared pipeline.
		// This preserves fenced code blocks correctly and adds shiki
		// syntax highlighting via the existing markdown.ts wiring.
		const rendered = await renderMarkdown(md);

		// 3. Walk the rendered top-level elements and tag each with the
		// corresponding source line span. Direct attribute attach avoids
		// wrapper divs that would muddy the DOM tree and shift the
		// document's spacing rhythm.
		const parser = new DOMParser();
		const doc = parser.parseFromString(`<div id="r">${rendered}</div>`, 'text/html');
		const root = doc.getElementById('r');
		if (!root) return rendered;
		const children = Array.from(root.children);
		// Alignment hardening: if the counts disagree (an html token produced
		// 2+ elements, etc.), tag only the overlapping prefix — a missing tag
		// degrades to snap-to-previous-block downstream instead of mis-tagging
		// every subsequent block.
		const n = Math.min(children.length, spans.length);
		for (let i = 0; i < n; i++) {
			children[i].setAttribute('data-source-line', String(spans[i].start));
			children[i].setAttribute('data-source-line-end', String(spans[i].end));
		}
		return root.innerHTML;
	}

	/**
	 * Snapshot the rendered document as the block index the anchor resolver
	 * matches against. `text` comes from `blockSearchText` — the same search
	 * space the mark applier wraps in — so occurrence counting agrees between
	 * capture, resolution and wrapping. `textContent` would NOT: it omits the
	 * block/<br> separators that selection semantics insert.
	 */
	function collectBlocks(root: HTMLElement): RenderedBlock[] {
		return Array.from(root.querySelectorAll<HTMLElement>('[data-source-line]')).map((el) => {
			const isHeading = /^H[1-6]$/.test(el.tagName);
			return {
				lineStart: Number(el.dataset.sourceLine),
				lineEnd: Number(el.dataset.sourceLineEnd ?? el.dataset.sourceLine),
				text: blockSearchText(el),
				headingSlug: isHeading ? slugify(el.textContent ?? '') : undefined
			};
		});
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

	$effect(() => {
		// Publish the block index after each RENDER only — deliberately not on
		// mark changes. The page derives marks from these blocks, so re-emitting
		// whenever marks change would feed that derivation back into itself.
		void html;
		if (!container) return;
		void tick().then(() => {
			if (container) onBlocks?.(collectBlocks(container));
		});
	});

	$effect(() => {
		// Capture is `selectionchange`-driven, not mouseup-driven: keyboard
		// (shift+arrow) and touch selections never produce a mouseup over the
		// document, so a mouse-only listener silently drops both.
		let timer: ReturnType<typeof setTimeout> | undefined;
		const handleSelectionChange = () => {
			clearTimeout(timer);
			timer = setTimeout(emitSelection, SELECTION_DEBOUNCE_MS);
		};
		document.addEventListener('selectionchange', handleSelectionChange);
		return () => {
			clearTimeout(timer);
			document.removeEventListener('selectionchange', handleSelectionChange);
		};
	});

	// Resolve a Range endpoint (text node OR element) to its containing
	// [data-source-line] block. Triple-click in Chromium frequently puts
	// the range endpoints at element boundaries (e.g. startContainer = <p>,
	// startOffset = 0) rather than inside a text node; in that case
	// `node.parentElement.closest(...)` walks PAST the containing block to
	// <article>, which has no data-source-line, and returns null — which
	// would trip the dismiss path below. Treating the node itself as the
	// closest() search root when it's already an element fixes that.
	function resolveBlock(node: Node): HTMLElement | null {
		const el = node.nodeType === Node.ELEMENT_NODE ? (node as Element) : node.parentElement;
		return (el?.closest('[data-source-line]') ?? null) as HTMLElement | null;
	}

	/** Count of non-overlapping `needle` occurrences in `haystack`. */
	function countOccurrences(haystack: string, needle: string): number {
		if (!needle) return 0;
		let n = 0;
		let i = haystack.indexOf(needle);
		while (i !== -1) {
			n++;
			i = haystack.indexOf(needle, i + needle.length);
		}
		return n;
	}

	function emitSelection() {
		const sel = window.getSelection();
		if (
			!container ||
			!sel ||
			sel.rangeCount === 0 ||
			sel.isCollapsed ||
			sel.toString().trim().length === 0
		) {
			onSelection?.(null);
			return;
		}
		// Firefox can hold several ranges (ctrl+drag); we anchor on the first
		// by design rather than trying to quote a discontiguous selection.
		const range = sel.getRangeAt(0);
		// A selection somewhere else on the page is not ours — neither to
		// report nor to dismiss.
		if (!container.contains(range.commonAncestorContainer)) return;

		const startEl = resolveBlock(range.startContainer);
		const endEl = resolveBlock(range.endContainer);
		if (!startEl || !endEl) {
			onSelection?.(null);
			return;
		}

		const quotedText = sel.toString();
		const lineStart = Number(startEl.dataset.sourceLine);
		const lineEnd = Number(endEl.dataset.sourceLineEnd ?? endEl.dataset.sourceLine);
		const rect = range.getBoundingClientRect();

		// Nearest heading at or above the starting block. Headings ARE top-level
		// siblings of the block, so each candidate has to be tested with
		// `matches()` before descending with `querySelector()` — querySelector
		// alone never sees a sibling <h2> and the slug comes back undefined.
		let headingSlug: string | undefined;
		let cursor: Element | null = startEl;
		while (cursor) {
			const h = cursor.matches('h1, h2, h3, h4, h5, h6')
				? cursor
				: cursor.querySelector('h1, h2, h3, h4, h5, h6');
			if (h) {
				headingSlug = slugify(h.textContent ?? '');
				break;
			}
			cursor = cursor.previousElementSibling;
		}

		// Occurrence and contexts are counted over a prefix Range covering
		// everything before the selection. Range.toString() applies the same
		// selection semantics `quotedText` came from, so the count agrees with
		// what the anchor resolver and the mark applier compute later.
		const prefix = document.createRange();
		prefix.selectNodeContents(container);
		prefix.setEnd(range.startContainer, range.startOffset);
		const prefixText = prefix.toString();
		const occurrence = countOccurrences(prefixText, quotedText);
		const contextBefore = prefixText.slice(-CONTEXT_CHARS) || undefined;

		const suffix = document.createRange();
		suffix.selectNodeContents(container);
		suffix.setStart(range.endContainer, range.endOffset);
		const contextAfter = suffix.toString().slice(0, CONTEXT_CHARS) || undefined;

		onSelection?.({
			lineStart,
			lineEnd,
			quotedText,
			occurrence,
			headingSlug,
			contextBefore,
			contextAfter,
			rect
		});
	}

	function handlePointerUp(e: PointerEvent) {
		const target = e.target as Element | null;
		const existingMark = target?.closest('mark[data-anno-id]') as HTMLElement | null;
		if (!existingMark) return;
		// A drag that merely ENDS on top of an existing mark is a selection,
		// not a click on that mark — `selectionchange` already owns it.
		if (window.getSelection()?.toString().trim()) return;
		onMarkClick?.(existingMark.dataset.annoId!);
	}
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<article bind:this={container} class="doc" onpointerup={handlePointerUp}>
	<!-- eslint-disable-next-line svelte/no-at-html-tags -->
	{@html html}
</article>
