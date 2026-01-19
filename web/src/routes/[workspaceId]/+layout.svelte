<script lang="ts">
	import { page } from '$app/stores';
	import { getContext } from 'svelte';
	import type { Writable } from 'svelte/store';
	import type { Workspace } from '$lib/api';
	import { goto } from '$app/navigation';

	// Get workspaces from root layout context
	const workspaces = getContext<Writable<Workspace[]>>('workspaces');

	// Current workspace from URL and context
	const workspaceId = $derived($page.params.workspaceId);
	const workspace = $derived($workspaces.find((ws) => ws.id === workspaceId));

	// Redirect to home if workspace not found
	$effect(() => {
		if ($workspaces.length > 0 && !workspace) {
			goto('/');
		}
	});

	let { children } = $props();
</script>

<svelte:head>
	<title>{workspace?.name ?? 'Loading...'} - Arc</title>
</svelte:head>

{#if workspace}
	{@render children()}
{:else}
	<div class="flex-1 flex items-center justify-center">
		<div class="text-text-muted animate-pulse">Loading workspace...</div>
	</div>
{/if}
