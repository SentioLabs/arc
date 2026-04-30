<script lang="ts">
	import { onMount } from 'svelte';
	import { listIssues } from '$lib/api';
	import type { Issue } from '$lib/api';
	import StatusBadge from './StatusBadge.svelte';
	import PriorityBadge from './PriorityBadge.svelte';
	import TypeBadge from './TypeBadge.svelte';

	interface Props {
		projectId: string;
		onSelect: (issueId: string) => void;
		onCancel: () => void;
	}

	let { projectId, onSelect, onCancel }: Props = $props();

	let search: string = $state('');
	let issues: Issue[] = $state([]);
	let loading: boolean = $state(false);
	let searchInputEl: HTMLInputElement | undefined = $state();
	let debounceTimer: ReturnType<typeof setTimeout> | undefined = $state();
	let dialogRef: HTMLDialogElement | undefined = $state();

	async function fetchIssues(query: string) {
		loading = true;
		try {
			const result = await listIssues(projectId, { q: query || undefined, limit: 20 });
			issues = result.data ?? [];
		} catch {
			issues = [];
		} finally {
			loading = false;
		}
	}

	function handleSearchInput() {
		if (debounceTimer) clearTimeout(debounceTimer);
		debounceTimer = setTimeout(() => {
			fetchIssues(search);
		}, 300);
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			e.preventDefault();
			onCancel();
		}
	}

	function handleBackdropClick(e: MouseEvent) {
		if (e.target === dialogRef) {
			onCancel();
		}
	}

	onMount(() => {
		dialogRef?.showModal();
		fetchIssues('');
		// Use a small delay to ensure the dialog is rendered before focusing
		setTimeout(() => searchInputEl?.focus(), 0);

		return () => {
			if (debounceTimer) clearTimeout(debounceTimer);
			if (dialogRef?.open) dialogRef.close();
		};
	});
</script>

<dialog
	bind:this={dialogRef}
	class="dialog-modal"
	onkeydown={handleKeydown}
	onclick={handleBackdropClick}
>
	<div class="dialog-content animate-dialog-in" style="max-width: 40rem; width: 100%;">
		<!-- Header -->
		<div class="flex items-center justify-between mb-4">
			<h2 class="text-lg font-semibold text-text-primary">Select Issue</h2>
			<button
				class="text-text-muted hover:text-text-primary transition-colors"
				onclick={onCancel}
				type="button"
				aria-label="Close"
			>
				<svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
					<path
						d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"
					/>
				</svg>
			</button>
		</div>

		<!-- Search input -->
		<div class="mb-4">
			<input
				bind:this={searchInputEl}
				bind:value={search}
				oninput={handleSearchInput}
				type="text"
				placeholder="Search issues..."
				class="w-full px-3 py-2 bg-surface-900 border border-border-subtle rounded-md text-sm text-text-primary placeholder-text-muted focus:outline-none focus:border-accent-primary"
			/>
		</div>

		<!-- Results list -->
		<div class="max-h-80 overflow-y-auto border border-border-subtle rounded-md">
			{#if loading}
				<div class="flex items-center justify-center py-8 text-text-muted text-sm">
					<svg class="w-4 h-4 animate-spin mr-2" viewBox="0 0 24 24" fill="none">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"
						></circle>
						<path
							class="opacity-75"
							fill="currentColor"
							d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
						></path>
					</svg>
					Loading...
				</div>
			{:else if issues.length === 0}
				<div class="py-8 text-center text-text-muted text-sm">
					{search ? 'No issues found' : 'No issues available'}
				</div>
			{:else}
				{#each issues as issue (issue.id)}
					<button
						class="w-full flex items-center gap-3 px-3 py-2.5 text-left hover:bg-surface-700 transition-colors border-b border-border-subtle last:border-b-0"
						onclick={() => onSelect(issue.id)}
						type="button"
					>
						<span class="font-mono text-xs text-text-muted shrink-0">
							{issue.id.slice(0, 13)}
						</span>
						{#if issue.issue_type}
							<span class="shrink-0">
								<TypeBadge type={issue.issue_type} showLabel={false} />
							</span>
						{/if}
						<span class="text-sm text-text-primary truncate flex-1 min-w-0">
							{issue.title}
						</span>
						{#if issue.status}
							<span class="shrink-0">
								<StatusBadge status={issue.status} size="sm" />
							</span>
						{/if}
						<span class="shrink-0">
							<PriorityBadge priority={issue.priority} showLabel={false} />
						</span>
					</button>
				{/each}
			{/if}
		</div>
	</div>
</dialog>
