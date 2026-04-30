<script lang="ts">
	import { onMount } from 'svelte';
	import { PasteClient, b64ToBytes, eventBytes } from '$lib/paste/client';
	import { importKey, encryptJSON, decryptJSON } from '$lib/paste/crypto';
	import { replayEvents, type CommentState } from '$lib/paste/events';
	import { getReviewerName, parseShareFragment, setReviewerName } from '$lib/paste/identity';
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
	let authorToken = $state<string | null>(null);
	let showNamePrompt = $state(false);

	// --- Share-link copy (author-only) ---
	let copiedShareLink = $state(false);
	let copyResetTimer: ReturnType<typeof setTimeout> | null = null;

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

	const isAuthor = $derived(authorToken !== null);

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
			authorToken = t;

			const resp = await client.get(data.id);
			plan = await decryptJSON<PlanPlaintext>(
				b64ToBytes(resp.plan_blob),
				b64ToBytes(resp.plan_iv),
				key
			);

			// Author URL flow: token + plan author name → auto-populate reviewer identity.
			// If the share was created without --author, fall through to the
			// reviewerName already loaded from localStorage at the top of onMount —
			// the lazy NamePromptModal will fire on first action. isAuthor still
			// stays true via authorToken in that case.
			if (authorToken && plan?.author_name) {
				reviewerName = plan.author_name;
				setReviewerName(plan.author_name);
			}

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

	function handleNameSaved(name: string) {
		reviewerName = name;
		showNamePrompt = false;
	}

	function clearSelection() {
		activeSelection = null;
		popoverMode = null;
		showQuickLabel = false;
		const sel = window.getSelection();
		sel?.removeAllRanges();
	}

	// Build the bare share URL (no &t=) from the current location and copy it.
	// Author URL fragment carries both k and t; reviewers must only ever see k.
	async function copyShareLink() {
		const { k } = parseShareFragment(window.location.hash);
		if (!k) return;
		const url = `${window.location.origin}${window.location.pathname}#k=${k}`;
		try {
			await navigator.clipboard.writeText(url);
		} catch {
			return;
		}
		copiedShareLink = true;
		if (copyResetTimer) clearTimeout(copyResetTimer);
		copyResetTimer = setTimeout(() => {
			copiedShareLink = false;
			copyResetTimer = null;
		}, 1500);
	}

	async function postEvent(event: EventPlaintext) {
		if (!key || !client) return;
		const { blob, iv } = await encryptJSON(event, key);
		await client.appendEvent(data.id, blob, iv);
	}

	async function postAuthorEvent(event: EventPlaintext) {
		// Phase B: identical to postEvent. Phase C will add an auth header here
		// and route the call through a server endpoint that verifies authorToken.
		return postEvent(event);
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

	// Name capture moved into FloatingToolbar.svelte: when reviewerName is
	// null, the toolbar renders an inline name field and gates its action
	// icons on it. By the time we receive an action here, we know the name
	// is set — so these handlers no longer need ensureName().
	async function handleToolbarAction(action: ToolbarAction) {
		if (!activeSelection) return;
		const sel = activeSelection;

		switch (action) {
			case 'praise':
				await createComment({
					body: 'Looks good',
					comment_type: 'praise',
					action: 'comment',
					anchor: buildAnchor(sel)
				});
				clearSelection();
				return;
			case 'comment':
				popoverMode = 'comment';
				return;
			case 'delete':
				await createComment({
					body: '',
					comment_type: 'comment',
					action: 'delete',
					anchor: buildAnchor(sel)
				});
				clearSelection();
				return;
			case 'suggest':
				popoverMode = 'suggest';
				return;
			case 'quick-label':
				showQuickLabel = true;
				return;
		}
	}

	function handleSetName(name: string) {
		reviewerName = name;
		setReviewerName(name);
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
		if (isAuthor && !isMyComment) {
			await postAuthorEvent(event);
		} else {
			await postEvent(event);
		}

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
		await postAuthorEvent(event);
		const next = new Map(comments);
		const target = next.get(commentId);
		if (target && isAuthor) {
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
				<div class="header-instruments flex items-center gap-3">
					{#if isAuthor}
						<button
							type="button"
							class="share-stamp"
							class:is-copied={copiedShareLink}
							onclick={copyShareLink}
							aria-label="Copy reviewer share link"
							aria-live="polite"
						>
							<span class="share-stamp-icon" aria-hidden="true">
								{#if copiedShareLink}
									<!-- check -->
									<svg viewBox="0 0 12 12" width="12" height="12">
										<path
											d="M2 6.4 L4.8 9 L10 3.2"
											fill="none"
											stroke="currentColor"
											stroke-width="1.6"
											stroke-linecap="round"
											stroke-linejoin="round"
										/>
									</svg>
								{:else}
									<!-- link -->
									<svg viewBox="0 0 12 12" width="12" height="12">
										<path
											d="M5 7.2 L7.2 5 M4.4 4 L3 5.4 a2.2 2.2 0 0 0 3.1 3.1 L7.4 7.2 M7.6 8 L9 6.6 a2.2 2.2 0 0 0 -3.1 -3.1 L4.6 4.8"
											fill="none"
											stroke="currentColor"
											stroke-width="1.4"
											stroke-linecap="round"
											stroke-linejoin="round"
										/>
									</svg>
								{/if}
							</span>
							<span class="share-stamp-label">
								{copiedShareLink ? 'Copied' : 'Share link'}
							</span>
						</button>
					{/if}
					{#if reviewerName}
						<button
							type="button"
							class="name-chip cursor-pointer hover:border-[var(--ink-comment-edge)]"
							onclick={() => (showNamePrompt = true)}
							title="Click to change name"
						>
							{reviewerName}{isAuthor ? ' · author' : ''}
						</button>
					{/if}
				</div>
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
			{reviewerName}
			onSetName={handleSetName}
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
		<NamePromptModal onSave={handleNameSaved} initialName={reviewerName ?? ''} />
	{/if}
</div>
