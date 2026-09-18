<script lang="ts">
	import { page } from '$app/stores';
	import RevisionReview from '$lib/planner/RevisionReview.svelte';
	const revision = $derived(Number($page.params.revision));
</script>

{#if Number.isSafeInteger(revision) && revision > 0 && $page.params.projectId && $page.params.planId}
	{#key `${$page.params.projectId}/${$page.params.planId}/${revision}`}
		<RevisionReview
			projectId={$page.params.projectId}
			planId={$page.params.planId}
			revisionNumber={revision}
		/>
	{/key}
{:else}<p role="alert">Invalid revision. Use a positive revision number.</p>{/if}
