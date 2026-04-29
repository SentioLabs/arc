<script lang="ts">
	import type { CommentType } from '$lib/paste/types';

	const labels: { id: CommentType; emoji: string; color: string; tone: string }[] = [
		{ id: 'praise', emoji: '🌟', color: 'bg-green-100 text-green-800', tone: 'Praise' },
		{ id: 'issue', emoji: '🔴', color: 'bg-red-100 text-red-800', tone: 'Issue' },
		{ id: 'suggestion', emoji: '💡', color: 'bg-blue-100 text-blue-800', tone: 'Suggestion' },
		{ id: 'question', emoji: '❓', color: 'bg-yellow-100 text-yellow-800', tone: 'Question' },
		{ id: 'nit', emoji: '💅', color: 'bg-gray-100 text-gray-800', tone: 'Nit' }
	];

	let { value = $bindable<CommentType>('comment') } = $props();
</script>

<div class="flex gap-1 flex-wrap">
	{#each labels as l}
		<button
			type="button"
			class="px-2 py-1 rounded text-xs {l.color} {value === l.id ? 'ring-2 ring-offset-1' : 'opacity-60'}"
			onclick={() => (value = l.id)}
		>
			{l.emoji}
			{l.tone}
		</button>
	{/each}
</div>
