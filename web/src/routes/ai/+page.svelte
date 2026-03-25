<script lang="ts">
	import {
		listAISessions,
		deleteAISession,
		batchDeleteAISessions,
		type AISessionResponse
	} from '$lib/api/ai';

	let sessions = $state<AISessionResponse[]>([]);
	let total = $state(0);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let selected = $state<Set<string>>(new Set());
	let deleting = $state(false);
	let confirmDeleteId = $state<string | null>(null);
	let confirmBatchDelete = $state(false);

	const allSelected = $derived(sessions.length > 0 && selected.size === sessions.length);
	const someSelected = $derived(selected.size > 0 && selected.size < sessions.length);

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

	function toggleSelect(id: string) {
		const next = new Set(selected);
		if (next.has(id)) {
			next.delete(id);
		} else {
			next.add(id);
		}
		selected = next;
	}

	function toggleSelectAll() {
		if (allSelected) {
			selected = new Set();
		} else {
			selected = new Set(sessions.map((s) => s.id));
		}
	}

	async function handleDelete(id: string) {
		deleting = true;
		try {
			await deleteAISession(id);
			confirmDeleteId = null;
			selected = new Set([...selected].filter((s) => s !== id));
			await loadSessions();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to delete session';
		} finally {
			deleting = false;
		}
	}

	async function handleBatchDelete() {
		if (selected.size === 0) return;
		deleting = true;
		try {
			await batchDeleteAISessions([...selected]);
			confirmBatchDelete = false;
			selected = new Set();
			await loadSessions();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to delete sessions';
		} finally {
			deleting = false;
		}
	}

	function truncateId(id: string): string {
		return id.substring(0, 8);
	}

	function formatDate(dateStr: string): string {
		const date = new Date(dateStr);
		return date.toLocaleString();
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
</script>

<div class="flex-1 p-6 animate-fade-in">
	<header class="mb-6 flex items-start justify-between">
		<div>
			<h1 class="text-2xl font-bold text-text-primary mb-1">AI Sessions</h1>
			<p class="text-text-secondary">
				{total} session{total !== 1 ? 's' : ''} tracked
			</p>
		</div>
		{#if selected.size > 0}
			<div class="flex items-center gap-3">
				<span class="text-sm text-text-secondary">
					{selected.size} selected
				</span>
				{#if confirmBatchDelete}
					<button
						class="btn btn-sm btn-ghost"
						onclick={() => (confirmBatchDelete = false)}
						disabled={deleting}
					>
						Cancel
					</button>
					<button
						class="btn btn-sm btn-danger"
						onclick={handleBatchDelete}
						disabled={deleting}
					>
						{deleting ? 'Deleting...' : `Delete ${selected.size}`}
					</button>
				{:else}
					<button
						class="btn btn-sm btn-ghost"
						onclick={() => (selected = new Set())}
					>
						Clear
					</button>
					<button
						class="btn btn-sm btn-danger"
						onclick={() => (confirmBatchDelete = true)}
					>
						Delete selected
					</button>
				{/if}
			</div>
		{/if}
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
						<th class="px-4 py-3 w-10">
							<input
								type="checkbox"
								class="checkbox"
								checked={allSelected}
								indeterminate={someSelected}
								onchange={toggleSelectAll}
								aria-label="Select all sessions"
							/>
						</th>
						<th class="px-4 py-3 font-medium">Session ID</th>
						<th class="px-4 py-3 font-medium">Started</th>
						<th class="px-4 py-3 font-medium">CWD</th>
						<th class="px-4 py-3 w-10"></th>
					</tr>
				</thead>
				<tbody>
					{#each sessions as session (session.id)}
						{@const isSelected = selected.has(session.id)}
						<tr
							class="group border-b border-border last:border-0 transition-colors {isSelected ? 'bg-primary-600/10' : 'hover:bg-surface-700/50'}"
						>
							<td class="px-4 py-3">
								<input
									type="checkbox"
									class="checkbox"
									checked={isSelected}
									onchange={() => toggleSelect(session.id)}
									aria-label="Select session {truncateId(session.id)}"
								/>
							</td>
							<td class="px-4 py-3">
								<a
									href="/ai/{session.id}"
									class="font-mono text-sm text-primary-400 hover:text-primary-300 transition-colors"
									title={session.id}
								>
									{truncateId(session.id)}
								</a>
							</td>
							<td class="px-4 py-3 text-sm text-text-secondary" title={formatDate(session.started_at)}>
								{timeAgo(session.started_at)}
							</td>
							<td
								class="px-4 py-3 text-sm text-text-secondary font-mono truncate max-w-xs"
								title={session.cwd ?? ''}
							>
								{session.cwd ?? '-'}
							</td>
							<td class="px-4 py-3">
								{#if confirmDeleteId === session.id}
									<div class="flex items-center gap-1">
										<button
											class="btn btn-sm btn-danger btn-icon"
											onclick={() => handleDelete(session.id)}
											disabled={deleting}
											title="Confirm delete"
										>
											<svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
												<polyline points="20 6 9 17 4 12" />
											</svg>
										</button>
										<button
											class="btn btn-sm btn-ghost btn-icon"
											onclick={() => (confirmDeleteId = null)}
											disabled={deleting}
											title="Cancel"
										>
											<svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
												<line x1="18" y1="6" x2="6" y2="18" />
												<line x1="6" y1="6" x2="18" y2="18" />
											</svg>
										</button>
									</div>
								{:else}
									<button
										class="btn btn-sm btn-ghost btn-icon opacity-0 group-hover:opacity-100 hover:!opacity-100 hover:text-status-blocked transition-all"
										onclick={() => (confirmDeleteId = session.id)}
										title="Delete session"
									>
										<svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
											<polyline points="3 6 5 6 21 6" />
											<path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
										</svg>
									</button>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>
