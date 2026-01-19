<script lang="ts">
	import type { components } from '$lib/api/types';
	import { issueTypeLabels } from '$lib/utils';

	type IssueType = components['schemas']['IssueType'];

	interface Props {
		type: IssueType;
		showLabel?: boolean;
	}

	let { type, showLabel = true }: Props = $props();

	// Custom SVG icons for each type - more refined than emoji
	const typeConfig: Record<IssueType, { icon: string; color: string }> = {
		bug: {
			icon: `<path d="M12 3C10.5 3 9.27 4.03 9.05 5.5L9 6H15L14.95 5.5C14.73 4.03 13.5 3 12 3ZM7.5 7.5L5.5 9.5L7 11L5 13V15H7V13L9 15V20H15V15L17 13V15H19V13L17 11L18.5 9.5L16.5 7.5H7.5Z"/>`,
			color: 'text-type-bug'
		},
		feature: {
			icon: `<path d="M12 2L14.09 8.26L20 9.27L15.18 13.14L16.18 19.02L12 16.77L7.82 19.02L8.82 13.14L4 9.27L9.91 8.26L12 2Z"/>`,
			color: 'text-type-feature'
		},
		task: {
			icon: `<path d="M4 4H20V6H4V4ZM4 11H20V13H4V11ZM4 18H20V20H4V18ZM2 4V6H3V4H2ZM2 11V13H3V11H2ZM2 18V20H3V18H2ZM21 4V6H22V4H21ZM21 11V13H22V11H21ZM21 18V20H22V18H21Z"/><path d="M6 8H18V9H6V8ZM6 15H18V16H6V15Z" opacity="0.5"/>`,
			color: 'text-type-task'
		},
		epic: {
			icon: `<path d="M12 2L3 7V17L12 22L21 17V7L12 2ZM12 4.5L18 8V16L12 19.5L6 16V8L12 4.5Z"/><path d="M12 8L8 10.5V15.5L12 18L16 15.5V10.5L12 8Z"/>`,
			color: 'text-type-epic'
		},
		chore: {
			icon: `<path d="M22.7 19L13.6 9.9C14.5 7.6 14 4.9 12.1 3C10.1 1 7.1 0.6 4.7 1.7L9 6L6 9L1.6 4.7C0.4 7.1 0.9 10.1 2.9 12.1C4.8 14 7.5 14.5 9.8 13.6L18.9 22.7C19.3 23.1 19.9 23.1 20.3 22.7L22.6 20.4C23.1 20 23.1 19.3 22.7 19Z"/>`,
			color: 'text-type-chore'
		}
	};

	const config = $derived(typeConfig[type]);
</script>

<span class="inline-flex items-center gap-1.5" title={issueTypeLabels[type]}>
	<svg
		viewBox="0 0 24 24"
		fill="currentColor"
		class="w-4 h-4 {config.color}"
		aria-hidden="true"
	>
		{@html config.icon}
	</svg>
	{#if showLabel}
		<span class="text-xs font-medium text-text-secondary">
			{issueTypeLabels[type]}
		</span>
	{/if}
</span>
