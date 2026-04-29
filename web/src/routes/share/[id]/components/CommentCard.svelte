<script lang="ts">
	import type { CommentState } from '$lib/paste/events';
	import ResolveControls from './ResolveControls.svelte';
	import SuggestionDiff from './SuggestionDiff.svelte';

	const { state, isAuthor } = $props<{ state: CommentState; isAuthor: boolean }>();
	const e = $derived(state.event);
</script>

<li class="p-3 border rounded space-y-2">
	<header class="flex items-center justify-between text-xs">
		<span class="font-semibold">{e.author_name}</span>
		<span class="px-2 py-0.5 rounded bg-gray-100"
			>{e.comment_type}{e.severity ? ` · ${e.severity}` : ''}</span
		>
	</header>
	<p>{e.body}</p>
	{#if e.suggested_text}
		<SuggestionDiff original="" suggested={e.suggested_text} />
	{/if}
	<!-- DriftBadge expects a status from the resolved anchor; passed in by caller in a future pass -->
	<footer class="flex items-center justify-between text-xs text-gray-500">
		<span>{state.status}</span>
		{#if isAuthor}
			<ResolveControls
				commentId={e.id}
				currentStatus={state.status}
				onResolve={async (_status, _reply) => {
					/* parent wires up */
				}}
			/>
		{/if}
	</footer>
</li>
