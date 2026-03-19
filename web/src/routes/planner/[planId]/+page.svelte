<script lang="ts">
	import { page } from '$app/stores';
	import {
		getPlan, updatePlanContent,
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

	// View modes: 'read' (rendered markdown), 'review' (line numbers + comments), 'edit' (raw editor)
	type ViewMode = 'read' | 'review' | 'edit';
	let viewMode = $state<ViewMode>('review');
	let editContent = $state('');

	// Comment state
	let activeCommentLine = $state<number | null>(null);
	let commentText = $state('');
	let overallFeedback = $state('');

	// Split content into lines for the review/edit views
	let contentLines = $derived((plan?.content ?? '').split('\n'));

	// Group comments by line number for indicators
	let commentsByLine = $derived.by(() => {
		const map = new Map<number | null, PlanComment[]>();
		for (const c of comments) {
			const key = c.line_number ?? null;
			if (!map.has(key)) map.set(key, []);
			map.get(key)!.push(c);
		}
		return map;
	});

	// Count of lines that have comments (for the review tab badge)
	let lineCommentCount = $derived(
		[...commentsByLine.entries()].filter(([k]) => k !== null).reduce((sum, [, v]) => sum + v.length, 0)
	);

	let overallComments = $derived(commentsByLine.get(null) ?? []);

	$effect(() => { if (planId) loadData(); });

	async function loadData() {
		if (!planId) return;
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
		if (!plan || !planId) return;
		try {
			const updated = await updatePlanContent(planId, editContent);
			plan = updated;
			viewMode = 'read';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to save';
		}
	}

	async function handleAddComment(lineNumber: number | null) {
		if (!planId) return;
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

	function switchToEdit() {
		editContent = plan?.content ?? '';
		viewMode = 'edit';
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
			<div class="min-w-0">
				<h1 class="text-xl font-semibold text-text-primary truncate">
					{plan.file_path.split('/').pop()}
				</h1>
				<p class="text-sm text-text-muted mt-1 truncate">{plan.file_path}</p>
			</div>
			<span class="px-3 py-1 rounded-full text-xs font-medium shrink-0 {statusColor(plan.status)}">
				{plan.status}
			</span>
		</div>

		<!-- View Mode Tabs -->
		<div class="flex gap-1 border-b border-surface-600">
			<button
				onclick={() => viewMode = 'review'}
				class="px-4 py-2 text-sm transition-colors flex items-center gap-1.5 {viewMode === 'review'
					? 'text-text-primary border-b-2 border-primary-500 -mb-px'
					: 'text-text-muted hover:text-text-secondary'}"
			>
				Review
				{#if lineCommentCount > 0}
					<span class="px-1.5 py-0.5 text-xs rounded-full bg-yellow-900/30 text-yellow-400">{lineCommentCount}</span>
				{/if}
			</button>
			<button
				onclick={() => viewMode = 'read'}
				class="px-4 py-2 text-sm transition-colors {viewMode === 'read'
					? 'text-text-primary border-b-2 border-primary-500 -mb-px'
					: 'text-text-muted hover:text-text-secondary'}"
			>
				Preview
			</button>
			<button
				onclick={switchToEdit}
				class="px-4 py-2 text-sm transition-colors {viewMode === 'edit'
					? 'text-text-primary border-b-2 border-primary-500 -mb-px'
					: 'text-text-muted hover:text-text-secondary'}"
			>
				Edit
			</button>
		</div>

		<!-- Review Mode: Line numbers with commenting -->
		{#if viewMode === 'review'}
			<div class="card">
				<div class="px-4 py-2 border-b border-surface-600 text-xs text-text-muted">
					Click a line number to add a comment
				</div>
				<div class="font-mono text-sm overflow-x-auto">
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
									<span class="pr-3 py-0.5 text-xs text-yellow-400 shrink-0">
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

		<!-- Preview Mode: Rendered Markdown -->
		{#if viewMode === 'read'}
			<div class="card p-6 markdown">
				<Markdown content={plan.content ?? ''} />
			</div>
		{/if}

		<!-- Edit Mode: Raw markdown editor -->
		{#if viewMode === 'edit'}
			<div class="card p-4 space-y-3">
				<textarea
					bind:value={editContent}
					class="w-full h-[32rem] bg-surface-700 text-text-primary font-mono text-sm p-4 rounded border border-surface-500 focus:border-primary-500 focus:outline-none resize-y"
					spellcheck="false"
				></textarea>
				<div class="flex gap-2 justify-end">
					<button onclick={() => viewMode = 'read'}
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

		<!-- Overall Feedback (visible in read and review modes) -->
		{#if viewMode !== 'edit'}
			<div class="card p-4 space-y-3">
				<label for="overall-feedback" class="text-xs text-text-muted uppercase tracking-wide">
					Overall Feedback
					{#if overallComments.length > 0}
						<span class="ml-1.5 px-1.5 py-0.5 rounded-full bg-yellow-900/30 text-yellow-400 normal-case">{overallComments.length}</span>
					{/if}
				</label>
				{#if overallComments.length > 0}
					<div class="space-y-2 mb-3">
						{#each overallComments as comment}
							<div class="text-sm text-text-secondary bg-surface-700 rounded p-3">
								{comment.content}
								<span class="text-xs text-text-muted ml-2">
									{formatRelativeTime(comment.created_at)}
								</span>
							</div>
						{/each}
					</div>
				{/if}
				<div class="flex gap-2">
					<textarea
						id="overall-feedback"
						bind:value={overallFeedback}
						placeholder="Overall feedback on this plan..."
						rows="2"
						class="flex-1 bg-surface-700 text-text-primary text-sm p-3 rounded border border-surface-500 focus:border-primary-500 focus:outline-none resize-y"
						onkeydown={(e) => { if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) handleAddComment(null); }}
					></textarea>
					<button
						onclick={() => handleAddComment(null)}
						disabled={!overallFeedback.trim()}
						class="self-end px-3 py-1.5 text-sm bg-primary-600 text-white rounded hover:bg-primary-500 disabled:opacity-30 disabled:cursor-not-allowed"
					>
						Add
					</button>
				</div>
				<p class="text-xs text-text-muted">Press Ctrl+Enter to submit</p>
			</div>
		{/if}
	</div>
{/if}
