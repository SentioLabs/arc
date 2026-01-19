<script lang="ts">
	import type { components } from '$lib/api/types';

	type Workspace = components['schemas']['Workspace'];

	interface Props {
		workspace?: Workspace;
		title?: string;
		showSearch?: boolean;
	}

	let { workspace, title, showSearch = true }: Props = $props();

	let searchQuery = $state('');
	let searchFocused = $state(false);

	function handleSearch(e: Event) {
		e.preventDefault();
		if (searchQuery.trim() && workspace) {
			const params = new URLSearchParams({ q: searchQuery.trim() });
			window.location.href = `/${workspace.id}/issues?${params.toString()}`;
		}
	}
</script>

<header class="sticky top-0 z-10 bg-surface-900/95 backdrop-blur border-b border-border">
	<div class="flex items-center justify-between gap-4 px-6 h-14">
		<!-- Left: Title/Breadcrumb -->
		<div class="flex items-center gap-3 min-w-0">
			{#if workspace}
				<nav class="flex items-center gap-2 text-sm">
					<a href="/{workspace.id}" class="text-text-muted hover:text-text-primary transition-colors">
						{workspace.name}
					</a>
					{#if title}
						<span class="text-text-muted">/</span>
						<span class="text-text-primary font-medium truncate">{title}</span>
					{/if}
				</nav>
			{:else if title}
				<h1 class="text-lg font-semibold text-text-primary">{title}</h1>
			{/if}
		</div>

		<!-- Right: Search & Actions -->
		<div class="flex items-center gap-3">
			{#if showSearch && workspace}
				<form onsubmit={handleSearch} class="relative">
					<div
						class="relative transition-all duration-200 {searchFocused
							? 'w-72'
							: 'w-56'}"
					>
						<svg
							class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-muted"
							viewBox="0 0 24 24"
							fill="currentColor"
						>
							<path
								d="M15.5 14h-.79l-.28-.27C15.41 12.59 16 11.11 16 9.5 16 5.91 13.09 3 9.5 3S3 5.91 3 9.5 5.91 16 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z"
							/>
						</svg>
						<input
							type="text"
							placeholder="Search issues..."
							bind:value={searchQuery}
							onfocus={() => (searchFocused = true)}
							onblur={() => (searchFocused = false)}
							class="w-full input pl-9 pr-3 py-1.5 text-sm"
						/>
						{#if searchQuery}
							<button
								type="button"
								onclick={() => (searchQuery = '')}
								class="absolute right-3 top-1/2 -translate-y-1/2 text-text-muted hover:text-text-primary transition-colors"
								aria-label="Clear search"
							>
								<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
									<path
										d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"
									/>
								</svg>
							</button>
						{/if}
					</div>
				</form>
			{/if}

			<!-- Quick actions -->
			<button
				type="button"
				class="btn btn-ghost p-2"
				title="Keyboard shortcuts"
			>
				<svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
					<path
						d="M20 5H4c-1.1 0-1.99.9-1.99 2L2 17c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V7c0-1.1-.9-2-2-2zm-9 3h2v2h-2V8zm0 3h2v2h-2v-2zM8 8h2v2H8V8zm0 3h2v2H8v-2zm-1 2H5v-2h2v2zm0-3H5V8h2v2zm9 7H8v-2h8v2zm0-4h-2v-2h2v2zm0-3h-2V8h2v2zm3 3h-2v-2h2v2zm0-3h-2V8h2v2z"
					/>
				</svg>
			</button>
		</div>
	</div>
</header>
