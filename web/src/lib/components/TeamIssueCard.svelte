<script lang="ts">
	import type { components } from '$lib/api/types';
	import StatusBadge from './StatusBadge.svelte';
	import PriorityBadge from './PriorityBadge.svelte';
	import TypeBadge from './TypeBadge.svelte';

	type TeamContextIssue = components['schemas']['TeamContextIssue'];
	type Status = components['schemas']['Status'];
	type IssueType = components['schemas']['IssueType'];

	interface Props {
		issue: TeamContextIssue;
		workspaceId: string;
	}

	let { issue, workspaceId }: Props = $props();

	const deps = $derived(issue.deps ?? []);
</script>

<a
	href="/{workspaceId}/issues/{issue.id}"
	class="group block card p-3 hover:border-border-focus/50 transition-all duration-200"
>
	<!-- Header: ID, Type, Priority -->
	<div class="flex items-center justify-between gap-2 mb-1.5">
		<div class="flex items-center gap-2">
			<span
				class="font-mono text-[11px] text-text-muted group-hover:text-primary-400 transition-colors"
			>
				{issue.id}
			</span>
			<TypeBadge type={issue.type as IssueType} showLabel={false} />
		</div>
		<PriorityBadge priority={issue.priority} showLabel={false} />
	</div>

	<!-- Title -->
	<h4
		class="text-sm font-medium text-text-primary leading-snug line-clamp-2 mb-2 group-hover:text-white transition-colors"
	>
		{issue.title}
	</h4>

	<!-- Footer: Status + Deps -->
	<div class="flex items-center justify-between gap-2">
		<StatusBadge status={issue.status as Status} size="sm" />

		{#if deps.length > 0}
			<div class="flex items-center gap-1">
				<svg class="w-3 h-3 text-text-muted" viewBox="0 0 24 24" fill="currentColor">
					<path
						d="M17 7h-4v2h4c1.65 0 3 1.35 3 3s-1.35 3-3 3h-4v2h4c2.76 0 5-2.24 5-5s-2.24-5-5-5zm-6 8H7c-1.65 0-3-1.35-3-3s1.35-3 3-3h4V7H7c-2.76 0-5 2.24-5 5s2.24 5 5 5h4v-2zm-3-4h8v2H8v-2z"
					/>
				</svg>
				<span class="text-[10px] text-text-muted font-mono">
					{deps.length}
				</span>
			</div>
		{/if}
	</div>
</a>
