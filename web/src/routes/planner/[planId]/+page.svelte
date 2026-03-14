<script lang="ts">
	import { page } from '$app/stores';
	import {
		getPlan, updatePlanContent, updatePlanStatus,
		listPlanComments, createPlanComment
	} from '$lib/api';
	import type { PlanWithContent, PlanComment } from '$lib/api';
	import Markdown from '$lib/components/Markdown.svelte';
	import { formatRelativeTime } from '$lib/utils';

	let planId = $derived($page.params.planId);

	let plan = $state<PlanWithContent | null>(null);
	let comments = $state<PlanComment[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let editing = $state(false);
	let editContent = $state('');

	let activeCommentLine = $state<number | null>(null);
	let commentText = $state('');
	let overallFeedback = $state('');
	let submitting = $state(false);

	let contentLines = $derived((plan?.content ?? '').split('\n'));

	let commentsByLine = $derived.by(() => {
		const map = new Map<number | null, PlanComment[]>();
		for (const c of comments) {
			const key = c.line_number ?? null;
			if (!map.has(key)) map.set(key, []);
			map.get(key)!.push(c);
		}
		return map;
	});

	let hasAnyComments = $derived(comments.length > 0 || overallFeedback.trim().length > 0);

	$effect(() => { if (planId) loadData(); });

	async function loadData() {
		loading = true;
		error = null;
		try {
			const [planData, commentsData] = await Promise.all([
				getPlan(planId),
				listPlanComments(planId)
			]);
			plan = planData;
			comments = commentsData;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load plan';
		} finally {
			loading = false;
		}
	}

	async function handleSaveEdit() {
		if (!plan) return;
		try {
			const updated = await updatePlanContent(planId, editContent);
			plan = updated;
			editing = false;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to save';
		}
	}

	async function handleAddComment(lineNumber: number | null) {
		const text = lineNumber === null ? overallFeedback : commentText;
		if (!text.trim()) return;
		try {
			const comment = await createPlanComment(planId, text, lineNumber ?? undefined);
			comments = [...comments, comment];
			commentText = '';
			if (lineNumber === null) overallFeedback = '';
			activeCommentLine = null;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to add comment';
		}
	}

	async function handleUpdateStatus(status: string) {
		submitting = true;
		try {
			if (status === 'in_review' && overallFeedback.trim()) {
				await handleAddComment(null);
			}
			const updated = await updatePlanStatus(planId, status);
			plan = { ...plan!, ...updated };
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to update status';
		} finally {
			submitting = false;
		}
	}

	function startEdit() {
		editContent = plan?.content ?? '';
		editing = true;
	}

	function statusColor(status: string): string {
		switch (status) {
			case 'draft': return 'bg-surface-600 text-text-secondary';
			case 'in_review': return 'bg-yellow-900/30 text-yellow-400 border border-yellow-800';
			case 'approved': return 'bg-green-900/30 text-green-400 border border-green-800';
			case 'rejected': return 'bg-red-900/30 text-red-400 border border-red-800';
			default: return 'bg-surface-600 text-text-secondary';
		}
	}
</script>

{#if loading}
	<div class="flex items-center justify-center py-20">
		<div class="text-text-muted animate-pulse">Loading plan...</div>
	</div>
{:else if error}
	<div class="flex items-center justify-center py-20">
		<div class="text-red-400">{error}</div>
	</div>
{:else if plan}
	<div class="max-w-5xl mx-auto p-6 space-y-6">
		<!-- Header -->
		<div class="flex items-center justify-between">
			<div>
				<h1 class="text-xl font-semibold text-text-primary">
					{plan.file_path.split('/').pop()}
				</h1>
				<p class="text-sm text-text-muted mt-1">{plan.file_path}</p>
			</div>
			<span class="px-3 py-1 rounded-full text-xs font-medium {statusColor(plan.status)}">
				{plan.status}
			</span>
		</div>

		<!-- Rendered Markdown (read-only view) -->
		{#if !editing}
			<div class="card p-6">
				<Markdown content={plan.content ?? ''} />
			</div>
		{/if}

		<!-- Raw Editor (edit mode) -->
		{#if editing}
			<div class="card p-4 space-y-3">
				<textarea
					bind:value={editContent}
					class="w-full h-96 bg-surface-700 text-text-primary font-mono text-sm p-4 rounded border border-surface-500 focus:border-primary-500 focus:outline-none resize-y"
				></textarea>
				<div class="flex gap-2 justify-end">
					<button onclick={() => editing = false}
						class="px-3 py-1.5 text-sm text-text-secondary hover:text-text-primary">
						Cancel
					</button>
					<button onclick={handleSaveEdit}
						class="px-3 py-1.5 text-sm bg-primary-600 text-white rounded hover:bg-primary-500">
						Save
					</button>
				</div>
			</div>
		{/if}

		<!-- Line Reference View (for commenting) -->
		{#if !editing}
			<div class="card">
				<div class="px-4 py-2 border-b border-surface-600 text-xs text-text-muted uppercase tracking-wide">
					Line Comments — click a line number to add a comment
				</div>
				<div class="font-mono text-sm">
					{#each contentLines as line, i}
						{@const lineNum = i + 1}
						{@const lineComments = commentsByLine.get(lineNum) ?? []}
						<div class="group">
							<div class="flex hover:bg-surface-700/50">
								<button
									onclick={() => activeCommentLine = activeCommentLine === lineNum ? null : lineNum}
									class="w-12 text-right pr-3 py-0.5 text-text-muted hover:text-primary-400 select-none shrink-0 cursor-pointer"
								>
									{lineNum}
								</button>
								<div class="flex-1 py-0.5 pr-4 text-text-primary whitespace-pre-wrap break-all">
									{line || '\u00A0'}
								</div>
								{#if lineComments.length > 0}
									<span class="pr-3 py-0.5 text-xs text-yellow-400">
										{lineComments.length}
									</span>
								{/if}
							</div>

							{#if lineComments.length > 0}
								<div class="ml-12 pl-3 border-l-2 border-yellow-800 bg-yellow-900/10 py-2 space-y-1">
									{#each lineComments as comment}
										<div class="text-sm text-text-secondary">
											{comment.content}
											<span class="text-xs text-text-muted ml-2">
												{formatRelativeTime(comment.created_at)}
											</span>
										</div>
									{/each}
								</div>
							{/if}

							{#if activeCommentLine === lineNum}
								<div class="ml-12 pl-3 py-2 flex gap-2">
									<input
										type="text"
										bind:value={commentText}
										placeholder="Add a comment on line {lineNum}..."
										class="flex-1 bg-surface-700 text-text-primary text-sm px-3 py-1.5 rounded border border-surface-500 focus:border-primary-500 focus:outline-none"
										onkeydown={(e) => { if (e.key === 'Enter') handleAddComment(lineNum); }}
									/>
									<button
										onclick={() => handleAddComment(lineNum)}
										class="px-3 py-1.5 text-sm bg-primary-600 text-white rounded hover:bg-primary-500"
									>
										Comment
									</button>
								</div>
							{/if}
						</div>
					{/each}
				</div>
			</div>
		{/if}

		<!-- Overall Feedback -->
		<div class="card p-4 space-y-3">
			<label class="text-xs text-text-muted uppercase tracking-wide">Overall Feedback</label>
			{#if (commentsByLine.get(null) ?? []).length > 0}
				<div class="space-y-2 mb-3">
					{#each commentsByLine.get(null) ?? [] as comment}
						<div class="text-sm text-text-secondary bg-surface-700 rounded p-3">
							{comment.content}
							<span class="text-xs text-text-muted ml-2">
								{formatRelativeTime(comment.created_at)}
							</span>
						</div>
					{/each}
				</div>
			{/if}
			<textarea
				bind:value={overallFeedback}
				placeholder="Overall feedback on this plan..."
				rows="3"
				class="w-full bg-surface-700 text-text-primary text-sm p-3 rounded border border-surface-500 focus:border-primary-500 focus:outline-none resize-y"
			></textarea>
		</div>

		<!-- Action Buttons -->
		<div class="flex gap-3 justify-end">
			{#if !editing}
				<button onclick={startEdit}
					class="px-4 py-2 text-sm text-text-secondary border border-surface-500 rounded hover:bg-surface-700">
					Edit
				</button>
			{/if}
			<button onclick={() => handleUpdateStatus('rejected')}
				disabled={submitting}
				class="px-4 py-2 text-sm text-red-400 border border-red-800 rounded hover:bg-red-900/30 disabled:opacity-50">
				Reject
			</button>
			<button onclick={() => handleUpdateStatus('in_review')}
				disabled={submitting || !hasAnyComments}
				title={hasAnyComments ? '' : 'Add at least one comment before submitting review'}
				class="px-4 py-2 text-sm text-yellow-400 border border-yellow-800 rounded hover:bg-yellow-900/30 disabled:opacity-50">
				Submit Review
			</button>
			<button onclick={() => handleUpdateStatus('approved')}
				disabled={submitting}
				class="px-4 py-2 text-sm text-green-400 border border-green-800 rounded hover:bg-green-900/30 disabled:opacity-50">
				Approve
			</button>
		</div>
	</div>
{/if}
