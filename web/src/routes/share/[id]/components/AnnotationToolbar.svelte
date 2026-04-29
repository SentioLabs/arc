<script lang="ts">
	import LabelPicker from './LabelPicker.svelte';
	import type { CommentType, Severity, Anchor } from '$lib/paste/types';

	const { onSubmit, anchor } = $props<{
		onSubmit: (payload: {
			body: string;
			comment_type: CommentType;
			severity?: Severity;
			suggested_text?: string;
			anchor: Anchor;
		}) => Promise<void>;
		anchor?: Anchor;
	}>();

	let body = $state('');
	let commentType = $state<CommentType>('comment');
	let severity = $state<Severity | undefined>(undefined);
	let suggested = $state('');
	let showSuggested = $state(false);

	async function submit() {
		if (!anchor || !body.trim()) return;
		await onSubmit({
			body: body.trim(),
			comment_type: commentType,
			severity: commentType === 'issue' ? severity : undefined,
			suggested_text: showSuggested ? suggested : undefined,
			anchor
		});
		body = '';
		suggested = '';
		showSuggested = false;
	}
</script>

<div class="space-y-2 p-2 border rounded">
	<textarea bind:value={body} placeholder="Comment…" class="w-full" rows="3"></textarea>
	<LabelPicker bind:value={commentType} />
	{#if commentType === 'issue'}
		<label>
			<input
				type="checkbox"
				checked={severity === 'important'}
				onchange={(e) => (severity = e.currentTarget.checked ? 'important' : 'nit')}
			/>
			Important
		</label>
	{/if}
	<label>
		<input type="checkbox" bind:checked={showSuggested} />
		Suggest replacement text
	</label>
	{#if showSuggested}
		<textarea
			bind:value={suggested}
			placeholder="Proposed replacement…"
			class="w-full"
			rows="3"
		></textarea>
	{/if}
	<button onclick={submit} disabled={!body.trim() || !anchor}>Post</button>
</div>
