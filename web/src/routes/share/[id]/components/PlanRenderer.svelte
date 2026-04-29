<script lang="ts">
	import { tick } from 'svelte';
	import { marked } from 'marked';
	import { slugify } from '$lib/paste/anchor';
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
	const html = $derived(renderWithSourceLines(markdown));

	function renderWithSourceLines(md: string): string {
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
