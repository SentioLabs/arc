<script lang="ts">
	import { formatRelativeTime } from '$lib/utils';
	import type { RailEntry } from './CommentRail.svelte';

	let {
		entry,
		active,
		onActivate,
		onSaveContent,
		onReanchor,
		onToggleResolve,
		onDelete
	}: {
		entry: RailEntry;
		active: boolean;
		onActivate: () => void;
		onSaveContent: (content: string) => Promise<void>;
		onReanchor: () => void;
		onToggleResolve: () => Promise<void>;
		onDelete: () => Promise<void>;
	} = $props();

	const comment = $derived(entry.comment);
	const resolved = $derived(!!comment.resolved_at);

	let editing = $state(false);
	let draft = $state('');
	let confirmingDelete = $state(false);
	let busy = $state(false);

	function startEdit() {
		draft = comment.content;
		editing = true;
	}

	async function saveEdit() {
		if (!draft.trim() || busy) return;
		busy = true;
		try {
			await onSaveContent(draft.trim());
			editing = false;
		} finally {
			busy = false;
		}
	}

	async function toggleResolve() {
		if (busy) return;
		busy = true;
		try {
			await onToggleResolve();
		} finally {
			busy = false;
		}
	}

	async function doDelete() {
		if (busy) return;
		busy = true;
		try {
			await onDelete();
		} finally {
			busy = false;
			confirmingDelete = false;
		}
	}
</script>

<article
	class="anno-card ui-sans px-3.5 py-3"
	class:is-active={active}
	class:is-resolved={resolved}
	data-comment-id={comment.id}
>
	<!-- clickable header region activates the card (scroll-sync to mark) -->
	<button
		type="button"
		class="anno-card-hit"
		onclick={onActivate}
		aria-label="Jump to highlighted text"
	>
		{#if comment.anchor}
			<blockquote class="quote text-[13px]">
				"{comment.anchor.quoted_text.length > 120
					? comment.anchor.quoted_text.slice(0, 120) + '…'
					: comment.anchor.quoted_text}"
			</blockquote>
		{:else if comment.line_number != null}
			<span class="anchor-ref">Line {comment.line_number}</span>
		{:else}
			<span class="anchor-ref">Overall feedback</span>
		{/if}
	</button>

	{#if entry.drifted || entry.orphaned}
		<div class="mt-2 flex flex-wrap items-center gap-2">
			{#if entry.drifted}
				<span class="badge badge-drifted">moved</span>
			{/if}
			{#if entry.orphaned}
				<span class="badge badge-orphaned">original text no longer in document</span>
			{/if}
		</div>
	{/if}

	{#if editing}
		<div class="mt-2 space-y-2">
			<textarea
				bind:value={draft}
				rows="3"
				class="w-full rounded-md border border-[var(--ink-rule)] bg-[var(--ink-paper)] p-2 text-[13px] text-[var(--ink-text)] focus:border-[var(--ink-comment-edge)] focus:outline-none"
			></textarea>
			<div class="flex justify-end gap-2">
				<button
					type="button"
					class="rounded-md px-2 py-1 text-[11px] text-[var(--ink-text-muted)] hover:bg-[var(--ink-paper)]"
					onclick={() => (editing = false)}
				>
					Cancel
				</button>
				<button
					type="button"
					disabled={!draft.trim() || busy}
					class="rounded-md border border-[var(--ink-comment-edge)] bg-[var(--ink-comment-bg)] px-2 py-1 text-[11px] font-medium text-[var(--ink-comment)] disabled:cursor-not-allowed disabled:opacity-50"
					onclick={saveEdit}
				>
					Save
				</button>
			</div>
		</div>
	{:else}
		<p class="anno-body mt-2 text-[13px]">{comment.content}</p>
		<footer class="mt-2 flex flex-wrap items-center gap-2 text-[11px] text-[var(--ink-text-faint)]">
			<time>{formatRelativeTime(comment.created_at)}</time>
			{#if comment.updated_at}
				<span title={comment.updated_at}>edited</span>
			{/if}
			<span class="grow"></span>
			<button
				type="button"
				class="rounded-md px-2 py-1 text-[var(--ink-text-muted)] hover:bg-[var(--ink-paper)]"
				aria-label="Edit comment"
				onclick={startEdit}
			>
				Edit
			</button>
			{#if !entry.orphaned}
				<button
					type="button"
					class="rounded-md px-2 py-1 text-[var(--ink-text-muted)] hover:bg-[var(--ink-paper)]"
					aria-label="Change highlighted text"
					onclick={onReanchor}
				>
					Change highlight
				</button>
			{/if}
			<button
				type="button"
				class="rounded-md px-2 py-1 text-[var(--ink-text-muted)] hover:bg-[var(--ink-paper)]"
				aria-label={resolved ? 'Unresolve comment' : 'Resolve comment'}
				disabled={busy}
				onclick={toggleResolve}
			>
				{resolved ? 'Unresolve' : 'Resolve'}
			</button>
			{#if confirmingDelete}
				<span class="flex items-center gap-1">
					Delete?
					<button
						type="button"
						class="rounded-md px-2 py-1 font-medium text-[var(--ink-comment)] hover:bg-[var(--ink-comment-bg)]"
						disabled={busy}
						onclick={doDelete}
					>
						Yes
					</button>
					<button
						type="button"
						class="rounded-md px-2 py-1 text-[var(--ink-text-muted)] hover:bg-[var(--ink-paper)]"
						onclick={() => (confirmingDelete = false)}
					>
						No
					</button>
				</span>
			{:else}
				<button
					type="button"
					class="rounded-md px-2 py-1 text-[var(--ink-text-muted)] hover:bg-[var(--ink-paper)]"
					aria-label="Delete comment"
					onclick={() => (confirmingDelete = true)}
				>
					Delete
				</button>
			{/if}
		</footer>
	{/if}
</article>
