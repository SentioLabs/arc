<script lang="ts">
	import type { CommentState } from '$lib/paste/events';
	import type { ResolutionStatus } from '$lib/paste/types';
	import { tick } from 'svelte';
	import { modifierGlyph } from './platform.ts';

	const {
		entry,
		isAuthor,
		reviewerName,
		isActive = false,
		onClick,
		onResolve,
		onEdit
	}: {
		entry: CommentState;
		isAuthor: boolean;
		reviewerName: string | null;
		isActive?: boolean;
		onClick: () => void;
		onResolve: (status: ResolutionStatus, reply?: string) => Promise<void>;
		onEdit: (body: string, suggestedText: string | undefined) => Promise<void>;
	} = $props();

	const modKey = modifierGlyph();

	const e = $derived(entry.event);
	const isMine = $derived(reviewerName !== null && e.author_name === reviewerName);
	const isDelete = $derived(e.action === 'delete');
	const isSuggestion = $derived(!!e.suggested_text);

	const chipClass = $derived(
		isDelete ? 'chip-delete' : e.comment_type === 'praise' ? 'chip-praise' : 'chip-comment'
	);

	const chipLabel = $derived(
		isDelete
			? 'Delete'
			: isSuggestion
				? 'Suggest'
				: e.comment_type === 'praise'
					? 'Praise'
					: e.comment_type === 'issue'
						? 'Issue'
						: e.comment_type === 'question'
							? 'Question'
							: e.comment_type === 'nit'
								? 'Nit'
								: 'Comment'
	);

	const chipIcon = $derived(
		isDelete
			? '✕'
			: isSuggestion
				? '💡'
				: e.comment_type === 'praise'
					? '👍'
					: e.comment_type === 'question'
						? '❓'
						: e.comment_type === 'nit'
							? '💅'
							: '💬'
	);

	function timeLabel(iso: string): string {
		const d = new Date(iso);
		const seconds = Math.max(0, Math.floor((Date.now() - d.getTime()) / 1000));
		if (seconds < 60) return 'now';
		if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
		if (seconds < 86400) return `${Math.floor(seconds / 3600)}h`;
		return `${Math.floor(seconds / 86400)}d`;
	}

	let showRejectReply = $state(false);
	let rejectReply = $state('');

	// Inline edit mode for the comment's author. The reviewer can refine their
	// own annotation (e.g. expand "expand this more" into a fully-formed
	// suggestion). Only `body` and `suggested_text` are editable here — the
	// anchor and action stay frozen.
	let isEditing = $state(false);
	let editBody = $state('');
	let editSuggested = $state('');
	let editTextarea: HTMLTextAreaElement | undefined = $state();
	let saving = $state(false);

	// Edits are only meaningful while the comment is still under discussion.
	// Once it's accepted/rejected/resolved the meaning has been "consumed" by
	// the plan author — editing the body would be misleading.
	//
	// Two roles can edit:
	//   - The original commenter (refining their own wording)
	//   - The plan author (sharpening thin reviewer feedback into something
	//     LLM-consumable without waiting on the reviewer to update)
	// The displayed `author_name` doesn't change either way; the edit event
	// records who actually edited.
	const canEdit = $derived(
		reviewerName !== null &&
			(e.author_name === reviewerName || isAuthor) &&
			(entry.status === 'open' || entry.status === 'reopened')
	);

	async function startEdit() {
		editBody = e.body ?? '';
		editSuggested = e.suggested_text ?? '';
		isEditing = true;
		await tick();
		editTextarea?.focus();
	}

	function cancelEdit() {
		isEditing = false;
		editBody = '';
		editSuggested = '';
	}

	async function saveEdit() {
		if (saving) return;
		const body = editBody.trim();
		const suggested = e.suggested_text !== undefined ? editSuggested : undefined;
		// Don't save if nothing actually changed — avoids polluting the event log.
		if (body === (e.body ?? '') && suggested === (e.suggested_text ?? undefined)) {
			cancelEdit();
			return;
		}
		saving = true;
		try {
			await onEdit(body, suggested);
			isEditing = false;
		} finally {
			saving = false;
		}
	}

	function handleEditKey(ev: KeyboardEvent) {
		if (ev.key === 'Escape') {
			ev.preventDefault();
			cancelEdit();
		} else if (ev.key === 'Enter' && (ev.metaKey || ev.ctrlKey)) {
			ev.preventDefault();
			void saveEdit();
		}
	}
</script>

<div
	class="anno-card ui-sans block w-full cursor-pointer px-3.5 py-3 text-left {isActive
		? 'is-active'
		: ''}"
	role="button"
	tabindex="0"
	data-anno-card-id={entry.event.id}
	onclick={onClick}
	onkeydown={(e) => {
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			onClick();
		}
	}}
>
	<header class="mb-2 flex items-center justify-between gap-2 text-[11px]">
		<div class="flex items-center gap-1.5 text-[var(--ink-text-muted)]">
			<svg
				class="h-3 w-3"
				viewBox="0 0 24 24"
				fill="none"
				stroke="currentColor"
				stroke-width="2"
				aria-hidden="true"
			>
				<circle cx="12" cy="8" r="3.5" />
				<path stroke-linecap="round" d="M5 20c0-3.5 3-6 7-6s7 2.5 7 6" />
			</svg>
			<span class="font-medium">{e.author_name}</span>
			{#if isMine}
				<span class="text-[var(--ink-text-faint)]">(me)</span>
			{/if}
		</div>
		<span class="ui-mono text-[10px] text-[var(--ink-text-faint)]">
			{timeLabel(e.created_at)}{#if entry.editedAt}
				<span title="Edited {entry.editedAt}"> · edited {timeLabel(entry.editedAt)}</span>
			{/if}
		</span>
	</header>

	<div class="mb-2 flex items-center gap-2">
		<span
			class="{chipClass} inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.08em]"
		>
			<span aria-hidden="true">{chipIcon}</span>
			{chipLabel}
		</span>
		{#if entry.status !== 'open'}
			<span class="text-[10px] uppercase tracking-[0.08em] text-[var(--ink-text-faint)]">
				· {entry.status}
			</span>
		{/if}
	</div>

	<!-- Quoted source text -->
	<div class="quote mb-2 text-[13px]">
		"{e.anchor.quoted_text}"
	</div>

	<!-- Body / suggestion. In edit mode this becomes inline textareas for
		 the comment's author to refine their own annotation. -->
	{#if isEditing}
		<div
			class="mb-2 space-y-2"
			onclick={(ev) => ev.stopPropagation()}
			onkeydown={(ev) => ev.stopPropagation()}
			role="presentation"
		>
			{#if e.suggested_text !== undefined}
				<label class="block">
					<span class="mb-1 block text-[10px] uppercase tracking-[0.08em] text-[var(--ink-text-faint)]">
						Replacement
					</span>
					<textarea
						bind:value={editSuggested}
						rows="3"
						placeholder="Replacement text…"
						class="w-full rounded-md border border-[var(--ink-rule)] bg-[var(--ink-paper)] p-2 text-[13px] text-[var(--ink-text)] focus:border-[var(--ink-comment-edge)] focus:outline-none"
						onkeydown={handleEditKey}
					></textarea>
				</label>
			{/if}
			<label class="block">
				{#if e.suggested_text !== undefined}
					<span class="mb-1 block text-[10px] uppercase tracking-[0.08em] text-[var(--ink-text-faint)]">
						Reason (optional)
					</span>
				{/if}
				<textarea
					bind:this={editTextarea}
					bind:value={editBody}
					rows={e.suggested_text !== undefined ? 2 : 3}
					placeholder={e.suggested_text !== undefined ? 'Why this change?' : 'Comment…'}
					class="w-full rounded-md border border-[var(--ink-rule)] bg-[var(--ink-paper)] p-2 text-[13px] text-[var(--ink-text)] focus:border-[var(--ink-comment-edge)] focus:outline-none"
					onkeydown={handleEditKey}
				></textarea>
			</label>
			<div class="flex items-center justify-between">
				<div class="text-[10px] text-[var(--ink-text-faint)]">
					<kbd class="ui-mono rounded border border-[var(--ink-rule)] bg-[var(--ink-paper)] px-1 py-0.5">{modKey} ⏎</kbd>
					to save · <kbd class="ui-mono rounded border border-[var(--ink-rule)] bg-[var(--ink-paper)] px-1 py-0.5">esc</kbd> to cancel
				</div>
				<div class="flex gap-2">
					<button
						type="button"
						class="rounded-md px-2 py-1 text-[11px] text-[var(--ink-text-muted)] hover:bg-[var(--ink-paper)]"
						onclick={cancelEdit}
					>
						Cancel
					</button>
					<button
						type="button"
						disabled={saving}
						class="rounded-md border border-[var(--ink-comment-edge)] bg-[var(--ink-comment-bg)] px-2 py-1 text-[11px] font-medium text-[var(--ink-comment)] disabled:opacity-50"
						onclick={saveEdit}
					>
						{saving ? 'Saving…' : 'Save'}
					</button>
				</div>
			</div>
		</div>
	{:else if e.suggested_text}
		<div class="mb-2 space-y-1">
			<div class="text-[10px] uppercase tracking-[0.08em] text-[var(--ink-text-faint)]">
				Replacement
			</div>
			<div class="body text-[13px] text-[var(--ink-praise)]">
				{e.suggested_text}
			</div>
		</div>
		{#if e.body}
			<div class="body text-[13px]">{e.body}</div>
		{/if}
	{:else if e.body}
		<div class="body text-[13px]">{e.body}</div>
	{/if}

	{#if entry.reply}
		<div
			class="mt-2 border-l-2 border-[var(--ink-rule)] pl-2 text-[12px] italic text-[var(--ink-text-muted)]"
		>
			↪ {entry.reply}
		</div>
	{/if}

	<!--
		Single action toolbar combining Edit + the resolution actions, with
		a middle-dot separator between the two groups when both are present
		(plan author looking at an editable comment). The separator uses the
		same `·` glyph the card already uses for header metadata, keeping the
		editorial aesthetic consistent across the component.

		Color hierarchy carries the visual semantics:
		  - Edit: `--ink-text-muted` (soft, recessive — it's a refinement)
		  - Accept: `--ink-praise` (sage green — load-bearing yes)
		  - Reject: `--ink-delete` (editorial red — load-bearing no)
		  - Resolve / Reopen: muted (closing actions, no decision weight)
	-->
	{#if (canEdit || isAuthor) && !isEditing && !showRejectReply}
		<div class="mt-3 flex flex-wrap items-center gap-1.5 border-t border-[var(--ink-rule)] pt-2">
			{#if canEdit}
				<button
					type="button"
					class="rounded-md px-2 py-1 text-[11px] text-[var(--ink-text-muted)] hover:bg-[var(--ink-paper)]"
					onclick={(ev) => {
						ev.stopPropagation();
						void startEdit();
					}}
					title="Refine the wording before deciding"
				>
					✎ Edit
				</button>
			{/if}

			{#if canEdit && isAuthor}
				<span
					class="select-none px-0.5 text-[11px] text-[var(--ink-text-faint)]"
					aria-hidden="true">·</span
				>
			{/if}

			{#if isAuthor}
				{#if entry.status !== 'accepted'}
					<button
						type="button"
						class="rounded-md px-2 py-1 text-[11px] text-[var(--ink-praise)] hover:bg-[var(--ink-praise-bg)]"
						onclick={(e) => {
							e.stopPropagation();
							onResolve('accepted');
						}}
					>
						✓ Accept
					</button>
				{/if}
				{#if entry.status !== 'resolved'}
					<button
						type="button"
						class="rounded-md px-2 py-1 text-[11px] text-[var(--ink-text-muted)] hover:bg-[var(--ink-paper)]"
						onclick={(e) => {
							e.stopPropagation();
							onResolve('resolved');
						}}
					>
						— Resolve
					</button>
				{/if}
				{#if entry.status !== 'rejected'}
					<button
						type="button"
						class="rounded-md px-2 py-1 text-[11px] text-[var(--ink-delete)] hover:bg-[var(--ink-delete-bg)]"
						onclick={(e) => {
							e.stopPropagation();
							showRejectReply = true;
						}}
					>
						✕ Reject
					</button>
				{/if}
				{#if entry.status !== 'open' && entry.status !== 'reopened'}
					<button
						type="button"
						class="rounded-md px-2 py-1 text-[11px] text-[var(--ink-text-muted)] hover:bg-[var(--ink-paper)]"
						onclick={(e) => {
							e.stopPropagation();
							onResolve('reopened');
						}}
					>
						↻ Reopen
					</button>
				{/if}
			{/if}
		</div>
	{/if}

	{#if showRejectReply}
		<div class="mt-3 space-y-2 border-t border-[var(--ink-rule)] pt-2">
			<textarea
				bind:value={rejectReply}
				rows="2"
				placeholder="Reason (optional)…"
				class="w-full rounded-md border border-[var(--ink-rule)] bg-[var(--ink-paper)] p-2 text-xs text-[var(--ink-text)] focus:border-[var(--ink-delete-edge)] focus:outline-none"
				onclick={(e) => e.stopPropagation()}
			></textarea>
			<div class="flex justify-end gap-2">
				<button
					type="button"
					class="rounded-md px-2 py-1 text-[11px] text-[var(--ink-text-muted)]"
					onclick={(e) => {
						e.stopPropagation();
						showRejectReply = false;
						rejectReply = '';
					}}
				>
					Cancel
				</button>
				<button
					type="button"
					class="rounded-md border border-[var(--ink-delete-edge)] bg-[var(--ink-delete-bg)] px-2 py-1 text-[11px] font-medium text-[var(--ink-delete)]"
					onclick={async (e) => {
						e.stopPropagation();
						await onResolve('rejected', rejectReply);
						showRejectReply = false;
						rejectReply = '';
					}}
				>
					Confirm reject
				</button>
			</div>
		</div>
	{/if}
</div>
