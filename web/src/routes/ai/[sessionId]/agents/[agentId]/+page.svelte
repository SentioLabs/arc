<script lang="ts">
	import { page } from '$app/stores';
	import {
		getAIAgent,
		getAgentTranscript,
		type AIAgentResponse
	} from '$lib/api/ai';
	import TranscriptViewer from '$lib/components/TranscriptViewer.svelte';

	const sessionId = $derived($page.params.sessionId);
	const agentId = $derived($page.params.agentId);

	let agent = $state<AIAgentResponse | null>(null);
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	let transcript = $state<any[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let transcriptNotFound = $state(false);

	$effect(() => {
		if (sessionId && agentId) {
			loadData();
		}
	});

	async function loadData() {
		const sid = sessionId;
		const aid = agentId;
		if (!sid || !aid) return;
		loading = true;
		error = null;
		transcriptNotFound = false;
		try {
			const [agentData, transcriptData] = await Promise.allSettled([
				getAIAgent(sid, aid),
				getAgentTranscript(sid, aid)
			]);

			if (agentData.status === 'fulfilled') {
				agent = agentData.value;
			} else {
				throw new Error(agentData.reason?.message ?? 'Failed to load agent');
			}

			if (transcriptData.status === 'fulfilled') {
				transcript = transcriptData.value;
			} else {
				transcriptNotFound = true;
				transcript = [];
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load agent data';
		} finally {
			loading = false;
		}
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
			<button class="btn btn-primary" onclick={loadData}>Retry</button>
		</div>
	{:else if agent}
		<!-- Breadcrumb -->
		<header class="mb-6">
			<div class="flex items-center gap-2 mb-2 text-sm">
				<a href="/ai" class="text-text-muted hover:text-text-primary transition-colors">
					AI Sessions
				</a>
				<span class="text-text-muted">/</span>
				<a href="/ai/{sessionId}" class="text-text-muted hover:text-text-primary transition-colors font-mono">
					Session {sessionId?.substring(0, 8)}
				</a>
				<span class="text-text-muted">/</span>
				<span class="text-text-primary">
					{agent.description ?? `Agent ${agentId?.substring(0, 8)}`}
				</span>
			</div>

			<!-- Agent metadata header -->
			<h1 class="text-2xl font-bold text-text-primary mb-3">
				{agent.description ?? 'Agent'}
			</h1>
			<div class="flex flex-wrap gap-x-6 gap-y-2 text-sm text-text-secondary">
				{#if agent.agent_type}
					<div>
						<span class="text-text-muted">Type:</span>
						{agent.agent_type}
					</div>
				{/if}
				{#if agent.model}
					<div class="font-mono">
						<span class="text-text-muted">Model:</span>
						{agent.model}
					</div>
				{/if}
				<div>
					<span class="text-text-muted">Status:</span>
					<span class={statusColor(agent.status)}>{agent.status}</span>
				</div>
				<div class="font-mono">
					<span class="text-text-muted">Duration:</span>
					{formatDuration(agent.duration_ms)}
				</div>
				<div class="font-mono">
					<span class="text-text-muted">Tokens:</span>
					{formatTokens(agent.total_tokens)}
				</div>
				<div class="font-mono">
					<span class="text-text-muted">Tool uses:</span>
					{agent.tool_use_count ?? '-'}
				</div>
			</div>
		</header>

		<!-- Transcript -->
		{#if transcriptNotFound}
			<div class="card p-8 text-center">
				<p class="text-text-muted">Transcript not found for this agent. The transcript file may not exist or may have been removed.</p>
			</div>
		{:else}
			<TranscriptViewer {transcript} />
		{/if}
	{/if}
</div>
