<script lang="ts">
	import { onMount } from 'svelte';
	import { PasteClient, b64ToBytes, eventBytes } from '$lib/paste/client';
	import { importKey, encryptJSON, decryptJSON } from '$lib/paste/crypto';
	import { replayEvents, type CommentState } from '$lib/paste/events';
	import { getReviewerName, parseShareFragment } from '$lib/paste/identity';
	import type {
		PlanPlaintext,
		EventPlaintext,
		CommentEvent,
		CommentType,
		EditEvent,
		ResolutionEvent,
		ResolutionStatus,
		Anchor
	} from '$lib/paste/types';
	import PlanRenderer from './components/PlanRenderer.svelte';
	import FloatingToolbar, { type ToolbarAction } from './components/FloatingToolbar.svelte';
	import CommentPopover, { type PopoverMode } from './components/CommentPopover.svelte';
	import QuickLabelPicker from './components/QuickLabelPicker.svelte';
	import AnnotationsPanel from './components/AnnotationsPanel.svelte';
	import NamePromptModal from './components/NamePromptModal.svelte';
	import type { InlineMark } from './components/inline-annotations.ts';

	const { data }: { data: { id: string } } = $props();

	// --- Decrypted state ---
	let plan = $state<PlanPlaintext | null>(null);
	let comments = $state(new Map<string, CommentState>());
	let key = $state<CryptoKey | null>(null);
	let loadError = $state<string | null>(null);

	// --- Reviewer identity ---
	let reviewerName = $state<string | null>(null);
	let showNamePrompt = $state(false);
	let pendingAfterName: (() => void) | null = null;

	// --- UI state for selection-driven actions ---
	type SelectionInfo = {
		lineStart: number;
		lineEnd: number;
		quotedText: string;
		headingSlug?: string;
		contextBefore?: string;
		contextAfter?: string;
		rect: DOMRect;
	};
	let activeSelection = $state<SelectionInfo | null>(null);
	let popoverMode = $state<PopoverMode | null>(null);
	let showQuickLabel = $state(false);
	let activeMarkId = $state<string | undefined>(undefined);

	let client: PasteClient | undefined;

	const isAuthor = $derived(
		reviewerName !== null &&
			plan !== null &&
			!!plan.author_name &&
			plan.author_name === reviewerName
	);

	const orderedStates = $derived.by(() => {
		return [...comments.values()].sort((a, b) =>
			b.event.created_at.localeCompare(a.event.created_at)
		);
	});

	const marks = $derived.by((): InlineMark[] => {
		const out: InlineMark[] = [];
		for (const state of comments.values()) {
			if (state.status === 'resolved' || state.status === 'rejected') continue;
			out.push({
				id: state.event.id,
				kind: state.event.action === 'delete' ? 'delete' : 'comment',
				lineStart: state.event.anchor.line_start,
				lineEnd: state.event.anchor.line_end,
				quotedText: state.event.anchor.quoted_text
			});
		}
		return out;
	});

	onMount(async () => {
		reviewerName = getReviewerName();
		client = new PasteClient(window.location.origin);

		try {
			const { k, t } = parseShareFragment(window.location.hash);
			if (!k) {
				loadError = 'Missing #k=<key> in URL — share link is incomplete.';
				return;
			}
			key = await importKey(k);
			// `t` is unused in this commit; consumed in the next task.
			void t;

			const resp = await client.get(data.id);
			plan = await decryptJSON<PlanPlaintext>(
				b64ToBytes(resp.plan_blob),
				b64ToBytes(resp.plan_iv),
				key
			);

			const events: EventPlaintext[] = [];
			for (const ev of resp.events) {
				const { blob, iv } = eventBytes(ev);
				try {
					events.push(await decryptJSON<EventPlaintext>(blob, iv, key));
				} catch {
					// skip undecryptable events
				}
			}
			comments = replayEvents(plan?.author_name, events);
		} catch (err) {
			loadError = (err as Error)?.message ?? 'Failed to load share';
		}
	});

	function ensureName(after: () => void) {
		if (reviewerName) {
			after();
			return;
		}
		pendingAfterName = after;
		showNamePrompt = true;
	}

	function handleNameSaved(name: string) {
		reviewerName = name;
		showNamePrompt = false;
		const cb = pendingAfterName;
		pendingAfterName = null;
		cb?.();
	}

	function clearSelection() {
		activeSelection = null;
		popoverMode = null;
		showQuickLabel = false;
		const sel = window.getSelection();
		sel?.removeAllRanges();
	}

	async function postEvent(event: EventPlaintext) {
		if (!key || !client) return;
		const { blob, iv } = await encryptJSON(event, key);
		await client.appendEvent(data.id, blob, iv);
	}

	function buildAnchor(sel: SelectionInfo): Anchor {
		return {
			line_start: sel.lineStart,
			line_end: sel.lineEnd,
			quoted_text: sel.quotedText,
			heading_slug: sel.headingSlug,
			context_before: sel.contextBefore,
			context_after: sel.contextAfter
		};
	}

	async function createComment(opts: {
		body: string;
		comment_type?: CommentType;
		action?: 'comment' | 'delete';
		suggested_text?: string;
		anchor: Anchor;
	}) {
		if (!reviewerName) return;
		const event: CommentEvent = {
			kind: 'comment',
			id: `c-${crypto.randomUUID()}`,
			author_name: reviewerName,
			action: opts.action ?? 'comment',
			comment_type: opts.comment_type ?? 'comment',
			body: opts.body,
			suggested_text: opts.suggested_text,
			anchor: opts.anchor,
			created_at: new Date().toISOString()
		};
		await postEvent(event);
		const next = new Map(comments);
		next.set(event.id, { event, status: 'open' });
		comments = next;
	}

	async function handleToolbarAction(action: ToolbarAction) {
		if (!activeSelection) return;
		const sel = activeSelection;

		switch (action) {
			case 'praise':
				ensureName(async () => {
					await createComment({
						body: 'Looks good',
						comment_type: 'praise',
						action: 'comment',
						anchor: buildAnchor(sel)
					});
					clearSelection();
				});
				return;
			case 'comment':
				ensureName(() => {
					popoverMode = 'comment';
				});
				return;
			case 'delete':
				ensureName(async () => {
					await createComment({
						body: '',
						comment_type: 'comment',
						action: 'delete',
						anchor: buildAnchor(sel)
					});
					clearSelection();
				});
				return;
			case 'suggest':
				ensureName(() => {
					popoverMode = 'suggest';
				});
				return;
			case 'quick-label':
				ensureName(() => {
					showQuickLabel = true;
				});
				return;
		}
	}

	async function handlePopoverSave(body: string, suggestedText?: string) {
		if (!activeSelection) return;
		await createComment({
			body,
			comment_type: suggestedText ? 'suggestion' : 'comment',
			action: 'comment',
			suggested_text: suggestedText,
			anchor: buildAnchor(activeSelection)
		});
		clearSelection();
	}

	async function handleQuickLabelPick(label: CommentType, presetBody: string) {
		if (!activeSelection) return;
		const sel = activeSelection;
		showQuickLabel = false;
		if (presetBody) {
			await createComment({
				body: presetBody,
				comment_type: label,
				action: 'comment',
				anchor: buildAnchor(sel)
			});
			clearSelection();
		} else {
			// No preset — open the comment popover so the user can write a body
			popoverMode = 'comment';
		}
	}

	async function handleEdit(
		commentId: string,
		body: string,
		suggestedText: string | undefined
	) {
		if (!reviewerName) return;
		const target = comments.get(commentId);
		if (!target) return;
		// Authorization mirror of replayEvents:
		//   - Original commenter can edit their own comment.
		//   - Plan author can edit any comment (sharpening thin feedback).
		// Failing fast here avoids posting events the replay would discard.
		const isMyComment = target.event.author_name === reviewerName;
		if (!isMyComment && !isAuthor) return;

		const event: EditEvent = {
			kind: 'edit',
			id: `e-${crypto.randomUUID()}`,
			comment_id: commentId,
			author_name: reviewerName,
			body,
			suggested_text: suggestedText,
			created_at: new Date().toISOString()
		};
		await postEvent(event);

		// Apply locally so the card updates without a round-trip refetch.
		const next = new Map(comments);
		next.set(commentId, {
			...target,
			event: {
				...target.event,
				body,
				suggested_text: suggestedText !== undefined ? suggestedText : target.event.suggested_text
			},
			editedAt: event.created_at
		});
		comments = next;
	}

	async function handleResolve(commentId: string, status: ResolutionStatus, reply?: string) {
		if (!reviewerName) return;
		const event: ResolutionEvent = {
			kind: 'resolution',
			id: `r-${crypto.randomUUID()}`,
			comment_id: commentId,
			status,
			reply,
			author_name: reviewerName,
			created_at: new Date().toISOString()
		};
		await postEvent(event);
		const next = new Map(comments);
		const target = next.get(commentId);
		if (target && plan?.author_name === reviewerName) {
			next.set(commentId, { ...target, status, reply, replyAt: event.created_at });
		}
		comments = next;
	}

	function handleSelection(sel: SelectionInfo | null) {
		if (!sel) {
			if (!popoverMode && !showQuickLabel) activeSelection = null;
			return;
		}
		activeSelection = sel;
		popoverMode = null;
		showQuickLabel = false;
	}

	function handleMarkClick(id: string) {
		activeMarkId = activeMarkId === id ? undefined : id;
		document
			.querySelector(`[data-anno-card-id="${id}"]`)
			?.scrollIntoView({ behavior: 'smooth', block: 'center' });
	}

	function handleCardClick(id: string) {
		activeMarkId = activeMarkId === id ? undefined : id;
	}
</script>

<svelte:head>
	<title>{plan?.title ?? 'Plan review'} · arc</title>
</svelte:head>

<div class="share-page grid h-screen">
	<!-- Document area: paper-grain sheet, viewport-centered between
		 the phantom desk column on the left and the rail on the right.
		 See `.share-page` rules in app.css for the grid geometry. -->
	<main class="doc-area overflow-y-auto">
		<div class="doc-inner flex flex-col px-10 py-12">
			<header
				class="mb-10 flex items-baseline justify-between border-b border-[var(--ink-rule)] pb-6"
			>
				<div>
					<div
						class="ui-mono mb-1 text-[10px] uppercase tracking-[0.16em] text-[var(--ink-text-faint)]"
					>
						Plan · {data.id}
					</div>
					<h1 class="ui-sans text-xl font-semibold text-[var(--ink-text)]">
						{plan?.title ?? 'Untitled plan'}
					</h1>
				</div>
				{#if reviewerName}
					<span class="name-chip">
						{reviewerName}{isAuthor ? ' · author' : ''}
					</span>
				{:else}
					<button
						type="button"
						class="name-chip cursor-pointer hover:border-[var(--ink-comment-edge)]"
						onclick={() => (showNamePrompt = true)}
					>
						Sign in
					</button>
				{/if}
			</header>

			{#if loadError}
				<div
					class="rounded-md border border-[var(--ink-delete-edge)] bg-[var(--ink-delete-bg)] p-4 text-sm text-[var(--ink-delete)]"
				>
					<strong class="font-semibold">Couldn't load plan.</strong>
					<div class="mt-1 text-[var(--ink-text-muted)]">{loadError}</div>
				</div>
			{:else if !plan}
				<div class="text-sm italic text-[var(--ink-text-faint)]">Decrypting plan…</div>
			{:else}
				<PlanRenderer
					markdown={plan.markdown}
					{marks}
					{activeMarkId}
					onSelection={handleSelection}
					onMarkClick={handleMarkClick}
				/>
			{/if}
		</div>
	</main>

	<!-- Right rail: annotations panel — sits in col 3 of the editorial spread. -->
	<aside
		class="rail-area overflow-y-auto border-l border-[var(--ink-rule)] bg-[var(--ink-paper-raised)]"
	>
		<div class="rail-inner mr-auto max-w-[360px]">
			<AnnotationsPanel
				states={orderedStates}
				{isAuthor}
				{reviewerName}
				activeId={activeMarkId}
				onCardClick={handleCardClick}
				onResolve={handleResolve}
				onEdit={handleEdit}
			/>
		</div>
	</aside>

	<!--
		Floating overlays must live inside `.share-page` so they inherit the
		`--ink-*` design tokens defined in app.css. Fixed positioning takes them
		out of grid flow, so they don't claim a grid cell.
	-->
	{#if activeSelection && !popoverMode && !showQuickLabel}
		<FloatingToolbar
			anchorRect={activeSelection.rect}
			onAction={handleToolbarAction}
			onDismiss={clearSelection}
		/>
	{/if}

	{#if activeSelection && popoverMode}
		<CommentPopover
			anchorRect={activeSelection.rect}
			mode={popoverMode}
			quotedText={activeSelection.quotedText}
			onSave={handlePopoverSave}
			onCancel={clearSelection}
		/>
	{/if}

	{#if activeSelection && showQuickLabel}
		<QuickLabelPicker
			anchorRect={activeSelection.rect}
			onPick={handleQuickLabelPick}
			onDismiss={() => (showQuickLabel = false)}
		/>
	{/if}

	{#if showNamePrompt}
		<NamePromptModal onSave={handleNameSaved} />
	{/if}
</div>
