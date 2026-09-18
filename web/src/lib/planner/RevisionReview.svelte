<script lang="ts">
	import { tick } from 'svelte';
	import { goto } from '$app/navigation';
	import {
		getMetadata,
		getRevision,
		listRevisions,
		listComments,
		listDispositions,
		listDecisions,
		listCommentVersions,
		allPages,
		addComment,
		mutateComment,
		saveRevision,
		decideRevision,
		addDisposition,
		updateMetadata,
		revisionURL,
		type Plan,
		type Revision,
		type Disposition
	} from '$lib/api/plans';
	import type { components } from '$lib/api/types';
	import { feedbackDisposition, reviewContext } from './review';
	import PlanRenderer from '$lib/planner/PlanRenderer.svelte';
	import FloatingToolbar from '$lib/planner/FloatingToolbar.svelte';
	import CommentPopover from '$lib/planner/CommentPopover.svelte';
	import CommentRail, { type RailEntry } from '$lib/planner/CommentRail.svelte';
	import { resolveAnchor } from '$lib/planner/anchor';
	import { railTopsEqual } from '$lib/planner/positioning';
	import type {
		AnchorResolution,
		CommentEditor,
		InlineMark,
		PlanComment,
		PlanCommentAnchor,
		RenderedBlock,
		SelectionPayload
	} from '$lib/planner/types';

	let {
		projectId,
		planId,
		revisionNumber
	}: { projectId: string; planId: string; revisionNumber: number } = $props();

	let statusBusy = $state(false);
	let metadata = $state<Plan | null>(null);
	let plan = $state<Revision | null>(null);
	let history = $state<components['schemas']['PlanRevision'][]>([]);
	let allComments = $state<PlanComment[]>([]);
	let commentEditors = $state<Record<string, CommentEditor>>({});
	let dispositions = $state<Disposition[]>([]);
	let decisions = $state<components['schemas']['PlanReviewEvent'][]>([]);
	let commentHistory = $state<components['schemas']['PlanCommentVersion'][]>([]);
	let captured = $state<ReturnType<typeof reviewContext> | null>(null);
	let stale = $state(false);
	let editBase = $state(0);
	let comparedBase = $state<Revision | null>(null);
	let editInitialized = false;
	let saveAttempt: { content: string; expected_revision: number; key: string } | null = null;
	let dispositionId = $state<string | null>(null);
	let dispositionKind = $state<'addressed' | 'deferred'>('addressed');
	let dispositionReason = $state('');
	const terminal = $derived(
		plan?.review_status === 'approved' || plan?.review_status === 'rejected'
	);
	const readOnly = $derived(metadata?.lifecycle === 'archived');
	const canDecide = $derived(
		!readOnly &&
			!stale &&
			!statusBusy &&
			metadata?.head_revision === revisionNumber &&
			plan?.review_status === 'in_review'
	);
	const priorComments = $derived(allComments.filter((c) => c.revision !== revisionNumber));
	const comments = $derived(
		allComments
			.filter((c) => c.revision === revisionNumber)
			.map((c) => ({
				...c,
				resolved_at:
					feedbackDisposition(c, dispositions, revisionNumber)?.disposition === 'addressed'
						? 'addressed'
						: undefined
			}))
	);
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
			if (!c.anchor || c.deleted_at) return [];
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

	const unresolvedCount = $derived(
		allComments.filter(
			(c) => feedbackDisposition(c, dispositions, revisionNumber)?.disposition !== 'addressed'
		).length
	);

	$effect(() => {
		if (planId && projectId && revisionNumber) loadData();
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
		try {
			const before = await getMetadata(projectId, planId);
			const [content, discussion, revisions, assessments, events] = await Promise.all([
				getRevision(projectId, planId, revisionNumber),
				allPages((offset) => listComments(projectId, planId, revisionNumber, offset)),
				allPages((offset) => listRevisions(projectId, planId, offset)),
				allPages((offset) => listDispositions(projectId, planId, revisionNumber, offset)),
				allPages((offset) => listDecisions(projectId, planId, revisionNumber, offset))
			]);
			const after = await getMetadata(projectId, planId);
			metadata = after;
			plan = content;
			for (const comment of discussion) {
				if (!commentEditors[comment.id])
					commentEditors[comment.id] = { editing: false, content: '', version: comment.version };
			}
			allComments = discussion;
			history = revisions;
			dispositions = assessments;
			decisions = events;
			captured = reviewContext(before, content);
			stale =
				before.version !== after.version || before.feedback_version !== after.feedback_version;
			if (stale) actionError = 'Plan changed while loading. Refresh review before deciding.';
			error = null;
			if (!stale) actionError = null;
		} catch (err) {
			const message = err instanceof Error ? err.message : 'Failed to load plan';
			if (plan) {
				actionError = message;
				stale = true;
			} else error = message;
		} finally {
			loading = false;
		}
	}

	async function failed(err: unknown) {
		actionError = err instanceof Error ? err.message : 'Request failed';
		stale = true;
		// Refresh server state without replacing editor/composer drafts or retrying a write.
		try {
			const [latest, discussion, assessments] = await Promise.all([
				getMetadata(projectId, planId),
				allPages((offset) => listComments(projectId, planId, revisionNumber, offset)),
				allPages((offset) => listDispositions(projectId, planId, revisionNumber, offset))
			]);
			metadata = latest;
			for (const comment of discussion) {
				if (!commentEditors[comment.id])
					commentEditors[comment.id] = { editing: false, content: '', version: comment.version };
			}
			allComments = discussion;
			dispositions = assessments;
		} catch {
			actionError += '. Could not refresh server state; use Refresh review.';
		}
	}

	async function recordDisposition() {
		const comment = allComments.find((c) => c.id === dispositionId);
		if (!comment || !metadata || !dispositionReason.trim() || statusBusy) return;
		statusBusy = true;
		try {
			await addDisposition(projectId, planId, revisionNumber, {
				comment_id: comment.id,
				expected_comment_version: comment.version,
				expected_feedback_version: metadata.feedback_version,
				disposition: dispositionKind,
				reason: dispositionReason
			});
			dispositionId = null;
			dispositionReason = '';
			await loadData();
		} catch (err) {
			await failed(err);
		} finally {
			statusBusy = false;
		}
	}

	async function lifecycle() {
		if (!metadata || statusBusy) return;
		statusBusy = true;
		try {
			await updateMetadata(projectId, planId, {
				expected_version: metadata.version,
				lifecycle: readOnly ? 'active' : 'archived'
			});
			await loadData();
		} catch (err) {
			await failed(err);
		} finally {
			statusBusy = false;
		}
	}

	async function showCommentHistory(comment: PlanComment) {
		if (comment.revision === null) return;
		try {
			commentHistory = await allPages((offset) =>
				listCommentVersions(projectId, planId, comment.revision!, comment.id, offset)
			);
		} catch (err) {
			await failed(err);
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
		if (!selection || readOnly) return;
		try {
			await addComment(projectId, planId, revisionNumber, {
				content: body,
				anchor: selectionToAnchor(selection)
			});
			composing = false;
			selection = null;
			window.getSelection()?.removeAllRanges();
			await loadData();
		} catch (err) {
			await failed(err);
		}
	}
	async function submitOverall(body: string) {
		if (!body.trim() || readOnly) return;
		try {
			await addComment(projectId, planId, revisionNumber, { content: body.trim() });
			overallFeedback = '';
			await loadData();
		} catch (err) {
			await failed(err);
		}
	}
	async function saveContent(
		id: string,
		content: string,
		expectedVersion: number
	): Promise<boolean> {
		const comment = comments.find((c) => c.id === id);
		if (!comment || readOnly) return false;
		try {
			await mutateComment(projectId, planId, revisionNumber, id, {
				content,
				expected_version: expectedVersion
			});
			await loadData();
			return true;
		} catch (err) {
			await failed(err);
			return false;
		}
	}
	async function toggleResolve(id: string) {
		const c = comments.find((c) => c.id === id);
		if (!c || readOnly) return;
		if (!c.resolved_at) {
			if (terminal || metadata?.head_revision !== revisionNumber) {
				actionError = 'Assess feedback on the current undecided revision.';
				return;
			}
			dispositionId = id;
			return;
		}
		try {
			await mutateComment(projectId, planId, revisionNumber, id, {
				reopen: true,
				expected_version: c.version
			});
			await loadData();
		} catch (err) {
			await failed(err);
		}
	}
	async function removeComment(id: string) {
		const c = comments.find((c) => c.id === id);
		if (!c || readOnly) return;
		try {
			await mutateComment(
				projectId,
				planId,
				revisionNumber,
				id,
				{ expected_version: c.version },
				true
			);
			await loadData();
		} catch (err) {
			await failed(err);
		}
	}

	// Re-anchor flow: card button arms it; the NEXT selection becomes the new anchor.
	async function handleSelection(sel: SelectionPayload | null) {
		// While the composer is open its textarea holds focus, which collapses the
		// document selection — PlanRenderer then reports `null` and would pull the
		// draft's anchor (and the popover with it) out from under the user. The
		// composer owns the selection until it closes.
		if (composing || readOnly) return;
		selection = sel;
		if (!sel) return;
		if (reanchoringId) {
			const id = reanchoringId;
			try {
				const comment = comments.find((c) => c.id === id)!;
				await mutateComment(projectId, planId, revisionNumber, id, {
					anchor: selectionToAnchor(sel),
					expected_version: comment.version
				});
				await loadData();
				reanchoringId = null;
				selection = null;
				window.getSelection()?.removeAllRanges();
				showToast('Highlight updated', 3000);
			} catch (err) {
				await failed(err);
			}
		}
	}

	async function compareCurrentHead() {
		if (statusBusy) return;
		statusBusy = true;
		comparedBase = null;
		try {
			const latest = await getMetadata(projectId, planId);
			comparedBase = await getRevision(projectId, planId, latest.head_revision);
		} catch (err) {
			await failed(err);
		} finally {
			statusBusy = false;
		}
	}

	function useComparedBase() {
		if (!comparedBase || readOnly || statusBusy) return;
		// Explicit editor reconciliation never changes the selected review or its preconditions.
		editBase = comparedBase.revision;
		saveAttempt = null;
	}

	async function handleSaveEdit() {
		if (!plan || statusBusy || readOnly) return;
		statusBusy = true;
		if (
			!saveAttempt ||
			saveAttempt.content !== editContent ||
			saveAttempt.expected_revision !== editBase
		)
			saveAttempt = { content: editContent, expected_revision: editBase, key: crypto.randomUUID() };
		try {
			const result = await saveRevision(
				projectId,
				planId,
				{ content: saveAttempt.content, expected_revision: saveAttempt.expected_revision },
				saveAttempt.key
			);
			await goto(revisionURL(projectId, planId, result.revision.revision));
		} catch (err) {
			await failed(err);
		} finally {
			statusBusy = false;
		}
	}
	async function setStatus(status: components['schemas']['PlanReviewRequest']['status']) {
		if (!captured || statusBusy || stale || readOnly) return;
		statusBusy = true;
		try {
			await decideRevision(projectId, planId, revisionNumber, { ...captured, status });
			await loadData();
		} catch (err) {
			await failed(err);
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
		if (!editInitialized) {
			editInitialized = true;
			editContent = plan?.content ?? '';
			editBase = revisionNumber;
		}
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
					{metadata?.title}
				</h1>
			</div>
			<div class="flex items-center gap-2 shrink-0">
				<span class="px-3 py-1 rounded-full text-xs font-medium {statusColor(plan.review_status)}">
					{plan.review_status}
				</span>
				<button class="btn-approve" onclick={() => setStatus('approved')} disabled={!canDecide}
					>Approve</button
				>
				<button
					class="btn-request"
					onclick={() => setStatus('changes_requested')}
					disabled={!canDecide || unresolvedCount === 0}
					title={unresolvedCount === 0 ? 'Add at least one comment first' : undefined}
				>
					Request changes
				</button>
				<button class="btn-reject" onclick={() => setStatus('rejected')} disabled={!canDecide}
					>Reject</button
				>
			</div>
		</div>

		<section
			data-testid="revision-metadata"
			class="card p-4 space-y-2"
			aria-label="Server metadata"
		>
			<p>Revision {revisionNumber} · {plan.review_status} · {metadata?.lifecycle}</p>
			<p class="break-all font-mono text-xs">
				SHA-256 {plan.content_sha256} · {plan.content_bytes} bytes
			</p>
			<p>Server metadata is authoritative. Uploaded frontmatter remains document content.</p>
			{#if metadata && metadata.head_revision !== revisionNumber}
				<p>
					Superseded by revision {metadata.head_revision}. This revision and its decision remain
					unchanged.
				</p>
				<a class="underline" href={revisionURL(projectId, planId, metadata.head_revision)}
					>View current head</a
				>
			{/if}
			<div class="flex flex-wrap gap-3">
				<a class="underline" href="/{projectId}/plans">All plans</a>
				<button onclick={loadData} disabled={statusBusy}>Refresh review</button>
				<button onclick={lifecycle} disabled={statusBusy}
					>{readOnly ? 'Restore plan' : 'Archive plan'}</button
				>
				{#if !terminal && plan.review_status !== 'in_review'}
					<button
						onclick={() => setStatus('in_review')}
						disabled={readOnly || stale || statusBusy || metadata?.head_revision !== revisionNumber}
						>Submit for review</button
					>
				{/if}
			</div>
			<nav aria-label="Revision history" class="flex flex-wrap gap-3">
				{#each history as revision (revision.revision)}
					<a class="underline" href={revisionURL(projectId, planId, revision.revision)}
						>Revision {revision.revision}</a
					>
				{/each}
			</nav>
		</section>
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
				disabled={readOnly}
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
								disabled={readOnly || !overallFeedback.trim()}
								class="rounded-md border border-[var(--ink-comment-edge)] bg-[var(--ink-comment-bg)] px-2.5 py-1.5 text-xs font-medium text-[var(--ink-comment)] disabled:cursor-not-allowed disabled:opacity-50"
								onclick={() => submitOverall(overallFeedback)}
							>
								Add
							</button>
						</div>
					</div>

					<CommentRail
						bind:editors={commentEditors}
						entries={railEntries}
						{activeId}
						{showResolved}
						onToggleShowResolved={() => (showResolved = !showResolved)}
						onActivate={(id) => {
							activeId = id;
							scrollMarkIntoView(id);
						}}
						onSaveContent={saveContent}
						{readOnly}
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

		<section class="card p-4 space-y-4" aria-label="Feedback obligations">
			<h2 class="font-semibold">Current and earlier feedback</h2>
			<p>
				Earlier comments retain their original revision and anchors. Deletion does not remove a
				feedback obligation.
			</p>
			{#each allComments as comment (comment.id)}
				{@const assessment = feedbackDisposition(comment, dispositions, revisionNumber)}
				<div class="border-b border-border pb-3" data-feedback-id={comment.id}>
					<p>
						{comment.content}
						{comment.deleted_at ? '(Deleted comment — tombstone retained)' : ''}
					</p>
					{#if comment.revision !== null}<a
							class="underline"
							href={revisionURL(projectId, planId, comment.revision)}
							>Original revision {comment.revision}</a
						>
					{:else}<p>Legacy discussion — original content revision unknown</p>{/if}
					{#if comment.anchor}<blockquote>
							“{comment.anchor.quoted_text}” · lines {comment.anchor.line_start}–{comment.anchor
								.line_end}
						</blockquote>{/if}
					<p>
						{assessment
							? `${assessment.disposition}: ${assessment.reason}`
							: 'Outstanding — reason required before approval'}
					</p>
					<button
						disabled={readOnly || terminal || metadata?.head_revision !== revisionNumber}
						onclick={() => {
							dispositionId = comment.id;
						}}>Assess feedback</button
					>
					{#if comment.revision !== null}<button onclick={() => showCommentHistory(comment)}
							>Comment history</button
						>{/if}
				</div>
			{/each}
			{#if priorComments.length > 0}<p>
					{priorComments.length} earlier or legacy comments; no anchors transferred to this revision.
				</p>{/if}
			{#if dispositionId}
				<div class="space-y-2">
					<label
						>Disposition <select aria-label="Disposition" bind:value={dispositionKind}
							><option value="addressed">Addressed</option><option value="deferred">Deferred</option
							></select
						></label
					>
					<label class="block"
						>Disposition reason<textarea class="input w-full" bind:value={dispositionReason}
						></textarea></label
					>
					<button disabled={statusBusy || !dispositionReason.trim()} onclick={recordDisposition}
						>Record disposition</button
					>
					<button onclick={() => (dispositionId = null)}>Cancel disposition</button>
				</div>
			{/if}
			<details>
				<summary>Disposition history ({dispositions.length})</summary>
				{#each dispositions as d (d.id)}<p>
						{d.comment_id} v{d.comment_version} → revision {d.target_revision}: {d.disposition} — {d.reason}
					</p>{/each}
			</details>
			<details>
				<summary>Review decisions ({decisions.length})</summary>
				{#each decisions as d (d.id)}<p>
						{d.status} · {d.created_at} · feedback version {d.feedback_version}
					</p>
					{#each d.dispositions as reason (reason.id)}<p>
							{reason.comment_id} v{reason.comment_version}: {reason.disposition} — {reason.reason}
						</p>{/each}
				{/each}
			</details>
			{#if commentHistory.length}<section aria-label="Comment history">
					{#each commentHistory as v (v.comment.version)}<p>
							Version {v.comment.version}: {v.comment.content}
							{v.comment.deleted_at ? '(deleted)' : ''}
						</p>
						<pre class="whitespace-pre-wrap">{JSON.stringify(v.comment.anchor)}</pre>{/each}
				</section>{/if}
		</section>
		<!-- Edit Mode: Raw markdown editor -->
		{#if viewMode === 'edit'}
			<div class="card p-4 space-y-3">
				<p>
					Draft save base: revision {editBase}. The selected review remains revision {revisionNumber}.
				</p>
				<button class="btn" disabled={statusBusy || readOnly} onclick={compareCurrentHead}
					>Compare with current head</button
				>
				{#if comparedBase}
					<section
						class="space-y-2 border border-border rounded p-3"
						aria-label="Remote plan revision"
					>
						<h3>Revision {comparedBase.revision} — remote content for comparison</h3>
						<p class="break-all font-mono text-xs">SHA-256 {comparedBase.content_sha256}</p>
						<pre class="whitespace-pre-wrap max-h-96 overflow-auto">{comparedBase.content}</pre>
						<p>
							Reconcile your draft below, then explicitly select this revision as its save base.
							Save creates a new revision and does not approve it.
						</p>
						<button
							class="btn"
							disabled={statusBusy || readOnly || editBase === comparedBase.revision}
							onclick={useComparedBase}>Use revision {comparedBase.revision} as save base</button
						>
					</section>
				{/if}
				<textarea
					aria-label="Plan content"
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
						disabled={statusBusy || readOnly}
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
