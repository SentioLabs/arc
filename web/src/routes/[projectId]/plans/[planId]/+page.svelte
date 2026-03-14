<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { getContext } from 'svelte';
	import type { Writable } from 'svelte/store';
	import {
		getPlan,
		updatePlan,
		updatePlanStatus,
		type Plan,
		type Project
	} from '$lib/api';
	import { renderMarkdown } from '$lib/markdown';
	import Header from '$lib/components/Header.svelte';

	const projects = getContext<Writable<Project[]>>('projects');
	const projectId = $derived($page.params.projectId);
	const project = $derived($projects.find((p) => p.id === projectId));
	const planId = $derived($page.params.planId);

	let plan = $state<Plan | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let saving = $state(false);

	let editTitle = $state('');
	let editContent = $state('');
	let renderedHtml = $state('');

	$effect(() => {
		if (projectId && planId) loadPlan();
	});

	$effect(() => {
		const c = editContent;
		const timeout = setTimeout(async () => {
			renderedHtml = await renderMarkdown(c);
		}, 300);
		return () => clearTimeout(timeout);
	});

	async function loadPlan() {
		if (!projectId || !planId) return;
		loading = true;
		error = null;
		try {
			const data = await getPlan(projectId, planId);
			plan = data;
			editTitle = data.title;
			editContent = data.content;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load plan';
		} finally {
			loading = false;
		}
	}

	async function handleSaveDraft() {
		if (!projectId || !planId) return;
		saving = true;
		try {
			const updated = await updatePlan(projectId, planId, {
				title: editTitle,
				content: editContent
			});
			plan = updated;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to save plan';
		} finally {
			saving = false;
		}
	}

	async function handleApprove() {
		if (!projectId || !planId) return;
		saving = true;
		try {
			await updatePlanStatus(projectId, planId, 'approved');
			goto(`/${projectId}/plans`);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to approve plan';
			saving = false;
		}
	}

	async function handleReject() {
		if (!projectId || !planId) return;
		saving = true;
		try {
			await updatePlanStatus(projectId, planId, 'rejected');
			goto(`/${projectId}/plans`);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to reject plan';
			saving = false;
		}
	}

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
</script>

<svelte:head>
	<title>{plan?.title ?? 'Plan'} - {project?.name ?? 'Arc'} - Arc</title>
</svelte:head>

{#if project}
	{#if loading}
		<Header {project} title="Loading..." showSearch={false} />
		<div class="flex-1 flex items-center justify-center">
			<div class="text-text-muted animate-pulse">Loading plan...</div>
		</div>
	{:else if error && !plan}
		<Header {project} title="Error" showSearch={false} />
		<div class="flex-1 flex items-center justify-center p-8">
			<div class="card p-8 text-center">
				<p class="text-status-blocked mb-4">{error}</p>
				<button class="btn btn-primary" onclick={loadPlan}>Retry</button>
			</div>
		</div>
	{:else if plan}
		<Header {project} title="Plan" showSearch={false} />

		<div class="flex-1 p-6 animate-fade-in">
			<div class="max-w-7xl mx-auto">
				<!-- Plan Header -->
				<header class="mb-6">
					<div class="flex items-center gap-3 mb-3">
						<span class="font-mono text-sm text-text-muted">{plan.id}</span>
						<span
							class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium {statusBadgeClass(plan.status)}"
						>
							{plan.status}
						</span>
					</div>
					<input
						type="text"
						bind:value={editTitle}
						class="input w-full text-2xl font-bold"
						placeholder="Plan title"
					/>
					{#if plan.issue_id}
						<div class="mt-2 text-sm text-text-muted">
							Linked issue:
							<a
								href="/{projectId}/issues/{plan.issue_id}"
								class="text-primary-400 hover:text-primary-300 transition-colors font-mono"
							>
								{plan.issue_id}
							</a>
						</div>
					{/if}
				</header>

				{#if error}
					<div class="mb-4 p-3 bg-red-900/20 border border-red-800 rounded-lg text-red-400 text-sm">
						{error}
					</div>
				{/if}

				<!-- Split pane editor -->
				<div class="grid grid-cols-2 gap-4">
					<!-- Left: Editor -->
					<div>
						<h3 class="text-xs font-medium text-text-muted uppercase tracking-wider mb-2">
							Editor
						</h3>
						<textarea
							bind:value={editContent}
							class="input font-mono w-full min-h-[600px] resize-y p-4 text-sm leading-relaxed"
							placeholder="Write your plan in Markdown..."
						></textarea>
					</div>

					<!-- Right: Preview -->
					<div>
						<h3 class="text-xs font-medium text-text-muted uppercase tracking-wider mb-2">
							Preview
						</h3>
						<div
							class="card p-4 min-h-[600px] overflow-y-auto"
						>
							{#if renderedHtml}
								<div class="markdown">
									{@html renderedHtml}
								</div>
							{:else if editContent}
								<div class="text-text-secondary whitespace-pre-wrap">{editContent}</div>
							{:else}
								<p class="text-text-muted text-sm">Preview will appear here...</p>
							{/if}
						</div>
					</div>
				</div>

				<!-- Footer actions -->
				<div class="flex items-center justify-between mt-6 pt-4 border-t border-border">
					<a
						href="/{projectId}/plans"
						class="btn btn-ghost text-sm"
					>
						Back to Plans
					</a>
					<div class="flex items-center gap-3">
						<button
							class="btn btn-primary"
							onclick={handleSaveDraft}
							disabled={saving}
						>
							{saving ? 'Saving...' : 'Save Draft'}
						</button>
						<button
							class="btn bg-green-700 hover:bg-green-600 text-white"
							onclick={handleApprove}
							disabled={saving}
						>
							Approve
						</button>
						<button
							class="btn bg-red-700 hover:bg-red-600 text-white"
							onclick={handleReject}
							disabled={saving}
						>
							Reject
						</button>
					</div>
				</div>
			</div>
		</div>
	{/if}
{/if}
