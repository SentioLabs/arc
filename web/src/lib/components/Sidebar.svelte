<script lang="ts">
	import type { components } from '$lib/api/types';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';

	type Workspace = components['schemas']['Workspace'];

	interface Props {
		workspaces?: Workspace[];
		currentWorkspace?: Workspace;
	}

	let { workspaces = [], currentWorkspace }: Props = $props();

	// Workspace search state
	let searchQuery = $state('');

	// Filter workspaces based on search
	const filteredWorkspaces = $derived.by(() => {
		if (!searchQuery.trim()) return workspaces;
		const q = searchQuery.toLowerCase().trim();
		return workspaces.filter(
			(w) =>
				w.name.toLowerCase().includes(q) ||
				w.prefix.toLowerCase().includes(q) ||
				(w.description?.toLowerCase().includes(q) ?? false)
		);
	});

	function selectWorkspace(ws: Workspace) {
		searchQuery = '';
		goto(`/${ws.id}`);
	}

	// Navigation items for workspace context
	const navItems = [
		{ label: 'Issues', href: 'issues', icon: 'issues' },
		{ label: 'Ready', href: 'ready', icon: 'ready' },
		{ label: 'Blocked', href: 'blocked', icon: 'blocked' }
	];

	// Icons as SVG paths
	const icons: Record<string, string> = {
		issues: 'M4 6h16v2H4V6zm0 5h16v2H4v-2zm0 5h16v2H4v-2z',
		ready: 'M9 16.2L4.8 12l-1.4 1.4L9 19 21 7l-1.4-1.4L9 16.2z',
		blocked:
			'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zM4 12c0-4.42 3.58-8 8-8 1.85 0 3.55.63 4.9 1.69L5.69 16.9C4.63 15.55 4 13.85 4 12zm8 8c-1.85 0-3.55-.63-4.9-1.69L18.31 7.1C19.37 8.45 20 10.15 20 12c0 4.42-3.58 8-8 8z',
		labels:
			'M17.63 5.84C17.27 5.33 16.67 5 16 5L5 5.01C3.9 5.01 3 5.9 3 7v10c0 1.1.9 1.99 2 1.99L16 19c.67 0 1.27-.33 1.63-.84L22 12l-4.37-6.16z',
		workspace:
			'M10 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2h-8l-2-2z'
	};

	function isActive(href: string): boolean {
		const path = $page.url.pathname;
		if (href === 'issues') {
			return path.includes('/issues') && !path.includes('/new');
		}
		return path.includes(`/${href}`);
	}
</script>

<aside class="w-64 h-screen bg-surface-800 border-r border-border flex flex-col sticky top-0">
	<!-- Logo/Brand -->
	<div class="p-4 border-b border-border">
		<a href="/" class="flex items-center gap-2 group">
			<div
				class="w-8 h-8 bg-primary-600 rounded-lg flex items-center justify-center group-hover:bg-primary-500 transition-colors"
			>
				<span class="font-mono font-bold text-white text-sm">A</span>
			</div>
			<span class="font-semibold text-lg text-text-primary">Arc</span>
		</a>
	</div>

	<!-- Workspace List with Search (only when viewing a workspace) -->
	{#if workspaces.length > 0 && currentWorkspace}
		<div class="p-3 border-b border-border">
			<div class="text-xs font-medium text-text-muted uppercase tracking-wider mb-2">
				Workspaces
			</div>
			<!-- Search input -->
			<div class="relative mb-2">
				<svg
					class="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-text-muted"
					viewBox="0 0 24 24"
					fill="currentColor"
				>
					<path
						d="M15.5 14h-.79l-.28-.27C15.41 12.59 16 11.11 16 9.5 16 5.91 13.09 3 9.5 3S3 5.91 3 9.5 5.91 16 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z"
					/>
				</svg>
				<input
					type="text"
					placeholder="Search..."
					bind:value={searchQuery}
					class="w-full input pl-8 pr-7 py-1.5 text-sm"
				/>
				{#if searchQuery}
					<button
						type="button"
						onclick={() => (searchQuery = '')}
						class="absolute right-2 top-1/2 -translate-y-1/2 text-text-muted hover:text-text-primary transition-colors"
						aria-label="Clear search"
					>
						<svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="currentColor">
							<path
								d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"
							/>
						</svg>
					</button>
				{/if}
			</div>
			<!-- Workspace list -->
			<div class="max-h-40 overflow-y-auto space-y-0.5">
				{#each filteredWorkspaces as ws (ws.id)}
					{@const isCurrent = ws.id === currentWorkspace?.id}
					<button
						type="button"
						class="w-full flex items-center gap-2 px-2 py-1.5 rounded text-left text-sm transition-colors {isCurrent
							? 'bg-primary-600/20 text-primary-400'
							: 'text-text-secondary hover:bg-surface-700 hover:text-text-primary'}"
						onclick={() => selectWorkspace(ws)}
					>
						<span
							class="flex-shrink-0 w-5 h-5 rounded bg-surface-600 flex items-center justify-center text-xs font-mono font-medium {isCurrent
								? 'text-primary-400'
								: 'text-text-muted'}"
						>
							{ws.prefix.charAt(0).toUpperCase()}
						</span>
						<span class="truncate">{ws.name}</span>
						{#if isCurrent}
							<svg class="w-3.5 h-3.5 ml-auto flex-shrink-0" viewBox="0 0 24 24" fill="currentColor">
								<path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z" />
							</svg>
						{/if}
					</button>
				{:else}
					<p class="text-xs text-text-muted px-2 py-1">No matching workspaces</p>
				{/each}
			</div>
		</div>
	{/if}

	<!-- Navigation -->
	{#if currentWorkspace}
		<nav class="flex-1 p-3 space-y-1 overflow-y-auto">
			<div class="text-xs font-medium text-text-muted uppercase tracking-wider px-2 mb-3">
				Navigation
			</div>

			{#each navItems as item (item.href)}
				{@const active = isActive(item.href)}
				<a href="/{currentWorkspace.id}/{item.href}" class="nav-link {active ? 'active' : ''}">
					<svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
						<path d={icons[item.icon]} />
					</svg>
					{item.label}
				</a>
			{/each}

			<!-- Global section -->
			<div class="pt-3 mt-3 border-t border-border">
				<div class="text-xs font-medium text-text-muted uppercase tracking-wider px-2 mb-2">
					Global
				</div>
				<a href="/labels" class="nav-link {$page.url.pathname === '/labels' ? 'active' : ''}">
					<svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
						<path d={icons.labels} />
					</svg>
					Labels
				</a>
			</div>

			<!-- Create Issue button -->
			<div class="pt-4 mt-4 border-t border-border">
				<a href="/{currentWorkspace.id}/issues/new" class="btn btn-primary w-full justify-center">
					<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
						<path d="M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6v2z" />
					</svg>
					New Issue
				</a>
			</div>
		</nav>
	{:else}
		<nav class="flex-1 p-3 flex flex-col overflow-hidden">
			<div class="text-xs font-medium text-text-muted uppercase tracking-wider px-2 mb-3">
				Workspaces
			</div>
			{#if workspaces.length > 0}
				<!-- Search input -->
				<div class="relative mb-3 flex-shrink-0">
					<svg
						class="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-text-muted"
						viewBox="0 0 24 24"
						fill="currentColor"
					>
						<path
							d="M15.5 14h-.79l-.28-.27C15.41 12.59 16 11.11 16 9.5 16 5.91 13.09 3 9.5 3S3 5.91 3 9.5 5.91 16 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z"
						/>
					</svg>
					<input
						type="text"
						placeholder="Search workspaces..."
						bind:value={searchQuery}
						class="w-full input pl-8 pr-7 py-1.5 text-sm"
					/>
					{#if searchQuery}
						<button
							type="button"
							onclick={() => (searchQuery = '')}
							class="absolute right-2 top-1/2 -translate-y-1/2 text-text-muted hover:text-text-primary transition-colors"
							aria-label="Clear search"
						>
							<svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="currentColor">
								<path
									d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"
								/>
							</svg>
						</button>
					{/if}
				</div>
				<!-- Workspace list -->
				<div class="flex-1 overflow-y-auto space-y-1">
					{#each filteredWorkspaces as ws (ws.id)}
						<a href="/{ws.id}" class="nav-link">
							<svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
								<path d={icons.workspace} />
							</svg>
							{ws.name}
						</a>
					{:else}
						<p class="text-sm text-text-muted px-2">No matching workspaces</p>
					{/each}
				</div>
			{:else}
				<p class="text-sm text-text-muted px-2">No workspaces yet</p>
			{/if}

			<!-- Global section -->
			<div class="pt-3 mt-3 border-t border-border">
				<a href="/labels" class="nav-link">
					<svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
						<path d={icons.labels} />
					</svg>
					Labels
				</a>
			</div>
		</nav>
	{/if}

	<!-- Footer -->
	<div class="p-3 border-t border-border text-xs text-text-muted">
		<span class="font-mono">arc</span> • Issue Tracker
	</div>
</aside>
