<script lang="ts">
	import type { components } from '$lib/api/types';
	import { page } from '$app/stores';

	type Workspace = components['schemas']['Workspace'];

	interface Props {
		workspaces?: Workspace[];
		currentWorkspace?: Workspace;
	}

	let { workspaces = [], currentWorkspace }: Props = $props();

	// Navigation items for workspace context
	const navItems = [
		{ label: 'Issues', href: 'issues', icon: 'issues' },
		{ label: 'Ready', href: 'ready', icon: 'ready' },
		{ label: 'Blocked', href: 'blocked', icon: 'blocked' },
		{ label: 'Labels', href: 'labels', icon: 'labels' }
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

	<!-- Workspace Selector -->
	{#if workspaces.length > 0}
		<div class="p-3 border-b border-border">
			<label
				for="workspace-selector"
				class="block text-xs font-medium text-text-muted uppercase tracking-wider mb-2"
			>
				Workspace
			</label>
			<div class="relative">
				<select
					id="workspace-selector"
					class="w-full input appearance-none pr-8 cursor-pointer"
					value={currentWorkspace?.id ?? ''}
					onchange={(e) => {
						const wsId = e.currentTarget.value;
						if (wsId) {
							window.location.href = `/${wsId}`;
						}
					}}
				>
					<option value="" disabled>Select workspace</option>
					{#each workspaces as ws (ws.id)}
						<option value={ws.id}>{ws.name}</option>
					{/each}
				</select>
				<svg
					class="absolute right-2 top-1/2 -translate-y-1/2 w-4 h-4 text-text-muted pointer-events-none"
					viewBox="0 0 24 24"
					fill="currentColor"
				>
					<path d="M7 10l5 5 5-5H7z" />
				</svg>
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
		<nav class="flex-1 p-3">
			<div class="text-xs font-medium text-text-muted uppercase tracking-wider px-2 mb-3">
				Workspaces
			</div>
			{#each workspaces as ws (ws.id)}
				<a href="/{ws.id}" class="nav-link">
					<svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
						<path d={icons.workspace} />
					</svg>
					{ws.name}
				</a>
			{/each}
			{#if workspaces.length === 0}
				<p class="text-sm text-text-muted px-2">No workspaces yet</p>
			{/if}
		</nav>
	{/if}

	<!-- Footer -->
	<div class="p-3 border-t border-border text-xs text-text-muted">
		<span class="font-mono">arc</span> • Issue Tracker
	</div>
</aside>
