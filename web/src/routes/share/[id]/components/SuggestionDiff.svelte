<script lang="ts">
	const { original, suggested } = $props<{ original: string; suggested: string }>();
</script>

<div class="text-sm font-mono">
	{#if original}
		<pre class="bg-red-50 line-through p-1">{original}</pre>
	{/if}
	<pre class="bg-green-50 p-1">{suggested}</pre>
</div>
