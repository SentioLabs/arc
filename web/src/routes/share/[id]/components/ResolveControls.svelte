<script lang="ts">
	import type { CommentState } from '$lib/paste/events';
	import type { ResolutionStatus } from '$lib/paste/types';

	const { commentId: _commentId, currentStatus, onResolve } = $props<{
		commentId: string;
		currentStatus: CommentState['status'];
		onResolve: (status: ResolutionStatus, reply?: string) => Promise<void>;
	}>();

	let showRejectReply = $state(false);
	let rejectReply = $state('');
</script>

<span class="flex gap-1">
	{#if currentStatus !== 'accepted'}
		<button class="text-green-700" onclick={() => onResolve('accepted')}>✓ Accept</button>
	{/if}
	{#if currentStatus !== 'resolved'}
		<button class="text-gray-700" onclick={() => onResolve('resolved')}>— Resolve</button>
	{/if}
	{#if currentStatus !== 'rejected'}
		<button class="text-red-700" onclick={() => (showRejectReply = true)}>✕ Reject</button>
	{/if}
	{#if currentStatus === 'resolved' || currentStatus === 'rejected' || currentStatus === 'accepted'}
		<button onclick={() => onResolve('reopened')}>Reopen</button>
	{/if}
</span>

{#if showRejectReply}
	<div class="mt-2 space-y-1">
		<textarea bind:value={rejectReply} placeholder="Optional reply…" class="w-full" rows="2"
		></textarea>
		<button
			onclick={async () => {
				await onResolve('rejected', rejectReply);
				showRejectReply = false;
				rejectReply = '';
			}}>Confirm reject</button
		>
	</div>
{/if}
