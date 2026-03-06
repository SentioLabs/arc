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

<tr class="{rowClass} group">
	<td
		class="select-none text-right pr-2 font-mono text-xs text-text-muted w-[3rem] min-w-[3rem] {gutterClass} relative"
	>
		{#if onAddComment}
			<button
				type="button"
				class="absolute left-0 top-1/2 -translate-y-1/2 -translate-x-1/2 w-5 h-5 rounded-full bg-primary-600 text-white flex items-center justify-center opacity-0 group-hover:opacity-100 hover:bg-primary-500 hover:shadow-glow transition-all z-10"
				aria-label="Add line comment"
				onclick={onAddComment}
			>
				<svg class="w-3 h-3" viewBox="0 0 24 24" fill="currentColor">
					<path d="M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6v2z" />
				</svg>
			</button>
		{/if}
		{oldNumber ?? ''}
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
