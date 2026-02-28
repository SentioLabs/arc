<script lang="ts">
	import { Header, IssueCard } from '$lib/components';
	import { page } from '$app/stores';
	import { getContext } from 'svelte';
	import type { Writable } from 'svelte/store';
	import { getReadyWork, listLabels, type Workspace, type Issue, type Label } from '$lib/api';

	const workspaces = getContext<Writable<Workspace[]>>('workspaces');
	const workspaceId = $derived($page.params.workspaceId);
	const workspace = $derived($workspaces.find((ws) => ws.id === workspaceId));

	let issues = $state<Issue[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let labelMap = $state(new Map<string, Label>());

	$effect(() => {
		if (workspaceId) {
			loadIssues();
			loadLabelMap();
		}
	});

	async function loadLabelMap() {
		try {
			const labels = await listLabels();
			labelMap = new Map(labels.map(l => [l.name, l]));
		} catch { /* labels are optional for display */ }
	}

	async function loadIssues() {
		if (!workspaceId) return;
		loading = true;
		error = null;
		try {
			issues = await getReadyWork(workspaceId, { limit: 50 });
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load ready work';
		} finally {
			loading = false;
		}
	}
</script>

{#if workspace}
	<Header {workspace} title="Ready Work" />

	<div class="flex-1 p-6 animate-fade-in">
		<header class="mb-6">
			<div class="flex items-center gap-3 mb-2">
				<div class="w-10 h-10 bg-status-open/20 rounded-lg flex items-center justify-center">
					<svg class="w-5 h-5 text-status-open" viewBox="0 0 24 24" fill="currentColor">
						<path d="M9 16.2L4.8 12l-1.4 1.4L9 19 21 7l-1.4-1.4L9 16.2z" />
					</svg>
				</div>
				<div>
					<h1 class="text-2xl font-bold text-text-primary">Ready Work</h1>
					<p class="text-sm text-text-secondary">
						{issues.length} issues with no blocking dependencies
					</p>
				</div>
			</div>
		</header>

		{#if loading}
			<div class="flex items-center justify-center py-12">
				<div class="text-text-muted animate-pulse">Loading...</div>
			</div>
		{:else if error}
			<div class="card p-8 text-center">
				<p class="text-status-blocked mb-4">{error}</p>
				<button class="btn btn-primary" onclick={loadIssues}>Retry</button>
			</div>
		{:else if issues.length === 0}
			<div class="card p-12 text-center">
				<div
					class="w-16 h-16 bg-surface-700 rounded-2xl flex items-center justify-center mx-auto mb-4"
				>
					<svg class="w-8 h-8 text-text-muted" viewBox="0 0 24 24" fill="currentColor">
						<path d="M9 16.2L4.8 12l-1.4 1.4L9 19 21 7l-1.4-1.4L9 16.2z" />
					</svg>
				</div>
				<h2 class="text-xl font-semibold text-text-primary mb-2">No ready work</h2>
				<p class="text-text-secondary">
					All open issues have blocking dependencies or are already in progress
				</p>
			</div>
		{:else}
			<div class="space-y-3">
				{#each issues as issue (issue.id)}
					<IssueCard {issue} {labelMap} href="/{workspaceId}/issues/{issue.id}" />
				{/each}
			</div>
		{/if}
	</div>
{/if}
