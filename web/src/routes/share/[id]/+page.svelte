<script lang="ts">
	import { onMount } from 'svelte';
	import { PasteClient, b64ToBytes, eventBytes } from '$lib/paste/client';
	import { importKey, encryptJSON, decryptJSON } from '$lib/paste/crypto';
	import { replayEvents, type CommentState } from '$lib/paste/events';
	import { getReviewerName } from '$lib/paste/identity';
	import type {
		PlanPlaintext,
		EventPlaintext,
		CommentEvent,
		CommentType,
		Severity,
		Anchor
	} from '$lib/paste/types';
	import PlanRenderer from './components/PlanRenderer.svelte';
	import AnnotationToolbar from './components/AnnotationToolbar.svelte';
	import CommentCard from './components/CommentCard.svelte';
	import NamePromptModal from './components/NamePromptModal.svelte';

	const { data } = $props<{ data: { id: string } }>();
	let plan = $state<PlanPlaintext | null>(null);
	let comments = $state(new Map<string, CommentState>());
	let key = $state<CryptoKey | null>(null);
	let reviewerName = $state(getReviewerName());
	let showNamePrompt = $state(false);
	let pendingComment = $state<{
		body: string;
		comment_type: CommentType;
		severity?: Severity;
		suggested_text?: string;
		anchor: Anchor;
	} | null>(null);
	let selection = $state<{
		lineStart: number;
		lineEnd: number;
		charStart?: number;
		charEnd?: number;
		quotedText: string;
		headingSlug?: string;
		contextBefore?: string;
		contextAfter?: string;
	} | undefined>(undefined);

	const anchor = $derived(
		selection
			? ({
					line_start: selection.lineStart,
					line_end: selection.lineEnd,
					char_start: selection.charStart,
					char_end: selection.charEnd,
					quoted_text: selection.quotedText,
					heading_slug: selection.headingSlug,
					context_before: selection.contextBefore,
					context_after: selection.contextAfter
				} satisfies Anchor)
			: undefined
	);

	const client = new PasteClient(window.location.origin);

	onMount(async () => {
		const fragment = window.location.hash.replace(/^#/, '');
		const params = new URLSearchParams(fragment);
		const k = params.get('k');
		if (!k) {
			console.error('missing #k=<key> in URL');
			return;
		}
		key = await importKey(k);

		const resp = await client.get(data.id);
		plan = await decryptJSON<PlanPlaintext>(b64ToBytes(resp.plan_blob), b64ToBytes(resp.plan_iv), key);

		const events: EventPlaintext[] = [];
		for (const ev of resp.events) {
			const { blob, iv } = eventBytes(ev);
			try {
				events.push(await decryptJSON<EventPlaintext>(blob, iv, key));
			} catch {
				// skip undecryptable events (corrupt or tampered)
			}
		}
		comments = replayEvents(plan?.author_name, events);
	});

	async function handleSubmitComment(payload: typeof pendingComment) {
		if (!payload || !key || !plan) return;
		if (!reviewerName) {
			pendingComment = payload;
			showNamePrompt = true;
			return;
		}
		const event: CommentEvent = {
			kind: 'comment',
			id: `c-${crypto.randomUUID()}`,
			author_name: reviewerName,
			comment_type: payload.comment_type,
			severity: payload.severity,
			body: payload.body,
			suggested_text: payload.suggested_text,
			anchor: payload.anchor,
			created_at: new Date().toISOString()
		};
		const { blob, iv } = await encryptJSON(event, key);
		await client.appendEvent(data.id, blob, iv);
		const next = new Map(comments);
		next.set(event.id, { event, status: 'open' });
		comments = next;
		pendingComment = null;
	}
</script>

{#if !plan}
	<p class="p-4 text-gray-500">Loading…</p>
{:else}
	<div class="grid grid-cols-[1fr_320px] gap-4 p-4">
		<PlanRenderer
			markdown={plan.markdown}
			onSelection={(a) => {
				selection = a;
			}}
		/>
		<aside>
			<AnnotationToolbar onSubmit={handleSubmitComment} anchor={anchor} />
			<ul class="space-y-2 mt-4">
				{#each [...comments.values()] as state (state.event.id)}
					<CommentCard {state} isAuthor={reviewerName === plan.author_name} />
				{/each}
			</ul>
		</aside>
	</div>
{/if}

{#if showNamePrompt}
	<NamePromptModal
		onSave={(name) => {
			reviewerName = name;
			showNamePrompt = false;
			if (pendingComment) handleSubmitComment(pendingComment);
		}}
	/>
{/if}
