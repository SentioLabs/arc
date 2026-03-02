<script lang="ts">
	interface Props {
		value: string;
		onSave: (newValue: string) => Promise<void>;
		placeholder?: string;
		class?: string;
	}

	let { value, onSave, placeholder = 'Click to edit', class: className = '' }: Props = $props();

	let editing = $state(false);
	let editValue = $state(value);
	let saving = $state(false);
	let inputEl: HTMLInputElement;

	// Sync editValue when value prop changes externally
	$effect(() => {
		editValue = value;
	});

	function startEdit() {
		editValue = value;
		editing = true;
		// Focus input after DOM update
		requestAnimationFrame(() => inputEl?.focus());
	}

	async function save() {
		const trimmed = editValue.trim();
		if (!trimmed || trimmed === value) {
			cancel();
			return;
		}
		saving = true;
		try {
			await onSave(trimmed);
			editing = false;
		} catch {
			// Revert on error - stay in edit mode
		} finally {
			saving = false;
		}
	}

	function cancel() {
		editValue = value;
		editing = false;
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
			save();
		}
		if (e.key === 'Escape') {
			e.preventDefault();
			cancel();
		}
	}
</script>

<div class={className}>
	{#if editing}
		<input
			bind:this={inputEl}
			type="text"
			bind:value={editValue}
			onblur={save}
			onkeydown={handleKeydown}
			disabled={saving}
			class="input w-full"
			{placeholder}
		/>
	{:else}
		<button
			type="button"
			onclick={startEdit}
			class="cursor-pointer hover:bg-surface-700/50 rounded px-1 -mx-1 transition-colors text-left w-full"
		>
			{#if value}
				<span>{value}</span>
			{:else}
				<span class="text-text-muted italic">{placeholder}</span>
			{/if}
		</button>
	{/if}
</div>
