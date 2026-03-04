<script lang="ts">
	import { page } from '$app/stores';
	import { getContext } from 'svelte';
	import type { Writable } from 'svelte/store';
	import {
		createReview,
		getReviewDiff,
		submitReview,
		type Workspace,
		type ReviewSession
	} from '$lib/api';
	import { parseDiff, getFileName, type DiffFile } from '$lib/review/parser';
	import { highlightDiffFile } from '$lib/review/highlight';
	import FileTree from '$lib/components/review/FileTree.svelte';
	import FileSection from '$lib/components/review/FileSection.svelte';
	import ReviewSubmit from '$lib/components/review/ReviewSubmit.svelte';

	const workspaces = getContext<Writable<Workspace[]>>('workspaces');
	const workspaceId = $derived($page.params.workspaceId);
	const workspace = $derived($workspaces.find((ws) => ws.id === workspaceId));

	let loading = $state(true);
	let error = $state<string | null>(null);
	let session = $state<ReviewSession | null>(null);
	let files = $state<DiffFile[]>([]);
	let totalAdditions = $state(0);
	let totalDeletions = $state(0);
	let highlightMaps = $state(new Map<string, Map<string, string>>());
	let fileComments = $state(new Map<string, string>());
	let overallComment = $state('');
	let viewedFiles = $state(new Set<string>());
	let activeFile = $state<string | null>(null);
	let collapsedFiles = $state(new Set<string>());
	let submitting = $state(false);
	let submitted = $state<'approved' | 'changes_requested' | null>(null);

	$effect(() => {
		if (workspaceId) {
			loadReview();
		}
	});

	async function loadReview() {
		loading = true;
		error = null;
		try {
			const base = $page.url.searchParams.get('base') ?? 'origin/main';
			const head = $page.url.searchParams.get('head') ?? 'HEAD';

			const reviewSession = await createReview(workspaceId, base, head);
			session = reviewSession;

			const rawDiff = await getReviewDiff(workspaceId, reviewSession.id);
			const parsed = parseDiff(rawDiff);
			files = parsed.files;
			totalAdditions = parsed.stats.totalAdditions;
			totalDeletions = parsed.stats.totalDeletions;

			const maps = await Promise.all(
				parsed.files.map(async (file) => {
					const name = getFileName(file);
					const map = await highlightDiffFile(file);
					return [name, map] as const;
				})
			);
			highlightMaps = new Map(maps);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load review';
		} finally {
			loading = false;
		}
	}

	function handleFileClick(filename: string) {
		activeFile = filename;
		const el = document.getElementById(filename);
		if (el) {
			el.scrollIntoView({ behavior: 'smooth', block: 'start' });
		}
	}

	function handleToggleViewed(filename: string) {
		const next = new Set(viewedFiles);
		if (next.has(filename)) {
			next.delete(filename);
		} else {
			next.add(filename);
		}
		viewedFiles = next;
	}

	function handleToggleCollapse(filename: string) {
		const next = new Set(collapsedFiles);
		if (next.has(filename)) {
			next.delete(filename);
		} else {
			next.add(filename);
		}
		collapsedFiles = next;
	}

	async function handleSubmit(decision: 'approve' | 'request_changes') {
		if (!session) return;
		submitting = true;
		try {
			const commentEntries = Object.fromEntries(fileComments);
			await submitReview(
				workspaceId,
				session.id,
				decision,
				overallComment || undefined,
				Object.keys(commentEntries).length > 0 ? commentEntries : undefined
			);
			submitted = decision === 'approve' ? 'approved' : 'changes_requested';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to submit review';
		} finally {
			submitting = false;
		}
	}
</script>

{#if workspace}
	{#if loading}
		<div class="flex items-center justify-center h-[calc(100vh-4rem)]">
			<div class="text-text-muted animate-pulse">Loading review...</div>
		</div>
	{:else if error}
		<div class="flex items-center justify-center h-[calc(100vh-4rem)]">
			<div class="card p-8 text-center">
				<p class="text-status-blocked mb-4">{error}</p>
				<button class="btn btn-primary" onclick={loadReview}>Retry</button>
			</div>
		</div>
	{:else if submitted}
		<div class="flex items-center justify-center h-[calc(100vh-4rem)]">
			<div class="card p-8 text-center">
				<div class="w-16 h-16 rounded-2xl flex items-center justify-center mx-auto mb-4 {submitted === 'approved' ? 'bg-green-600/20' : 'bg-amber-600/20'}">
					{#if submitted === 'approved'}
						<svg class="w-8 h-8 text-green-400" viewBox="0 0 24 24" fill="currentColor">
							<path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z" />
						</svg>
					{:else}
						<svg class="w-8 h-8 text-amber-400" viewBox="0 0 24 24" fill="currentColor">
							<path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z" />
						</svg>
					{/if}
				</div>
				<h2 class="text-xl font-semibold text-text-primary mb-2">
					{submitted === 'approved' ? 'Review Approved' : 'Changes Requested'}
				</h2>
				<p class="text-text-secondary mb-4">Your review has been submitted successfully.</p>
				<a href="/{workspaceId}/issues" class="btn btn-primary">Back to Issues</a>
			</div>
		</div>
	{:else}
		<div class="flex h-[calc(100vh-4rem)]">
			<!-- Left sidebar: FileTree -->
			<aside class="w-64 border-r border-border flex flex-col bg-surface-800">
				<FileTree {files} {viewedFiles} {activeFile} onFileClick={handleFileClick} onToggleViewed={handleToggleViewed} />
			</aside>

			<!-- Main area -->
			<div class="flex-1 flex flex-col overflow-hidden">
				<!-- Top bar: branch info + stats -->
				<div class="px-4 py-3 border-b border-border bg-surface-800 flex items-center justify-between">
					<span class="text-sm text-text-secondary font-mono">{session?.base}...{session?.head}</span>
					<span class="text-xs text-text-muted">{files.length} files, +{totalAdditions} -{totalDeletions}</span>
				</div>

				<!-- Scrollable diff viewer -->
				<div class="flex-1 overflow-y-auto p-4 space-y-6">
					{#each files as file (getFileName(file))}
						<div id={getFileName(file)}>
							<FileSection
								{file}
								highlightMap={highlightMaps.get(getFileName(file)) ?? new Map()}
								comment={fileComments.get(getFileName(file)) ?? ''}
								onCommentChange={(c) => { fileComments.set(getFileName(file), c); fileComments = new Map(fileComments); }}
								collapsed={collapsedFiles.has(getFileName(file))}
								onToggleCollapse={() => handleToggleCollapse(getFileName(file))}
							/>
						</div>
					{/each}
				</div>

				<!-- Bottom submit bar -->
				<div class="border-t border-border p-4 bg-surface-800">
					<ReviewSubmit
						comment={overallComment}
						onCommentChange={(c) => overallComment = c}
						onSubmit={handleSubmit}
						{submitting}
					/>
				</div>
			</div>
		</div>
	{/if}
{/if}
