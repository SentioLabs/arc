<script lang="ts">
	import { Header, Select } from '$lib/components';
	import RoleLane from '$lib/components/RoleLane.svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { getContext } from 'svelte';
	import type { Writable } from 'svelte/store';
	import {
		getTeamContext,
		listIssues,
		type Workspace,
		type Issue,
		type TeamContext
	} from '$lib/api';

	const workspaces = getContext<Writable<Workspace[]>>('workspaces');
	const workspaceId = $derived($page.params.workspaceId);
	const workspace = $derived($workspaces.find((ws) => ws.id === workspaceId));

	// Epic selection via URL param
	const epicId = $derived($page.url.searchParams.get('epic_id') ?? '');

	let teamContext = $state<TeamContext | null>(null);
	let epics = $state<Issue[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let planExpanded = $state(false);

	// Role color palette
	const roleColors: Record<string, string> = {
		frontend: '#3b82f6',
		backend: '#22c55e',
		design: '#f59e0b',
		devops: '#ef4444',
		testing: '#8b5cf6',
		data: '#06b6d4',
		mobile: '#ec4899',
		infra: '#f97316'
	};

	function getRoleColor(role: string): string {
		if (roleColors[role]) return roleColors[role];
		// Hash-based fallback for unknown roles
		let hash = 0;
		for (let i = 0; i < role.length; i++) {
			hash = role.charCodeAt(i) + ((hash << 5) - hash);
		}
		const hue = Math.abs(hash) % 360;
		return `hsl(${hue}, 65%, 55%)`;
	}

	// Load epics on workspace change
	$effect(() => {
		if (workspaceId) {
			loadEpics();
		}
	});

	// Load team context when epic changes or on initial load
	$effect(() => {
		if (workspaceId) {
			loadTeamContext();
		}
	});

	async function loadEpics() {
		try {
			const result = await listIssues(workspaceId, { type: 'epic', status: 'open', limit: 100 });
			epics = result.data ?? [];
		} catch {
			/* epics are optional */
		}
	}

	async function loadTeamContext() {
		loading = true;
		error = null;
		try {
			teamContext = await getTeamContext(workspaceId, epicId || undefined);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load team context';
		} finally {
			loading = false;
		}
	}

	function selectEpic(value: string) {
		const params = new URLSearchParams($page.url.searchParams);
		if (value) {
			params.set('epic_id', value);
		} else {
			params.delete('epic_id');
		}
		goto(`/${workspaceId}/teams?${params}`, { noScroll: true });
	}

	const roles = $derived(teamContext ? Object.entries(teamContext.roles) : []);
	const totalIssues = $derived(
		roles.reduce((sum, [, role]) => sum + role.issues.length, 0) +
			(teamContext?.unassigned?.length ?? 0)
	);
	const epicOptions = $derived([
		{ value: '', label: 'All teammate issues' },
		...epics.map((e) => ({ value: e.id, label: `${e.id}: ${e.title}` }))
	]);
</script>

{#if workspace}
	<Header {workspace} title="Teams" showSearch={false} />

	<div class="flex-1 p-6 animate-fade-in">
		<!-- Page header -->
		<header class="mb-6">
			<div class="flex items-center gap-3 mb-4">
				<div class="w-10 h-10 bg-primary-600/20 rounded-lg flex items-center justify-center">
					<svg class="w-5 h-5 text-primary-400" viewBox="0 0 24 24" fill="currentColor">
						<path
							d="M16 11c1.66 0 2.99-1.34 2.99-3S17.66 5 16 5c-1.66 0-3 1.34-3 3s1.34 3 3 3zm-8 0c1.66 0 2.99-1.34 2.99-3S9.66 5 8 5C6.34 5 5 6.34 5 8s1.34 3 3 3zm0 2c-2.33 0-7 1.17-7 3.5V19h14v-2.5c0-2.33-4.67-3.5-7-3.5zm8 0c-.29 0-.62.02-.97.05 1.16.84 1.97 1.97 1.97 3.45V19h6v-2.5c0-2.33-4.67-3.5-7-3.5z"
						/>
					</svg>
				</div>
				<div class="flex-1">
					<h1 class="text-2xl font-bold text-text-primary">Teams</h1>
					<p class="text-sm text-text-secondary">
						{#if loading}
							Loading team context...
						{:else}
							{totalIssues} issues across {roles.length} roles
						{/if}
					</p>
				</div>
			</div>

			<!-- Epic selector -->
			<div class="flex items-center gap-3">
				<span class="text-sm text-text-secondary">Scope:</span>
				<Select options={epicOptions} value={epicId} placeholder="All teammate issues" onchange={selectEpic} />
			</div>
		</header>

		<!-- Epic plan summary (if epic selected and has plan) -->
		{#if teamContext?.epic?.plan}
			<div class="card p-4 mb-6">
				<button
					type="button"
					class="flex items-center gap-2 w-full text-left"
					onclick={() => (planExpanded = !planExpanded)}
				>
					<svg
						class="w-4 h-4 text-text-muted transition-transform {planExpanded
							? 'rotate-90'
							: ''}"
						viewBox="0 0 24 24"
						fill="currentColor"
					>
						<path d="M10 6L8.59 7.41 13.17 12l-4.58 4.59L10 18l6-6z" />
					</svg>
					<span class="text-sm font-medium text-text-primary">
						Epic Plan: {teamContext.epic.title}
					</span>
				</button>
				{#if planExpanded}
					<pre
						class="mt-3 text-sm text-text-secondary whitespace-pre-wrap font-mono bg-surface-800 rounded p-3 max-h-64 overflow-y-auto">{teamContext.epic.plan}</pre>
				{/if}
			</div>
		{/if}

		<!-- Content -->
		{#if loading}
			<div class="flex items-center justify-center py-12">
				<div class="text-text-muted animate-pulse">Loading...</div>
			</div>
		{:else if error}
			<div class="card p-8 text-center">
				<p class="text-status-blocked mb-4">{error}</p>
				<button class="btn btn-primary" onclick={loadTeamContext}>Retry</button>
			</div>
		{:else if roles.length === 0 && (teamContext?.unassigned?.length ?? 0) === 0}
			<div class="card p-12 text-center">
				<div
					class="w-16 h-16 bg-surface-700 rounded-2xl flex items-center justify-center mx-auto mb-4"
				>
					<svg class="w-8 h-8 text-text-muted" viewBox="0 0 24 24" fill="currentColor">
						<path
							d="M16 11c1.66 0 2.99-1.34 2.99-3S17.66 5 16 5c-1.66 0-3 1.34-3 3s1.34 3 3 3zm-8 0c1.66 0 2.99-1.34 2.99-3S9.66 5 8 5C6.34 5 5 6.34 5 8s1.34 3 3 3zm0 2c-2.33 0-7 1.17-7 3.5V19h14v-2.5c0-2.33-4.67-3.5-7-3.5zm8 0c-.29 0-.62.02-.97.05 1.16.84 1.97 1.97 1.97 3.45V19h6v-2.5c0-2.33-4.67-3.5-7-3.5z"
						/>
					</svg>
				</div>
				<h2 class="text-xl font-semibold text-text-primary mb-2">No team assignments</h2>
				<p class="text-text-secondary">
					Add <code class="font-mono text-xs bg-surface-700 px-1.5 py-0.5 rounded"
						>teammate:role</code
					> labels to issues to see them grouped here.
				</p>
			</div>
		{:else}
			<!-- Role lanes -->
			<div class="flex gap-4 overflow-x-auto pb-4">
				{#each roles as [role, data] (role)}
					<RoleLane {role} issues={data.issues} {workspaceId} color={getRoleColor(role)} />
				{/each}

				<!-- Unassigned lane (only when filtering by epic) -->
				{#if teamContext?.unassigned && teamContext.unassigned.length > 0}
					<RoleLane
						role="unassigned"
						issues={teamContext.unassigned}
						{workspaceId}
						color="#6b7280"
					/>
				{/if}
			</div>
		{/if}
	</div>
{/if}
