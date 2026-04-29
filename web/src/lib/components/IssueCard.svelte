<script lang="ts">
	import type { components } from '$lib/api/types';
	import { formatRelativeTime } from '$lib/utils';
	import { stripMarkdown } from '$lib/markdown';
	import StatusBadge from './StatusBadge.svelte';
	import PriorityBadge from './PriorityBadge.svelte';
	import TypeBadge from './TypeBadge.svelte';
	import InlineSelect from './InlineSelect.svelte';
	import CopyIdButton from './CopyIdButton.svelte';

	type Issue = components['schemas']['Issue'];
	type Label = components['schemas']['Label'];

	interface Props {
		issue: Issue;
		href?: string;
		compact?: boolean;
		labelMap?: Map<string, Label>;
		onStatusChange?: (issueId: string, newStatus: string) => Promise<void>;
	}

	let { issue, href, compact = false, labelMap, onStatusChange }: Props = $props();

	let cardHovered = $state(false);

	const statusOptions = [
		{ value: 'open', label: 'Open' },
		{ value: 'in_progress', label: 'In Progress' },
		{ value: 'blocked', label: 'Blocked' },
		{ value: 'deferred', label: 'Deferred' },
		{ value: 'closed', label: 'Closed' }
	];
</script>

{#if href}
	<a
		{href}
		class="group block card hover:border-border-focus/50 transition-all duration-200 {compact
			? 'p-3'
			: 'p-4'}"
		onmouseenter={() => (cardHovered = true)}
		onmouseleave={() => (cardHovered = false)}
	>
		{@render cardContent()}
	</a>
{:else}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="card {compact ? 'p-3' : 'p-4'}"
		onmouseenter={() => (cardHovered = true)}
		onmouseleave={() => (cardHovered = false)}
	>
		{@render cardContent()}
	</div>
{/if}

{#snippet cardContent()}
	<div class="space-y-3">
		<!-- Header row: ID, Type, Priority -->
		<div class="flex items-center justify-between gap-3">
			<div class="flex items-center gap-3">
				<!-- Issue ID + copy button -->
				<span class="inline-flex items-center gap-1">
					<span
						class="font-mono text-xs text-text-muted tracking-tight group-hover:text-primary-400 transition-colors"
					>
						{issue.id}
					</span>
					<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
					<span
						role="presentation"
						onclick={(e) => {
							e.preventDefault();
							e.stopPropagation();
						}}
					>
						<CopyIdButton value={issue.id} reveal="hover" groupHovered={cardHovered} />
					</span>
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
				{stripMarkdown(issue.description)}
			</p>
		{/if}

		<!-- Footer: Status, Labels, Meta -->
		<div class="flex items-center justify-between gap-3 pt-1">
			<div class="flex items-center gap-2 flex-wrap">
				{#if onStatusChange}
					<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
					<div
						role="presentation"
						onclick={(e) => {
							e.preventDefault();
							e.stopPropagation();
						}}
					>
						<InlineSelect
							value={issue.status}
							options={statusOptions}
							onSave={(v) => onStatusChange(issue.id, v)}
						>
							<StatusBadge status={issue.status} size="sm" />
						</InlineSelect>
					</div>
				{:else}
					<StatusBadge status={issue.status} size="sm" />
				{/if}

				<!-- Labels -->
				{#if issue.labels && issue.labels.length > 0}
					<div class="flex items-center gap-1">
						{#each issue.labels.slice(0, 3) as label (label)}
							{@const color = labelMap?.get(label)?.color}
							<span
								class="px-1.5 py-0.5 text-[10px] font-medium rounded border"
								style={color
									? `background-color: ${color}20; color: ${color}; border-color: ${color}40`
									: ''}
								class:bg-surface-600={!color}
								class:text-text-secondary={!color}
								class:border-transparent={!color}
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
				<span title={new Date(issue.updated_at).toLocaleString()}>
					{formatRelativeTime(issue.updated_at)}
				</span>
			</div>
		</div>
	</div>
{/snippet}
