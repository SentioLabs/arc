<script lang="ts">
	import { Markdown, ConfirmDialog } from '$lib/components';
	import IssuePicker from '$lib/components/IssuePicker.svelte';
	import { formatDateTime, formatRelativeTime } from '$lib/utils';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import {
		listAllPlans,
		updatePlan,
		updatePlanStatus,
		deletePlan,
		type Plan
	} from '$lib/api';

	const planId = $derived($page.params.planId);

	let plan = $state<Plan | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let menuOpen = $state(false);
	let deleteConfirmOpen = $state(false);
	let deleting = $state(false);
	let issuePickerOpen = $state(false);

	// svelte-ignore non_reactive_update
	let menuButtonEl: HTMLButtonElement;
	// svelte-ignore non_reactive_update
	let menuDropdownEl: HTMLDivElement;

	$effect(() => {
		if (planId) loadPlan();
	});

	async function loadPlan() {
		loading = true;
		error = null;
		try {
			const plans = await listAllPlans();
			const found = plans.find((p) => p.id === planId);
			if (!found) {
				error = 'Plan not found';
				return;
			}
			plan = found;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load plan';
		} finally {
			loading = false;
		}
	}

	function toggleMenu() {
		menuOpen = !menuOpen;
	}

	// Close menu on click outside
	$effect(() => {
		if (!menuOpen) return;

		function handleClickOutside(e: MouseEvent) {
			if (
				menuButtonEl &&
				!menuButtonEl.contains(e.target as Node) &&
				menuDropdownEl &&
				!menuDropdownEl.contains(e.target as Node)
			) {
				menuOpen = false;
			}
		}

		document.addEventListener('click', handleClickOutside);
		return () => document.removeEventListener('click', handleClickOutside);
	});

	async function handleStatusChange(status: 'draft' | 'approved' | 'rejected') {
		if (!plan) return;
		try {
			await updatePlanStatus(plan.project_id, plan.id, status);
			await loadPlan();
		} catch (err) {
			alert(err instanceof Error ? err.message : 'Failed to update status');
		}
		menuOpen = false;
	}

	async function handleUnlink() {
		if (!plan) return;
		try {
			await updatePlan(plan.project_id, plan.id, { issue_id: '' });
			await loadPlan();
		} catch (err) {
			alert(err instanceof Error ? err.message : 'Failed to unlink issue');
		}
		menuOpen = false;
	}

	function handleLinkIssue() {
		menuOpen = false;
		issuePickerOpen = true;
	}

	async function handleIssueSelected(issueId: string) {
		if (!plan) return;
		issuePickerOpen = false;
		try {
			await updatePlan(plan.project_id, plan.id, { issue_id: issueId });
			await loadPlan();
		} catch (err) {
			alert(err instanceof Error ? err.message : 'Failed to link issue');
		}
	}

	function handleIssuePickerCancel() {
		issuePickerOpen = false;
	}

	function handleDeleteClick() {
		menuOpen = false;
		deleteConfirmOpen = true;
	}

	async function handleDeleteConfirm() {
		if (!plan) return;
		deleting = true;
		try {
			await deletePlan(plan.project_id, plan.id);
			goto('/plans');
		} catch (err) {
			alert(err instanceof Error ? err.message : 'Failed to delete plan');
		} finally {
			deleting = false;
			deleteConfirmOpen = false;
		}
	}

	function handleDeleteCancel() {
		deleteConfirmOpen = false;
	}

	const statusBadgeClass = $derived.by(() => {
		if (!plan) return '';
		switch (plan.status) {
			case 'draft':
				return 'bg-surface-600 text-text-secondary';
			case 'approved':
				return 'bg-green-900/30 text-green-400 border border-green-800';
			case 'rejected':
				return 'bg-red-900/30 text-red-400 border border-red-800';
			default:
				return 'bg-surface-600 text-text-secondary';
		}
	});

	const isLinked = $derived(plan?.issue_id != null && plan.issue_id !== '');
</script>

{#if loading}
	<div class="flex items-center justify-center py-20">
		<div class="text-text-muted">Loading plan...</div>
	</div>
{:else if error}
	<div class="flex items-center justify-center py-20">
		<div class="text-red-400">{error}</div>
	</div>
{:else if plan}
	<div class="max-w-4xl mx-auto px-6 py-8">
		<!-- Back link -->
		<a href="/plans" class="text-sm text-text-muted hover:text-text-secondary transition-colors mb-6 inline-block">
			&larr; Plans
		</a>

		<!-- Header row -->
		<div class="flex items-start justify-between gap-4 mb-4">
			<h1 class="text-2xl font-bold text-text-primary">{plan.title}</h1>
			<div class="relative">
				<button
					bind:this={menuButtonEl}
					type="button"
					onclick={toggleMenu}
					class="p-2 text-text-muted hover:text-text-primary hover:bg-surface-700 rounded-md transition-colors cursor-pointer"
					aria-label="Plan actions"
				>
					&#8942;
				</button>

				{#if menuOpen}
					<div
						bind:this={menuDropdownEl}
						class="absolute right-0 top-full mt-1 w-56 bg-surface-800 border border-border rounded-lg shadow-lg z-20 overflow-hidden py-1"
					>
						<!-- Link to Issue -->
						<button
							type="button"
							class="w-full text-left px-4 py-2 text-sm text-text-primary hover:bg-surface-700 transition-colors cursor-pointer"
							onclick={handleLinkIssue}
						>
							{isLinked ? 'Change Linked Issue' : 'Link to Issue'}
						</button>

						<!-- Unlink Issue (if linked) -->
						{#if isLinked}
							<button
								type="button"
								class="w-full text-left px-4 py-2 text-sm text-text-primary hover:bg-surface-700 transition-colors cursor-pointer"
								onclick={handleUnlink}
							>
								Unlink Issue
							</button>
						{/if}

						<!-- Separator -->
						<div class="border-t border-border my-1"></div>

						<!-- Status actions based on current status -->
						{#if plan.status !== 'approved'}
							<button
								type="button"
								class="w-full text-left px-4 py-2 text-sm text-text-primary hover:bg-surface-700 transition-colors cursor-pointer"
								onclick={() => handleStatusChange('approved')}
							>
								Approve
							</button>
						{/if}
						{#if plan.status !== 'rejected'}
							<button
								type="button"
								class="w-full text-left px-4 py-2 text-sm text-text-primary hover:bg-surface-700 transition-colors cursor-pointer"
								onclick={() => handleStatusChange('rejected')}
							>
								Reject
							</button>
						{/if}
						{#if plan.status !== 'draft'}
							<button
								type="button"
								class="w-full text-left px-4 py-2 text-sm text-text-primary hover:bg-surface-700 transition-colors cursor-pointer"
								onclick={() => handleStatusChange('draft')}
							>
								Revert to Draft
							</button>
						{/if}

						<!-- Separator -->
						<div class="border-t border-border my-1"></div>

						<!-- Delete -->
						<button
							type="button"
							class="w-full text-left px-4 py-2 text-sm text-red-400 hover:bg-surface-700 transition-colors cursor-pointer"
							onclick={handleDeleteClick}
						>
							Delete Plan
						</button>
					</div>
				{/if}
			</div>
		</div>

		<!-- Metadata row -->
		<div class="flex flex-wrap items-center gap-3 mb-6 text-sm">
			<span class="px-2 py-0.5 text-xs font-medium rounded {statusBadgeClass}">
				{plan.status}
			</span>

			{#if isLinked}
				<a
					href="/{plan.project_id}/issues/{plan.issue_id}"
					class="text-primary-400 hover:text-primary-300 transition-colors"
				>
					{plan.issue_id}
				</a>
			{:else}
				<span class="text-text-muted">(unlinked)</span>
			{/if}

			<span class="text-text-muted">{plan.project_id}</span>

			<span class="text-text-muted" title={formatDateTime(plan.created_at)}>
				Created {formatRelativeTime(plan.created_at)}
			</span>

			<span class="text-text-muted" title={formatDateTime(plan.updated_at)}>
				Updated {formatRelativeTime(plan.updated_at)}
			</span>
		</div>

		<!-- Markdown content -->
		<div class="prose-container">
			<Markdown content={plan.content} />
		</div>
	</div>

	<!-- Delete confirmation dialog -->
	<ConfirmDialog
		open={deleteConfirmOpen}
		title="Delete Plan"
		message="Are you sure you want to delete this plan? This action cannot be undone."
		confirmLabel="Delete"
		variant="danger"
		loading={deleting}
		onconfirm={handleDeleteConfirm}
		oncancel={handleDeleteCancel}
	/>

	<!-- Issue picker dialog -->
	{#if issuePickerOpen && plan}
		<IssuePicker
			projectId={plan.project_id}
			onSelect={handleIssueSelected}
			onCancel={handleIssuePickerCancel}
		/>
	{/if}
{/if}
