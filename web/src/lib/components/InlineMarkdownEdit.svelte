<script lang="ts">
	import Markdown from './Markdown.svelte';

	interface Props {
		value: string;
		onSave: (newValue: string) => Promise<void>;
		placeholder?: string;
	}

	let { value, onSave, placeholder = 'Click to edit' }: Props = $props();

	let editing = $state(false);
	let editValue = $state(value);
	let saving = $state(false);
	let textareaEl: HTMLTextAreaElement;

	// Sync editValue when value prop changes externally
	$effect(() => {
		if (!editing) editValue = value;
	});

	function startEdit() {
		editValue = value;
		editing = true;
		requestAnimationFrame(() => textareaEl?.focus());
	}

	async function save() {
		const trimmed = editValue.trim();
		if (trimmed === value.trim()) {
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
</script>

<div class="relative">
	{#if editing}
		<textarea
			bind:this={textareaEl}
			bind:value={editValue}
			disabled={saving}
			class="input w-full min-h-[120px] font-mono text-sm"
			{placeholder}
		></textarea>
		<div class="flex gap-2 mt-2">
			<button
				type="button"
				onclick={save}
				disabled={saving}
				class="btn btn-primary text-sm"
			>
				{saving ? 'Saving...' : 'Save'}
			</button>
			<button
				type="button"
				onclick={cancel}
				disabled={saving}
				class="btn btn-ghost text-sm"
			>
				Cancel
			</button>
		</div>
	{:else}
		{#if value}
			<button
				type="button"
				onclick={startEdit}
				class="absolute top-1 right-1 text-xs text-text-muted hover:text-text-primary cursor-pointer transition-colors z-10"
			>
				Edit
			</button>
			<Markdown content={value} />
		{:else}
			<button
				type="button"
				onclick={startEdit}
				class="w-full text-left cursor-pointer hover:bg-surface-700/50 rounded px-2 py-2 transition-colors"
			>
				<span class="text-text-muted italic">{placeholder}</span>
			</button>
		{/if}
	{/if}
</div>
