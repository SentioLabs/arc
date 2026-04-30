<script lang="ts">
	import type { CommentState } from '$lib/paste/events';
	import AnnotationCard from './AnnotationCard.svelte';

	const {
		states,
		isAuthor,
		reviewerName,
		activeId,
		onCardClick,
		onResolve,
		onEdit,
		onRetract
	}: {
		states: CommentState[];
		isAuthor: boolean;
		reviewerName: string | null;
		activeId?: string;
		onCardClick: (id: string) => void;
		onResolve: (
			commentId: string,
			status: 'accepted' | 'rejected' | 'resolved' | 'reopened',
			reply?: string
		) => Promise<void>;
		onEdit: (commentId: string, body: string, suggestedText: string | undefined) => Promise<void>;
		onRetract: (commentId: string) => Promise<void>;
	} = $props();

	const visibleStates = $derived(states); // could filter by status later
</script>

<aside class="ui-sans flex h-full flex-col" aria-label="Annotations">
	<header class="flex items-center justify-between border-b border-[var(--ink-rule)] px-5 py-4">
		<h2 class="text-[11px] font-semibold uppercase tracking-[0.12em] text-[var(--ink-text-muted)]">
			Annotations
		</h2>
		<span
			class="ui-mono text-[11px] text-[var(--ink-text-faint)]"
			aria-label="{visibleStates.length} annotations"
		>
			{visibleStates.length}
		</span>
	</header>

	<div class="flex-1 overflow-y-auto px-5 py-4">
		{#if visibleStates.length === 0}
			<div class="flex flex-col items-center justify-center py-16 text-center">
				<svg
					class="mb-3 h-8 w-8 text-[var(--ink-text-faint)]"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="1.5"
					aria-hidden="true"
				>
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						d="M3 6.5L7.5 11l-2 7 7-3 7 3-2-7L21 6.5l-7-1L12 0l-2 5.5-7 1z"
					/>
				</svg>
				<p class="text-xs italic text-[var(--ink-text-faint)]">No annotations yet</p>
				<p class="mt-1 text-[11px] text-[var(--ink-text-faint)]">Highlight text to begin</p>
			</div>
		{:else}
			<ul class="space-y-3">
				{#each visibleStates as entry (entry.event.id)}
					<li>
						<AnnotationCard
							{entry}
							{isAuthor}
							{reviewerName}
							isActive={activeId === entry.event.id}
							onClick={() => onCardClick(entry.event.id)}
							onResolve={(status, reply) => onResolve(entry.event.id, status, reply)}
							onEdit={(body, suggestedText) => onEdit(entry.event.id, body, suggestedText)}
							onRetract={() => onRetract(entry.event.id)}
						/>
					</li>
				{/each}
			</ul>
		{/if}
	</div>
</aside>
