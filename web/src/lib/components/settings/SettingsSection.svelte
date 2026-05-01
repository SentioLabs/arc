<script lang="ts">
	import type { Snippet } from 'svelte';
	let { title, warn, children }: { title: string; warn?: string; children: Snippet } = $props();
</script>

<section class="card p-6 space-y-4">
	<header>
		<h2 class="text-base font-semibold text-text-primary">{title}</h2>
		{#if warn}
			<p class="text-sm text-amber-400 mt-1">⚠ {warn}</p>
		{/if}
	</header>
	<div class="space-y-4">
		{@render children()}
	</div>
</section>
