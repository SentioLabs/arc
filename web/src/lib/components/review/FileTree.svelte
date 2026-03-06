<script lang="ts">
	import type { DiffFile } from '$lib/review/parser';
	import { getFileName } from '$lib/review/parser';

	interface Props {
		files: DiffFile[];
		viewedFiles: Set<string>;
		activeFile: string | null;
		onFileClick: (filename: string) => void;
		onToggleViewed: (filename: string) => void;
	}

	let { files, viewedFiles, activeFile, onFileClick, onToggleViewed }: Props = $props();

	const totalAdditions = $derived(files.reduce((sum, f) => sum + f.addedLines, 0));
	const totalDeletions = $derived(files.reduce((sum, f) => sum + f.deletedLines, 0));
</script>

<!-- Summary bar -->
<div class="px-3 py-2 text-xs text-text-muted border-b border-border flex items-center justify-between">
	<span>{files.length} files changed</span>
	<span class="font-mono flex items-center gap-2">
		{#if totalAdditions > 0}
			<span class="text-green-400">+{totalAdditions}</span>
		{/if}
		{#if totalDeletions > 0}
			<span class="text-red-400">-{totalDeletions}</span>
		{/if}
	</span>
</div>

<!-- Scrollable file list -->
<div class="overflow-y-auto overflow-x-auto flex-1">
	{#each files as file (getFileName(file))}
		{@const name = getFileName(file)}
		{@const isActive = activeFile === name}
		{@const isViewed = viewedFiles.has(name)}
		<button
			type="button"
			class="w-full flex items-center gap-2 px-3 py-1.5 text-left transition-colors hover:bg-surface-700 {isActive
				? 'bg-surface-700'
				: ''}"
			title={name}
			onclick={() => onFileClick(name)}
		>
			<!-- Viewed checkbox -->
			<input
				type="checkbox"
				class="checkbox shrink-0"
				checked={isViewed}
				onclick={(e: MouseEvent) => {
					e.stopPropagation();
					onToggleViewed(name);
				}}
			/>

			<!-- Status indicator -->
			{#if file.isNew}
				<span class="w-2 h-2 rounded-full bg-green-500 shrink-0"></span>
			{:else if file.isDeleted}
				<span class="w-2 h-2 rounded-full bg-red-500 shrink-0"></span>
			{:else}
				<span class="w-2 h-2 rounded-full bg-yellow-500 shrink-0"></span>
			{/if}

			<!-- Filename -->
			<span class="font-mono text-sm text-text-primary whitespace-nowrap flex-1">{name}</span>

			<!-- Line stats -->
			<span class="flex items-center gap-1 text-xs font-mono shrink-0">
				{#if file.addedLines > 0}
					<span class="text-green-400">+{file.addedLines}</span>
				{/if}
				{#if file.deletedLines > 0}
					<span class="text-red-400">-{file.deletedLines}</span>
				{/if}
			</span>
		</button>
	{/each}
</div>
