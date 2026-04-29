<script lang="ts">
	import CommentCard from './CommentCard.svelte';
	import type { CommentState } from '$lib/paste/events';

	const { parent, replies, isAuthor } = $props<{
		parent: CommentState;
		replies: CommentState[];
		isAuthor: boolean;
	}>();
</script>

<div>
	<CommentCard state={parent} {isAuthor} />
	{#if replies.length}
		<ul class="ml-6 mt-2 space-y-2 border-l pl-3">
			{#each replies as r (r.event.id)}
				<CommentCard state={r} {isAuthor} />
			{/each}
		</ul>
	{/if}
</div>
