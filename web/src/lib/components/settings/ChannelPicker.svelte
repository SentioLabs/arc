<script lang="ts">
	type Channel = 'stable' | 'rc' | 'nightly';
	let { value, onChange, label = 'Channel', disabled = false }: {
		value: Channel;
		onChange: (v: Channel) => void;
		label?: string;
		disabled?: boolean;
	} = $props();
	const options: Channel[] = ['stable', 'rc', 'nightly'];
</script>

<fieldset class="border-0 p-0 m-0">
	<legend class="sr-only">{label}</legend>
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
</fieldset>
