<script lang="ts">
	interface Option {
		value: string;
		label: string;
	}

	interface Props {
		options: Option[];
		value: string;
		placeholder?: string;
		onchange: (value: string) => void;
	}

	let { options, value, placeholder = 'Select...', onchange }: Props = $props();

	let open = $state(false);
	let highlightedIndex = $state(-1);
	let triggerEl: HTMLButtonElement;
	// Using bind:this for click-outside detection - not reactive state
	// svelte-ignore non_reactive_update
	let listEl: HTMLUListElement;

	const selectedOption = $derived(options.find((opt) => opt.value === value));
	const displayLabel = $derived(selectedOption?.label ?? placeholder);

	// Handle click outside to close
	$effect(() => {
		if (!open) return;

		function handleClickOutside(e: MouseEvent) {
			if (triggerEl && !triggerEl.contains(e.target as Node) && listEl && !listEl.contains(e.target as Node)) {
				open = false;
			}
		}

		document.addEventListener('click', handleClickOutside);
		return () => document.removeEventListener('click', handleClickOutside);
	});

	// Reset highlighted index when opening
	$effect(() => {
		if (open) {
			const currentIndex = options.findIndex((opt) => opt.value === value);
			highlightedIndex = currentIndex >= 0 ? currentIndex : 0;
		}
	});

	function toggle() {
		open = !open;
	}

	function select(option: Option) {
		onchange(option.value);
		open = false;
		triggerEl?.focus();
	}

	function handleKeydown(e: KeyboardEvent) {
		if (!open) {
			if (e.key === 'Enter' || e.key === ' ' || e.key === 'ArrowDown') {
				e.preventDefault();
				open = true;
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
					select(options[highlightedIndex]);
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
		class="select-trigger"
		aria-haspopup="listbox"
		aria-expanded={open}
		onclick={toggle}
		onkeydown={handleKeydown}
	>
		<span class="truncate">{displayLabel}</span>
		<svg
			class="select-chevron {open ? 'rotate-180' : ''}"
			viewBox="0 0 24 24"
			fill="currentColor"
		>
			<path d="M7 10l5 5 5-5z" />
		</svg>
	</button>

	{#if open}
		<ul
			bind:this={listEl}
			class="select-dropdown"
			role="listbox"
			tabindex="-1"
			aria-activedescendant={highlightedIndex >= 0 ? `option-${highlightedIndex}` : undefined}
		>
			{#each options as option, i (option.value)}
				<!-- svelte-ignore a11y_click_events_have_key_events -->
				<li
					id="option-{i}"
					role="option"
					aria-selected={option.value === value}
					class="select-option {i === highlightedIndex ? 'highlighted' : ''} {option.value === value ? 'selected' : ''}"
					onclick={() => select(option)}
					onmouseenter={() => (highlightedIndex = i)}
				>
					<span class="truncate">{option.label}</span>
					{#if option.value === value}
						<svg class="w-4 h-4 flex-shrink-0" viewBox="0 0 24 24" fill="currentColor">
							<path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z" />
						</svg>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}
</div>
