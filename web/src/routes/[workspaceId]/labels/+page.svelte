<script lang="ts">
	import { Header } from '$lib/components';
	import { page } from '$app/stores';
	import { getContext } from 'svelte';
	import type { Writable } from 'svelte/store';
	import { listLabels, type Workspace, type Label } from '$lib/api';

	const workspaces = getContext<Writable<Workspace[]>>('workspaces');
	const workspaceId = $derived($page.params.workspaceId);
	const workspace = $derived($workspaces.find((ws) => ws.id === workspaceId));

	let labels = $state<Label[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	$effect(() => {
		if (workspaceId) loadLabels();
	});

	async function loadLabels() {
		if (!workspaceId) return;
		loading = true;
		error = null;
		try {
			labels = await listLabels(workspaceId);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load labels';
		} finally {
			loading = false;
		}
	}
</script>

{#if workspace}
	<Header {workspace} title="Labels" />

	<div class="flex-1 p-6 animate-fade-in">
		<header class="mb-6">
			<h1 class="text-2xl font-bold text-text-primary mb-2">Labels</h1>
			<p class="text-text-secondary">{labels.length} labels in this workspace</p>
		</header>

		{#if loading}
			<div class="flex items-center justify-center py-12">
				<div class="text-text-muted animate-pulse">Loading...</div>
			</div>
		{:else if error}
			<div class="card p-8 text-center">
				<p class="text-status-blocked mb-4">{error}</p>
				<button class="btn btn-primary" onclick={loadLabels}>Retry</button>
			</div>
		{:else if labels.length === 0}
			<div class="card p-12 text-center">
				<div
					class="w-16 h-16 bg-surface-700 rounded-2xl flex items-center justify-center mx-auto mb-4"
				>
					<svg class="w-8 h-8 text-text-muted" viewBox="0 0 24 24" fill="currentColor">
						<path
							d="M17.63 5.84C17.27 5.33 16.67 5 16 5L5 5.01C3.9 5.01 3 5.9 3 7v10c0 1.1.9 1.99 2 1.99L16 19c.67 0 1.27-.33 1.63-.84L22 12l-4.37-6.16z"
						/>
					</svg>
				</div>
				<h2 class="text-xl font-semibold text-text-primary mb-2">No labels yet</h2>
				<p class="text-text-secondary mb-4">Create labels using the CLI to organize your issues</p>
				<div
					class="bg-surface-800 rounded-lg p-4 font-mono text-sm text-text-secondary text-left inline-block"
				>
					<code>arc label create "bug" --color "#ef4444"</code>
				</div>
			</div>
		{:else}
			<div class="grid sm:grid-cols-2 lg:grid-cols-3 gap-4">
				{#each labels as label (label.name)}
					<div class="card p-4">
						<div class="flex items-center gap-3 mb-2">
							<div
								class="w-4 h-4 rounded-full"
								style="background-color: {label.color || '#6b7280'}"
							></div>
							<h3 class="font-medium text-text-primary">{label.name}</h3>
						</div>
						{#if label.description}
							<p class="text-sm text-text-secondary">{label.description}</p>
						{:else}
							<p class="text-sm text-text-muted">No description</p>
						{/if}
					</div>
				{/each}
			</div>
		{/if}
	</div>
{/if}
