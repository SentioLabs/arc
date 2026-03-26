<script lang="ts">
	import { Header, ConfirmDialog, FilesystemBrowser } from '$lib/components';
	import RecentAISessions from '$lib/components/RecentAISessions.svelte';
	import { page } from '$app/stores';
	import { getContext } from 'svelte';
	import type { Writable } from 'svelte/store';
	import {
		getProjectStats,
		updateProject,
		listWorkspaces,
		createWorkspace,
		deleteWorkspace,
		mergeProjects,
		type Project,
		type Statistics,
		type Workspace,
		type MergeResult
	} from '$lib/api';

	// Get projects from context
	const projects = getContext<Writable<Project[]>>('projects');
	const projectId = $derived($page.params.projectId);
	const project = $derived($projects.find((p) => p.id === projectId));

	// Local state for stats
	let stats = $state<Statistics | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Workspaces state (directory paths)
	let paths = $state<Workspace[]>([]);
	let pathsLoading = $state(true);

	// Path management panel state
	let managingPaths = $state(false);
	let addPathOpen = $state(false);
	let addPathMode = $state<'browse' | 'manual'>('browse');
	let manualPath = $state('');
	let selectedPath = $state('');
	let newPathLabel = $state('');
	let addingPath = $state(false);
	let deletePathConfirm = $state<{ pathId: string; pathValue: string } | null>(null);
	let deletingPath = $state(false);

	// Rename state
	let renameEditing = $state(false);
	let renameValue = $state('');
	let renameSaving = $state(false);
	let renameError = $state('');
	// svelte-ignore non_reactive_update
	let renameInputEl: HTMLInputElement;

	function startRename() {
		renameValue = project?.name ?? '';
		renameError = '';
		renameEditing = true;
		requestAnimationFrame(() => renameInputEl?.focus());
	}

	function cancelRename() {
		renameEditing = false;
		renameError = '';
	}

	async function saveRename() {
		const trimmed = renameValue.trim();
		if (!trimmed || trimmed === project?.name) {
			cancelRename();
			return;
		}
		renameSaving = true;
		renameError = '';
		try {
			const updated = await updateProject(projectId!, { name: trimmed });
			projects.update((list) =>
				list.map((p) => (p.id === updated.id ? updated : p))
			);
			renameEditing = false;
		} catch (err) {
			renameError = err instanceof Error ? err.message : 'Failed to rename project';
		} finally {
			renameSaving = false;
		}
	}

	function handleRenameKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter') { e.preventDefault(); saveRename(); }
		if (e.key === 'Escape') { e.preventDefault(); cancelRename(); }
	}

	function handleRenameBlur(e: FocusEvent) {
		// Don't cancel if focus moves to save/cancel buttons
		const related = e.relatedTarget as HTMLElement | null;
		if (related?.closest('.rename-actions')) return;
		cancelRename();
	}

	// Merge state
	let mergeDialogOpen = $state(false);
	let merging = $state(false);
	let mergeResult: MergeResult | null = $state(null);

	// Header actions menu — only merge now (manage paths moved inline)
	const headerActions = [
		{
			id: 'merge',
			label: 'Merge Into This Project',
			icon: '<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor"><path d="M17 20.41L18.41 19 15 15.59 13.59 17 17 20.41zM7.5 8H11v5.59L5.59 19 7 20.41l6-6V8h3.5L12 3.5 7.5 8z"/></svg>'
		}
	];

	function handleHeaderAction(actionId: string) {
		if (actionId === 'merge') {
			mergeResult = null;
			mergeDialogOpen = true;
		}
	}

	// Merge source options: all projects except current
	const mergeSourceOptions = $derived.by(() => {
		return $projects
			.filter((p) => p.id !== projectId)
			.map((p) => ({ value: p.id, label: p.name }));
	});

	// Load stats when project changes
	$effect(() => {
		if (projectId) {
			loadStats();
			loadPaths();
		}
	});

	async function loadStats() {
		if (!projectId) return;
		loading = true;
		error = null;
		try {
			stats = await getProjectStats(projectId);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load stats';
		} finally {
			loading = false;
		}
	}

	async function loadPaths() {
		if (!projectId) return;
		pathsLoading = true;
		try {
			paths = await listWorkspaces(projectId);
		} catch (err) {
			console.error('Failed to load paths:', err);
			paths = [];
		} finally {
			pathsLoading = false;
		}
	}

	// Group paths: canonical paths first, then symlinks indented below their canonical counterpart
	const groupedPaths = $derived.by(() => {
		const canonicalPaths = paths.filter((p) => p.path_type !== 'symlink');
		const symlinkPaths = paths.filter((p) => p.path_type === 'symlink');

		type GroupedEntry = { id: string; path: string; label: string; isSymlink: boolean };
		const entries: GroupedEntry[] = [];

		for (const cp of canonicalPaths) {
			entries.push({ id: cp.id, path: cp.path, label: cp.label ?? '-', isSymlink: false });
			// Find symlinks in the same workspace (all paths are for the same workspace)
			for (const sp of symlinkPaths) {
				entries.push({
					id: sp.id,
					path: sp.path,
					label: '(symlink)',
					isSymlink: true
				});
			}
			// Remove matched symlinks so they don't appear under other canonical paths
			// Since all symlinks are for the same workspace, we only show them once
			// under the first canonical path
			symlinkPaths.length = 0;
		}

		// Any remaining symlinks without a canonical counterpart
		for (const sp of symlinkPaths) {
			entries.push({ id: sp.id, path: sp.path, label: '(symlink)', isSymlink: true });
		}

		return entries;
	});

	// Compute column widths for terminal-style alignment
	const pathColumns = $derived.by(() => {
		const allDisplayPaths = groupedPaths.map((e) => (e.isSymlink ? `  \u2192 ${e.path}` : e.path));
		const pathW = Math.max(4, ...allDisplayPaths.map((p) => p.length));
		const labelW = Math.max(5, ...groupedPaths.map((e) => e.label.length));
		return { pathW, labelW };
	});

	function padRight(str: string, len: number): string {
		return str + ' '.repeat(Math.max(0, len - str.length));
	}

	function makeSeparator(len: number): string {
		return '─'.repeat(len);
	}

	// Path management functions
	function getAddPathValue(): string {
		return addPathMode === 'browse' ? selectedPath : manualPath;
	}

	function handleBrowseSelect(path: string) {
		selectedPath = path;
	}

	function resetAddPathForm() {
		addPathOpen = false;
		manualPath = '';
		selectedPath = '';
		newPathLabel = '';
		addPathMode = 'browse';
	}

	async function submitAddPath() {
		const pathValue = getAddPathValue();
		if (!pathValue.trim() || !projectId) return;
		addingPath = true;
		try {
			const created = await createWorkspace(projectId, {
				path: pathValue.trim(),
				label: newPathLabel.trim() || undefined
			});
			paths = [...paths, created];
			resetAddPathForm();
		} catch (err) {
			console.error('Failed to add path:', err);
		} finally {
			addingPath = false;
		}
	}

	function confirmDeletePathAction(pathId: string, pathValue: string) {
		deletePathConfirm = { pathId, pathValue };
	}

	async function executeDeletePath() {
		if (!deletePathConfirm || !projectId) return;
		deletingPath = true;
		const { pathId } = deletePathConfirm;
		try {
			await deleteWorkspace(projectId, pathId);
			paths = paths.filter((p) => p.id !== pathId);
			deletePathConfirm = null;
		} catch (err) {
			console.error('Failed to delete path:', err);
		} finally {
			deletingPath = false;
		}
	}

	function cancelDeletePath() {
		deletePathConfirm = null;
	}

	// Merge functions
	let selectedMergeSources = $state<string[]>([]);

	function toggleMergeSource(id: string) {
		if (selectedMergeSources.includes(id)) {
			selectedMergeSources = selectedMergeSources.filter((s) => s !== id);
		} else {
			selectedMergeSources = [...selectedMergeSources, id];
		}
	}

	async function confirmMerge() {
		if (selectedMergeSources.length === 0 || !projectId) return;
		merging = true;
		try {
			const result = await mergeProjects(projectId, selectedMergeSources);
			mergeResult = result;

			projects.update((current) =>
				current
					.filter((w) => !result.sources_deleted.includes(w.id))
					.map((w) => (w.id === result.target_project.id ? result.target_project : w))
			);

			loadStats();
			loadPaths();
		} catch (err) {
			console.error('Failed to merge projects:', err);
		} finally {
			merging = false;
		}
	}

	function cancelMerge() {
		mergeDialogOpen = false;
		selectedMergeSources = [];
		mergeResult = null;
	}

	function closeMergeAfterSuccess() {
		mergeDialogOpen = false;
		selectedMergeSources = [];
		mergeResult = null;
	}

	// Dashboard stat cards - derived from stats
	const statCards = $derived(
		stats
			? [
					{ label: 'Total Issues', value: stats.total_issues, color: 'primary' },
					{ label: 'Open', value: stats.open_issues, color: 'status-open' },
					{ label: 'In Progress', value: stats.in_progress_issues, color: 'status-in-progress' },
					{ label: 'Ready', value: stats.ready_issues, color: 'primary' },
					{ label: 'Blocked', value: stats.blocked_issues, color: 'status-blocked' },
					{ label: 'Closed', value: stats.closed_issues, color: 'status-closed' }
				]
			: []
	);
</script>

{#if project}
	<Header project={project} actions={headerActions} onaction={handleHeaderAction} />

	<div class="flex-1 p-8 animate-fade-in">
		<header class="mb-8">
			{@render projectNameEditor(project)}
			{#if project.description}
				<p class="text-text-secondary">{project.description}</p>
			{/if}
		</header>

		{#if loading}
			<div class="flex items-center justify-center py-12">
				<div class="text-text-muted animate-pulse">Loading stats...</div>
			</div>
		{:else if error}
			<div class="card p-8 text-center">
				<p class="text-status-blocked mb-4">{error}</p>
				<button class="btn btn-primary" onclick={loadStats}>Retry</button>
			</div>
		{:else if stats}
			<!-- Stats Grid -->
			<div class="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4 mb-8">
				{#each statCards as stat (stat.label)}
					<div class="card p-4">
						<div class="text-xs font-medium text-text-muted uppercase tracking-wider mb-2">
							{stat.label}
						</div>
						<div class="text-2xl font-bold text-{stat.color}-500">
							{stat.value}
						</div>
					</div>
				{/each}
			</div>

			<!-- Recent AI Sessions -->
			<section class="mb-8">
				<RecentAISessions projectId={project.id} />
			</section>

			<!-- Paths Section — terminal-style display -->
			<section class="mb-8">
				<div class="flex items-center justify-between mb-3">
					<div class="flex items-center gap-3">
						<svg class="w-5 h-5 text-text-muted" viewBox="0 0 24 24" fill="currentColor">
							<path d="M10 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2h-8l-2-2z" />
						</svg>
						<h2 class="text-lg font-semibold text-text-primary">Paths</h2>
						<span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-surface-700 text-text-muted">
							{paths.length}
						</span>
					</div>
				</div>

				{#if pathsLoading}
					<div class="flex items-center gap-2 text-sm text-text-muted py-4">
						<svg class="w-4 h-4 animate-spin" viewBox="0 0 24 24" fill="none">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
						</svg>
						Loading paths...
					</div>
				{:else if paths.length === 0}
					<div class="bg-surface-900 border border-border-subtle rounded-lg p-6 text-center">
						<p class="text-sm text-text-muted font-mono">
							No paths registered
						</p>
						<button
							type="button"
							class="mt-3 text-sm text-primary-400 hover:text-primary-300 transition-colors"
							onclick={() => { managingPaths = true; addPathOpen = true; }}
						>
							+ Add a path
						</button>
					</div>
				{:else}
					<!-- Terminal-style table matching `arc paths` output -->
					<div class="bg-surface-900 border border-border-subtle rounded-lg overflow-x-auto">
						<pre class="font-mono text-xs leading-relaxed p-4 text-text-primary whitespace-pre"><span class="text-text-muted">{padRight('PATH', pathColumns.pathW)}  LABEL</span>
<span class="text-text-muted/50">{makeSeparator(pathColumns.pathW)}  {makeSeparator(pathColumns.labelW)}</span>
{#each groupedPaths as entry, i}{#if i > 0}{'\n'}{/if}{#if entry.isSymlink}<span class="text-text-muted">  {padRight('\u2192 ' + entry.path, pathColumns.pathW - 2)}  {entry.label}</span>{:else}{padRight(entry.path, pathColumns.pathW)}  <span class="text-text-secondary">{entry.label}</span>{/if}{/each}</pre>
					</div>
				{/if}

				<!-- Expandable manage paths section -->
				<div class="mt-3">
					<button
						type="button"
						class="flex items-center gap-2 text-sm text-text-muted hover:text-text-secondary transition-colors group"
						onclick={() => { managingPaths = !managingPaths; if (!managingPaths) addPathOpen = false; }}
					>
						<svg
							class="w-4 h-4 transition-transform {managingPaths ? 'rotate-90' : ''}"
							viewBox="0 0 24 24"
							fill="currentColor"
						>
							<path d="M10 6L8.59 7.41 13.17 12l-4.58 4.59L10 18l6-6z" />
						</svg>
						<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
							<path d="M3 17.25V21h3.75L17.81 9.94l-3.75-3.75L3 17.25zM20.71 7.04c.39-.39.39-1.02 0-1.41l-2.34-2.34c-.39-.39-1.02-.39-1.41 0l-1.83 1.83 3.75 3.75 1.83-1.83z" />
						</svg>
						Manage paths
					</button>

					{#if managingPaths}
						<div class="mt-3 animate-fade-in">
							<div class="card p-5">
								<!-- Editable path list with delete buttons — terminal-style -->
								{#if paths.length > 0}
									<div class="bg-surface-900 border border-border-subtle rounded-md overflow-x-auto mb-4">
										<div class="flex">
											<pre class="font-mono text-xs leading-relaxed p-4 text-text-primary whitespace-pre flex-1"><span class="text-text-muted">{padRight('PATH', pathColumns.pathW)}  LABEL</span>
<span class="text-text-muted/50">{makeSeparator(pathColumns.pathW)}  {makeSeparator(pathColumns.labelW)}</span>
{#each groupedPaths as entry, i}{#if i > 0}{'\n'}{/if}{#if entry.isSymlink}<span class="text-text-muted">  {padRight('\u2192 ' + entry.path, pathColumns.pathW - 2)}  {entry.label}</span>{:else}{padRight(entry.path, pathColumns.pathW)}  <span class="text-text-secondary">{entry.label}</span>{/if}{/each}</pre>
											<div class="flex flex-col pt-4 pr-3" style="padding-top: calc(1rem + 2lh);">
												{#each groupedPaths as entry (entry.id)}
													<div class="flex items-center justify-center" style="height: 1lh;">
														<button
															type="button"
															class="text-text-muted hover:text-status-blocked transition-colors"
															title="Delete {entry.path}"
															onclick={() => confirmDeletePathAction(entry.id, entry.path)}
														>
															<svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="currentColor">
																<path d="M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12zM8 9h8v10H8V9zm7.5-5l-1-1h-5l-1 1H5v2h14V4h-3.5z" />
															</svg>
														</button>
													</div>
												{/each}
											</div>
										</div>
									</div>
								{/if}

								<!-- Add path toggle -->
								{#if !addPathOpen}
									<button
										type="button"
										class="flex items-center gap-2 text-sm text-primary-400 hover:text-primary-300 transition-colors"
										onclick={() => (addPathOpen = true)}
									>
										<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
											<path d="M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6v2z" />
										</svg>
										Add Path
									</button>
								{:else}
									<!-- Add path form -->
									<div class="bg-surface-800 border border-border-subtle rounded-lg p-4 animate-fade-in">
										<h4 class="text-xs font-medium text-text-muted uppercase tracking-wider mb-3">Add a directory path</h4>

										<!-- Mode toggle -->
										<div class="flex gap-1 mb-4 p-1 bg-surface-900 rounded-lg w-fit">
											<button
												type="button"
												class="px-3 py-1.5 text-xs font-medium rounded-md transition-colors {addPathMode === 'browse' ? 'bg-surface-600 text-text-primary' : 'text-text-muted hover:text-text-secondary'}"
												onclick={() => (addPathMode = 'browse')}
											>
												Browse
											</button>
											<button
												type="button"
												class="px-3 py-1.5 text-xs font-medium rounded-md transition-colors {addPathMode === 'manual' ? 'bg-surface-600 text-text-primary' : 'text-text-muted hover:text-text-secondary'}"
												onclick={() => (addPathMode = 'manual')}
											>
												Manual
											</button>
										</div>

										{#if addPathMode === 'browse'}
											<FilesystemBrowser onSelect={handleBrowseSelect} />
											{#if selectedPath}
												<div class="mt-3 flex items-center gap-2 p-2 bg-surface-900 rounded-lg">
													<svg class="w-4 h-4 text-primary-400 flex-shrink-0" viewBox="0 0 24 24" fill="currentColor">
														<path d="M9 16.2L4.8 12l-1.4 1.4L9 19 21 7l-1.4-1.4L9 16.2z" />
													</svg>
													<span class="text-sm font-mono text-text-primary truncate">{selectedPath}</span>
												</div>
											{/if}
										{:else}
											<input
												type="text"
												bind:value={manualPath}
												placeholder="Enter absolute directory path (e.g. /home/user/projects/my-app)"
												class="input w-full text-sm font-mono"
											/>
										{/if}

										<!-- Label input -->
										<div class="mt-3">
											<input
												type="text"
												bind:value={newPathLabel}
												placeholder="Label (optional)"
												class="input w-full text-sm"
											/>
										</div>

										<!-- Actions -->
										<div class="mt-4 flex items-center justify-end gap-2">
											<button
												type="button"
												class="btn btn-ghost btn-sm text-xs"
												onclick={resetAddPathForm}
												disabled={addingPath}
											>
												Cancel
											</button>
											<button
												type="button"
												class="btn btn-primary btn-sm text-xs"
												onclick={submitAddPath}
												disabled={addingPath || !getAddPathValue().trim()}
											>
												{#if addingPath}
													<svg class="w-3 h-3 animate-spin" viewBox="0 0 24 24" fill="none">
														<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
														<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
													</svg>
													Adding...
												{:else}
													Add Path
												{/if}
											</button>
										</div>
									</div>
								{/if}
							</div>
						</div>
					{/if}
				</div>
			</section>

			<!-- Quick Actions -->
			<div class="grid md:grid-cols-3 gap-4 mb-8">
				<a
					href="/{project.id}/issues"
					class="card p-6 hover:border-border-focus/50 transition-all group"
				>
					<div class="flex items-center gap-4">
						<div
							class="w-12 h-12 bg-surface-700 rounded-xl flex items-center justify-center group-hover:bg-primary-600/20 transition-colors"
						>
							<svg
								class="w-6 h-6 text-text-secondary group-hover:text-primary-400 transition-colors"
								viewBox="0 0 24 24"
								fill="currentColor"
							>
								<path d="M4 6h16v2H4V6zm0 5h16v2H4v-2zm0 5h16v2H4v-2z" />
							</svg>
						</div>
						<div>
							<h3 class="font-medium text-text-primary group-hover:text-white transition-colors">
								All Issues
							</h3>
							<p class="text-sm text-text-muted">Browse and filter issues</p>
						</div>
					</div>
				</a>

				<a
					href="/{project.id}/ready"
					class="card p-6 hover:border-border-focus/50 transition-all group"
				>
					<div class="flex items-center gap-4">
						<div
							class="w-12 h-12 bg-surface-700 rounded-xl flex items-center justify-center group-hover:bg-status-open/20 transition-colors"
						>
							<svg
								class="w-6 h-6 text-text-secondary group-hover:text-status-open transition-colors"
								viewBox="0 0 24 24"
								fill="currentColor"
							>
								<path d="M9 16.2L4.8 12l-1.4 1.4L9 19 21 7l-1.4-1.4L9 16.2z" />
							</svg>
						</div>
						<div>
							<h3 class="font-medium text-text-primary group-hover:text-white transition-colors">
								Ready Work
							</h3>
							<p class="text-sm text-text-muted">{stats.ready_issues} issues ready to start</p>
						</div>
					</div>
				</a>

				<a
					href="/{project.id}/blocked"
					class="card p-6 hover:border-border-focus/50 transition-all group"
				>
					<div class="flex items-center gap-4">
						<div
							class="w-12 h-12 bg-surface-700 rounded-xl flex items-center justify-center group-hover:bg-status-blocked/20 transition-colors"
						>
							<svg
								class="w-6 h-6 text-text-secondary group-hover:text-status-blocked transition-colors"
								viewBox="0 0 24 24"
								fill="currentColor"
							>
								<path
									d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zM4 12c0-4.42 3.58-8 8-8 1.85 0 3.55.63 4.9 1.69L5.69 16.9C4.63 15.55 4 13.85 4 12zm8 8c-1.85 0-3.55-.63-4.9-1.69L18.31 7.1C19.37 8.45 20 10.15 20 12c0 4.42-3.58 8-8 8z"
								/>
							</svg>
						</div>
						<div>
							<h3 class="font-medium text-text-primary group-hover:text-white transition-colors">
								Blocked
							</h3>
							<p class="text-sm text-text-muted">{stats.blocked_issues} issues blocked</p>
						</div>
					</div>
				</a>
			</div>

			<!-- Lead Time -->
			{#if stats.avg_lead_time_hours}
				<div class="card p-6">
					<h3 class="text-sm font-medium text-text-muted uppercase tracking-wider mb-3">
						Average Lead Time
					</h3>
					<div class="flex items-baseline gap-2">
						<span class="text-3xl font-bold text-text-primary">
							{Math.round(stats.avg_lead_time_hours)}
						</span>
						<span class="text-text-secondary">hours</span>
					</div>
					<p class="text-sm text-text-muted mt-2">From issue creation to close</p>
				</div>
			{/if}
		{/if}
	</div>
{/if}

<!-- Delete Path Confirmation -->
<ConfirmDialog
	open={deletePathConfirm !== null}
	title="Delete path?"
	message="Remove this path from the project: {deletePathConfirm?.pathValue ?? ''}"
	confirmLabel="Delete Path"
	loading={deletingPath}
	onconfirm={executeDeletePath}
	oncancel={cancelDeletePath}
/>

<!-- Merge Dialog -->
{#if mergeDialogOpen}
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
							Moved {mergeResult.issues_moved} {mergeResult.issues_moved === 1 ? 'issue' : 'issues'} and {mergeResult.plans_moved} {mergeResult.plans_moved === 1 ? 'plan' : 'plans'} into <strong>{project?.name}</strong>.
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
						<h2 class="text-lg font-semibold text-text-primary">Merge into {project?.name}</h2>
						<p class="text-sm text-text-secondary mt-1">
							Select projects to merge into this one. All issues and plans will be moved here. The source projects will be deleted.
						</p>
					</div>
				</div>

				<!-- Source project selection -->
				<div class="mb-6">
					<div class="text-xs font-medium text-text-muted uppercase tracking-wider mb-2">
						Select projects to merge
					</div>
					<div class="bg-surface-900 border border-border-subtle rounded-md max-h-60 overflow-y-auto">
						{#each mergeSourceOptions as option (option.value)}
							{@const isSelected = selectedMergeSources.includes(option.value)}
							<button
								type="button"
								class="w-full flex items-center gap-3 px-3 py-2.5 text-sm text-left border-b border-border-subtle last:border-b-0 transition-colors {isSelected ? 'bg-primary-600/10 text-text-primary' : 'text-text-secondary hover:bg-surface-800'}"
								onclick={() => toggleMergeSource(option.value)}
							>
								<input
									type="checkbox"
									class="checkbox"
									checked={isSelected}
									onclick={(e) => e.stopPropagation()}
									onchange={() => toggleMergeSource(option.value)}
								/>
								<span class="font-mono">{option.label}</span>
							</button>
						{/each}
					</div>
					{#if selectedMergeSources.length > 0}
						<p class="text-xs text-text-muted mt-2">{selectedMergeSources.length} project{selectedMergeSources.length === 1 ? '' : 's'} selected</p>
					{/if}
				</div>

				<!-- Warning -->
				{#if selectedMergeSources.length > 0}
					<div class="flex items-center gap-2 p-3 bg-priority-high/10 border border-priority-high/20 rounded-md mb-6">
						<svg class="w-4 h-4 text-priority-high shrink-0" viewBox="0 0 24 24" fill="currentColor">
							<path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z" />
						</svg>
						<span class="text-xs text-priority-high">Selected projects will be permanently deleted after merge</span>
					</div>
				{/if}

				<!-- Actions -->
				<div class="flex items-center justify-end gap-3">
					<button class="btn btn-ghost" onclick={cancelMerge} disabled={merging} type="button">
						Cancel
					</button>
					<button
						class="btn btn-primary"
						onclick={confirmMerge}
						disabled={merging || selectedMergeSources.length === 0}
						type="button"
					>
						{#if merging}
							<svg class="w-4 h-4 animate-spin" viewBox="0 0 24 24" fill="none">
								<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
								<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
							</svg>
							Merging...
						{:else}
							Merge {selectedMergeSources.length} project{selectedMergeSources.length === 1 ? '' : 's'}
						{/if}
					</button>
				</div>
			{/if}
		</div>
	</dialog>
{/if}

{#snippet projectNameEditor(proj: Project)}
	<div class="flex items-center gap-2 mb-2 group/rename">
		{#if renameEditing}
			<div class="flex-1">
				<div class="flex items-center gap-2">
					<input
						bind:this={renameInputEl}
						type="text"
						bind:value={renameValue}
						onblur={handleRenameBlur}
						onkeydown={handleRenameKeydown}
						disabled={renameSaving}
						class="input text-2xl font-bold text-text-primary w-full py-1.5"
						placeholder="Project name"
					/>
					<div class="rename-actions flex items-center gap-1.5 shrink-0">
						<button
							type="button"
							class="btn btn-primary btn-sm"
							disabled={renameSaving || !renameValue.trim() || renameValue.trim() === proj.name}
							onclick={saveRename}
						>
							{#if renameSaving}
								<svg class="w-3.5 h-3.5 animate-spin" viewBox="0 0 24 24" fill="none">
									<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
									<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
								</svg>
							{:else}
								Save
							{/if}
						</button>
						<button
							type="button"
							class="btn btn-ghost btn-sm"
							disabled={renameSaving}
							onclick={cancelRename}
						>
							Cancel
						</button>
					</div>
				</div>
				{#if renameError}
					<p class="text-xs text-status-blocked mt-1.5">{renameError}</p>
				{/if}
			</div>
		{:else}
			<h1
				class="text-3xl font-bold text-text-primary cursor-pointer hover:bg-surface-700/30 rounded px-1 -mx-1 transition-colors"
				ondblclick={startRename}
			>
				{proj.name}
			</h1>
			<button
				type="button"
				class="opacity-0 group-hover/rename:opacity-100 transition-opacity text-text-muted hover:text-primary-400 p-1 rounded hover:bg-surface-700/50"
				title="Rename project"
				onclick={startRename}
			>
				<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
					<path d="M3 17.25V21h3.75L17.81 9.94l-3.75-3.75L3 17.25zM20.71 7.04c.39-.39.39-1.02 0-1.41l-2.34-2.34c-.39-.39-1.02-.39-1.41 0l-1.83 1.83 3.75 3.75 1.83-1.83z" />
				</svg>
			</button>
		{/if}
	</div>
{/snippet}
