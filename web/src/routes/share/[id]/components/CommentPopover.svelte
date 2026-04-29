<script lang="ts">
	import { onMount, onDestroy, tick } from 'svelte';

	export type PopoverMode = 'comment' | 'suggest';

	const {
		anchorRect,
		mode,
		quotedText,
		initialBody = '',
		onSave,
		onCancel
	}: {
		anchorRect: DOMRect;
		mode: PopoverMode;
		quotedText: string;
		initialBody?: string;
		onSave: (body: string, suggestedText?: string) => void;
		onCancel: () => void;
	} = $props();

	let popover: HTMLDivElement | undefined = $state();
	let bodyInput: HTMLTextAreaElement | undefined = $state();
	let body = $state('');
	let suggested = $state('');
	$effect(() => {
		// Initialize body once from the prop's initial value
		if (initialBody && body === '') body = initialBody;
	});

	function computePosition(rect: DOMRect, m: PopoverMode): { top: number; left: number } {
		// Same anchoring as toolbar: above the selection.
		const POPOVER_HEIGHT = m === 'suggest' ? 200 : 120;
		const GAP = 12;
		const top = rect.top + window.scrollY - POPOVER_HEIGHT - GAP;
		const left = rect.left + window.scrollX + rect.width / 2;
		return { top: Math.max(8 + window.scrollY, top), left };
	}

	const position = $derived(computePosition(anchorRect, mode));
	const isSuggest = $derived(mode === 'suggest');

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			e.preventDefault();
			onCancel();
		} else if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
			e.preventDefault();
			save();
		}
	}

	function handleDocumentClick(e: MouseEvent) {
		if (!popover) return;
		if (popover.contains(e.target as Node)) return;
		onCancel();
	}

	function save() {
		const trimmedBody = body.trim();
		if (mode === 'comment' && !trimmedBody) return;
		if (mode === 'suggest' && !suggested.trim()) return;
		onSave(trimmedBody, mode === 'suggest' ? suggested : undefined);
	}

	onMount(async () => {
		document.addEventListener('keydown', handleKeydown);
		document.addEventListener('mousedown', handleDocumentClick);
		await tick();
		bodyInput?.focus();
	});

	onDestroy(() => {
		document.removeEventListener('keydown', handleKeydown);
		document.removeEventListener('mousedown', handleDocumentClick);
	});
</script>

<div
	bind:this={popover}
	class="floating-toolbar fixed z-[100] w-[360px] p-3 ui-sans"
	style="top: {position.top}px; left: {position.left}px; transform: translateX(-50%);"
	role="dialog"
	aria-label={isSuggest ? 'Propose replacement text' : 'Add a comment'}
>
	<div class="mb-2 text-[10px] uppercase tracking-[0.08em] text-[var(--ink-text-faint)]">
		{isSuggest ? 'Suggest replacement' : 'Comment'}
	</div>

	<div class="quote mb-2 text-sm">
		"{quotedText.length > 80 ? quotedText.slice(0, 80) + '…' : quotedText}"
	</div>

	{#if isSuggest}
		<label class="mb-2 block text-[10px] uppercase tracking-[0.08em] text-[var(--ink-text-faint)]">
			<span class="mb-1 block">Replacement</span>
			<textarea
				bind:value={suggested}
				rows="3"
				placeholder="Replacement text…"
				class="w-full rounded-md border border-[var(--ink-rule)] bg-[var(--ink-paper)] p-2 text-sm normal-case tracking-normal text-[var(--ink-text)] focus:border-[var(--ink-comment-edge)] focus:outline-none"
			></textarea>
		</label>
	{/if}

	<label class="block text-[10px] uppercase tracking-[0.08em] text-[var(--ink-text-faint)]">
		{#if isSuggest}
			<span class="mb-1 block">Reason (optional)</span>
		{/if}
		<textarea
			bind:this={bodyInput}
			bind:value={body}
			rows={isSuggest ? 2 : 3}
			placeholder={isSuggest ? 'Why this change?' : 'Add a comment…'}
			class="w-full rounded-md border border-[var(--ink-rule)] bg-[var(--ink-paper)] p-2 text-sm normal-case tracking-normal text-[var(--ink-text)] focus:border-[var(--ink-comment-edge)] focus:outline-none"
		></textarea>
	</label>

	<div class="mt-3 flex items-center justify-between gap-2">
		<div class="text-[10px] text-[var(--ink-text-faint)]">
			<kbd class="ui-mono rounded border border-[var(--ink-rule)] bg-[var(--ink-paper)] px-1 py-0.5"
				>⌘ ⏎</kbd
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
				onclick={onCancel}
			>
				Cancel
			</button>
			<button
				type="button"
				disabled={mode === 'comment' ? !body.trim() : !suggested.trim()}
				class="rounded-md border border-[var(--ink-comment-edge)] bg-[var(--ink-comment-bg)] px-2.5 py-1.5 text-xs font-medium text-[var(--ink-comment)] hover:bg-[var(--ink-comment-bg)] disabled:cursor-not-allowed disabled:opacity-50"
				onclick={save}
			>
				Save
			</button>
		</div>
	</div>
</div>
