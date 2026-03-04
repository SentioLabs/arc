<script lang="ts">
	interface Props {
		type: 'insert' | 'delete' | 'context';
		oldNumber: number | undefined;
		newNumber: number | undefined;
		highlightedHtml: string;
	}

	let { type, oldNumber, newNumber, highlightedHtml }: Props = $props();

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

<tr class={rowClass}>
	<td
		class="select-none text-right pr-2 font-mono text-xs text-text-muted w-[3rem] min-w-[3rem] {gutterClass}"
	>
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
