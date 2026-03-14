<script lang="ts">
	interface Option {
		value: string;
		label: string;
	}

	interface Props {
		options: Option[];
		values: string[];
		placeholder?: string;
		onchange: (values: string[]) => void;
	}

	let { options, values, placeholder = 'Select...', onchange }: Props = $props();

	let open = $state(false);
	let highlightedIndex = $state(-1);
	let triggerEl: HTMLButtonElement;
	// Using bind:this for click-outside detection - not reactive state
	// svelte-ignore non_reactive_update
	let listEl: HTMLUListElement;

	const selectedOptions = $derived(options.filter((opt) => values.includes(opt.value)));
	const visibleChips = $derived(selectedOptions.slice(0, 2));
	const overflowCount = $derived(selectedOptions.length - 2);

	// Handle click outside to close
	$effect(() => {
		if (!open) return;

		function handleClickOutside(e: MouseEvent) {
			if (
				triggerEl &&
				!triggerEl.contains(e.target as Node) &&
				listEl &&
				!listEl.contains(e.target as Node)
			) {
				open = false;
			}
		}

		document.addEventListener('click', handleClickOutside);
		return () => document.removeEventListener('click', handleClickOutside);
	});

	// Reset highlighted index when opening
	$effect(() => {
		if (open) {
			const firstSelectedIndex = options.findIndex((opt) => values.includes(opt.value));
			highlightedIndex = firstSelectedIndex >= 0 ? firstSelectedIndex : 0;
		}
	});

	function toggle() {
		open = !open;
	}

	function toggleOption(option: Option) {
		if (values.includes(option.value)) {
			onchange(values.filter((v) => v !== option.value));
		} else {
			onchange([...values, option.value]);
		}
	}

	function removeValue(val: string, e: MouseEvent) {
		e.stopPropagation();
		onchange(values.filter((v) => v !== val));
	}

	function handleKeydown(e: KeyboardEvent) {
		if (!open) {
			if (e.key === 'Enter' || e.key === ' ' || e.key === 'ArrowDown') {
				e.preventDefault();
				open = true;
				return;
			}
			if (e.key === 'Backspace' && values.length > 0) {
				e.preventDefault();
				onchange(values.slice(0, -1));
				return;
			}
			return;
		}

		switch (e.key) {
			case 'Escape':
				e.preventDefault();
				open = false;
				triggerEl?.focus();
				break;
			case 'ArrowDown':
				e.preventDefault();
				highlightedIndex = (highlightedIndex + 1) % options.length;
				break;
			case 'ArrowUp':
				e.preventDefault();
				highlightedIndex = highlightedIndex <= 0 ? options.length - 1 : highlightedIndex - 1;
				break;
			case 'Enter':
			case ' ':
				e.preventDefault();
				if (highlightedIndex >= 0 && highlightedIndex < options.length) {
					toggleOption(options[highlightedIndex]);
				}
				break;
			case 'Tab':
				open = false;
				break;
		}
	}
</script>

<div class="relative inline-block">
	<button
		bind:this={triggerEl}
		type="button"
		class="select-trigger flex-wrap"
		aria-haspopup="listbox"
		aria-expanded={open}
		onclick={toggle}
		onkeydown={handleKeydown}
	>
		{#if values.length === 0}
			<span class="truncate">{placeholder}</span>
		{:else}
			{#each visibleChips as chip (chip.value)}
				<span class="multi-select-chip">
					{chip.label}
					<!-- svelte-ignore a11y_click_events_have_key_events -->
					<button type="button" onclick={(e) => removeValue(chip.value, e)} aria-label="Remove {chip.label}">
						&times;
					</button>
				</span>
			{/each}
			{#if overflowCount > 0}
				<span class="multi-select-chip">+{overflowCount}</span>
			{/if}
		{/if}
		<svg class="select-chevron {open ? 'rotate-180' : ''}" viewBox="0 0 24 24" fill="currentColor">
			<path d="M7 10l5 5 5-5z" />
		</svg>
	</button>

	{#if open}
		<ul
			bind:this={listEl}
			class="select-dropdown"
			role="listbox"
			aria-multiselectable="true"
			tabindex="-1"
			aria-activedescendant={highlightedIndex >= 0 ? `multi-option-${highlightedIndex}` : undefined}
		>
			{#each options as option, i (option.value)}
				<!-- svelte-ignore a11y_click_events_have_key_events -->
				<li
					id="multi-option-{i}"
					role="option"
					aria-selected={values.includes(option.value)}
					class="select-option {i === highlightedIndex ? 'highlighted' : ''} {values.includes(option.value)
						? 'selected'
						: ''}"
					onclick={() => toggleOption(option)}
					onmouseenter={() => (highlightedIndex = i)}
				>
					<svg
						class="w-4 h-4 flex-shrink-0"
						viewBox="0 0 24 24"
						fill={values.includes(option.value) ? 'currentColor' : 'none'}
						stroke="currentColor"
						stroke-width="2"
					>
						{#if values.includes(option.value)}
							<path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z" stroke="none" />
						{:else}
							<rect x="3" y="3" width="18" height="18" rx="2" fill="none" />
						{/if}
					</svg>
					<span class="truncate">{option.label}</span>
				</li>
			{/each}
		</ul>
	{/if}
</div>
