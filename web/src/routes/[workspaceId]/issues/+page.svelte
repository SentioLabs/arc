<script lang="ts">
	import { Header, IssueCard, Select } from '$lib/components';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { getContext } from 'svelte';
	import type { Writable } from 'svelte/store';
	import { listIssues, type Workspace, type Issue, type IssueFilters } from '$lib/api';

	// Get workspace from context
	const workspaces = getContext<Writable<Workspace[]>>('workspaces');
	const workspaceId = $derived($page.params.workspaceId);
	const workspace = $derived($workspaces.find((ws) => ws.id === workspaceId));

	// Local state
	let issues = $state<Issue[]>([]);
	let total = $state(0);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Get filters from URL
	const filters = $derived<IssueFilters>({
		status: $page.url.searchParams.get('status') || undefined,
		type: $page.url.searchParams.get('type') || undefined,
		priority: $page.url.searchParams.get('priority')
			? parseInt($page.url.searchParams.get('priority')!)
			: undefined,
		q: $page.url.searchParams.get('q') || undefined,
		limit: 50,
		offset: $page.url.searchParams.get('offset')
			? parseInt($page.url.searchParams.get('offset')!)
			: 0
	});

	// Load issues when filters change
	$effect(() => {
		if (workspaceId) {
			loadIssues();
		}
	});

	async function loadIssues() {
		if (!workspaceId) return;
		loading = true;
		error = null;
		try {
			const result = await listIssues(workspaceId, filters);
			issues = result.data ?? [];
			total = result.total ?? issues.length;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load issues';
		} finally {
			loading = false;
		}
	}

	// Status options
	const statuses = [
		{ value: '', label: 'All Statuses' },
		{ value: 'open', label: 'Open' },
		{ value: 'in_progress', label: 'In Progress' },
		{ value: 'blocked', label: 'Blocked' },
		{ value: 'deferred', label: 'Deferred' },
		{ value: 'closed', label: 'Closed' }
	];

	// Type options
	const types = [
		{ value: '', label: 'All Types' },
		{ value: 'bug', label: 'Bug' },
		{ value: 'feature', label: 'Feature' },
		{ value: 'task', label: 'Task' },
		{ value: 'epic', label: 'Epic' },
		{ value: 'chore', label: 'Chore' }
	];

	// Priority options
	const priorities = [
		{ value: '', label: 'All Priorities' },
		{ value: '0', label: 'Critical (P0)' },
		{ value: '1', label: 'High (P1)' },
		{ value: '2', label: 'Medium (P2)' },
		{ value: '3', label: 'Low (P3)' },
		{ value: '4', label: 'Backlog (P4)' }
	];

	function updateFilter(key: string, value: string) {
		const params = new URLSearchParams($page.url.searchParams);
		if (value) {
			params.set(key, value);
		} else {
			params.delete(key);
		}
		params.delete('offset');
		goto(`?${params.toString()}`, { keepFocus: true });
	}

	function clearFilters() {
		goto(`/${workspaceId}/issues`);
	}

	const hasActiveFilters = $derived(
		filters.status || filters.type || filters.priority !== undefined || filters.q
	);
</script>

{#if workspace}
	<Header {workspace} title="Issues" />

	<div class="flex-1 p-6 animate-fade-in">
		<!-- Filters -->
		<div class="flex flex-wrap items-center gap-3 mb-6">
			<Select
				options={statuses}
				value={filters.status ?? ''}
				onchange={(v) => updateFilter('status', v)}
			/>

			<Select
				options={types}
				value={filters.type ?? ''}
				onchange={(v) => updateFilter('type', v)}
			/>

			<Select
				options={priorities}
				value={filters.priority?.toString() ?? ''}
				onchange={(v) => updateFilter('priority', v)}
			/>

			{#if hasActiveFilters}
				<button type="button" class="btn btn-ghost text-sm" onclick={clearFilters}>
					Clear filters
				</button>
			{/if}

			<div class="flex-1"></div>

			<span class="text-sm text-text-muted">
				{total} issues
			</span>
		</div>

		<!-- Content -->
		{#if loading}
			<div class="flex items-center justify-center py-12">
				<div class="text-text-muted animate-pulse">Loading issues...</div>
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
						<path d="M4 6h16v2H4V6zm0 5h16v2H4v-2zm0 5h16v2H4v-2z" />
					</svg>
				</div>
				{#if hasActiveFilters}
					<h2 class="text-xl font-semibold text-text-primary mb-2">No matching issues</h2>
					<p class="text-text-secondary mb-4">Try adjusting your filters</p>
					<button type="button" class="btn btn-primary" onclick={clearFilters}>
						Clear filters
					</button>
				{:else}
					<h2 class="text-xl font-semibold text-text-primary mb-2">No issues yet</h2>
					<p class="text-text-secondary mb-4">Create your first issue to get started</p>
				{/if}
			</div>
		{:else}
			<div class="space-y-3">
				{#each issues as issue (issue.id)}
					<IssueCard {issue} href="/{workspaceId}/issues/{issue.id}" />
				{/each}
			</div>

			<!-- Pagination -->
			{#if total > issues.length}
				<div class="flex justify-center gap-2 mt-8">
					{#if (filters.offset ?? 0) > 0}
						<button
							type="button"
							class="btn btn-ghost"
							onclick={() => {
								const newOffset = Math.max(0, (filters.offset ?? 0) - 50);
								updateFilter('offset', newOffset > 0 ? newOffset.toString() : '');
							}}
						>
							Previous
						</button>
					{/if}
					<button
						type="button"
						class="btn btn-ghost"
						onclick={() => {
							const newOffset = (filters.offset ?? 0) + 50;
							updateFilter('offset', newOffset.toString());
						}}
					>
						Next
					</button>
				</div>
			{/if}
		{/if}
	</div>
{/if}
