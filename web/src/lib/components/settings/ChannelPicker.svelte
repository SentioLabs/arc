<script lang="ts">
	type Channel = 'stable' | 'rc' | 'nightly';
	let { value, onChange, disabled = false }: {
		value: Channel;
		onChange: (v: Channel) => void;
		disabled?: boolean;
	} = $props();
	const options: Channel[] = ['stable', 'rc', 'nightly'];
</script>

<div class="flex gap-4">
	{#each options as opt}
		<label class="inline-flex items-center gap-2 text-sm">
			<input
				type="radio"
				name="channel"
				value={opt}
				checked={value === opt}
				{disabled}
				onchange={() => onChange(opt)}
			/>
			{opt}
		</label>
	{/each}
</div>
