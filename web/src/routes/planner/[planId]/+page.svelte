<script lang="ts">
	import { page } from '$app/stores';
	import { tick } from 'svelte';
	import {
		getPlan,
		updatePlanContent,
		updatePlanStatus,
		listPlanComments,
		createPlanComment,
		updatePlanComment,
		deletePlanComment
	} from '$lib/api';
	import type { PlanWithContent } from '$lib/api';
	import PlanRenderer from '$lib/planner/PlanRenderer.svelte';
	import FloatingToolbar from '$lib/planner/FloatingToolbar.svelte';
	import CommentPopover from '$lib/planner/CommentPopover.svelte';
	import CommentRail, { type RailEntry } from '$lib/planner/CommentRail.svelte';
	import { resolveAnchor } from '$lib/planner/anchor';
	import { railTopsEqual } from '$lib/planner/positioning';
	import type {
		AnchorResolution,
		InlineMark,
		PlanComment,
		PlanCommentAnchor,
		RenderedBlock,
		SelectionPayload
	} from '$lib/planner/types';

	let planId = $derived($page.params.planId);

	let plan = $state<PlanWithContent | null>(null);
	let comments = $state<PlanComment[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let actionError = $state<string | null>(null);
	let toast = $state<string | null>(null);

	// View modes: 'review' (rendered doc + highlights + rail), 'edit' (raw editor)
	type ViewMode = 'review' | 'edit';
	let viewMode = $state<ViewMode>('review');
	let editContent = $state('');

	let blocks = $state<RenderedBlock[]>([]);
	let selection = $state<SelectionPayload | null>(null);
	let composing = $state(false); // popover open for a NEW comment
	let activeId = $state<string | null>(null);
	let showResolved = $state(false);
	let reanchoringId = $state<string | null>(null); // comment awaiting a new selection
	let anchorTops = $state<Record<string, number>>({});
	let docWrap = $state<HTMLElement | undefined>(); // wraps PlanRenderer; position: relative
	let railWrap = $state<HTMLElement | undefined>(); // right-hand column: composer + rail
	let overallFeedback = $state('');
	let pendingOrphanBefore = $state<number | null>(null);
	let statusBusy = $state(false);

	// Per-comment resolution against current blocks.
	const resolutions = $derived.by(() => {
		const map = new Map<string, ReturnType<typeof resolveAnchor>>();
		for (const c of comments) {
			if (c.anchor) map.set(c.id, resolveAnchor(blocks, c.anchor));
		}
		return map;
	});

	const marks = $derived.by<InlineMark[]>(() =>
		comments.flatMap((c) => {
			if (!c.anchor) return [];
			const r = resolutions.get(c.id)!;
			if (r.status === 'orphaned') return [];
			return [
				{
					id: c.id,
					quotedText: c.anchor.quoted_text,
					occurrence: r.occurrence,
					lineStart: r.lineStart,
					lineEnd: r.lineEnd,
					resolved: !!c.resolved_at,
					drifted: r.status === 'drifted'
				}
			];
		})
	);

	// An anchor cannot be judged until PlanRenderer has published a block index:
	// `resolveAnchor` against an empty document reports every anchor as orphaned.
	// A genuinely empty plan is the one case where that verdict is already true,
	// so it is the plan's own content — not the block count alone — that tells us
	// whether parsing is still outstanding.
	const docParsed = $derived(blocks.length > 0 || !(plan?.content ?? '').trim());

	const railEntries = $derived.by<RailEntry[]>(() => {
		const entries: RailEntry[] = [];
		for (const c of comments) {
			// Hold anchored comments out of the rail until the document has been
			// parsed, instead of letting them render as orphaned and then jump
			// sections a tick later. The wait is bounded by the first render.
			if (c.anchor && !docParsed) continue;
			const r = c.anchor ? resolutions.get(c.id) : undefined;
			const orphaned = r?.status === 'orphaned';
			entries.push({
				comment: c,
				drifted: r?.status === 'drifted',
				orphaned,
				anchorTop: orphaned ? null : (anchorTops[c.id] ?? unmeasuredTop(c, r))
			});
		}
		// pinned (anchorTop null) first by created_at, then positioned by anchorTop
		return entries.sort((a, b) => {
			if ((a.anchorTop === null) !== (b.anchorTop === null)) return a.anchorTop === null ? -1 : 1;
			if (a.anchorTop === null) return a.comment.created_at.localeCompare(b.comment.created_at);
			return (a.anchorTop as number) - (b.anchorTop as number);
		});
	});

	const unresolvedCount = $derived(comments.filter((c) => !c.resolved_at).length);

	$effect(() => {
		if (planId) loadData();
	});

	$effect(() => {
		// Marks are not the only thing that moves a highlight, and a mark change
		// is not the only thing that invalidates a measurement:
		//   - the document column reflows on its own (web fonts swapping in,
		//     shiki replacing a plain <pre> with a highlighted one, images),
		//   - the rail stack ABOVE `.rail-positioned` — the composer, the header,
		//     the pinned cards — grows and shrinks, moving the origin every card
		//     top is measured against (see `railOriginOffset`).
		// Neither fires `resize` and neither changes `marks`, so a one-shot pass
		// keyed on marks alone leaves stale tops until the window is resized.
		// Observing both columns makes the measurement self-healing instead.
		//
		// This cannot feed back on itself. Everything a pass writes ends up on
		// `.rail-slot`'s `top`, and those slots are absolutely positioned, so they
		// contribute nothing to the size of either observed element. (Below
		// 1100px they are `position: static` and ignore `top` outright.) A
		// measurement therefore can never resize what triggered it.
		if (!docWrap || !railWrap) return;
		const ro = new ResizeObserver(scheduleMeasure);
		ro.observe(docWrap);
		ro.observe(railWrap);
		window.addEventListener('resize', scheduleMeasure);

		// The one reflow an observer structurally CANNOT see. app.css pulls
		// Newsreader and Instrument Sans from Google Fonts with `display=swap`, so
		// the document first paints in the fallback face and re-flows when the real
		// one arrives — after the marks were measured. That reflow re-cuts every
		// line box and moves every highlight while the column's measured height
		// rounds to the same value, so no element resizes and neither the observer
		// nor `resize` fires. `loadingdone` is the signal for it; `fonts.ready`
		// covers the case where the faces were already cached and no event fires.
		const onFontsSettled = () => scheduleMeasure();
		document.fonts.addEventListener('loadingdone', onFontsSettled);
		void document.fonts.ready.then(onFontsSettled);

		return () => {
			ro.disconnect();
			window.removeEventListener('resize', scheduleMeasure);
			document.fonts.removeEventListener('loadingdone', onFontsSettled);
			if (measureFrame !== null) cancelAnimationFrame(measureFrame);
			measureFrame = null;
		};
	});

	async function loadData() {
		if (!planId) return;
		loading = true;
		error = null;
		try {
			const [planData, commentsData] = await Promise.all([
				getPlan(planId),
				listPlanComments(planId)
			]);
			plan = planData;
			comments = commentsData;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load plan';
		} finally {
			loading = false;
		}
	}

	// `.rail-slot` cards are positioned (via `top: Npx`) relative to
	// `.rail-positioned`'s own top edge, which sits below the overall-feedback
	// composer, rail header, and any pinned cards in the right-hand column.
	// `anchorTop` is measured relative to the doc wrapper's top, so every card
	// renders offset by that stack's height unless we reconcile the two
	// coordinate origins here. Clamped at 0 so an anchor above the fold (were
	// the stack ever taller than the anchor itself) doesn't go negative.
	function railOriginOffset(docTop: number): number {
		const railPositioned = document.querySelector<HTMLElement>('.rail-positioned');
		return railPositioned ? railPositioned.getBoundingClientRect().top - docTop : 0;
	}

	let measureFrame: number | null = null;

	// The observers can fire several times for one layout change (doc column and
	// rail settle independently, a card grows, fonts swap). Collapse the burst
	// into a single pass per frame so we neither thrash layout nor re-render the
	// rail more than once for the same settle.
	function scheduleMeasure() {
		if (measureFrame !== null) return;
		measureFrame = requestAnimationFrame(() => {
			measureFrame = null;
			measureAnchorTops();
		});
	}

	/**
	 * Read every highlight's position. Safe to call on any layout change: one
	 * forced layout plus a walk of the marks, and it writes nothing when the
	 * layout hasn't actually moved.
	 *
	 * PlanRenderer's `onMarksApplied` calls this synchronously — that callback is
	 * the only moment the current marks are guaranteed to be in the DOM, which is
	 * why the measurement hangs off it rather than off a frame delay.
	 */
	function measureAnchorTops() {
		if (!docWrap) return;
		const base = docWrap.getBoundingClientRect().top;
		const offset = railOriginOffset(base);
		const tops: Record<string, number> = {};
		for (const m of docWrap.querySelectorAll<HTMLElement>('mark[data-anno-id]')) {
			const id = m.dataset.annoId!;
			if (!(id in tops)) tops[id] = Math.max(0, m.getBoundingClientRect().top - base - offset);
		}
		if (!railTopsEqual(anchorTops, tops)) anchorTops = tops;
	}

	/**
	 * Where a card sits when its own highlight has not been measured: at the top
	 * of the block its anchor resolved into. The <mark> lives inside that block,
	 * so this lands within a line of the final value.
	 *
	 * That it is a NUMBER matters more than its precision. `null` routes a card
	 * into the rail's PINNED section, so an anchored comment would mount there
	 * and be re-mounted into the positioned section one measurement later — and
	 * that unmount/mount is the DOM churn measurement used to race against.
	 * Legacy line_number-only comments (no anchor, so never a mark) reach the
	 * same path and stay on it.
	 */
	function unmeasuredTop(c: PlanComment, r: AnchorResolution | undefined): number | null {
		const line = r ? r.lineStart : c.line_number;
		return line == null ? null : blockTop(line);
	}

	/** Top of the last tagged block starting at or before `line`, doc-relative. */
	function blockTop(line: number): number | null {
		if (!docWrap) return null;
		let el: HTMLElement | null = null;
		for (const b of docWrap.querySelectorAll<HTMLElement>('[data-source-line]')) {
			if (Number(b.dataset.sourceLine) <= line) el = b;
			else break;
		}
		if (!el) return null;
		const base = docWrap.getBoundingClientRect().top;
		return Math.max(0, el.getBoundingClientRect().top - base - railOriginOffset(base));
	}

	async function handleBlocks(b: RenderedBlock[]) {
		blocks = b;
		await tick();
		measureAnchorTops();
		if (pendingOrphanBefore !== null) {
			const before = pendingOrphanBefore;
			pendingOrphanBefore = null;
			const after = comments.filter(
				(c) => c.anchor && resolutions.get(c.id)?.status !== 'orphaned'
			).length;
			if (after < before) {
				const lost = before - after;
				showToast(`${lost} comment${lost === 1 ? '' : 's'} lost their anchor`, 5000);
			}
		}
	}

	function showToast(message: string, ms: number) {
		toast = message;
		setTimeout(() => (toast = null), ms);
	}

	function selectionToAnchor(sel: SelectionPayload): PlanCommentAnchor {
		return {
			line_start: sel.lineStart,
			line_end: sel.lineEnd,
			quoted_text: sel.quotedText,
			occurrence: sel.occurrence,
			heading_slug: sel.headingSlug,
			context_before: sel.contextBefore,
			context_after: sel.contextAfter
		};
	}

	async function submitNewComment(body: string) {
		if (!planId || !selection) return;
		try {
			const created = await createPlanComment(
				planId,
				body,
				undefined,
				selectionToAnchor(selection)
			);
			comments = [...comments, created];
			composing = false;
			selection = null;
			window.getSelection()?.removeAllRanges();
		} catch (err) {
			actionError = err instanceof Error ? err.message : 'Failed to add comment';
		}
	}

	async function submitOverall(body: string) {
		if (!planId || !body.trim()) return;
		try {
			const created = await createPlanComment(planId, body.trim());
			comments = [...comments, created];
			overallFeedback = '';
		} catch (err) {
			actionError = err instanceof Error ? err.message : 'Failed to add comment';
		}
	}

	async function saveContent(id: string, content: string) {
		if (!planId) return;
		try {
			const updated = await updatePlanComment(planId, id, { content });
			comments = comments.map((c) => (c.id === id ? updated : c));
		} catch (err) {
			actionError = err instanceof Error ? err.message : 'Failed to save comment';
		}
	}

	async function toggleResolve(id: string) {
		if (!planId) return;
		const c = comments.find((x) => x.id === id);
		if (!c) return;
		try {
			const updated = await updatePlanComment(planId, id, { resolved: !c.resolved_at });
			comments = comments.map((x) => (x.id === id ? updated : x));
		} catch (err) {
			actionError = err instanceof Error ? err.message : 'Failed to update comment';
		}
	}

	async function removeComment(id: string) {
		if (!planId) return;
		try {
			await deletePlanComment(planId, id);
			comments = comments.filter((c) => c.id !== id);
			if (activeId === id) activeId = null;
		} catch (err) {
			actionError = err instanceof Error ? err.message : 'Failed to delete comment';
		}
	}

	// Re-anchor flow: card button arms it; the NEXT selection becomes the new anchor.
	async function handleSelection(sel: SelectionPayload | null) {
		// While the composer is open its textarea holds focus, which collapses the
		// document selection — PlanRenderer then reports `null` and would pull the
		// draft's anchor (and the popover with it) out from under the user. The
		// composer owns the selection until it closes.
		if (composing) return;
		selection = sel;
		if (!sel) return;
		if (reanchoringId) {
			const id = reanchoringId;
			try {
				const updated = await updatePlanComment(planId!, id, { anchor: selectionToAnchor(sel) });
				comments = comments.map((c) => (c.id === id ? updated : c));
				reanchoringId = null;
				selection = null;
				window.getSelection()?.removeAllRanges();
				showToast('Highlight updated', 3000);
			} catch (err) {
				actionError = err instanceof Error ? err.message : 'Failed to change highlight';
			}
		}
	}

	async function handleSaveEdit() {
		if (!plan || !planId) return;
		const before = comments.filter(
			(c) => c.anchor && resolutions.get(c.id)?.status !== 'orphaned'
		).length;
		try {
			const updated = await updatePlanContent(planId, editContent);
			plan = updated;
		} catch (err) {
			actionError = err instanceof Error ? err.message : 'Failed to save';
			return;
		}
		pendingOrphanBefore = before;
		viewMode = 'review';
		// The next onBlocks fire (handleBlocks) reflects the freshly-saved content;
		// it consumes pendingOrphanBefore to report any newly orphaned comments.
	}

	async function setStatus(status: string) {
		if (!planId || statusBusy) return;
		statusBusy = true;
		try {
			const updated = await updatePlanStatus(planId, status);
			if (plan) plan = { ...plan, status: updated.status };
		} catch (err) {
			actionError = err instanceof Error ? err.message : 'Failed to update status';
		} finally {
			statusBusy = false;
		}
	}

	function scrollMarkIntoView(id: string) {
		docWrap
			?.querySelector<HTMLElement>(`mark[data-anno-id="${CSS.escape(id)}"]`)
			?.scrollIntoView({ behavior: 'smooth', block: 'center' });
	}

	function scrollCardIntoView(id: string) {
		document
			.querySelector<HTMLElement>(`[data-comment-id="${CSS.escape(id)}"]`)
			?.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
	}

	// Hover cross-linking is delegated and DOM-class based: marks are re-created
	// on every apply, so hover state must not live on them.
	function handleDocHover(e: PointerEvent) {
		const mark = (e.target as Element).closest?.('mark[data-anno-id]') as HTMLElement | null;
		setHovered(mark?.dataset.annoId ?? null);
	}

	function handleRailHover(e: PointerEvent) {
		const card = (e.target as Element).closest?.('[data-comment-id]') as HTMLElement | null;
		setHovered(card?.dataset.commentId ?? null);
	}

	function setHovered(id: string | null) {
		document.querySelectorAll('.is-hovered').forEach((el) => el.classList.remove('is-hovered'));
		if (!id) return;
		docWrap
			?.querySelectorAll(`mark[data-anno-id="${CSS.escape(id)}"]`)
			.forEach((el) => el.classList.add('is-hovered'));
		document.querySelector(`[data-comment-id="${CSS.escape(id)}"]`)?.classList.add('is-hovered');
	}

	function switchToEdit() {
		editContent = plan?.content ?? '';
		viewMode = 'edit';
	}

	function statusColor(status: string): string {
		switch (status) {
			case 'draft':
				return 'bg-surface-600 text-text-secondary';
			case 'in_review':
				return 'bg-yellow-900/30 text-yellow-400 border border-yellow-800';
			case 'approved':
				return 'bg-green-900/30 text-green-400 border border-green-800';
			case 'changes_requested':
				return 'bg-amber-900/30 text-amber-400 border border-amber-800';
			case 'rejected':
				return 'bg-red-900/30 text-red-400 border border-red-800';
			default:
				return 'bg-surface-600 text-text-secondary';
		}
	}
</script>

{#if loading}
	<div class="flex items-center justify-center py-20">
		<div class="text-text-muted animate-pulse">Loading plan...</div>
	</div>
{:else if error}
	<div class="flex items-center justify-center py-20">
		<div class="text-red-400">{error}</div>
	</div>
{:else if plan}
	<div class="max-w-5xl mx-auto p-6 space-y-6">
		<!-- Header -->
		<div class="flex items-center justify-between gap-4">
			<div class="min-w-0">
				<h1 class="text-xl font-semibold text-text-primary truncate">
					{plan.file_path.split('/').pop()}
				</h1>
				<p class="text-sm text-text-muted mt-1 truncate">{plan.file_path}</p>
			</div>
			<div class="flex items-center gap-2 shrink-0">
				<span class="px-3 py-1 rounded-full text-xs font-medium {statusColor(plan.status)}">
					{plan.status}
				</span>
				<button
					class="btn-approve"
					onclick={() => setStatus('approved')}
					disabled={statusBusy || plan.status === 'approved'}>Approve</button
				>
				<button
					class="btn-request"
					onclick={() => setStatus('changes_requested')}
					disabled={statusBusy || unresolvedCount === 0}
					title={unresolvedCount === 0 ? 'Add at least one comment first' : undefined}
				>
					Request changes
				</button>
				<button class="btn-reject" onclick={() => setStatus('rejected')} disabled={statusBusy}
					>Reject</button
				>
			</div>
		</div>

		<!-- View Mode Tabs -->
		<div class="flex gap-1 border-b border-surface-600">
			<button
				onclick={() => (viewMode = 'review')}
				class="px-4 py-2 text-sm transition-colors flex items-center gap-1.5 {viewMode === 'review'
					? 'text-text-primary border-b-2 border-primary-500 -mb-px'
					: 'text-text-muted hover:text-text-secondary'}"
			>
				Review
				{#if unresolvedCount > 0}
					<span class="px-1.5 py-0.5 text-xs rounded-full bg-yellow-900/30 text-yellow-400"
						>{unresolvedCount}</span
					>
				{/if}
			</button>
			<button
				onclick={switchToEdit}
				class="px-4 py-2 text-sm transition-colors {viewMode === 'edit'
					? 'text-text-primary border-b-2 border-primary-500 -mb-px'
					: 'text-text-muted hover:text-text-secondary'}"
			>
				Edit
			</button>
		</div>

		{#if actionError}
			<div
				class="action-error flex items-center justify-between gap-3 rounded-lg border border-red-800 bg-red-900/30 px-3 py-2 text-sm text-red-400"
				role="alert"
			>
				<span>{actionError}</span>
				<button
					type="button"
					class="text-red-400 hover:text-red-300"
					aria-label="Dismiss error"
					onclick={() => (actionError = null)}>✕</button
				>
			</div>
		{/if}

		<!-- Review Mode: rendered document + margin rail -->
		{#if viewMode === 'review'}
			{#if reanchoringId}
				<div class="reanchor-banner">
					Select the new text for this comment — or
					<button type="button" class="underline" onclick={() => (reanchoringId = null)}>
						cancel
					</button>
				</div>
			{/if}

			<div class="planner-review">
				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<div
					class="planner-doc markdown"
					bind:this={docWrap}
					style="position: relative"
					onpointerover={handleDocHover}
				>
					<PlanRenderer
						markdown={plan.content ?? ''}
						{marks}
						activeMarkId={activeId ?? undefined}
						onSelection={handleSelection}
						onMarkClick={(id) => {
							activeId = id;
							scrollCardIntoView(id);
						}}
						onBlocks={handleBlocks}
						onMarksApplied={measureAnchorTops}
					/>
				</div>

				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<div bind:this={railWrap} onpointerover={handleRailHover}>
					<div class="mb-6 space-y-2">
						<label
							for="overall-feedback"
							class="block text-[10px] uppercase tracking-[0.08em] text-[var(--ink-text-faint)]"
						>
							Overall feedback
						</label>
						<textarea
							id="overall-feedback"
							bind:value={overallFeedback}
							rows="2"
							placeholder="Overall feedback on this plan…"
							class="w-full rounded-md border border-[var(--ink-rule)] bg-[var(--ink-paper)] p-2 text-[13px] text-[var(--ink-text)] focus:border-[var(--ink-comment-edge)] focus:outline-none"
							onkeydown={(e) => {
								if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) submitOverall(overallFeedback);
							}}
						></textarea>
						<div class="flex items-center justify-between gap-2">
							<span class="text-[10px] text-[var(--ink-text-faint)]"
								>Press Ctrl+Enter to submit</span
							>
							<button
								type="button"
								disabled={!overallFeedback.trim()}
								class="rounded-md border border-[var(--ink-comment-edge)] bg-[var(--ink-comment-bg)] px-2.5 py-1.5 text-xs font-medium text-[var(--ink-comment)] disabled:cursor-not-allowed disabled:opacity-50"
								onclick={() => submitOverall(overallFeedback)}
							>
								Add
							</button>
						</div>
					</div>

					<CommentRail
						entries={railEntries}
						{activeId}
						{showResolved}
						onToggleShowResolved={() => (showResolved = !showResolved)}
						onActivate={(id) => {
							activeId = id;
							scrollMarkIntoView(id);
						}}
						onSaveContent={saveContent}
						onReanchor={(id) => {
							composing = false;
							selection = null;
							window.getSelection()?.removeAllRanges();
							reanchoringId = id;
						}}
						onToggleResolve={toggleResolve}
						onDelete={removeComment}
					/>
				</div>
			</div>

			{#if selection && !composing && !reanchoringId}
				<FloatingToolbar
					anchorRect={selection.rect}
					onComment={() => (composing = true)}
					onDismiss={() => {
						selection = null;
					}}
				/>
			{/if}
			{#if selection && composing}
				<CommentPopover
					anchorRect={selection.rect}
					quotedText={selection.quotedText}
					onSave={submitNewComment}
					onCancel={() => {
						composing = false;
						selection = null;
					}}
				/>
			{/if}
		{/if}

		<!-- Edit Mode: Raw markdown editor -->
		{#if viewMode === 'edit'}
			<div class="card p-4 space-y-3">
				<textarea
					bind:value={editContent}
					class="w-full h-[32rem] bg-surface-700 text-text-primary font-mono text-sm p-4 rounded border border-surface-500 focus:border-primary-500 focus:outline-none resize-y"
					spellcheck="false"
				></textarea>
				<div class="flex gap-2 justify-end">
					<button
						onclick={() => (viewMode = 'review')}
						class="px-3 py-1.5 text-sm text-text-secondary hover:text-text-primary"
					>
						Cancel
					</button>
					<button
						onclick={handleSaveEdit}
						class="px-3 py-1.5 text-sm bg-primary-600 text-white rounded hover:bg-primary-500"
					>
						Save
					</button>
				</div>
			</div>
		{/if}
	</div>

	{#if toast}<div class="toast">{toast}</div>{/if}
{/if}
