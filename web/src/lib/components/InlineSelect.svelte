<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Option {
		value: string;
		label: string;
	}

	interface Props {
		value: string;
		options: Option[];
		onSave: (newValue: string) => Promise<void>;
		children: Snippet;
	}

	let { value, options, onSave, children }: Props = $props();

	const instanceId = crypto.randomUUID().slice(0, 8);
	let open = $state(false);
	let saving = $state(false);
	let highlightedIndex = $state(-1);
	let triggerEl: HTMLButtonElement;
	// Using bind:this for click-outside detection - not reactive state
	// svelte-ignore non_reactive_update
	let listEl: HTMLUListElement;

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
			const currentIndex = options.findIndex((opt) => opt.value === value);
			highlightedIndex = currentIndex >= 0 ? currentIndex : 0;
		}
	});

	function toggle() {
		if (!saving) {
			open = !open;
		}
	}

	async function select(option: Option) {
		if (option.value === value) {
			open = false;
			triggerEl?.focus();
			return;
		}
		saving = true;
		try {
			await onSave(option.value);
			open = false;
			triggerEl?.focus();
		} catch {
			// Stay open on error
		} finally {
			saving = false;
		}
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
		class="cursor-pointer hover:ring-2 hover:ring-primary-500/30 rounded transition-all"
		aria-haspopup="listbox"
		aria-expanded={open}
		disabled={saving}
		onclick={toggle}
		onkeydown={handleKeydown}
	>
		{@render children()}
	</button>

	{#if open}
		<ul
			bind:this={listEl}
			class="select-dropdown"
			role="listbox"
			tabindex="-1"
			aria-activedescendant={highlightedIndex >= 0 ? `select-${instanceId}-option-${highlightedIndex}` : undefined}
		>
			{#each options as option, i (option.value)}
				<!-- svelte-ignore a11y_click_events_have_key_events -->
				<li
					id="select-{instanceId}-option-{i}"
					role="option"
					aria-selected={option.value === value}
					class="select-option {i === highlightedIndex ? 'highlighted' : ''} {option.value === value
						? 'selected'
						: ''}"
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
