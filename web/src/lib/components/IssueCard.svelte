<script lang="ts">
	import type { components } from '$lib/api/types';
	import { formatRelativeTime } from '$lib/utils';
	import StatusBadge from './StatusBadge.svelte';
	import PriorityBadge from './PriorityBadge.svelte';
	import TypeBadge from './TypeBadge.svelte';

	type Issue = components['schemas']['Issue'];

	interface Props {
		issue: Issue;
		href?: string;
		compact?: boolean;
	}

	let { issue, href, compact = false }: Props = $props();
</script>

{#if href}
	<a
		{href}
		class="group block card hover:border-border-focus/50 transition-all duration-200 {compact
			? 'p-3'
			: 'p-4'}"
	>
		{@render cardContent()}
	</a>
{:else}
	<div class="card {compact ? 'p-3' : 'p-4'}">
		{@render cardContent()}
	</div>
{/if}

{#snippet cardContent()}
	<div class="space-y-3">
		<!-- Header row: ID, Type, Priority -->
		<div class="flex items-center justify-between gap-3">
			<div class="flex items-center gap-3">
				<!-- Issue ID - monospace, subtle -->
				<span
					class="font-mono text-xs text-text-muted tracking-tight group-hover:text-primary-400 transition-colors"
				>
					{issue.id}
				</span>
				<TypeBadge type={issue.issue_type} showLabel={false} />
			</div>
			<PriorityBadge priority={issue.priority} showLabel={false} />
		</div>

		<!-- Title -->
		<h3
			class="font-medium text-text-primary leading-snug group-hover:text-white transition-colors {compact
				? 'text-sm line-clamp-1'
				: 'text-base line-clamp-2'}"
		>
			{issue.title}
		</h3>

		<!-- Description preview (if not compact and exists) -->
		{#if !compact && issue.description}
			<p class="text-sm text-text-secondary line-clamp-2 leading-relaxed">
				{issue.description}
			</p>
		{/if}

		<!-- Footer: Status, Labels, Meta -->
		<div class="flex items-center justify-between gap-3 pt-1">
			<div class="flex items-center gap-2 flex-wrap">
				<StatusBadge status={issue.status} size="sm" />

				<!-- Labels -->
				{#if issue.labels && issue.labels.length > 0}
					<div class="flex items-center gap-1">
						{#each issue.labels.slice(0, 3) as label (label)}
							<span
								class="px-1.5 py-0.5 text-[10px] font-medium bg-surface-600 text-text-secondary rounded"
							>
								{label}
							</span>
						{/each}
						{#if issue.labels.length > 3}
							<span class="text-[10px] text-text-muted">
								+{issue.labels.length - 3}
							</span>
						{/if}
					</div>
				{/if}
			</div>

			<!-- Meta info -->
			<div class="flex items-center gap-3 text-xs text-text-muted">
				{#if issue.assignee}
					<span class="flex items-center gap-1" title="Assigned to {issue.assignee}">
						<svg class="w-3 h-3" viewBox="0 0 24 24" fill="currentColor">
							<path
								d="M12 12C14.21 12 16 10.21 16 8C16 5.79 14.21 4 12 4C9.79 4 8 5.79 8 8C8 10.21 9.79 12 12 12ZM12 14C9.33 14 4 15.34 4 18V20H20V18C20 15.34 14.67 14 12 14Z"
							/>
						</svg>
						<span class="max-w-16 truncate">{issue.assignee}</span>
					</span>
				{/if}
				<span title={new Date(issue.updated_at).toLocaleString()}>
					{formatRelativeTime(issue.updated_at)}
				</span>
			</div>
		</div>
	</div>
{/snippet}
