<script lang="ts">
	import '../app.css';
	import { Sidebar } from '$lib/components';
	import { listProjects, type Project } from '$lib/api';
	import { page } from '$app/stores';
	import { setContext } from 'svelte';
	import { writable } from 'svelte/store';

	// Create stores for projects that can be accessed by child components
	const projectsStore = writable<Project[]>([]);
	const loadingStore = writable(true);
	const errorStore = writable<string | null>(null);

	setContext('projects', projectsStore);
	setContext('projectsLoading', loadingStore);
	setContext('projectsError', errorStore);

	// Current project from URL
	const currentProjectId = $derived($page.params.projectId);
	const currentProject = $derived($projectsStore.find((p) => p.id === currentProjectId));

	// /share/[id] is a focused review surface that opts out of arc's app shell.
	// On a public arc-paste deploy, /api/v1/projects doesn't exist, so loading
	// projects there would put the entire SPA into an error state and the
	// share page would never render. Even on local arc-server, hiding arc's
	// chrome keeps the share URL behaving consistently across hosts.
	const isShareRoute = $derived($page.url.pathname.startsWith('/share/'));

	// Load projects on mount (skipped on /share routes)
	$effect(() => {
		if (!isShareRoute) loadProjects();
	});

	async function loadProjects() {
		loadingStore.set(true);
		errorStore.set(null);
		try {
			const projects = await listProjects();
			projectsStore.set(projects);
		} catch (err) {
			errorStore.set(err instanceof Error ? err.message : 'Failed to load projects');
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

{#if isShareRoute}
	<!-- Focused share-review mode: no arc chrome, no projects fetch. -->
	{@render children()}
{:else}
	<div class="flex min-h-screen bg-surface-900">
		<Sidebar projects={$projectsStore} {currentProject} />

		<main class="flex-1 min-w-0 flex flex-col">
			{#if $loadingStore}
				<div class="flex-1 flex items-center justify-center">
					<div class="text-text-muted animate-pulse">Loading...</div>
				</div>
			{:else if $errorStore}
				<div class="flex-1 flex items-center justify-center p-8">
					<div class="card p-8 text-center max-w-md">
						<div
							class="w-12 h-12 bg-status-blocked/20 rounded-xl flex items-center justify-center mx-auto mb-4"
						>
							<svg class="w-6 h-6 text-status-blocked" viewBox="0 0 24 24" fill="currentColor">
								<path
									d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z"
								/>
							</svg>
						</div>
						<h2 class="text-lg font-semibold text-text-primary mb-2">Connection Error</h2>
						<p class="text-sm text-text-secondary mb-4">{$errorStore}</p>
						<button class="btn btn-primary" onclick={loadProjects}>Retry</button>
					</div>
				</div>
			{:else}
				{@render children()}
			{/if}
		</main>
	</div>
{/if}
