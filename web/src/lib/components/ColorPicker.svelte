<script lang="ts">
	const palette = [
		'#ef4444', '#f97316', '#f59e0b', '#eab308',
		'#84cc16', '#22c55e', '#14b8a6', '#06b6d4',
		'#3b82f6', '#6366f1', '#8b5cf6', '#a855f7',
		'#d946ef', '#ec4899', '#f43f5e', '#6b7280',
	];

	interface Props {
		value: string;
		onchange: (color: string) => void;
	}

	let { value, onchange }: Props = $props();

	let hexInput = $state('');

	// Keep hexInput in sync when value changes externally
	$effect(() => {
		hexInput = value || '';
	});

	function selectColor(color: string) {
		hexInput = color;
		onchange(color);
	}

	function handleHexInput() {
		const cleaned = hexInput.trim();
		if (/^#[0-9a-fA-F]{6}$/.test(cleaned)) {
			onchange(cleaned);
		}
	}
</script>

<div class="space-y-3">
	<!-- Palette grid -->
	<div class="grid grid-cols-8 gap-1.5">
		{#each palette as color (color)}
			<button
				type="button"
				class="w-7 h-7 rounded-md border-2 transition-all flex items-center justify-center
					{value === color ? 'border-white scale-110' : 'border-transparent hover:border-border-focus/50 hover:scale-105'}"
				style="background-color: {color}"
				onclick={() => selectColor(color)}
				title={color}
			>
				{#if value === color}
					<svg class="w-3.5 h-3.5 text-white drop-shadow-sm" viewBox="0 0 24 24" fill="currentColor">
						<path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z" />
					</svg>
				{/if}
			</button>
		{/each}
	</div>

	<!-- Hex input -->
	<div class="flex items-center gap-2">
		{#if value}
			<div
				class="w-7 h-7 rounded-md border border-border flex-shrink-0"
				style="background-color: {value}"
			></div>
		{/if}
		<input
			type="text"
			placeholder="#000000"
			bind:value={hexInput}
			oninput={handleHexInput}
			class="input text-sm font-mono flex-1"
			maxlength="7"
		/>
	</div>
</div>
