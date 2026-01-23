<script lang="ts">
	import { priorityLabels } from '$lib/utils';

	interface Props {
		priority: number;
		showLabel?: boolean;
	}

	let { priority, showLabel = true }: Props = $props();

	// Priority visual representation - bars that fill based on urgency
	const priorityConfig: Record<number, { color: string; bars: number; label: string }> = {
		0: { color: 'bg-priority-critical', bars: 4, label: 'P0' },
		1: { color: 'bg-priority-high', bars: 3, label: 'P1' },
		2: { color: 'bg-priority-medium', bars: 2, label: 'P2' },
		3: { color: 'bg-priority-low', bars: 1, label: 'P3' },
		4: { color: 'bg-priority-none', bars: 0, label: 'P4' }
	};

	const config = $derived(priorityConfig[priority] ?? priorityConfig[4]);
</script>

<span
	class="inline-flex items-center gap-1.5"
	title={priorityLabels[priority] ?? 'Unknown Priority'}
>
	<!-- Priority bars visualization -->
	<span class="flex items-end gap-0.5 h-3">
		{#each [1, 2, 3, 4] as bar}
			<span
				class="w-1 rounded-sm transition-all {bar <= config.bars ? config.color : 'bg-surface-600'}"
				style="height: {bar * 25}%"
			></span>
		{/each}
	</span>
	{#if showLabel}
		<span class="text-xs font-mono font-medium text-text-secondary">
			{config.label}
		</span>
	{/if}
</span>
