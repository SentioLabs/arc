<script lang="ts">
	interface Props {
		type: 'insert' | 'delete' | 'context';
		oldNumber: number | undefined;
		newNumber: number | undefined;
		highlightedHtml: string;
		onAddComment?: () => void;
	}

	let { type, oldNumber, newNumber, highlightedHtml, onAddComment }: Props = $props();

	const rowClass = $derived(
		type === 'insert'
			? 'bg-green-950/30'
			: type === 'delete'
				? 'bg-red-950/30'
				: ''
	);

	const gutterClass = $derived(
		type === 'insert'
			? 'bg-green-950/50'
			: type === 'delete'
				? 'bg-red-950/50'
				: ''
	);

	const borderClass = $derived(
		type === 'insert'
			? 'border-l-2 border-green-500'
			: type === 'delete'
				? 'border-l-2 border-red-500'
				: ''
	);
</script>

<tr class="{rowClass} group relative">
	<td
		class="select-none text-right pr-2 font-mono text-xs text-text-muted w-[3rem] min-w-[3rem] {gutterClass} relative"
	>
		{oldNumber ?? ''}
		{#if onAddComment}
			<button
				type="button"
				class="absolute inset-0 flex items-center justify-center opacity-0 group-hover:opacity-100 text-accent-400 hover:text-accent-300 transition-opacity"
				aria-label="Add line comment"
				onclick={onAddComment}
			>
				<svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="currentColor">
					<path d="M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6v2z" />
				</svg>
			</button>
		{/if}
	</td>
	<td
		class="select-none text-right pr-2 font-mono text-xs text-text-muted w-[3rem] min-w-[3rem] {gutterClass}"
	>
		{newNumber ?? ''}
	</td>
	<td class="{borderClass} px-2">
		<pre class="font-mono text-sm leading-6 whitespace-pre-wrap"><code>{@html highlightedHtml}</code></pre>
	</td>
</tr>
