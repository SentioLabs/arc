<script lang="ts">
	interface LabelInfo {
		name: string;
		color?: string;
		description?: string;
	}

	interface Props {
		currentLabels: string[];
		allLabels: LabelInfo[];
		onAdd: (labelName: string) => Promise<void>;
		onRemove: (labelName: string) => Promise<void>;
	}

	let { currentLabels, allLabels, onAdd, onRemove }: Props = $props();

	let open = $state(false);
	let search = $state('');
	let triggerEl: HTMLButtonElement;
	// svelte-ignore non_reactive_update
	let dropdownEl: HTMLDivElement;
	// svelte-ignore non_reactive_update
	let searchEl: HTMLInputElement;

	const availableLabels = $derived(
		allLabels
			.filter((l) => !currentLabels.includes(l.name))
			.filter((l) => l.name.toLowerCase().includes(search.toLowerCase()))
	);

	function getLabelColor(name: string): string | undefined {
		return allLabels.find((l) => l.name === name)?.color;
	}

	function toggleDropdown() {
		open = !open;
		if (open) {
			search = '';
			requestAnimationFrame(() => searchEl?.focus());
		}
	}

	async function addLabel(name: string) {
		try {
			await onAdd(name);
			open = false;
		} catch {
			// Stay open on error
		}
	}

	// Handle click outside to close
	$effect(() => {
		if (!open) return;

		function handleClickOutside(e: MouseEvent) {
			if (
				triggerEl &&
				!triggerEl.contains(e.target as Node) &&
				dropdownEl &&
				!dropdownEl.contains(e.target as Node)
			) {
				open = false;
			}
		}

		document.addEventListener('click', handleClickOutside);
		return () => document.removeEventListener('click', handleClickOutside);
	});
</script>

<div class="flex flex-wrap items-center gap-2">
	{#each currentLabels as label (label)}
		{@const color = getLabelColor(label)}
		<span
			class="px-2 py-1 text-xs font-medium rounded border inline-flex items-center gap-1"
			style={color ? `background-color: ${color}20; color: ${color}; border-color: ${color}40` : ''}
			class:bg-surface-600={!color}
			class:text-text-secondary={!color}
			class:border-transparent={!color}
		>
			{label}
			<button
				type="button"
				onclick={() => onRemove(label)}
				class="hover:opacity-70 cursor-pointer ml-0.5"
				aria-label="Remove label {label}"
			>
				&times;
			</button>
		</span>
	{/each}

	<div class="relative inline-block">
		<button
			bind:this={triggerEl}
			type="button"
			onclick={toggleDropdown}
			class="px-2 py-1 text-xs font-medium text-text-muted border border-dashed border-border rounded cursor-pointer hover:border-primary-500 hover:text-primary-400 transition-colors"
		>
			+ Add
		</button>

		{#if open}
			<div
				bind:this={dropdownEl}
				class="absolute top-full left-0 mt-1 w-56 bg-surface-800 border border-border rounded-lg shadow-lg z-20 overflow-hidden"
			>
				<div class="p-2">
					<input
						bind:this={searchEl}
						bind:value={search}
						type="text"
						class="input w-full text-sm"
						placeholder="Search labels..."
					/>
				</div>
				<ul class="max-h-48 overflow-y-auto">
					{#each availableLabels as label (label.name)}
						<!-- svelte-ignore a11y_click_events_have_key_events -->
						<li
							role="option"
							aria-selected={false}
							class="flex items-center gap-2 px-3 py-1.5 text-sm cursor-pointer hover:bg-surface-700 transition-colors"
							onclick={() => addLabel(label.name)}
						>
							<span
								class="w-2 h-2 rounded-full flex-shrink-0"
								style={label.color
									? `background-color: ${label.color}`
									: 'background-color: var(--color-text-muted)'}
							></span>
							<span class="truncate">{label.name}</span>
						</li>
					{:else}
						<li class="px-3 py-2 text-sm text-text-muted">No labels found</li>
					{/each}
				</ul>
			</div>
		{/if}
	</div>
</div>
