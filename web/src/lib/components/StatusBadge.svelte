<script lang="ts">
	import type { components } from '$lib/api/types';
	import { statusLabels } from '$lib/utils';

	type Status = components['schemas']['Status'];

	interface Props {
		status: Status;
		size?: 'sm' | 'md';
	}

	let { status, size = 'md' }: Props = $props();

	const statusStyles: Record<Status, string> = {
		open: 'bg-status-open/15 text-status-open border-status-open/30',
		in_progress: 'bg-status-in-progress/15 text-status-in-progress border-status-in-progress/30',
		blocked: 'bg-status-blocked/15 text-status-blocked border-status-blocked/30',
		deferred: 'bg-status-deferred/15 text-status-deferred border-status-deferred/30',
		closed: 'bg-surface-600 text-text-muted border-surface-500'
	};

	const sizeStyles = {
		sm: 'text-[10px] px-1.5 py-0.5',
		md: 'text-xs px-2 py-0.5'
	};
</script>

<span
	class="inline-flex items-center gap-1 font-mono font-semibold uppercase tracking-wider border rounded {statusStyles[
		status
	]} {sizeStyles[size]}"
>
	<!-- Status indicator dot -->
	<span
		class="w-1.5 h-1.5 rounded-full {status === 'in_progress'
			? 'bg-status-in-progress animate-pulse'
			: status === 'blocked'
				? 'bg-status-blocked'
				: status === 'open'
					? 'bg-status-open'
					: status === 'deferred'
						? 'bg-status-deferred'
						: 'bg-surface-400'}"
	></span>
	{statusLabels[status]}
</span>
