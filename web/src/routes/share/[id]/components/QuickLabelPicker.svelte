<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import type { CommentType } from '$lib/paste/types';

	const {
		anchorRect,
		onPick,
		onDismiss
	}: {
		anchorRect: DOMRect;
		onPick: (label: CommentType, presetBody: string) => void;
		onDismiss: () => void;
	} = $props();

	const labels: { id: CommentType; emoji: string; label: string; preset: string }[] = [
		{ id: 'praise', emoji: '🌟', label: 'Praise', preset: 'Looks good' },
		{ id: 'issue', emoji: '🔴', label: 'Issue', preset: '' },
		{ id: 'suggestion', emoji: '💡', label: 'Suggestion', preset: '' },
		{ id: 'question', emoji: '❓', label: 'Question', preset: '' },
		{ id: 'nit', emoji: '💅', label: 'Nit', preset: '' }
	];

	let popover: HTMLDivElement | undefined = $state();

	function computePosition(rect: DOMRect): { top: number; left: number } {
		const HEIGHT = 220;
		const GAP = 12;
		const top = rect.top + window.scrollY - HEIGHT - GAP;
		const left = rect.left + window.scrollX + rect.width / 2;
		return { top: Math.max(8 + window.scrollY, top), left };
	}

	const position = $derived(computePosition(anchorRect));

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			e.preventDefault();
			onDismiss();
		}
		// Number shortcuts: 1..5 picks the corresponding label
		const n = parseInt(e.key, 10);
		if (!isNaN(n) && n >= 1 && n <= labels.length) {
			e.preventDefault();
			const l = labels[n - 1];
			onPick(l.id, l.preset);
		}
	}

	function handleDocumentClick(e: MouseEvent) {
		if (!popover) return;
		if (popover.contains(e.target as Node)) return;
		onDismiss();
	}

	onMount(() => {
		document.addEventListener('keydown', handleKeydown);
		document.addEventListener('mousedown', handleDocumentClick);
	});

	onDestroy(() => {
		document.removeEventListener('keydown', handleKeydown);
		document.removeEventListener('mousedown', handleDocumentClick);
	});
</script>

<div
	bind:this={popover}
	class="floating-toolbar fixed z-[100] w-[220px] p-1 ui-sans"
	style="top: {position.top}px; left: {position.left}px; transform: translateX(-50%);"
	role="menu"
	aria-label="Pick a label"
>
	{#each labels as l, i (l.id)}
		<button
			type="button"
			class="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm text-[var(--ink-text)] hover:bg-[var(--ink-paper)]"
			onclick={() => onPick(l.id, l.preset)}
			role="menuitem"
		>
			<span class="text-base leading-none">{l.emoji}</span>
			<span class="flex-1">{l.label}</span>
			<span class="ui-mono text-[10px] text-[var(--ink-text-faint)]">{i + 1}</span>
		</button>
	{/each}
</div>
