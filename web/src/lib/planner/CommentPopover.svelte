<script lang="ts">
	import { onMount, onDestroy, tick } from 'svelte';
	import { clampedAnchorLeft } from './positioning';
	import { modifierGlyph } from './platform';

	const POPOVER_WIDTH = 360; // must match `w-[360px]` on the popover element

	const modKey = modifierGlyph();

	let {
		anchorRect,
		quotedText,
		initialBody = '',
		submitLabel = 'Save',
		onSave,
		onCancel
	}: {
		anchorRect: DOMRect;
		quotedText: string;
		initialBody?: string;
		submitLabel?: string;
		onSave: (body: string) => void | Promise<void>;
		onCancel: () => void;
	} = $props();

	let popover: HTMLDivElement | undefined = $state();
	let bodyInput: HTMLTextAreaElement | undefined = $state();
	let body = $state(initialBody);
	let saving = $state(false);

	const dirty = $derived(body.trim() !== initialBody.trim());

	function computePosition(rect: DOMRect): { top: number; left: number } {
		// Same anchoring as the toolbar: above the selection. `left` is the
		// horizontal center the caller will pair with `translateX(-50%)`,
		// so we clamp to the viewport so the popover doesn't clip off-screen
		// when the selection is near the doc's left or right edge.
		const POPOVER_HEIGHT = 120;
		const GAP = 12;
		const top = rect.top - POPOVER_HEIGHT - GAP;
		const rawLeft = rect.left + rect.width / 2;
		const left = clampedAnchorLeft(rawLeft, POPOVER_WIDTH, window.innerWidth);
		return { top: Math.max(8, top), left };
	}

	const position = $derived(computePosition(anchorRect));

	function requestCancel() {
		if (dirty && !window.confirm('Discard this comment draft?')) return;
		onCancel();
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			e.preventDefault();
			requestCancel();
		} else if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
			e.preventDefault();
			save();
		}
	}

	function handleDocumentClick(e: MouseEvent) {
		if (!popover) return;
		if (popover.contains(e.target as Node)) return;
		requestCancel();
	}

	// Scroll dismissal is incidental — the page moved, the user didn't ask to
	// close anything. If the draft is dirty, do nothing: the composer is
	// viewport-fixed, so it stays put and the user keeps typing. If the draft
	// is clean, dismiss silently (no confirm — there's nothing to lose).
	// Escape, outside click, and Cancel are deliberate dismissals and still
	// route through requestCancel() for the dirty-draft confirm.
	function handleScroll() {
		if (dirty) return;
		onCancel();
	}

	async function save() {
		if (saving) return;
		const trimmedBody = body.trim();
		if (!trimmedBody) return;
		saving = true;
		try {
			await onSave(trimmedBody);
		} finally {
			saving = false;
		}
	}

	onMount(async () => {
		document.addEventListener('keydown', handleKeydown);
		document.addEventListener('mousedown', handleDocumentClick);
		window.addEventListener('scroll', handleScroll, { capture: true, passive: true });
		await tick();
		bodyInput?.focus();
	});

	onDestroy(() => {
		document.removeEventListener('keydown', handleKeydown);
		document.removeEventListener('mousedown', handleDocumentClick);
		window.removeEventListener('scroll', handleScroll, { capture: true });
	});
</script>

<div
	bind:this={popover}
	class="floating-toolbar fixed z-[100] w-[360px] p-3 ui-sans"
	style="top: {position.top}px; left: {position.left}px; transform: translateX(-50%);"
	role="dialog"
	aria-label="Add a comment"
>
	<div class="mb-2 text-[10px] uppercase tracking-[0.08em] text-[var(--ink-text-faint)]">
		Comment
	</div>

	<div class="quote mb-2 text-sm">
		"{quotedText.length > 80 ? quotedText.slice(0, 80) + '…' : quotedText}"
	</div>

	<label class="block text-[10px] uppercase tracking-[0.08em] text-[var(--ink-text-faint)]">
		<textarea
			bind:this={bodyInput}
			bind:value={body}
			rows="3"
			placeholder="Add a comment…"
			class="w-full rounded-md border border-[var(--ink-rule)] bg-[var(--ink-paper)] p-2 text-sm normal-case tracking-normal text-[var(--ink-text)] focus:border-[var(--ink-comment-edge)] focus:outline-none"
		></textarea>
	</label>

	<div class="mt-3 flex items-center justify-between gap-2">
		<div class="text-[10px] text-[var(--ink-text-faint)]">
			<kbd class="ui-mono rounded border border-[var(--ink-rule)] bg-[var(--ink-paper)] px-1 py-0.5"
				>{modKey} ⏎</kbd
			>
			to save ·
			<kbd class="ui-mono rounded border border-[var(--ink-rule)] bg-[var(--ink-paper)] px-1 py-0.5"
				>esc</kbd
			> to cancel
		</div>
		<div class="flex gap-2">
			<button
				type="button"
				class="rounded-md px-2.5 py-1.5 text-xs font-medium text-[var(--ink-text-muted)] hover:bg-[var(--ink-paper)]"
				onclick={requestCancel}
			>
				Cancel
			</button>
			<button
				type="button"
				disabled={saving || !body.trim()}
				class="rounded-md border border-[var(--ink-comment-edge)] bg-[var(--ink-comment-bg)] px-2.5 py-1.5 text-xs font-medium text-[var(--ink-comment)] hover:bg-[var(--ink-comment-bg)] disabled:cursor-not-allowed disabled:opacity-50"
				onclick={save}
			>
				{submitLabel}
			</button>
		</div>
	</div>
</div>
