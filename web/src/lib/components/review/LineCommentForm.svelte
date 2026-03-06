<script lang="ts">
	interface Props {
		comment: string;
		onSave: (text: string) => void;
		onCancel: () => void;
	}

	let { comment, onSave, onCancel }: Props = $props();

	// eslint-disable-next-line svelte/valid-compile -- intentional: form is freshly mounted each time
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

<tr class="animate-fade-in">
	<td colspan="3" class="p-0">
		<div class="border-t border-b border-primary-600/30 bg-surface-900/80 flex">
			<div class="w-1 bg-primary-600 shrink-0"></div>
			<div class="flex-1 px-3 py-2 space-y-2">
				<textarea
					class="input w-full min-h-[4rem] font-mono text-sm resize-y bg-surface-800"
					placeholder="Add a line comment..."
					bind:value={draft}
					bind:this={textareaEl}
					onkeydown={(e) => {
						if (e.key === 'Escape') onCancel();
						if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) handleSave();
					}}
				></textarea>
				<div class="flex items-center justify-between">
					<span class="text-xs text-text-muted">
						<kbd class="px-1 py-0.5 rounded bg-surface-700 text-text-muted font-mono text-[0.65rem]">⌘↵</kbd> to save
					</span>
					<div class="flex items-center gap-2">
						<button type="button" class="btn btn-ghost btn-sm" onclick={onCancel}>
							Cancel
						</button>
						<button type="button" class="btn btn-primary btn-sm" onclick={handleSave} disabled={!draft.trim()}>
							Save
						</button>
					</div>
				</div>
			</div>
		</div>
	</td>
</tr>
