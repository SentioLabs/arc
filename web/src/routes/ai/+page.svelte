<script lang="ts">
	import { listAISessions, type AISessionResponse } from '$lib/api/ai';

	let sessions = $state<AISessionResponse[]>([]);
	let total = $state(0);
	let loading = $state(true);
	let error = $state<string | null>(null);

	$effect(() => {
		loadSessions();
	});

	async function loadSessions() {
		loading = true;
		error = null;
		try {
			const result = await listAISessions(100, 0);
			sessions = result.data ?? [];
			total = result.total ?? sessions.length;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load AI sessions';
		} finally {
			loading = false;
		}
	}

	function truncateId(id: string): string {
		return id.substring(0, 8);
	}

	function formatDate(dateStr: string): string {
		const date = new Date(dateStr);
		return date.toLocaleString();
	}
</script>

<div class="flex-1 p-6 animate-fade-in">
	<header class="mb-6">
		<h1 class="text-2xl font-bold text-text-primary mb-1">AI Sessions</h1>
		<p class="text-text-secondary">
			{sessions.length} session{sessions.length !== 1 ? 's' : ''} tracked
		</p>
	</header>

	{#if loading}
		<div class="flex items-center justify-center py-12">
			<div class="text-text-muted animate-pulse">Loading...</div>
		</div>
	{:else if error}
		<div class="card p-8 text-center">
			<p class="text-status-blocked mb-4">{error}</p>
			<button class="btn btn-primary" onclick={loadSessions}>Retry</button>
		</div>
	{:else if sessions.length === 0}
		<div class="card p-12 text-center">
			<div
				class="w-16 h-16 bg-surface-700 rounded-2xl flex items-center justify-center mx-auto mb-4"
			>
				<svg class="w-8 h-8 text-text-muted" viewBox="0 0 24 24" fill="currentColor">
					<path
						d="M20 18c1.1 0 1.99-.9 1.99-2L22 6c0-1.1-.9-2-2-2H4c-1.1 0-2 .9-2 2v10c0 1.1.9 2 2 2H0v2h24v-2h-4zM4 6h16v10H4V6z"
					/>
				</svg>
			</div>
			<h2 class="text-xl font-semibold text-text-primary mb-2">No AI sessions yet</h2>
			<p class="text-text-secondary">Sessions will appear here when AI agents start working</p>
		</div>
	{:else}
		<div class="card overflow-hidden">
			<table class="w-full">
				<thead>
					<tr class="border-b border-border text-left text-sm text-text-muted">
						<th class="px-4 py-3 font-medium">Session ID</th>
						<th class="px-4 py-3 font-medium">Started At</th>
						<th class="px-4 py-3 font-medium">CWD</th>
					</tr>
				</thead>
				<tbody>
					{#each sessions as session (session.id)}
						<tr class="border-b border-border last:border-0 hover:bg-surface-700/50 transition-colors">
							<td class="px-4 py-3">
								<a
									href="/ai/{session.id}"
									class="font-mono text-sm text-primary-400 hover:text-primary-300 transition-colors"
									title={session.id}
								>
									{truncateId(session.id)}
								</a>
							</td>
							<td class="px-4 py-3 text-sm text-text-secondary">
								{formatDate(session.started_at)}
							</td>
							<td class="px-4 py-3 text-sm text-text-secondary font-mono truncate max-w-xs" title={session.cwd ?? ''}>
								{session.cwd ?? '-'}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>
