<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { listAISessions, type AISessionResponse } from '$lib/api/ai';

	let { projectId }: { projectId: string } = $props();

	let sessions = $state<AISessionResponse[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let refreshInterval: ReturnType<typeof setInterval> | undefined;

	async function loadSessions() {
		try {
			const result = await listAISessions(projectId, 5, 0);
			sessions = result.data ?? [];
			error = null;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load sessions';
		} finally {
			loading = false;
		}
	}

	function truncateId(id: string): string {
		if (id.length <= 10) return id;
		return id.substring(0, 4) + '\u2026' + id.slice(-4);
	}

	function timeAgo(dateStr: string): string {
		const now = Date.now();
		const then = new Date(dateStr).getTime();
		const diffMs = now - then;
		const mins = Math.floor(diffMs / 60000);
		if (mins < 1) return 'just now';
		if (mins < 60) return `${mins}m ago`;
		const hours = Math.floor(mins / 60);
		if (hours < 24) return `${hours}h ago`;
		const days = Math.floor(hours / 24);
		return `${days}d ago`;
	}

	function formatStatusSummary(summary: AISessionResponse['agent_summary']): Array<{ text: string; className: string }> {
		if (!summary) return [];
		const parts: Array<{ text: string; className: string }> = [];
		if (summary.running_count && summary.running_count > 0) {
			parts.push({ text: `${summary.running_count} running`, className: 'text-status-active' });
		}
		if (summary.completed_count && summary.completed_count > 0) {
			parts.push({ text: `${summary.completed_count} completed`, className: 'text-status-closed' });
		}
		if (summary.error_count && summary.error_count > 0) {
			parts.push({ text: `${summary.error_count} error`, className: 'text-status-blocked' });
		}
		return parts;
	}

	onMount(() => {
		loadSessions();
		refreshInterval = setInterval(loadSessions, 30000);
	});

	onDestroy(() => {
		if (refreshInterval) {
			clearInterval(refreshInterval);
		}
	});
</script>

<section class="mb-8">
	<div class="flex items-center justify-between mb-3">
		<div class="flex items-center gap-3">
			<svg class="w-5 h-5 text-text-muted" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
				<path d="M12 2a4 4 0 0 1 4 4c0 1.5-.8 2.8-2 3.4V11h3a3 3 0 0 1 3 3v1.5a2.5 2.5 0 0 1 0 5V21a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1v-.5a2.5 2.5 0 0 1 0-5V14a3 3 0 0 1 3-3h3V9.4A4 4 0 0 1 12 2z" />
				<circle cx="9" cy="17" r="1" fill="currentColor" />
				<circle cx="15" cy="17" r="1" fill="currentColor" />
			</svg>
			<h2 class="text-lg font-semibold text-text-primary">Recent AI Sessions</h2>
			{#if !loading && sessions.length > 0}
				<span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-surface-700 text-text-muted">
					{sessions.length}
				</span>
			{/if}
		</div>
		<a href="/{projectId}/ai" class="text-sm text-accent-blue hover:text-accent-blue/80 transition-colors">
			View all &rarr;
		</a>
	</div>

	{#if loading}
		<div class="flex items-center gap-2 text-sm text-text-muted py-4">
			<svg class="w-4 h-4 animate-spin" viewBox="0 0 24 24" fill="none">
				<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
				<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
			</svg>
			Loading sessions...
		</div>
	{:else if error}
		<div class="card p-4 text-center">
			<p class="text-status-blocked text-sm">{error}</p>
		</div>
	{:else if sessions.length === 0}
		<div class="card p-4 text-center">
			<p class="text-text-muted text-sm">No AI sessions yet</p>
		</div>
	{:else}
		<div class="card divide-y divide-surface-600">
			{#each sessions as session (session.id)}
				{@const parts = formatStatusSummary(session.agent_summary)}
				<div class="flex items-center gap-4 px-4 py-3">
					<a
						href="/{projectId}/ai/{session.id}"
						class="font-mono text-sm text-accent-blue hover:underline shrink-0"
					>
						{truncateId(session.id)}
					</a>
					<span class="text-xs text-text-muted shrink-0">
						{timeAgo(session.started_at)}
					</span>
					<span class="text-xs ml-auto">
						{#if parts.length > 0}
							{#each parts as part, i}
								{#if i > 0}<span class="text-text-muted mx-1">&middot;</span>{/if}
								<span class={part.className}>{part.text}</span>
							{/each}
						{:else}
							<span class="text-text-muted">—</span>
						{/if}
					</span>
				</div>
			{/each}
		</div>
	{/if}
</section>
