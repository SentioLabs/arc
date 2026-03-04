<script lang="ts">
	interface Props {
		comment: string;
		onCommentChange: (comment: string) => void;
		onSubmit: (decision: 'approve' | 'request_changes') => void;
		submitting: boolean;
	}

	let { comment, onCommentChange, onSubmit, submitting }: Props = $props();

	function handleInput(e: Event) {
		const target = e.target as HTMLTextAreaElement;
		onCommentChange(target.value);
	}
</script>

<div class="space-y-3">
	<!-- Review comment textarea -->
	<textarea
		class="input w-full resize-y"
		rows={4}
		placeholder="Leave an overall review comment..."
		value={comment}
		oninput={handleInput}
	></textarea>

	<!-- Button row -->
	<div class="flex items-center gap-2">
		<button
			type="button"
			class="flex items-center gap-2 px-4 py-2 rounded font-medium text-sm bg-green-600 hover:bg-green-500 text-white transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
			disabled={submitting}
			onclick={() => onSubmit('approve')}
		>
			<!-- Check icon -->
			<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
				<path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z" />
			</svg>
			Approve
		</button>

		<button
			type="button"
			class="flex items-center gap-2 px-4 py-2 rounded font-medium text-sm bg-amber-600 hover:bg-amber-500 text-white transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
			disabled={submitting}
			onclick={() => onSubmit('request_changes')}
		>
			<!-- X icon -->
			<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
				<path
					d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"
				/>
			</svg>
			Request Changes
		</button>
	</div>
</div>
