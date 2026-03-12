<script lang="ts">
	import { getContext, onMount } from 'svelte';
	import type { Writable } from 'svelte/store';
	import {
		deleteWorkspaces,
		mergeWorkspaces,
		listWorkspacePaths,
		type Workspace as Project,
		type MergeResult,
		type WorkspacePath
	} from '$lib/api';
	import { ConfirmDialog, Select } from '$lib/components';

	const projects = getContext<Writable<Project[]>>('workspaces');

	// Project insights state (reporting dashboard)
	let projectPathsMap = $state<Record<string, WorkspacePath[]>>({});
	let insightsLoading = $state(false);
	let insightsLoaded = $state(false);

	// Load paths for all projects (for insights)
	async function loadAllPaths() {
		if (insightsLoaded || insightsLoading || $projects.length === 0) return;
		insightsLoading = true;
		try {
			const results = await Promise.all(
				$projects.map(async (p) => {
					const paths = await listWorkspacePaths(p.id);
					return [p.id, paths] as const;
				})
			);
			const map: Record<string, WorkspacePath[]> = {};
			for (const [id, paths] of results) {
				map[id] = paths;
			}
			projectPathsMap = map;
			insightsLoaded = true;
		} catch (err) {
			console.error('Failed to load project paths:', err);
		} finally {
			insightsLoading = false;
		}
	}

	onMount(() => {
		loadAllPaths();
	});

	// Derived insights
	const allPaths = $derived(Object.values(projectPathsMap).flat());

	const orphanedProjects = $derived(
		$projects.filter((ws) => {
			const paths = projectPathsMap[ws.id];
			return paths !== undefined && paths.length === 0;
		})
	);

	const mostActivePaths = $derived(
		[...allPaths]
			.filter((p) => p.last_accessed_at)
			.sort((a, b) => {
				const dateA = new Date(a.last_accessed_at!).getTime();
				const dateB = new Date(b.last_accessed_at!).getTime();
				return dateB - dateA;
			})
			.slice(0, 5)
	);

	const hostDistribution = $derived.by(() => {
		const counts: Record<string, number> = {};
		for (const p of allPaths) {
			const host = p.hostname || 'unknown';
			counts[host] = (counts[host] || 0) + 1;
		}
		return Object.entries(counts).sort((a, b) => b[1] - a[1]);
	});

	function formatRelativeTime(dateStr: string): string {
		const date = new Date(dateStr);
		const now = new Date();
		const diffMs = now.getTime() - date.getTime();
		const diffMins = Math.floor(diffMs / 60000);
		if (diffMins < 1) return 'just now';
		if (diffMins < 60) return `${diffMins}m ago`;
		const diffHours = Math.floor(diffMins / 60);
		if (diffHours < 24) return `${diffHours}h ago`;
		const diffDays = Math.floor(diffHours / 24);
		if (diffDays < 30) return `${diffDays}d ago`;
		return date.toLocaleDateString();
	}

	function getProjectName(projectId: string): string {
		const p = $projects.find((proj) => proj.id === projectId);
		return p?.name ?? projectId;
	}

	// Search state
	let searchQuery = $state('');
	let searchFocused = $state(false);

	// Filter projects based on search query
	const filteredProjects = $derived.by(() => {
		if (!searchQuery.trim()) return $projects;
		const q = searchQuery.toLowerCase().trim();
		return $projects.filter(
			(w) =>
				w.name.toLowerCase().includes(q) ||
				w.prefix.toLowerCase().includes(q) ||
				(w.description?.toLowerCase().includes(q) ?? false)
		);
	});

	function clearSearch() {
		searchQuery = '';
	}

	// Edit mode state
	let editMode = $state(false);
	let selectedIds = $state<Set<string>>(new Set());
	let deleteDialogOpen = $state(false);
	let deleting = $state(false);
	let projectsToDelete = $state<Project[]>([]);

	// Merge state
	let mergeDialogOpen = $state(false);
	let mergeTargetId = $state('');
	let merging = $state(false);
	let projectsToMerge: Project[] = $state([]);
	let mergeResult: MergeResult | null = $state(null);

	// Workspace paths state (directory paths, read-only expansion)
	let expandedPaths = $state<Set<string>>(new Set());
	let workspacePaths = $state<Record<string, WorkspacePath[]>>({});
	let pathsLoading = $state<Set<string>>(new Set());
	let pathCounts = $state<Record<string, number>>({});

	function togglePaths(projectId: string, event: MouseEvent) {
		event.preventDefault();
		event.stopPropagation();
		const newSet = new Set(expandedPaths);
		if (newSet.has(projectId)) {
			newSet.delete(projectId);
		} else {
			newSet.add(projectId);
		}
		expandedPaths = newSet;
	}

	$effect(() => {
		for (const pid of expandedPaths) {
			if (!workspacePaths[pid] && !pathsLoading.has(pid)) {
				loadPaths(pid);
			}
		}
	});

	async function loadPaths(projectId: string) {
		const newLoading = new Set(pathsLoading);
		newLoading.add(projectId);
		pathsLoading = newLoading;
		try {
			const paths = await listWorkspacePaths(projectId);
			workspacePaths = { ...workspacePaths, [projectId]: paths };
			pathCounts = { ...pathCounts, [projectId]: paths.length };
		} catch (err) {
			console.error('Failed to load paths for project', projectId, err);
			workspacePaths = { ...workspacePaths, [projectId]: [] };
		} finally {
			const done = new Set(pathsLoading);
			done.delete(projectId);
			pathsLoading = done;
		}
	}

	function formatDate(dateStr?: string): string {
		if (!dateStr) return '-';
		const date = new Date(dateStr);
		return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
	}

	// Derived state
	const selectedCount = $derived(selectedIds.size);
	const allSelected = $derived($projects.length > 0 && selectedIds.size === $projects.length);
	const someSelected = $derived(selectedIds.size > 0 && selectedIds.size < $projects.length);

	function toggleEditMode() {
		editMode = !editMode;
		if (editMode) {
			searchQuery = '';
		} else {
			selectedIds = new Set();
		}
	}

	function toggleSelection(id: string) {
		const newSet = new Set(selectedIds);
		if (newSet.has(id)) {
			newSet.delete(id);
		} else {
			newSet.add(id);
		}
		selectedIds = newSet;
	}

	function toggleSelectAll() {
		if (allSelected) {
			selectedIds = new Set();
		} else {
			selectedIds = new Set($projects.map((w) => w.id));
		}
	}

	function handleDeleteSelected() {
		projectsToDelete = $projects.filter((w) => selectedIds.has(w.id));
		deleteDialogOpen = true;
	}

	function handleDeleteSingle(proj: Project) {
		projectsToDelete = [proj];
		deleteDialogOpen = true;
	}

	async function confirmDelete() {
		if (projectsToDelete.length === 0) return;

		deleting = true;
		try {
			const idsToDelete = projectsToDelete.map((w) => w.id);
			await deleteWorkspaces(idsToDelete);

			projects.update((current) => current.filter((w) => !idsToDelete.includes(w.id)));

			selectedIds = new Set();
			projectsToDelete = [];
			deleteDialogOpen = false;

			if ($projects.length === 0) {
				editMode = false;
			}
		} catch (err) {
			console.error('Failed to delete projects:', err);
		} finally {
			deleting = false;
		}
	}

	function cancelDelete() {
		deleteDialogOpen = false;
		projectsToDelete = [];
	}

	// Merge target options: all projects except those being merged
	const mergeTargetOptions = $derived.by(() => {
		const sourceIds = new Set(projectsToMerge.map((w) => w.id));
		return $projects
			.filter((w) => !sourceIds.has(w.id))
			.map((w) => ({ value: w.id, label: w.name }));
	});

	function handleMerge() {
		projectsToMerge = $projects.filter((w) => selectedIds.has(w.id));
		mergeTargetId = '';
		mergeResult = null;
		mergeDialogOpen = true;
	}

	async function confirmMerge() {
		if (!mergeTargetId || projectsToMerge.length === 0) return;

		merging = true;
		try {
			const sourceIds = projectsToMerge.map((w) => w.id);
			const result = await mergeWorkspaces(mergeTargetId, sourceIds);
			mergeResult = result;

			projects.update((current) =>
				current
					.filter((w) => !result.sources_deleted.includes(w.id))
					.map((w) => (w.id === result.target_workspace.id ? result.target_workspace : w))
			);

			selectedIds = new Set();
		} catch (err) {
			console.error('Failed to merge projects:', err);
		} finally {
			merging = false;
		}
	}

	function cancelMerge() {
		mergeDialogOpen = false;
		projectsToMerge = [];
		mergeResult = null;
		mergeTargetId = '';
	}

	function closeMergeAfterSuccess() {
		mergeDialogOpen = false;
		projectsToMerge = [];
		mergeResult = null;
		mergeTargetId = '';

		if ($projects.length <= 1) {
			editMode = false;
		}
	}
</script>

<div class="p-8 max-w-4xl mx-auto animate-fade-in">
	<header class="mb-8">
		<div class="flex items-center justify-between gap-4 mb-3">
			<h1 class="text-4xl font-bold text-text-primary">Projects</h1>
			{#if $projects.length > 0}
				<button
					class="btn {editMode ? 'btn-primary' : 'btn-ghost'} btn-sm"
					onclick={toggleEditMode}
				>
					{#if editMode}
						<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
							<path d="M9 16.2L4.8 12l-1.4 1.4L9 19 21 7l-1.4-1.4L9 16.2z" />
						</svg>
						Done
					{:else}
						<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
							<path
								d="M3 17.25V21h3.75L17.81 9.94l-3.75-3.75L3 17.25zM20.71 7.04c.39-.39.39-1.02 0-1.41l-2.34-2.34c-.39-.39-1.02-.39-1.41 0l-1.83 1.83 3.75 3.75 1.83-1.83z"
							/>
						</svg>
						Edit
					{/if}
				</button>
			{/if}
		</div>
		<p class="text-lg text-text-secondary">Select a project to view and manage issues</p>

		<!-- Search box (hidden in edit mode) -->
		{#if $projects.length > 0 && !editMode}
			<div
				class="mt-6 relative transition-all duration-200 {searchFocused ? 'max-w-md' : 'max-w-sm'}"
			>
				<svg
					class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-muted"
					viewBox="0 0 24 24"
					fill="currentColor"
				>
					<path
						d="M15.5 14h-.79l-.28-.27C15.41 12.59 16 11.11 16 9.5 16 5.91 13.09 3 9.5 3S3 5.91 3 9.5 5.91 16 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z"
					/>
				</svg>
				<input
					type="text"
					placeholder="Search projects..."
					bind:value={searchQuery}
					onfocus={() => (searchFocused = true)}
					onblur={() => (searchFocused = false)}
					class="w-full input pl-9 pr-8 py-2 text-sm"
				/>
				{#if searchQuery}
					<button
						type="button"
						onclick={clearSearch}
						class="absolute right-3 top-1/2 -translate-y-1/2 text-text-muted hover:text-text-primary transition-colors"
						aria-label="Clear search"
					>
						<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
							<path
								d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"
							/>
						</svg>
					</button>
				{/if}
			</div>
		{/if}
	</header>

	<!-- Select all / batch actions bar -->
	{#if editMode && $projects.length > 0}
		<div
			class="flex items-center justify-between gap-4 mb-6 p-3 bg-surface-800 border border-border rounded-lg animate-fade-in"
		>
			<label class="flex items-center gap-3 cursor-pointer select-none">
				<input
					type="checkbox"
					class="checkbox"
					checked={allSelected}
					indeterminate={someSelected}
					onchange={toggleSelectAll}
				/>
				<span class="text-sm text-text-secondary">
					{#if selectedCount === 0}
						Select all
					{:else}
						{selectedCount} selected
					{/if}
				</span>
			</label>

			{#if selectedCount > 0}
				<div class="flex items-center gap-2">
					{#if $projects.length > selectedCount}
						<button class="btn btn-primary btn-sm" onclick={handleMerge}>
							<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
								<path d="M17 20.41L18.41 19 15 15.59 13.59 17 17 20.41zM7.5 8H11v5.59L5.59 19 7 20.41l6-6V8h3.5L12 3.5 7.5 8z" />
							</svg>
							Merge into...
						</button>
					{/if}
				<button class="btn btn-danger btn-sm" onclick={handleDeleteSelected}>
					<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
						<path
							d="M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12zM8 9h8v10H8V9zm7.5-5l-1-1h-5l-1 1H5v2h14V4h-3.5z"
						/>
					</svg>
					Delete {selectedCount === 1 ? 'project' : `${selectedCount} projects`}
				</button>
				</div>
			{/if}
		</div>
	{/if}

	{#if $projects.length === 0}
		<div class="card p-12 text-center">
			<div
				class="w-16 h-16 bg-surface-700 rounded-2xl flex items-center justify-center mx-auto mb-4"
			>
				<svg class="w-8 h-8 text-text-muted" viewBox="0 0 24 24" fill="currentColor">
					<path
						d="M10 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2h-8l-2-2z"
					/>
				</svg>
			</div>
			<h2 class="text-xl font-semibold text-text-primary mb-2">No projects yet</h2>
			<p class="text-text-secondary mb-6">Create a project using the CLI to get started</p>
			<div
				class="bg-surface-800 rounded-lg p-4 font-mono text-sm text-text-secondary text-left inline-block"
			>
				<code>arc init my-project</code>
			</div>
		</div>
	{:else if filteredProjects.length === 0}
		<!-- No results from search -->
		<div class="card p-12 text-center">
			<div
				class="w-16 h-16 bg-surface-700 rounded-2xl flex items-center justify-center mx-auto mb-4"
			>
				<svg class="w-8 h-8 text-text-muted" viewBox="0 0 24 24" fill="currentColor">
					<path
						d="M15.5 14h-.79l-.28-.27C15.41 12.59 16 11.11 16 9.5 16 5.91 13.09 3 9.5 3S3 5.91 3 9.5 5.91 16 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z"
					/>
				</svg>
			</div>
			<h2 class="text-xl font-semibold text-text-primary mb-2">No matching projects</h2>
			<p class="text-text-secondary mb-4">No projects match "{searchQuery}"</p>
			<button type="button" class="btn btn-primary" onclick={clearSearch}> Clear search </button>
		</div>
	{:else}
		<div class="grid gap-4 sm:grid-cols-2">
			{#each filteredProjects as project (project.id)}
				{@const isSelected = selectedIds.has(project.id)}
				{#if editMode}
					<!-- Edit mode: clickable card for selection -->
					<button
						type="button"
						class="card p-6 transition-all duration-200 group relative text-left cursor-pointer {isSelected
							? 'border-primary-500 bg-primary-600/5'
							: 'hover:border-border-focus/50'}"
						onclick={() => toggleSelection(project.id)}
					>
						<!-- Selection checkbox -->
						<div class="absolute top-4 right-4 z-10">
							<input
								type="checkbox"
								class="checkbox"
								checked={isSelected}
								onclick={(e) => e.stopPropagation()}
								onchange={() => toggleSelection(project.id)}
							/>
						</div>

						<div class="pr-8">
							{@render projectContent(project)}
						</div>

						<!-- Delete button (on hover) -->
						<span
							class="absolute bottom-4 right-4 btn btn-ghost btn-icon btn-sm opacity-0 group-hover:opacity-100 transition-opacity text-text-muted hover:text-status-blocked hover:bg-status-blocked/10 hover:border-status-blocked/30"
							role="button"
							tabindex={0}
							title="Delete project"
							onclick={(e) => {
								e.stopPropagation();
								handleDeleteSingle(project);
							}}
							onkeydown={(e) => {
								if (e.key === 'Enter' || e.key === ' ') {
									e.preventDefault();
									e.stopPropagation();
									handleDeleteSingle(project);
								}
							}}
						>
							<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
								<path
									d="M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12zM8 9h8v10H8V9zm7.5-5l-1-1h-5l-1 1H5v2h14V4h-3.5z"
								/>
							</svg>
						</span>
					</button>
				{:else}
					<!-- Normal mode: link to project -->
					<a
						href="/{project.id}"
						class="card p-6 transition-all duration-200 group hover:border-border-focus/50 block"
					>
						{@render projectContent(project)}
					</a>
				{/if}
			{/each}
		</div>
	{/if}

	<!-- Project Insights Section -->
	{#if $projects.length > 0 && insightsLoaded && !editMode}
		<section class="mt-12 animate-fade-in">
			<h2 class="text-2xl font-bold text-text-primary mb-6">Project Insights</h2>

			<div class="grid gap-4 sm:grid-cols-3">
				<!-- Orphaned Projects Card -->
				<div class="card p-6">
					<div class="flex items-center gap-3 mb-4">
						<div class="w-8 h-8 bg-status-blocked/20 rounded-lg flex items-center justify-center">
							<svg class="w-4 h-4 text-status-blocked" viewBox="0 0 24 24" fill="currentColor">
								<path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z" />
							</svg>
						</div>
						<h3 class="text-sm font-semibold text-text-primary">Orphaned Projects</h3>
					</div>
					<p class="text-3xl font-bold text-text-primary mb-2">{orphanedProjects.length}</p>
					{#if orphanedProjects.length === 0}
						<p class="text-xs text-text-muted">All projects have registered paths</p>
					{:else}
						<ul class="space-y-1">
							{#each orphanedProjects as ws}
								<li class="text-xs text-text-secondary truncate" title={ws.name}>
									{ws.name}
								</li>
							{/each}
						</ul>
					{/if}
				</div>

				<!-- Most Active Paths Card -->
				<div class="card p-6">
					<div class="flex items-center gap-3 mb-4">
						<div class="w-8 h-8 bg-primary-600/20 rounded-lg flex items-center justify-center">
							<svg class="w-4 h-4 text-primary-400" viewBox="0 0 24 24" fill="currentColor">
								<path d="M13 3c-4.97 0-9 4.03-9 9H1l3.89 3.89.07.14L9 12H6c0-3.87 3.13-7 7-7s7 3.13 7 7-3.13 7-7 7c-1.93 0-3.68-.79-4.94-2.06l-1.42 1.42C8.27 19.99 10.51 21 13 21c4.97 0 9-4.03 9-9s-4.03-9-9-9zm-1 5v5l4.28 2.54.72-1.21-3.5-2.08V8H12z" />
							</svg>
						</div>
						<h3 class="text-sm font-semibold text-text-primary">Most Active Paths</h3>
					</div>
					{#if mostActivePaths.length === 0}
						<p class="text-xs text-text-muted">No path activity recorded</p>
					{:else}
						<ul class="space-y-2">
							{#each mostActivePaths as wp}
								<li class="text-xs">
									<div class="flex items-center justify-between gap-2">
										<span class="font-mono text-text-secondary truncate" title={wp.path}>
											{wp.path.split('/').slice(-2).join('/')}
										</span>
										<span class="text-text-muted whitespace-nowrap">
											{formatRelativeTime(wp.last_accessed_at!)}
										</span>
									</div>
									<span class="text-text-muted">{getProjectName(wp.workspace_id)}</span>
								</li>
							{/each}
						</ul>
					{/if}
				</div>

				<!-- Host Distribution Card -->
				<div class="card p-6">
					<div class="flex items-center gap-3 mb-4">
						<div class="w-8 h-8 bg-green-600/20 rounded-lg flex items-center justify-center">
							<svg class="w-4 h-4 text-green-400" viewBox="0 0 24 24" fill="currentColor">
								<path d="M20 18c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2H4c-1.1 0-2 .9-2 2v10c0 1.1.9 2 2 2H0v2h24v-2h-4zM4 6h16v10H4V6z" />
							</svg>
						</div>
						<h3 class="text-sm font-semibold text-text-primary">Host Distribution</h3>
					</div>
					{#if hostDistribution.length === 0}
						<p class="text-xs text-text-muted">No host data available</p>
					{:else}
						<ul class="space-y-2">
							{#each hostDistribution as [host, count]}
								<li class="flex items-center justify-between gap-2">
									<span class="text-xs text-text-secondary truncate" title={host}>{host}</span>
									<div class="flex items-center gap-2">
										<div class="h-2 bg-primary-600/30 rounded-full" style="width: {Math.max(count * 20, 8)}px"></div>
										<span class="text-xs font-mono text-text-muted">{count}</span>
									</div>
								</li>
							{/each}
						</ul>
					{/if}
				</div>
			</div>
		</section>
	{/if}

	{#if insightsLoading && $projects.length > 0}
		<div class="mt-8 text-center">
			<p class="text-sm text-text-muted">Loading project insights...</p>
		</div>
	{/if}
</div>

<!-- Confirmation Dialog -->
<ConfirmDialog
	open={deleteDialogOpen}
	title={projectsToDelete.length === 1
		? 'Delete project?'
		: `Delete ${projectsToDelete.length} projects?`}
	message="All issues, labels, and data within {projectsToDelete.length === 1
		? 'this project'
		: 'these projects'} will be permanently deleted."
	items={projectsToDelete.map((w) => w.name)}
	confirmLabel={projectsToDelete.length === 1 ? 'Delete Project' : 'Delete Projects'}
	loading={deleting}
	onconfirm={confirmDelete}
	oncancel={cancelDelete}
/>

<!-- Merge Dialog -->
{#if mergeDialogOpen}
	{@const targetOptions = mergeTargetOptions}
	<dialog
		class="dialog-modal"
		open
		onclick={(e) => { if (e.target === e.currentTarget && !merging) cancelMerge(); }}
		onkeydown={(e) => { if (e.key === 'Escape' && !merging) { e.preventDefault(); cancelMerge(); } }}
	>
		<div class="dialog-content animate-dialog-in">
			{#if mergeResult}
				<!-- Success state -->
				<div class="flex items-start gap-4 mb-6">
					<div class="shrink-0 w-11 h-11 rounded-lg flex items-center justify-center bg-status-open/20">
						<svg class="w-5 h-5 text-status-open" viewBox="0 0 24 24" fill="currentColor">
							<path d="M9 16.2L4.8 12l-1.4 1.4L9 19 21 7l-1.4-1.4L9 16.2z" />
						</svg>
					</div>
					<div class="flex-1 min-w-0">
						<h2 class="text-lg font-semibold text-text-primary">Merge complete</h2>
						<p class="text-sm text-text-secondary mt-1">
							Moved {mergeResult.issues_moved} {mergeResult.issues_moved === 1 ? 'issue' : 'issues'} and {mergeResult.plans_moved} {mergeResult.plans_moved === 1 ? 'plan' : 'plans'} into <strong>{mergeResult.target_workspace.name}</strong>.
						</p>
					</div>
				</div>

				<div class="flex items-center justify-end">
					<button class="btn btn-primary" onclick={closeMergeAfterSuccess} type="button">
						Done
					</button>
				</div>
			{:else}
				<!-- Merge form -->
				<div class="flex items-start gap-4 mb-6">
					<div class="shrink-0 w-11 h-11 rounded-lg flex items-center justify-center bg-primary-600/20">
						<svg class="w-5 h-5 text-primary-400" viewBox="0 0 24 24" fill="currentColor">
							<path d="M17 20.41L18.41 19 15 15.59 13.59 17 17 20.41zM7.5 8H11v5.59L5.59 19 7 20.41l6-6V8h3.5L12 3.5 7.5 8z" />
						</svg>
					</div>
					<div class="flex-1 min-w-0">
						<h2 class="text-lg font-semibold text-text-primary">Merge projects</h2>
						<p class="text-sm text-text-secondary mt-1">
							Move all issues and plans from the selected projects into a target project. The source projects will be deleted.
						</p>
					</div>
				</div>

				<!-- Source projects list -->
				<div class="mb-4">
					<div class="text-xs font-medium text-text-muted uppercase tracking-wider mb-2">
						{projectsToMerge.length === 1 ? 'Project to merge' : `${projectsToMerge.length} projects to merge`}
					</div>
					<div class="bg-surface-900 border border-border-subtle rounded-md max-h-40 overflow-y-auto">
						{#each projectsToMerge as ws (ws.id)}
							<div class="px-3 py-2 text-sm font-mono text-text-primary border-b border-border-subtle last:border-b-0">
								{ws.name}
							</div>
						{/each}
					</div>
				</div>

				<!-- Target project select -->
				<div class="mb-6">
					<label class="block text-xs font-medium text-text-muted uppercase tracking-wider mb-2" for="merge-target">
						Merge into
					</label>
					<Select
						options={targetOptions}
						value={mergeTargetId}
						placeholder="Select target project..."
						onchange={(v) => { mergeTargetId = v; }}
					/>
				</div>

				<!-- Warning -->
				<div class="flex items-center gap-2 p-3 bg-priority-high/10 border border-priority-high/20 rounded-md mb-6">
					<svg class="w-4 h-4 text-priority-high shrink-0" viewBox="0 0 24 24" fill="currentColor">
						<path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z" />
					</svg>
					<span class="text-xs text-priority-high">Source projects will be permanently deleted after merge</span>
				</div>

				<!-- Actions -->
				<div class="flex items-center justify-end gap-3">
					<button class="btn btn-ghost" onclick={cancelMerge} disabled={merging} type="button">
						Cancel
					</button>
					<button
						class="btn btn-primary"
						onclick={confirmMerge}
						disabled={merging || !mergeTargetId}
						type="button"
					>
						{#if merging}
							<svg class="w-4 h-4 animate-spin" viewBox="0 0 24 24" fill="none">
								<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
								<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
							</svg>
							Merging...
						{:else}
							Merge {projectsToMerge.length === 1 ? 'project' : 'projects'}
						{/if}
					</button>
				</div>
			{/if}
		</div>
	</dialog>
{/if}

{#snippet projectContent(project: Project)}
	<div class="flex items-start justify-between mb-4">
		<div
			class="w-10 h-10 bg-primary-600/20 rounded-lg flex items-center justify-center group-hover:bg-primary-600/30 transition-colors"
		>
			<span class="font-mono font-bold text-primary-400 text-sm uppercase">
				{project.prefix}
			</span>
		</div>
		{#if !editMode}
			<svg
				class="w-5 h-5 text-text-muted group-hover:text-primary-400 transition-colors"
				viewBox="0 0 24 24"
				fill="currentColor"
			>
				<path d="M10 6L8.59 7.41 13.17 12l-4.58 4.59L10 18l6-6z" />
			</svg>
		{/if}
	</div>

	<div class="flex items-center gap-2 mb-1">
		<h3 class="text-lg font-semibold text-text-primary group-hover:text-white transition-colors">
			{project.name}
		</h3>
		{#if pathCounts[project.id] !== undefined && pathCounts[project.id] > 0}
			<span
				class="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-primary-600/20 text-primary-400"
			>
				{pathCounts[project.id]} {pathCounts[project.id] === 1 ? 'path' : 'paths'}
			</span>
		{/if}
	</div>

	{#if project.description}
		<p class="text-sm text-text-secondary line-clamp-2 mb-4">
			{project.description}
		</p>
	{:else}
		<p class="text-sm text-text-muted mb-4">No description</p>
	{/if}

	<!-- Expandable paths section (read-only) -->
	<div class="mt-2">
		<button
			type="button"
			class="flex items-center gap-1 text-xs text-text-muted hover:text-text-secondary transition-colors"
			onclick={(e) => togglePaths(project.id, e)}
		>
			<svg
				class="w-3 h-3 transition-transform {expandedPaths.has(project.id) ? 'rotate-90' : ''}"
				viewBox="0 0 24 24"
				fill="currentColor"
			>
				<path d="M10 6L8.59 7.41 13.17 12l-4.58 4.59L10 18l6-6z" />
			</svg>
			{expandedPaths.has(project.id) ? 'Hide paths' : 'Show paths'}
		</button>

		{#if expandedPaths.has(project.id)}
			<div class="mt-2 animate-fade-in">
				{#if pathsLoading.has(project.id)}
					<div class="flex items-center gap-2 text-xs text-text-muted py-2">
						<svg class="w-3 h-3 animate-spin" viewBox="0 0 24 24" fill="none">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
						</svg>
						Loading paths...
					</div>
				{:else if (workspacePaths[project.id] ?? []).length === 0}
					<p class="text-xs text-text-muted py-2">No paths configured</p>
				{:else}
					<div class="border border-border-subtle rounded-md overflow-hidden">
						<table class="w-full text-xs">
							<thead>
								<tr class="bg-surface-800">
									<th class="text-left px-2 py-1.5 text-text-muted font-medium">Path</th>
									<th class="text-left px-2 py-1.5 text-text-muted font-medium">Label</th>
									<th class="text-left px-2 py-1.5 text-text-muted font-medium">Host</th>
									<th class="text-left px-2 py-1.5 text-text-muted font-medium">Last Accessed</th>
								</tr>
							</thead>
							<tbody>
								{#each workspacePaths[project.id] ?? [] as wp (wp.id)}
									<tr class="border-t border-border-subtle hover:bg-surface-800/50">
										<td class="px-2 py-1.5 font-mono text-text-primary truncate max-w-[200px]" title={wp.path}>{wp.path}</td>
										<td class="px-2 py-1.5 text-text-secondary">{wp.label ?? '-'}</td>
										<td class="px-2 py-1.5 text-text-secondary">{wp.hostname ?? '-'}</td>
										<td class="px-2 py-1.5 text-text-secondary">{formatDate(wp.last_accessed_at)}</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{/if}
			</div>
		{/if}
	</div>
{/snippet}
