<script lang="ts">
	const palette = [
		'#ef4444', // red
		'#f97316', // orange
		'#eab308', // yellow
		'#22c55e', // green
		'#06b6d4', // cyan
		'#3b82f6', // blue
		'#8b5cf6', // purple
		'#6b7280' // gray
	];

	interface Props {
		value: string;
		onchange: (color: string) => void;
	}

	let { value, onchange }: Props = $props();

	let hexInput = $state('');
	let colorInputEl: HTMLInputElement | undefined = $state();

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

	function openCustomPicker() {
		colorInputEl?.click();
	}

	function handleCustomColor(e: Event) {
		const target = e.target as HTMLInputElement;
		selectColor(target.value);
	}

	const isCustomColor = $derived(value && !palette.includes(value));
</script>

<div class="space-y-3">
	<!-- Presets + custom color trigger -->
	<div class="flex flex-wrap items-center gap-2">
		{#each palette as color (color)}
			<button
				type="button"
				class="h-7 w-7 rounded-md border-2 transition-all flex items-center justify-center
					{value === color
					? 'border-white scale-110'
					: 'border-transparent hover:border-border-focus/50 hover:scale-105'}"
				style="background-color: {color}"
				onclick={() => selectColor(color)}
				title={color}
			>
				{#if value === color}
					<svg
						class="w-3.5 h-3.5 text-white drop-shadow-sm"
						viewBox="0 0 24 24"
						fill="currentColor"
					>
						<path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z" />
					</svg>
				{/if}
			</button>
		{/each}

		<!-- Custom color button -->
		<button
			type="button"
			class="h-7 w-7 rounded-md border-2 transition-all flex items-center justify-center
				{isCustomColor
				? 'border-white scale-110'
				: 'border-transparent hover:border-border-focus/50 hover:scale-105'}"
			style={isCustomColor
				? `background-color: ${value}`
				: 'background: conic-gradient(#ef4444, #f97316, #eab308, #22c55e, #06b6d4, #3b82f6, #8b5cf6, #ef4444)'}
			onclick={openCustomPicker}
			title="Custom color"
		>
			{#if isCustomColor}
				<svg class="w-3.5 h-3.5 text-white drop-shadow-sm" viewBox="0 0 24 24" fill="currentColor">
					<path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z" />
				</svg>
			{/if}
		</button>

		<input
			type="color"
			bind:this={colorInputEl}
			value={value || '#3b82f6'}
			onchange={handleCustomColor}
			class="sr-only"
			tabindex={-1}
		/>
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
