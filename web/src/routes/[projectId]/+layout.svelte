<script lang="ts">
	import { page } from '$app/stores';
	import { getContext } from 'svelte';
	import type { Writable } from 'svelte/store';
	import type { Workspace as Project } from '$lib/api';
	import { goto } from '$app/navigation';

	// Get projects from root layout context
	const projects = getContext<Writable<Project[]>>('workspaces');

	// Current project from URL and context
	const projectId = $derived($page.params.projectId);
	const project = $derived($projects.find((p) => p.id === projectId));

	// Redirect to home if project not found
	$effect(() => {
		if ($projects.length > 0 && !project) {
			goto('/');
		}
	});

	let { children } = $props();
</script>

<svelte:head>
	<title>{project?.name ?? 'Loading...'} - Arc</title>
</svelte:head>

{#if project}
	{@render children()}
{:else}
	<div class="flex-1 flex items-center justify-center">
		<div class="text-text-muted animate-pulse">Loading project...</div>
	</div>
{/if}
