<script lang="ts">
	import { Header, Select } from '$lib/components';
	import { page } from '$app/stores';
	import { getContext } from 'svelte';
	import type { Writable } from 'svelte/store';
	import type { Project } from '$lib/api';
	import { api } from '$lib/api/client';
	import type { components } from '$lib/api/types';

	type Plan = components['schemas']['Plan'];

	// Get project from context
	const projects = getContext<Writable<Project[]>>('projects');
	const projectId = $derived($page.params.projectId);
	const project = $derived($projects.find((p) => p.id === projectId));

	// Local state
	let plans = $state<Plan[]>([]);
	let loading = $state(true);
	let error = $state('');
	let statusFilter = $state('');

	// Load plans when projectId or statusFilter changes
	$effect(() => {
		if (projectId) {
			loadPlans();
		}
	});

	async function loadPlans() {
		if (!projectId) return;
		loading = true;
		error = '';
		try {
			const { data, error: apiError } = await api.GET('/projects/{projectId}/plans', {
				params: {
					path: { projectId },
					query: { status: (statusFilter || undefined) as 'draft' | 'approved' | 'rejected' | undefined }
				}
			});
			if (apiError) throw apiError;
			plans = data ?? [];
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load plans';
		} finally {
			loading = false;
		}
	}

	// Status options for plans
	const statuses = [
		{ value: '', label: 'All Statuses' },
		{ value: 'draft', label: 'Draft' },
		{ value: 'approved', label: 'Approved' },
		{ value: 'rejected', label: 'Rejected' }
	];

	function handleStatusFilterChange(value: string) {
		statusFilter = value;
	}

	function statusClass(status: string): string {
		switch (status) {
			case 'approved':
				return 'text-status-closed bg-status-closed/10';
			case 'rejected':
				return 'text-status-blocked bg-status-blocked/10';
			default:
				return 'text-status-open bg-status-open/10';
		}
	}

	function formatDate(dateStr: string): string {
		const date = new Date(dateStr);
		return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
	}
</script>

{#if project}
	<Header {project} title="Plans" showSearch={false} />

	<div class="flex-1 p-6 animate-fade-in">
		<!-- Filters -->
		<div class="flex flex-wrap items-center gap-3 mb-6">
			<Select
				options={statuses}
				value={statusFilter}
				onchange={handleStatusFilterChange}
			/>

			<div class="flex-1"></div>

			<span class="text-sm text-text-muted">
				{plans.length} plans
			</span>
		</div>

		<!-- Content -->
		{#if loading}
			<div class="flex items-center justify-center py-12">
				<div class="text-text-muted animate-pulse">Loading plans...</div>
			</div>
		{:else if error}
			<div class="card p-8 text-center">
				<p class="text-status-blocked mb-4">{error}</p>
				<button class="btn btn-primary" onclick={loadPlans}>Retry</button>
			</div>
		{:else if plans.length === 0}
			<div class="card p-12 text-center">
				<div
					class="w-16 h-16 bg-surface-700 rounded-2xl flex items-center justify-center mx-auto mb-4"
				>
					<svg class="w-8 h-8 text-text-muted" viewBox="0 0 24 24" fill="currentColor">
						<path d="M14 2H6c-1.1 0-2 .9-2 2v16c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V8l-6-6zm-1 7V3.5L18.5 9H13zM6 20V4h5v7h7v9H6z" />
					</svg>
				</div>
				{#if statusFilter}
					<h2 class="text-xl font-semibold text-text-primary mb-2">No matching plans</h2>
					<p class="text-text-secondary mb-4">Try adjusting your filter</p>
					<button type="button" class="btn btn-primary" onclick={() => (statusFilter = '')}>
						Clear filter
					</button>
				{:else}
					<h2 class="text-xl font-semibold text-text-primary mb-2">No plans yet</h2>
					<p class="text-text-secondary mb-4">Plans will appear here when created</p>
				{/if}
			</div>
		{:else}
			<div class="space-y-3">
				{#each plans as plan (plan.id)}
					<a
						href="/plans/{plan.id}"
						class="card p-4 flex items-center gap-4 hover:bg-surface-700 transition-colors cursor-pointer block"
					>
						<div class="flex-1 min-w-0">
							<div class="flex items-center gap-2 mb-1">
								<h3 class="text-sm font-medium text-text-primary truncate">
									{plan.title}
								</h3>
							</div>
							<div class="flex items-center gap-3 text-xs text-text-muted">
								<span class="font-mono">{plan.id}</span>
								{#if plan.issue_id}
									<span>Issue: {plan.issue_id}</span>
								{/if}
								<span>{formatDate(plan.created_at)}</span>
							</div>
						</div>
						<span
							class="px-2 py-0.5 rounded-full text-xs font-medium capitalize {statusClass(plan.status)}"
						>
							{plan.status}
						</span>
					</a>
				{/each}
			</div>
		{/if}
	</div>
{/if}
