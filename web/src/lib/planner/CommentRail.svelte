<script module lang="ts">
	import type { PlanComment } from './types';

	// Rail entry assembled by the page from comments + anchor resolutions.
	// (Task-internal shape, but its FIELD NAMES are shared with the review
	// page, which imports this type from here — keep exact.)
	export type RailEntry = {
		comment: PlanComment;
		drifted: boolean;
		orphaned: boolean;
		/** container-relative Y of the highlight; null for overall/orphaned/legacy-unrendered */
		anchorTop: number | null;
	};
</script>

<script lang="ts">
	import { computeCardTops } from './positioning';
	import CommentCard from './CommentCard.svelte';

	let {
		entries,
		activeId,
		showResolved = false,
		onToggleShowResolved,
		onActivate,
		onSaveContent,
		onReanchor,
		onToggleResolve,
		onDelete
	}: {
		entries: RailEntry[];
		activeId: string | null;
		showResolved?: boolean;
		onToggleShowResolved: () => void;
		onActivate: (id: string) => void;
		onSaveContent: (id: string, content: string) => Promise<void>;
		onReanchor: (id: string) => void;
		onToggleResolve: (id: string) => Promise<void>;
		onDelete: (id: string) => Promise<void>;
	} = $props();

	const resolvedCount = $derived(entries.filter((e) => !!e.comment.resolved_at).length);
	const visible = $derived(showResolved ? entries : entries.filter((e) => !e.comment.resolved_at));
	const pinned = $derived(visible.filter((e) => e.anchorTop === null));
	const positioned = $derived(visible.filter((e) => e.anchorTop !== null));

	// Measured heights, keyed by comment id (bind:clientHeight per card wrapper).
	let heights = $state<Record<string, number>>({});
	const tops = $derived.by(() => {
		const rects = positioned.map((e) => ({
			top: e.anchorTop as number,
			height: heights[e.comment.id] ?? 120
		}));
		const activeIdx = positioned.findIndex((e) => e.comment.id === activeId);
		return computeCardTops(rects, activeIdx === -1 ? null : activeIdx);
	});
</script>

<aside class="planner-rail" aria-label="Review comments">
	<header class="rail-header">
		<span>{visible.length} comment{visible.length === 1 ? '' : 's'}</span>
		{#if resolvedCount > 0}
			<button
				type="button"
				class="rounded-md px-2 py-1 text-[var(--ink-text-faint)] hover:bg-[var(--ink-paper)] hover:text-[var(--ink-text)]"
				onclick={onToggleShowResolved}
			>
				{showResolved ? 'Hide' : 'Show'} resolved ({resolvedCount})
			</button>
		{/if}
	</header>

	{#if visible.length === 0}
		<p class="rail-empty">Select text in the document to leave the first comment.</p>
	{/if}

	<div class="rail-pinned">
		{#each pinned as entry (entry.comment.id)}
			<div bind:clientHeight={heights[entry.comment.id]}>
				<CommentCard
					{entry}
					active={entry.comment.id === activeId}
					onActivate={() => onActivate(entry.comment.id)}
					onSaveContent={(c) => onSaveContent(entry.comment.id, c)}
					onReanchor={() => onReanchor(entry.comment.id)}
					onToggleResolve={() => onToggleResolve(entry.comment.id)}
					onDelete={() => onDelete(entry.comment.id)}
				/>
			</div>
		{/each}
	</div>

	<div class="rail-positioned">
		{#each positioned as entry, i (entry.comment.id)}
			<div class="rail-slot" style="top: {tops[i]}px" bind:clientHeight={heights[entry.comment.id]}>
				<CommentCard
					{entry}
					active={entry.comment.id === activeId}
					onActivate={() => onActivate(entry.comment.id)}
					onSaveContent={(c) => onSaveContent(entry.comment.id, c)}
					onReanchor={() => onReanchor(entry.comment.id)}
					onToggleResolve={() => onToggleResolve(entry.comment.id)}
					onDelete={() => onDelete(entry.comment.id)}
				/>
			</div>
		{/each}
	</div>
</aside>

<style>
	.rail-slot {
		position: absolute;
		left: 0;
		right: 0;
		transition: top 200ms var(--ease, ease);
	}
</style>
