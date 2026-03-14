<script lang="ts">
	import { page } from '$app/stores';
	import {
		getAISession,
		listAIAgents,
		type AISessionResponse,
		type AIAgentResponse
	} from '$lib/api/ai';

	const sessionId = $derived($page.params.sessionId);

	let session = $state<AISessionResponse | null>(null);
	let agents = $state<AIAgentResponse[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	$effect(() => {
		if (sessionId) {
			loadSession();
		}
	});

	async function loadSession() {
		const id = sessionId;
		if (!id) return;
		loading = true;
		error = null;
		try {
			const [sessionData, agentsData] = await Promise.all([
				getAISession(id),
				listAIAgents(id)
			]);
			session = sessionData;
			agents = agentsData;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load AI session';
		} finally {
			loading = false;
		}
	}

	function formatDate(dateStr: string): string {
		const date = new Date(dateStr);
		return date.toLocaleString();
	}

	function formatDuration(ms: number | undefined): string {
		if (ms === undefined || ms === null) return '-';
		if (ms < 1000) return `${ms}ms`;
		const seconds = Math.floor(ms / 1000);
		if (seconds < 60) return `${seconds}s`;
		const minutes = Math.floor(seconds / 60);
		const remainingSeconds = seconds % 60;
		return `${minutes}m ${remainingSeconds}s`;
	}

	function formatTokens(tokens: number | undefined): string {
		if (tokens === undefined || tokens === null) return '-';
		if (tokens >= 1000) return `${(tokens / 1000).toFixed(1)}k`;
		return String(tokens);
	}

	function statusColor(status: string): string {
		switch (status) {
			case 'completed':
				return 'text-status-open';
			case 'running':
				return 'text-primary-400';
			case 'error':
				return 'text-status-blocked';
			default:
				return 'text-text-muted';
		}
	}
</script>

<div class="flex-1 p-6 animate-fade-in">
	{#if loading}
		<div class="flex items-center justify-center py-12">
			<div class="text-text-muted animate-pulse">Loading...</div>
		</div>
	{:else if error}
		<div class="card p-8 text-center">
			<p class="text-status-blocked mb-4">{error}</p>
			<button class="btn btn-primary" onclick={loadSession}>Retry</button>
		</div>
	{:else if session}
		<!-- Header -->
		<header class="mb-6">
			<div class="flex items-center gap-2 mb-2">
				<a href="/ai" class="text-text-muted hover:text-text-primary transition-colors text-sm">
					AI Sessions
				</a>
				<span class="text-text-muted text-sm">/</span>
			</div>
			<h1 class="text-2xl font-bold text-text-primary mb-3 font-mono break-all">
				{session.id}
			</h1>
			<div class="flex flex-wrap gap-x-6 gap-y-2 text-sm text-text-secondary">
				<div>
					<span class="text-text-muted">Started:</span>
					{formatDate(session.started_at)}
				</div>
				{#if session.cwd}
					<div class="font-mono">
						<span class="text-text-muted">CWD:</span>
						{session.cwd}
					</div>
				{/if}
				<div>
					<span class="text-text-muted">Agents:</span>
					{agents.length}
				</div>
			</div>
		</header>

		<!-- Agents table -->
		{#if agents.length === 0}
			<div class="card p-8 text-center">
				<p class="text-text-muted">No agents recorded for this session</p>
			</div>
		{:else}
			<div class="card overflow-hidden">
				<div class="px-4 py-3 border-b border-border">
					<h2 class="text-sm font-medium text-text-muted uppercase tracking-wider">
						Agents ({agents.length})
					</h2>
				</div>
				<div class="overflow-x-auto">
					<table class="w-full">
						<thead>
							<tr class="border-b border-border text-left text-sm text-text-muted">
								<th class="px-4 py-3 font-medium">Description</th>
								<th class="px-4 py-3 font-medium">Type</th>
								<th class="px-4 py-3 font-medium">Model</th>
								<th class="px-4 py-3 font-medium">Status</th>
								<th class="px-4 py-3 font-medium">Duration</th>
								<th class="px-4 py-3 font-medium">Tokens</th>
								<th class="px-4 py-3 font-medium">Tool Uses</th>
							</tr>
						</thead>
						<tbody>
							{#each agents as agent (agent.id)}
								<tr class="border-b border-border last:border-0 hover:bg-surface-700/50 transition-colors">
									<td class="px-4 py-3 text-sm text-text-primary max-w-xs truncate" title={agent.description ?? ''}>
										<a
											href="/ai/{session.id}/agents/{agent.id}"
											class="hover:text-primary-400 transition-colors"
										>
											{agent.description ?? '-'}
										</a>
									</td>
									<td class="px-4 py-3 text-sm text-text-secondary">
										{agent.agent_type ?? '-'}
									</td>
									<td class="px-4 py-3 text-sm text-text-secondary font-mono">
										{agent.model ?? '-'}
									</td>
									<td class="px-4 py-3 text-sm font-medium {statusColor(agent.status)}">
										{agent.status}
									</td>
									<td class="px-4 py-3 text-sm text-text-secondary font-mono">
										{formatDuration(agent.duration_ms)}
									</td>
									<td class="px-4 py-3 text-sm text-text-secondary font-mono">
										{formatTokens(agent.total_tokens)}
									</td>
									<td class="px-4 py-3 text-sm text-text-secondary font-mono">
										{agent.tool_use_count ?? '-'}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</div>
		{/if}
	{/if}
</div>
