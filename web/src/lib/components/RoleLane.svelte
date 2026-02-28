<script lang="ts">
	import type { components } from '$lib/api/types';
	import TeamIssueCard from './TeamIssueCard.svelte';

	type TeamContextIssue = components['schemas']['TeamContextIssue'];

	interface Props {
		role: string;
		issues: TeamContextIssue[];
		workspaceId: string;
		color?: string;
	}

	let { role, issues, workspaceId, color = '#6366f1' }: Props = $props();

	const displayRole = $derived(role.charAt(0).toUpperCase() + role.slice(1));
</script>

<div class="flex flex-col min-w-[280px] max-w-[340px]">
	<!-- Lane header -->
	<div class="flex items-center gap-3 mb-3 pb-2 border-b-2" style="border-color: {color}">
		<div
			class="w-2 h-2 rounded-full flex-shrink-0"
			style="background-color: {color}"
		></div>
		<h3 class="text-sm font-semibold text-text-primary uppercase tracking-wider">
			{displayRole}
		</h3>
		<span
			class="ml-auto px-1.5 py-0.5 text-[10px] font-mono font-semibold rounded bg-surface-600 text-text-secondary"
		>
			{issues.length}
		</span>
	</div>

	<!-- Issue cards -->
	<div class="space-y-2 flex-1">
		{#each issues as issue (issue.id)}
			<TeamIssueCard {issue} {workspaceId} />
		{:else}
			<p class="text-xs text-text-muted py-4 text-center">No issues</p>
		{/each}
	</div>
</div>
