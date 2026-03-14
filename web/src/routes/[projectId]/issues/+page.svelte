<script lang="ts">
	import { Header, IssueCard, MultiSelect } from '$lib/components';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { getContext } from 'svelte';
	import type { Writable } from 'svelte/store';
	import {
		listIssues,
		listLabels,
		updateIssue,
		type Project,
		type Issue,
		type Label,
		type IssueFilters,
		type UpdateIssueRequest
	} from '$lib/api';

	// Get project from context
	const projects = getContext<Writable<Project[]>>('projects');
	const projectId = $derived($page.params.projectId);
	const project = $derived($projects.find((p) => p.id === projectId));

	// Local state
	let issues = $state<Issue[]>([]);
	let total = $state(0);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let labelMap = $state(new Map<string, Label>());

	// Get filters from URL
	const filters = $derived<IssueFilters>({
		status:
			$page.url.searchParams.getAll('status').length > 0
				? $page.url.searchParams.getAll('status')
				: undefined,
		type:
			$page.url.searchParams.getAll('type').length > 0
				? $page.url.searchParams.getAll('type')
				: undefined,
		priority:
			$page.url.searchParams.getAll('priority').length > 0
				? $page.url.searchParams.getAll('priority').map(Number)
				: undefined,
		q: $page.url.searchParams.get('q') || undefined,
		limit: 50,
		offset: $page.url.searchParams.get('offset')
			? parseInt($page.url.searchParams.get('offset')!)
			: 0
	});

	// Load issues when filters change
	$effect(() => {
		if (projectId) {
			loadIssues();
			loadLabelMap();
		}
	});

	async function loadLabelMap() {
		try {
			const labels = await listLabels();
			labelMap = new Map(labels.map((l) => [l.name, l]));
		} catch {
			/* labels are optional for display */
		}
	}

	async function loadIssues() {
		if (!projectId) return;
		loading = true;
		error = null;
		try {
			const result = await listIssues(projectId, filters);
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
		{ value: 'open', label: 'Open' },
		{ value: 'in_progress', label: 'In Progress' },
		{ value: 'blocked', label: 'Blocked' },
		{ value: 'deferred', label: 'Deferred' },
		{ value: 'closed', label: 'Closed' }
	];

	// Type options
	const types = [
		{ value: 'bug', label: 'Bug' },
		{ value: 'feature', label: 'Feature' },
		{ value: 'task', label: 'Task' },
		{ value: 'epic', label: 'Epic' },
		{ value: 'chore', label: 'Chore' }
	];

	// Priority options
	const priorities = [
		{ value: '0', label: 'Critical (P0)' },
		{ value: '1', label: 'High (P1)' },
		{ value: '2', label: 'Medium (P2)' },
		{ value: '3', label: 'Low (P3)' },
		{ value: '4', label: 'Backlog (P4)' }
	];

	function updateFilter(key: string, values: string[]) {
		const params = new URLSearchParams($page.url.searchParams);
		params.delete(key);
		for (const v of values) {
			params.append(key, v);
		}
		params.delete('offset');
		goto(`?${params.toString()}`, { keepFocus: true });
	}

	function clearFilters() {
		goto(`/${projectId}/issues`);
	}

	async function handleStatusChange(issueId: string, newStatus: string) {
		if (!projectId) return;
		await updateIssue(projectId, issueId, { status: newStatus } as UpdateIssueRequest);
		await loadIssues();
	}

	const hasActiveFilters = $derived(
		(filters.status?.length ?? 0) > 0 ||
			(filters.type?.length ?? 0) > 0 ||
			(filters.priority?.length ?? 0) > 0 ||
			filters.q
	);
</script>

{#if project}
	<Header project={project} title="Issues" />

	<div class="flex-1 p-6 animate-fade-in">
		<!-- Filters -->
		<div class="flex flex-wrap items-center gap-3 mb-6">
			<MultiSelect
				options={statuses}
				values={filters.status ?? []}
				placeholder="All Statuses"
				onchange={(v) => updateFilter('status', v)}
			/>

			<MultiSelect
				options={types}
				values={filters.type ?? []}
				placeholder="All Types"
				onchange={(v) => updateFilter('type', v)}
			/>

			<MultiSelect
				options={priorities}
				values={(filters.priority ?? []).map(String)}
				placeholder="All Priorities"
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
					<IssueCard {issue} {labelMap} href="/{projectId}/issues/{issue.id}" onStatusChange={handleStatusChange} />
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
								const params = new URLSearchParams($page.url.searchParams);
								if (newOffset > 0) {
									params.set('offset', newOffset.toString());
								} else {
									params.delete('offset');
								}
								goto(`?${params.toString()}`, { keepFocus: true });
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
							const params = new URLSearchParams($page.url.searchParams);
							params.set('offset', newOffset.toString());
							goto(`?${params.toString()}`, { keepFocus: true });
						}}
					>
						Next
					</button>
				</div>
			{/if}
		{/if}
	</div>
{/if}
