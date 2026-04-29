<script lang="ts">
	const { status } = $props<{ status: 'ok' | 'drifted' | 'orphaned' }>();
</script>

{#if status === 'drifted'}
	<span class="px-1.5 py-0.5 rounded bg-yellow-100 text-yellow-800 text-xs">anchor moved</span>
{:else if status === 'orphaned'}
	<span class="px-1.5 py-0.5 rounded bg-red-100 text-red-800 text-xs">content removed</span>
{/if}
