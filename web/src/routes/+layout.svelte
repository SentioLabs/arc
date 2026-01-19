<script lang="ts">
	import '../app.css';
	import { Sidebar } from '$lib/components';
	import { listWorkspaces, type Workspace } from '$lib/api';
	import { page } from '$app/stores';
	import { setContext } from 'svelte';
	import { writable } from 'svelte/store';

	// Create stores for workspaces that can be accessed by child components
	const workspacesStore = writable<Workspace[]>([]);
	const loadingStore = writable(true);
	const errorStore = writable<string | null>(null);

	setContext('workspaces', workspacesStore);
	setContext('workspacesLoading', loadingStore);
	setContext('workspacesError', errorStore);

	// Current workspace from URL
	const currentWorkspaceId = $derived($page.params.workspaceId);
	const currentWorkspace = $derived(
		$workspacesStore.find((ws) => ws.id === currentWorkspaceId)
	);

	// Load workspaces on mount
	$effect(() => {
		loadWorkspaces();
	});

	async function loadWorkspaces() {
		loadingStore.set(true);
		errorStore.set(null);
		try {
			const workspaces = await listWorkspaces();
			workspacesStore.set(workspaces);
		} catch (err) {
			errorStore.set(err instanceof Error ? err.message : 'Failed to load workspaces');
		} finally {
			loadingStore.set(false);
		}
	}

	let { children } = $props();
</script>

<svelte:head>
	<title>Arc - Issue Tracker</title>
	<meta name="description" content="AI-assisted issue tracking for modern development workflows" />
</svelte:head>

<div class="flex min-h-screen bg-surface-900">
	<Sidebar workspaces={$workspacesStore} {currentWorkspace} />

	<main class="flex-1 min-w-0 flex flex-col">
		{#if $loadingStore}
			<div class="flex-1 flex items-center justify-center">
				<div class="text-text-muted animate-pulse">Loading...</div>
			</div>
		{:else if $errorStore}
			<div class="flex-1 flex items-center justify-center p-8">
				<div class="card p-8 text-center max-w-md">
					<div class="w-12 h-12 bg-status-blocked/20 rounded-xl flex items-center justify-center mx-auto mb-4">
						<svg class="w-6 h-6 text-status-blocked" viewBox="0 0 24 24" fill="currentColor">
							<path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z"/>
						</svg>
					</div>
					<h2 class="text-lg font-semibold text-text-primary mb-2">Connection Error</h2>
					<p class="text-sm text-text-secondary mb-4">{$errorStore}</p>
					<button class="btn btn-primary" onclick={loadWorkspaces}>Retry</button>
				</div>
			</div>
		{:else}
			{@render children()}
		{/if}
	</main>
</div>
