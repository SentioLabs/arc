<script lang="ts">
	interface Props {
		onSubmit: (text: string) => Promise<void>;
	}

	let { onSubmit }: Props = $props();

	let text = $state('');
	let submitting = $state(false);

	const canSubmit = $derived(text.trim().length > 0 && !submitting);

	async function handleSubmit() {
		if (!canSubmit) return;
		submitting = true;
		try {
			await onSubmit(text.trim());
			text = '';
		} catch {
			// Keep text on error so user can retry
		} finally {
			submitting = false;
		}
	}
</script>

<div>
	<textarea
		bind:value={text}
		disabled={submitting}
		class="input w-full min-h-[80px] text-sm"
		placeholder="Add a comment..."
		aria-label="Add a comment"
	></textarea>
	<div class="flex justify-end mt-2">
		<button
			type="button"
			onclick={handleSubmit}
			disabled={!canSubmit}
			class="btn btn-primary text-sm"
		>
			{submitting ? 'Posting...' : 'Post comment'}
		</button>
	</div>
</div>
