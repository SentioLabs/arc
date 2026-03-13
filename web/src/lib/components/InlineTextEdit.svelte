<script lang="ts">
	interface Props {
		value: string;
		onSave: (newValue: string) => Promise<void>;
		placeholder?: string;
		class?: string;
		inputClass?: string;
		showButtons?: boolean;
		errorMessage?: string;
	}

	let {
		value,
		onSave,
		placeholder = 'Click to edit',
		class: className = '',
		inputClass = '',
		showButtons = false,
		errorMessage = ''
	}: Props = $props();

	let editing = $state(false);
	// svelte-ignore state_referenced_locally
	let editValue = $state(value);
	let saving = $state(false);
	// svelte-ignore non_reactive_update
	let inputEl: HTMLInputElement;

	// Sync editValue when value prop changes externally
	$effect(() => {
		if (!editing) editValue = value;
	});

	export function startEdit() {
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
			// Stay in edit mode on error
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

	function handleBlur(e: FocusEvent) {
		if (showButtons) {
			// With buttons, don't save on blur — let the buttons handle it.
			// But if focus leaves the entire edit area (not to save/cancel), cancel.
			const related = e.relatedTarget as HTMLElement | null;
			if (related?.closest('.inline-edit-actions')) return;
			cancel();
		} else {
			save();
		}
	}
</script>

<div class={className}>
	{#if editing}
		<div class="flex items-center gap-2">
			<input
				bind:this={inputEl}
				type="text"
				bind:value={editValue}
				onblur={handleBlur}
				onkeydown={handleKeydown}
				disabled={saving}
				class={inputClass || 'input w-full'}
				{placeholder}
			/>
			{#if showButtons}
				<div class="inline-edit-actions flex items-center gap-1.5 shrink-0">
					<button
						type="button"
						class="btn btn-primary btn-sm"
						disabled={saving || !editValue.trim() || editValue.trim() === value}
						onmousedown={(e: MouseEvent) => e.preventDefault()}
						onclick={save}
					>
						{#if saving}
							<svg class="w-3.5 h-3.5 animate-spin" viewBox="0 0 24 24" fill="none">
								<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
								<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
							</svg>
						{:else}
							Save
						{/if}
					</button>
					<button
						type="button"
						class="btn btn-ghost btn-sm"
						disabled={saving}
						onmousedown={(e: MouseEvent) => e.preventDefault()}
						onclick={cancel}
					>
						Cancel
					</button>
				</div>
			{/if}
		</div>
		{#if errorMessage}
			<p class="text-xs text-status-blocked mt-1">{errorMessage}</p>
		{/if}
	{:else}
		<button
			type="button"
			onclick={startEdit}
			ondblclick={startEdit}
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
