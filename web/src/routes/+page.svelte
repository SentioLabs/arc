<script lang="ts">
	import { getContext } from 'svelte';
	import type { Writable } from 'svelte/store';
	import type { Workspace } from '$lib/api';

	// Get workspaces from root layout context
	const workspaces = getContext<Writable<Workspace[]>>('workspaces');
</script>

<div class="p-8 max-w-4xl mx-auto animate-fade-in">
	<header class="mb-12">
		<h1 class="text-4xl font-bold text-text-primary mb-3">Workspaces</h1>
		<p class="text-lg text-text-secondary">Select a workspace to view and manage issues</p>
	</header>

	{#if $workspaces.length === 0}
		<div class="card p-12 text-center">
			<div
				class="w-16 h-16 bg-surface-700 rounded-2xl flex items-center justify-center mx-auto mb-4"
			>
				<svg class="w-8 h-8 text-text-muted" viewBox="0 0 24 24" fill="currentColor">
					<path
						d="M10 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2h-8l-2-2z"
					/>
				</svg>
			</div>
			<h2 class="text-xl font-semibold text-text-primary mb-2">No workspaces yet</h2>
			<p class="text-text-secondary mb-6">Create a workspace using the CLI to get started</p>
			<div
				class="bg-surface-800 rounded-lg p-4 font-mono text-sm text-text-secondary text-left inline-block"
			>
				<code>arc init my-project</code>
			</div>
		</div>
	{:else}
		<div class="grid gap-4 sm:grid-cols-2">
			{#each $workspaces as workspace, i (workspace.id)}
				<a
					href="/{workspace.id}"
					class="card p-6 hover:border-border-focus/50 transition-all duration-200 group animate-slide-up"
					style="animation-delay: {i * 50}ms"
				>
					<div class="flex items-start justify-between mb-4">
						<div
							class="w-10 h-10 bg-primary-600/20 rounded-lg flex items-center justify-center group-hover:bg-primary-600/30 transition-colors"
						>
							<span class="font-mono font-bold text-primary-400 text-sm uppercase">
								{workspace.prefix}
							</span>
						</div>
						<svg
							class="w-5 h-5 text-text-muted group-hover:text-primary-400 transition-colors"
							viewBox="0 0 24 24"
							fill="currentColor"
						>
							<path d="M10 6L8.59 7.41 13.17 12l-4.58 4.59L10 18l6-6z" />
						</svg>
					</div>

					<h3
						class="text-lg font-semibold text-text-primary group-hover:text-white transition-colors mb-1"
					>
						{workspace.name}
					</h3>

					{#if workspace.description}
						<p class="text-sm text-text-secondary line-clamp-2 mb-4">
							{workspace.description}
						</p>
					{:else}
						<p class="text-sm text-text-muted mb-4">No description</p>
					{/if}

					{#if workspace.path}
						<div class="flex items-center gap-2 text-xs text-text-muted font-mono">
							<svg class="w-3 h-3" viewBox="0 0 24 24" fill="currentColor">
								<path
									d="M10 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2h-8l-2-2z"
								/>
							</svg>
							<span class="truncate">{workspace.path}</span>
						</div>
					{/if}
				</a>
			{/each}
		</div>
	{/if}
</div>
