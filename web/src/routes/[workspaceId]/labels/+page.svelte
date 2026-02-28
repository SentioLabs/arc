<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	onMount(() => {
		goto('/labels', { replaceState: true });
	});
</script>

<div class="flex-1 p-6 flex items-center justify-center">
	<p class="text-text-muted">Redirecting to global labels...</p>
</div>
