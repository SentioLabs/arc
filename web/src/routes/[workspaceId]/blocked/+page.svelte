<script lang="ts">
	import { Header, StatusBadge, PriorityBadge, TypeBadge } from '$lib/components';
	import { formatRelativeTime } from '$lib/utils';
	import { page } from '$app/stores';
	import { getContext } from 'svelte';
	import type { Writable } from 'svelte/store';
	import { getBlockedIssues, type Workspace, type BlockedIssue } from '$lib/api';

	const workspaces = getContext<Writable<Workspace[]>>('workspaces');
	const workspaceId = $derived($page.params.workspaceId);
	const workspace = $derived($workspaces.find((ws) => ws.id === workspaceId));

	let issues = $state<BlockedIssue[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	$effect(() => {
		if (workspaceId) loadIssues();
	});

	async function loadIssues() {
		if (!workspaceId) return;
		loading = true;
		error = null;
		try {
			issues = await getBlockedIssues(workspaceId, 50);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load blocked issues';
		} finally {
			loading = false;
		}
	}
</script>

{#if workspace}
	<Header {workspace} title="Blocked Issues" />

	<div class="flex-1 p-6 animate-fade-in">
		<header class="mb-6">
			<div class="flex items-center gap-3 mb-2">
				<div class="w-10 h-10 bg-status-blocked/20 rounded-lg flex items-center justify-center">
					<svg class="w-5 h-5 text-status-blocked" viewBox="0 0 24 24" fill="currentColor">
						<path
							d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zM4 12c0-4.42 3.58-8 8-8 1.85 0 3.55.63 4.9 1.69L5.69 16.9C4.63 15.55 4 13.85 4 12zm8 8c-1.85 0-3.55-.63-4.9-1.69L18.31 7.1C19.37 8.45 20 10.15 20 12c0 4.42-3.58 8-8 8z"
						/>
					</svg>
				</div>
				<div>
					<h1 class="text-2xl font-bold text-text-primary">Blocked Issues</h1>
					<p class="text-sm text-text-secondary">{issues.length} issues waiting on dependencies</p>
				</div>
			</div>
		</header>

		{#if loading}
			<div class="flex items-center justify-center py-12">
				<div class="text-text-muted animate-pulse">Loading...</div>
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
					<svg class="w-8 h-8 text-status-open" viewBox="0 0 24 24" fill="currentColor">
						<path d="M9 16.2L4.8 12l-1.4 1.4L9 19 21 7l-1.4-1.4L9 16.2z" />
					</svg>
				</div>
				<h2 class="text-xl font-semibold text-text-primary mb-2">No blocked issues</h2>
				<p class="text-text-secondary">All issues are ready to work on or already resolved</p>
			</div>
		{:else}
			<div class="space-y-3">
				{#each issues as issue, i (issue.id)}
					<a
						href="/{workspaceId}/issues/{issue.id}"
						class="card p-4 block hover:border-border-focus/50 transition-all duration-200 group animate-slide-up"
						style="animation-delay: {Math.min(i * 30, 300)}ms"
					>
						<div class="flex items-start gap-4">
							<div class="flex-shrink-0 mt-1">
								<div
									class="w-8 h-8 bg-status-blocked/20 rounded-lg flex items-center justify-center"
								>
									<span class="text-sm font-mono font-bold text-status-blocked">
										{issue.blocked_by_count}
									</span>
								</div>
							</div>

							<div class="flex-1 min-w-0">
								<div class="flex items-center gap-3 mb-2">
									<span
										class="font-mono text-xs text-text-muted group-hover:text-primary-400 transition-colors"
									>
										{issue.id}
									</span>
									<TypeBadge type={issue.issue_type} showLabel={false} />
									<PriorityBadge priority={issue.priority} showLabel={false} />
								</div>

								<h3
									class="font-medium text-text-primary group-hover:text-white transition-colors mb-2 line-clamp-1"
								>
									{issue.title}
								</h3>

								<div class="flex items-center gap-2 text-xs">
									<span class="text-text-muted">Blocked by:</span>
									<div class="flex flex-wrap gap-1">
										{#each issue.blocked_by.slice(0, 4) as blockerId (blockerId)}
											<span
												class="px-1.5 py-0.5 bg-status-blocked/10 text-status-blocked font-mono rounded"
											>
												{blockerId}
											</span>
										{/each}
										{#if issue.blocked_by.length > 4}
											<span class="text-text-muted">+{issue.blocked_by.length - 4} more</span>
										{/if}
									</div>
								</div>
							</div>

							<div class="flex-shrink-0 text-xs text-text-muted">
								{formatRelativeTime(issue.updated_at)}
							</div>
						</div>
					</a>
				{/each}
			</div>
		{/if}
	</div>
{/if}
