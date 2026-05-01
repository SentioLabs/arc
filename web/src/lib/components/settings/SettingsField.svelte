<script lang="ts">
	import type { Snippet } from 'svelte';
	let { label, help, error, requiresRestart, children }: {
		label: string;
		help?: string;
		error?: string;
		requiresRestart?: boolean;
		children: Snippet;
	} = $props();
</script>

<label class="block">
	<span class="block text-sm font-medium text-text-primary mb-1">
		{label}
		{#if requiresRestart}
			<span class="ml-2 text-xs text-text-muted">↻ requires restart</span>
		{/if}
	</span>
	{@render children()}
	{#if help && !error}
		<span class="block text-xs text-text-muted mt-1">{help}</span>
	{/if}
	{#if error}
		<span class="block text-xs text-status-blocked mt-1">{error}</span>
	{/if}
</label>
