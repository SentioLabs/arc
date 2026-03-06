<script lang="ts">
	import type { DiffFile } from '$lib/review/parser';
	import { getFileName } from '$lib/review/parser';
	import { getLineContent } from '$lib/review/highlight';
	import DiffLine from './DiffLine.svelte';
	import LineCommentForm from './LineCommentForm.svelte';

	interface Props {
		file: DiffFile;
		highlightMap: Map<string, string>;
		comment: string;
		onCommentChange: (c: string) => void;
		collapsed: boolean;
		onToggleCollapse: () => void;
		lineComments: Array<{ line: number; comment: string }>;
		onAddLineComment: (line: number) => void;
		onSaveLineComment: (line: number, text: string) => void;
		onDeleteLineComment: (line: number) => void;
	}

	let {
		file,
		highlightMap,
		comment,
		onCommentChange,
		collapsed,
		onToggleCollapse,
		lineComments,
		onAddLineComment,
		onSaveLineComment,
		onDeleteLineComment
	}: Props = $props();

	let activeCommentLine = $state<number | null>(null);
	let editingCommentLine = $state<number | null>(null);

	let showCommentInput = $state(false);
	let commentDraft = $state('');

	const filename = $derived(getFileName(file));

	function escapeHtml(text: string): string {
		return text
			.replace(/&/g, '&amp;')
			.replace(/</g, '&lt;')
			.replace(/>/g, '&gt;')
			.replace(/"/g, '&quot;');
	}

	function getHighlightedHtml(lineContent: string): string {
		return highlightMap.get(lineContent) ?? escapeHtml(lineContent);
	}

	function handleAddComment() {
		commentDraft = comment;
		showCommentInput = true;
	}

	function handleSaveComment() {
		onCommentChange(commentDraft);
		showCommentInput = false;
	}

	function handleCancelComment() {
		commentDraft = '';
		showCommentInput = false;
	}
</script>

<div class="border border-border rounded-t overflow-hidden">
	<!-- File header -->
	<button
		type="button"
		class="sticky top-0 z-10 flex items-center gap-3 w-full px-3 py-2 bg-surface-700 border-b border-border text-left cursor-pointer hover:bg-surface-600 transition-colors"
		onclick={onToggleCollapse}
	>
		<!-- Chevron -->
		<svg
			class="w-4 h-4 text-text-muted transition-transform {collapsed ? '' : 'rotate-90'}"
			viewBox="0 0 24 24"
			fill="currentColor"
		>
			<path d="M10 6L16 12L10 18V6Z" />
		</svg>

		<!-- Filename -->
		<span class="font-mono text-sm text-text-primary truncate flex-1">
			{filename}
		</span>

		<!-- Stats -->
		<span class="flex items-center gap-2 text-xs font-mono shrink-0">
			{#if file.addedLines > 0}
				<span class="text-green-400">+{file.addedLines}</span>
			{/if}
			{#if file.deletedLines > 0}
				<span class="text-red-400">-{file.deletedLines}</span>
			{/if}
		</span>
	</button>

	{#if !collapsed}
		<!-- Diff table -->
		<div class="overflow-x-auto">
			<table class="w-full border-collapse">
				<tbody>
					{#each file.blocks as block}
						<!-- Hunk header -->
						<tr>
							<td
								colspan="3"
								class="px-3 py-1 text-xs font-mono text-text-muted bg-surface-800 border-b border-border-subtle"
							>
								{block.header}
							</td>
						</tr>
						<!-- Diff lines -->
						{#each block.lines as line}
							{@const lineNumber = line.newNumber ?? line.oldNumber}
							<DiffLine
								type={line.type}
								oldNumber={line.oldNumber}
								newNumber={line.newNumber}
								highlightedHtml={getHighlightedHtml(getLineContent(line))}
								onAddComment={() => {
									if (lineNumber != null) {
										activeCommentLine = lineNumber;
										editingCommentLine = null;
									}
								}}
							/>
							{#if lineNumber != null}
								{@const existingComment = lineComments.find((c) => c.line === lineNumber)}
								{#if existingComment && editingCommentLine !== lineNumber}
									<tr>
										<td
											colspan="3"
											class="px-3 py-2 bg-surface-800 border-t border-b border-border-subtle"
										>
											<div class="flex items-start gap-2">
												<p
													class="text-sm text-text-secondary flex-1 whitespace-pre-wrap"
												>
													{existingComment.comment}
												</p>
												<button
													type="button"
													class="btn btn-ghost btn-sm shrink-0"
													onclick={() => {
														editingCommentLine = lineNumber;
														activeCommentLine = null;
													}}
												>
													Edit
												</button>
												<button
													type="button"
													class="btn btn-ghost btn-sm shrink-0 text-red-400"
													onclick={() => onDeleteLineComment(lineNumber)}
												>
													Delete
												</button>
											</div>
										</td>
									</tr>
								{/if}
								{#if editingCommentLine === lineNumber}
									<LineCommentForm
										comment={existingComment?.comment ?? ''}
										onSave={(text) => {
											onSaveLineComment(lineNumber, text);
											editingCommentLine = null;
										}}
										onCancel={() => {
											editingCommentLine = null;
										}}
									/>
								{/if}
								{#if activeCommentLine === lineNumber && editingCommentLine !== lineNumber}
									<LineCommentForm
										comment=""
										onSave={(text) => {
											onSaveLineComment(lineNumber, text);
											activeCommentLine = null;
										}}
										onCancel={() => {
											activeCommentLine = null;
										}}
									/>
								{/if}
							{/if}
						{/each}
					{/each}
				</tbody>
			</table>
		</div>

		<!-- Per-file comment -->
		<div class="border-t border-border px-3 py-2 bg-surface-800">
			{#if comment && !showCommentInput}
				<div class="flex items-start gap-2">
					<p class="text-sm text-text-secondary flex-1 whitespace-pre-wrap">{comment}</p>
					<button
						type="button"
						class="btn btn-ghost btn-sm shrink-0"
						onclick={handleAddComment}
					>
						Edit
					</button>
				</div>
			{:else if showCommentInput}
				<div class="space-y-2">
					<textarea
						class="input w-full min-h-[4rem] font-mono text-sm resize-y"
						placeholder="Add a comment about this file..."
						bind:value={commentDraft}
					></textarea>
					<div class="flex items-center gap-2 justify-end">
						<button type="button" class="btn btn-ghost btn-sm" onclick={handleCancelComment}>
							Cancel
						</button>
						<button type="button" class="btn btn-primary btn-sm" onclick={handleSaveComment}>
							Save
						</button>
					</div>
				</div>
			{:else}
				<button
					type="button"
					class="text-sm text-text-muted hover:text-text-secondary transition-colors"
					onclick={handleAddComment}
				>
					Add comment
				</button>
			{/if}
		</div>
	{/if}
</div>
