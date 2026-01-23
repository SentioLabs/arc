<script lang="ts">
	import { getContext } from 'svelte';
	import type { Writable } from 'svelte/store';
	import { deleteWorkspaces, type Workspace } from '$lib/api';
	import { ConfirmDialog } from '$lib/components';

	const workspaces = getContext<Writable<Workspace[]>>('workspaces');

	// Search state
	let searchQuery = $state('');
	let searchFocused = $state(false);

	// Filter workspaces based on search query
	const filteredWorkspaces = $derived(() => {
		if (!searchQuery.trim()) return $workspaces;
		const q = searchQuery.toLowerCase().trim();
		return $workspaces.filter(
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
	let workspacesToDelete = $state<Workspace[]>([]);

	// Derived state
	const selectedCount = $derived(selectedIds.size);
	const allSelected = $derived($workspaces.length > 0 && selectedIds.size === $workspaces.length);
	const someSelected = $derived(selectedIds.size > 0 && selectedIds.size < $workspaces.length);

	function toggleEditMode() {
		editMode = !editMode;
		if (editMode) {
			// Clear search when entering edit mode to show all workspaces
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
			selectedIds = new Set($workspaces.map((w) => w.id));
		}
	}

	function handleDeleteSelected() {
		workspacesToDelete = $workspaces.filter((w) => selectedIds.has(w.id));
		deleteDialogOpen = true;
	}

	function handleDeleteSingle(workspace: Workspace) {
		workspacesToDelete = [workspace];
		deleteDialogOpen = true;
	}

	async function confirmDelete() {
		if (workspacesToDelete.length === 0) return;

		deleting = true;
		try {
			const idsToDelete = workspacesToDelete.map((w) => w.id);
			await deleteWorkspaces(idsToDelete);

			// Update the store
			workspaces.update((current) => current.filter((w) => !idsToDelete.includes(w.id)));

			// Clear selection
			selectedIds = new Set();
			workspacesToDelete = [];
			deleteDialogOpen = false;

			// Exit edit mode if no workspaces left
			if ($workspaces.length === 0) {
				editMode = false;
			}
		} catch (err) {
			console.error('Failed to delete workspaces:', err);
			// TODO: Show error toast
		} finally {
			deleting = false;
		}
	}

	function cancelDelete() {
		deleteDialogOpen = false;
		workspacesToDelete = [];
	}
</script>

<div class="p-8 max-w-4xl mx-auto animate-fade-in">
	<header class="mb-8">
		<div class="flex items-center justify-between gap-4 mb-3">
			<h1 class="text-4xl font-bold text-text-primary">Workspaces</h1>
			{#if $workspaces.length > 0}
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
		<p class="text-lg text-text-secondary">Select a workspace to view and manage issues</p>

		<!-- Search box (hidden in edit mode) -->
		{#if $workspaces.length > 0 && !editMode}
			<div class="mt-6 relative transition-all duration-200 {searchFocused ? 'max-w-md' : 'max-w-sm'}">
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
					placeholder="Search workspaces..."
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
	{#if editMode && $workspaces.length > 0}
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
				<button class="btn btn-danger btn-sm" onclick={handleDeleteSelected}>
					<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
						<path
							d="M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12zM8 9h8v10H8V9zm7.5-5l-1-1h-5l-1 1H5v2h14V4h-3.5z"
						/>
					</svg>
					Delete {selectedCount === 1 ? 'workspace' : `${selectedCount} workspaces`}
				</button>
			{/if}
		</div>
	{/if}

	{#if $workspaces.length === 0}
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
			<h2 class="text-xl font-semibold text-text-primary mb-2">No workspaces yet</h2>
			<p class="text-text-secondary mb-6">Create a workspace using the CLI to get started</p>
			<div
				class="bg-surface-800 rounded-lg p-4 font-mono text-sm text-text-secondary text-left inline-block"
			>
				<code>arc init my-project</code>
			</div>
		</div>
	{:else if filteredWorkspaces().length === 0}
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
			<h2 class="text-xl font-semibold text-text-primary mb-2">No matching workspaces</h2>
			<p class="text-text-secondary mb-4">No workspaces match "{searchQuery}"</p>
			<button type="button" class="btn btn-primary" onclick={clearSearch}>
				Clear search
			</button>
		</div>
	{:else}
		<div class="grid gap-4 sm:grid-cols-2">
			{#each filteredWorkspaces() as workspace (workspace.id)}
				{@const isSelected = selectedIds.has(workspace.id)}
				{#if editMode}
					<!-- Edit mode: clickable card for selection -->
					<button
						type="button"
						class="card p-6 transition-all duration-200 group relative text-left cursor-pointer {isSelected
							? 'border-primary-500 bg-primary-600/5'
							: 'hover:border-border-focus/50'}"
						onclick={() => toggleSelection(workspace.id)}
					>
						<!-- Selection checkbox -->
						<div class="absolute top-4 right-4 z-10">
							<input
								type="checkbox"
								class="checkbox"
								checked={isSelected}
								onclick={(e) => e.stopPropagation()}
								onchange={() => toggleSelection(workspace.id)}
							/>
						</div>

						<div class="pr-8">
							{@render workspaceContent(workspace)}
						</div>

						<!-- Delete button (on hover) -->
						<span
							class="absolute bottom-4 right-4 btn btn-ghost btn-icon btn-sm opacity-0 group-hover:opacity-100 transition-opacity text-text-muted hover:text-status-blocked hover:bg-status-blocked/10 hover:border-status-blocked/30"
							role="button"
							tabindex={0}
							title="Delete workspace"
							onclick={(e) => {
								e.stopPropagation();
								handleDeleteSingle(workspace);
							}}
							onkeydown={(e) => {
								if (e.key === 'Enter' || e.key === ' ') {
									e.preventDefault();
									e.stopPropagation();
									handleDeleteSingle(workspace);
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
					<!-- Normal mode: link to workspace -->
					<a
						href="/{workspace.id}"
						class="card p-6 transition-all duration-200 group hover:border-border-focus/50 block"
					>
						{@render workspaceContent(workspace)}
					</a>
				{/if}
			{/each}
		</div>
	{/if}
</div>

<!-- Confirmation Dialog -->
<ConfirmDialog
	open={deleteDialogOpen}
	title={workspacesToDelete.length === 1
		? 'Delete workspace?'
		: `Delete ${workspacesToDelete.length} workspaces?`}
	message="All issues, labels, and data within {workspacesToDelete.length === 1
		? 'this workspace'
		: 'these workspaces'} will be permanently deleted."
	items={workspacesToDelete.map((w) => w.name)}
	confirmLabel={workspacesToDelete.length === 1 ? 'Delete Workspace' : 'Delete Workspaces'}
	loading={deleting}
	onconfirm={confirmDelete}
	oncancel={cancelDelete}
/>

{#snippet workspaceContent(workspace: Workspace)}
	<div class="flex items-start justify-between mb-4">
		<div
			class="w-10 h-10 bg-primary-600/20 rounded-lg flex items-center justify-center group-hover:bg-primary-600/30 transition-colors"
		>
			<span class="font-mono font-bold text-primary-400 text-sm uppercase">
				{workspace.prefix}
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

	<h3 class="text-lg font-semibold text-text-primary group-hover:text-white transition-colors mb-1">
		{workspace.name}
	</h3>

	{#if workspace.description}
		<p class="text-sm text-text-secondary line-clamp-2 mb-4">
			{workspace.description}
		</p>
	{:else}
		<p class="text-sm text-text-muted mb-4">No description</p>
	{/if}

	{#if workspace.path}
		<div class="flex items-center gap-2 text-xs text-text-muted font-mono">
			<svg class="w-3 h-3" viewBox="0 0 24 24" fill="currentColor">
				<path
					d="M10 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2h-8l-2-2z"
				/>
			</svg>
			<span class="truncate">{workspace.path}</span>
		</div>
	{/if}
{/snippet}
