<script lang="ts">
	interface Props {
		comment: string;
		onSave: (text: string) => void;
		onCancel: () => void;
	}

	let { comment, onSave, onCancel }: Props = $props();

	let draft = $state(comment);
	let textareaEl = $state<HTMLTextAreaElement | null>(null);

	$effect(() => {
		if (textareaEl) {
			textareaEl.focus();
		}
	});

	function handleSave() {
		onSave(draft);
	}
</script>

<tr>
	<td colspan="3" class="px-3 py-2 bg-surface-800 border-t border-b border-border">
		<div class="space-y-2">
			<textarea
				class="input w-full min-h-[4rem] font-mono text-sm resize-y"
				placeholder="Add a line comment..."
				bind:value={draft}
				bind:this={textareaEl}
			></textarea>
			<div class="flex items-center gap-2 justify-end">
				<button type="button" class="btn btn-ghost btn-sm" onclick={onCancel}>
					Cancel
				</button>
				<button type="button" class="btn btn-primary btn-sm" onclick={handleSave}>
					Save
				</button>
			</div>
		</div>
	</td>
</tr>
