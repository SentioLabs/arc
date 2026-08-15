<script lang="ts">
	import { onMount, onDestroy, tick } from 'svelte';
	import { clampedAnchorLeft } from './positioning';

	let {
		anchorRect,
		onComment,
		onDismiss
	}: {
		anchorRect: DOMRect;
		onComment: () => void;
		onDismiss: () => void;
	} = $props();

	let toolbar: HTMLDivElement | undefined = $state();
	// Measured after the toolbar mounts (its width is content-driven, so we
	// can't know it ahead of time). We seed with a conservative upper bound
	// (220px) so the first paint is already clamped sensibly; the measured
	// value then refines it on the next tick.
	let measuredWidth = $state(220);

	function computePosition(rect: DOMRect, width: number): { top: number; left: number } {
		const TOOLBAR_HEIGHT = 44;
		const GAP = 12;
		const top = rect.top - TOOLBAR_HEIGHT - GAP;
		const rawLeft = rect.left + rect.width / 2;
		const left = clampedAnchorLeft(rawLeft, width, window.innerWidth);
		return { top: Math.max(8, top), left };
	}

	const position = $derived(computePosition(anchorRect, measuredWidth));

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			e.preventDefault();
			onDismiss();
		}
	}

	function handleDocumentClick(e: MouseEvent) {
		if (!toolbar) return;
		if (toolbar.contains(e.target as Node)) return;
		const sel = window.getSelection();
		if (sel && sel.toString().trim().length > 0) return;
		onDismiss();
	}

	function handleScroll() {
		onDismiss();
	}

	onMount(async () => {
		document.addEventListener('keydown', handleKeydown);
		document.addEventListener('mousedown', handleDocumentClick);
		window.addEventListener('scroll', handleScroll, { capture: true, passive: true });
		// Wait one tick so the toolbar has rendered, then measure it. This
		// refines `measuredWidth` from the seed (220) to the true rendered
		// width — typically ~200px depending on the icon set.
		await tick();
		if (toolbar) measuredWidth = toolbar.offsetWidth;
	});

	onDestroy(() => {
		document.removeEventListener('keydown', handleKeydown);
		document.removeEventListener('mousedown', handleDocumentClick);
		window.removeEventListener('scroll', handleScroll, { capture: true });
	});
</script>

<div
	bind:this={toolbar}
	class="floating-toolbar fixed z-[100] flex flex-row items-center gap-0.5 p-1 ui-sans"
	style="top: {position.top}px; left: {position.left}px; transform: translateX(-50%);"
	role="toolbar"
	aria-label="Annotation actions"
>
	<button
		type="button"
		class="toolbar-action"
		aria-label="Comment on selection"
		onclick={onComment}
	>
		<svg
			class="h-4 w-4"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			stroke-width="2"
			aria-hidden="true"
		>
			<path
				stroke-linecap="round"
				stroke-linejoin="round"
				d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"
			/>
		</svg>
		Comment
	</button>
</div>
