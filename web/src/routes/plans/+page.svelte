<script lang="ts">
	import { goto } from '$app/navigation';
	import { listAllPlans, type Plan } from '$lib/api';
	import { Select } from '$lib/components';
	import { formatRelativeTime } from '$lib/utils';

	let plans = $state<Plan[]>([]);
	let loading = $state(true);
	let error = $state('');
	let statusFilter = $state('');

	const statusOptions = [
		{ value: '', label: 'All Statuses' },
		{ value: 'draft', label: 'Draft' },
		{ value: 'approved', label: 'Approved' },
		{ value: 'rejected', label: 'Rejected' }
	];

	function statusBadgeClass(status: string): string {
		switch (status) {
			case 'approved':
				return 'bg-green-900/30 text-green-400';
			case 'rejected':
				return 'bg-red-900/30 text-red-400';
			default:
				return 'bg-surface-600 text-text-secondary';
		}
	}

	async function loadPlans() {
		loading = true;
		error = '';
		try {
			plans = await listAllPlans(statusFilter || undefined);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load plans';
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		loadPlans();
	});
</script>

<div class="flex-1 p-6 animate-fade-in">
	<header class="mb-6">
		<h1 class="text-2xl font-bold text-text-primary mb-1">Plans</h1>
		<p class="text-text-secondary">
			{plans.length} plan{plans.length !== 1 ? 's' : ''}
		</p>
	</header>

	<!-- Filters -->
	<div class="flex flex-wrap items-center gap-3 mb-6">
		<Select
			options={statusOptions}
			value={statusFilter}
			onchange={(v) => (statusFilter = v)}
		/>
	</div>

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
					<path d="M14 2H6c-1.1 0-2 .9-2 2v16c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V8l-6-6zm-1 7V3.5L18.5 9H13z" />
				</svg>
			</div>
			<h2 class="text-xl font-semibold text-text-primary mb-2">No plans yet</h2>
			<p class="text-text-secondary">Plans will appear here when they are created</p>
		</div>
	{:else}
		<div class="space-y-3">
			{#each plans as plan (plan.id)}
				<!-- svelte-ignore a11y_click_events_have_key_events -->
				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<div
					class="card block p-4 hover:bg-surface-700/50 transition-colors cursor-pointer"
					onclick={() => goto(`/plans/${plan.id}`)}
				>
					<div class="flex items-start justify-between gap-4">
						<div class="min-w-0 flex-1">
							<div class="flex items-center gap-2 mb-1">
								<span class="font-mono text-xs text-text-muted">{plan.id}</span>
								<span
									class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium {statusBadgeClass(plan.status)}"
								>
									{plan.status}
								</span>
							</div>
							<h3 class="text-text-primary font-medium truncate">{plan.title}</h3>
							<div class="flex items-center gap-3 mt-2 text-xs text-text-muted">
								<span>Project: <span class="font-mono">{plan.project_id}</span></span>
								<span>
									Issue: {#if plan.issue_id}<a
											href="/{plan.project_id}/issues/{plan.issue_id}"
											class="text-primary-400 hover:text-primary-300 transition-colors font-mono"
											onclick={(e) => e.stopPropagation()}
										>{plan.issue_id}</a
									>{:else}<span class="text-text-muted">(unlinked)</span>{/if}
								</span>
								<span>{formatRelativeTime(plan.updated_at)}</span>
							</div>
						</div>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>
