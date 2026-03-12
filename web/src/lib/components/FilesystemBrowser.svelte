<script lang="ts">
	import { browseFilesystem, type BrowseEntry } from '$lib/api';

	interface Props {
		onSelect: (path: string) => void;
	}

	let { onSelect }: Props = $props();

	let currentDir = $state('');
	let entries = $state<BrowseEntry[]>([]);
	let loading = $state(false);
	let error = $state<string | null>(null);

	async function browse() {
		if (!currentDir.trim()) return;
		loading = true;
		error = null;
		try {
			entries = await browseFilesystem(currentDir.trim());
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to browse directory';
			entries = [];
		} finally {
			loading = false;
		}
	}

	function navigateTo(path: string) {
		currentDir = path;
		browse();
	}

	function navigateUp() {
		const parts = currentDir.replace(/\/+$/, '').split('/');
		parts.pop();
		const parent = parts.join('/') || '/';
		navigateTo(parent);
	}

	function selectCurrent() {
		if (currentDir.trim()) {
			onSelect(currentDir.trim());
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			browse();
		}
	}
</script>

<div class="space-y-3">
	<!-- Path input and controls -->
	<div class="flex gap-2">
		<input
			type="text"
			bind:value={currentDir}
			onkeydown={handleKeydown}
			placeholder="Enter directory path (e.g. /home/user/projects)"
			class="input flex-1 text-sm font-mono"
		/>
		<button
			type="button"
			class="btn btn-primary btn-sm"
			onclick={browse}
			disabled={loading || !currentDir.trim()}
		>
			{#if loading}
				<svg class="w-4 h-4 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
					<path d="M12 2a10 10 0 0 1 10 10" stroke-linecap="round" />
				</svg>
			{:else}
				Browse
			{/if}
		</button>
	</div>

	<!-- Navigation and select controls -->
	{#if entries.length > 0 || error}
		<div class="flex gap-2">
			<button
				type="button"
				class="btn btn-ghost btn-sm"
				onclick={navigateUp}
				title="Go to parent directory"
			>
				<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
					<path d="M20 11H7.83l5.59-5.59L12 4l-8 8 8 8 1.41-1.41L7.83 13H20v-2z" />
				</svg>
				Up
			</button>
			<button
				type="button"
				class="btn btn-primary btn-sm ml-auto"
				onclick={selectCurrent}
				disabled={!currentDir.trim()}
			>
				Select this directory
			</button>
		</div>
	{/if}

	<!-- Error display -->
	{#if error}
		<div class="p-3 bg-status-blocked/10 border border-status-blocked/30 rounded-lg text-sm text-status-blocked">
			{error}
		</div>
	{/if}

	<!-- Directory listing -->
	{#if loading}
		<div class="p-4 text-center text-text-muted text-sm animate-pulse">
			Loading directory contents...
		</div>
	{:else if entries.length > 0}
		<div class="border border-border rounded-lg overflow-hidden max-h-64 overflow-y-auto">
			{#each entries as entry (entry.path)}
				{#if entry.is_dir}
					<button
						type="button"
						class="w-full flex items-center gap-3 px-3 py-2 text-sm text-left hover:bg-surface-700 transition-colors border-b border-border last:border-b-0"
						onclick={() => navigateTo(entry.path)}
					>
						<!-- Folder icon -->
						<svg class="w-4 h-4 flex-shrink-0 {entry.is_git_repo ? 'text-primary-400' : 'text-text-muted'}" viewBox="0 0 24 24" fill="currentColor">
							<path d="M10 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2h-8l-2-2z" />
						</svg>
						<span class="truncate font-mono text-text-primary">{entry.name}</span>
						{#if entry.is_git_repo}
							<span class="flex-shrink-0 text-xs px-1.5 py-0.5 rounded bg-primary-600/20 text-primary-400 font-medium">
								git
							</span>
						{/if}
						<!-- Chevron right -->
						<svg class="w-4 h-4 flex-shrink-0 text-text-muted ml-auto" viewBox="0 0 24 24" fill="currentColor">
							<path d="M10 6L8.59 7.41 13.17 12l-4.58 4.59L10 18l6-6z" />
						</svg>
					</button>
				{/if}
			{/each}
		</div>
	{/if}
</div>
