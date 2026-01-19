<script lang="ts">
	import { Header } from '$lib/components';
	import { page } from '$app/stores';
	import { getContext } from 'svelte';
	import type { Writable } from 'svelte/store';
	import { getWorkspaceStats, type Workspace, type Statistics } from '$lib/api';

	// Get workspaces from context
	const workspaces = getContext<Writable<Workspace[]>>('workspaces');
	const workspaceId = $derived($page.params.workspaceId);
	const workspace = $derived($workspaces.find((ws) => ws.id === workspaceId));

	// Local state for stats
	let stats = $state<Statistics | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Load stats when workspace changes
	$effect(() => {
		if (workspaceId) {
			loadStats();
		}
	});

	async function loadStats() {
		if (!workspaceId) return;
		loading = true;
		error = null;
		try {
			stats = await getWorkspaceStats(workspaceId);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load stats';
		} finally {
			loading = false;
		}
	}

	// Dashboard stat cards - derived from stats
	const statCards = $derived(
		stats
			? [
					{ label: 'Total Issues', value: stats.total_issues, color: 'primary' },
					{ label: 'Open', value: stats.open_issues, color: 'status-open' },
					{ label: 'In Progress', value: stats.in_progress_issues, color: 'status-in-progress' },
					{ label: 'Ready', value: stats.ready_issues, color: 'primary' },
					{ label: 'Blocked', value: stats.blocked_issues, color: 'status-blocked' },
					{ label: 'Closed', value: stats.closed_issues, color: 'status-closed' }
				]
			: []
	);
</script>

{#if workspace}
	<Header {workspace} />

	<div class="flex-1 p-8 animate-fade-in">
		<header class="mb-8">
			<h1 class="text-3xl font-bold text-text-primary mb-2">
				{workspace.name}
			</h1>
			{#if workspace.description}
				<p class="text-text-secondary">{workspace.description}</p>
			{/if}
		</header>

		{#if loading}
			<div class="flex items-center justify-center py-12">
				<div class="text-text-muted animate-pulse">Loading stats...</div>
			</div>
		{:else if error}
			<div class="card p-8 text-center">
				<p class="text-status-blocked mb-4">{error}</p>
				<button class="btn btn-primary" onclick={loadStats}>Retry</button>
			</div>
		{:else if stats}
			<!-- Stats Grid -->
			<div class="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4 mb-8">
				{#each statCards as stat, i (stat.label)}
					<div class="card p-4 animate-slide-up" style="animation-delay: {i * 50}ms">
						<div class="text-xs font-medium text-text-muted uppercase tracking-wider mb-2">
							{stat.label}
						</div>
						<div class="text-2xl font-bold text-{stat.color}-500">
							{stat.value}
						</div>
					</div>
				{/each}
			</div>

			<!-- Quick Actions -->
			<div class="grid md:grid-cols-3 gap-4 mb-8">
				<a
					href="/{workspace.id}/issues"
					class="card p-6 hover:border-border-focus/50 transition-all group"
				>
					<div class="flex items-center gap-4">
						<div
							class="w-12 h-12 bg-surface-700 rounded-xl flex items-center justify-center group-hover:bg-primary-600/20 transition-colors"
						>
							<svg
								class="w-6 h-6 text-text-secondary group-hover:text-primary-400 transition-colors"
								viewBox="0 0 24 24"
								fill="currentColor"
							>
								<path d="M4 6h16v2H4V6zm0 5h16v2H4v-2zm0 5h16v2H4v-2z" />
							</svg>
						</div>
						<div>
							<h3 class="font-medium text-text-primary group-hover:text-white transition-colors">
								All Issues
							</h3>
							<p class="text-sm text-text-muted">Browse and filter issues</p>
						</div>
					</div>
				</a>

				<a
					href="/{workspace.id}/ready"
					class="card p-6 hover:border-border-focus/50 transition-all group"
				>
					<div class="flex items-center gap-4">
						<div
							class="w-12 h-12 bg-surface-700 rounded-xl flex items-center justify-center group-hover:bg-status-open/20 transition-colors"
						>
							<svg
								class="w-6 h-6 text-text-secondary group-hover:text-status-open transition-colors"
								viewBox="0 0 24 24"
								fill="currentColor"
							>
								<path d="M9 16.2L4.8 12l-1.4 1.4L9 19 21 7l-1.4-1.4L9 16.2z" />
							</svg>
						</div>
						<div>
							<h3 class="font-medium text-text-primary group-hover:text-white transition-colors">
								Ready Work
							</h3>
							<p class="text-sm text-text-muted">{stats.ready_issues} issues ready to start</p>
						</div>
					</div>
				</a>

				<a
					href="/{workspace.id}/blocked"
					class="card p-6 hover:border-border-focus/50 transition-all group"
				>
					<div class="flex items-center gap-4">
						<div
							class="w-12 h-12 bg-surface-700 rounded-xl flex items-center justify-center group-hover:bg-status-blocked/20 transition-colors"
						>
							<svg
								class="w-6 h-6 text-text-secondary group-hover:text-status-blocked transition-colors"
								viewBox="0 0 24 24"
								fill="currentColor"
							>
								<path
									d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zM4 12c0-4.42 3.58-8 8-8 1.85 0 3.55.63 4.9 1.69L5.69 16.9C4.63 15.55 4 13.85 4 12zm8 8c-1.85 0-3.55-.63-4.9-1.69L18.31 7.1C19.37 8.45 20 10.15 20 12c0 4.42-3.58 8-8 8z"
								/>
							</svg>
						</div>
						<div>
							<h3 class="font-medium text-text-primary group-hover:text-white transition-colors">
								Blocked
							</h3>
							<p class="text-sm text-text-muted">{stats.blocked_issues} issues blocked</p>
						</div>
					</div>
				</a>
			</div>

			<!-- Lead Time -->
			{#if stats.avg_lead_time_hours}
				<div class="card p-6">
					<h3 class="text-sm font-medium text-text-muted uppercase tracking-wider mb-3">
						Average Lead Time
					</h3>
					<div class="flex items-baseline gap-2">
						<span class="text-3xl font-bold text-text-primary">
							{Math.round(stats.avg_lead_time_hours)}
						</span>
						<span class="text-text-secondary">hours</span>
					</div>
					<p class="text-sm text-text-muted mt-2">From issue creation to close</p>
				</div>
			{/if}
		{/if}
	</div>
{/if}
