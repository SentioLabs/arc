<script lang="ts">
	import { onMount } from 'svelte';

	interface Props {
		open: boolean;
		title: string;
		message?: string;
		items?: string[];
		confirmLabel?: string;
		cancelLabel?: string;
		variant?: 'danger' | 'warning';
		loading?: boolean;
		onconfirm: () => void;
		oncancel: () => void;
	}

	let {
		open = false,
		title,
		message,
		items = [],
		confirmLabel = 'Confirm',
		cancelLabel = 'Cancel',
		variant = 'danger',
		loading = false,
		onconfirm,
		oncancel
	}: Props = $props();

	let dialogRef: HTMLDialogElement | undefined = $state();

	$effect(() => {
		if (!dialogRef) return;
		if (open && !dialogRef.open) {
			dialogRef.showModal();
		} else if (!open && dialogRef.open) {
			dialogRef.close();
		}
	});

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape' && !loading) {
			e.preventDefault();
			oncancel();
		}
	}

	function handleBackdropClick(e: MouseEvent) {
		if (e.target === dialogRef && !loading) {
			oncancel();
		}
	}

	onMount(() => {
		return () => {
			if (dialogRef?.open) dialogRef.close();
		};
	});
</script>

<dialog
	bind:this={dialogRef}
	class="dialog-modal"
	onkeydown={handleKeydown}
	onclick={handleBackdropClick}
>
	{#if open}
		<div class="dialog-content animate-dialog-in">
			<!-- Header with icon -->
			<div class="flex items-start gap-4 mb-6">
				<div
					class="shrink-0 w-11 h-11 rounded-lg flex items-center justify-center {variant ===
					'danger'
						? 'bg-status-blocked/20'
						: 'bg-priority-high/20'}"
				>
					{#if variant === 'danger'}
						<svg class="w-5 h-5 text-status-blocked" viewBox="0 0 24 24" fill="currentColor">
							<path
								d="M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12zM8 9h8v10H8V9zm7.5-5l-1-1h-5l-1 1H5v2h14V4h-3.5z"
							/>
						</svg>
					{:else}
						<svg class="w-5 h-5 text-priority-high" viewBox="0 0 24 24" fill="currentColor">
							<path d="M1 21h22L12 2 1 21zm12-3h-2v-2h2v2zm0-4h-2v-4h2v4z" />
						</svg>
					{/if}
				</div>
				<div class="flex-1 min-w-0">
					<h2 class="text-lg font-semibold text-text-primary">{title}</h2>
					{#if message}
						<p class="text-sm text-text-secondary mt-1">{message}</p>
					{/if}
				</div>
			</div>

			<!-- Items list (if provided) -->
			{#if items.length > 0}
				<div class="mb-6">
					<div class="text-xs font-medium text-text-muted uppercase tracking-wider mb-2">
						{items.length === 1 ? 'Workspace to delete' : `${items.length} workspaces to delete`}
					</div>
					<div
						class="bg-surface-900 border border-border-subtle rounded-md max-h-40 overflow-y-auto"
					>
						{#each items as item, i (i)}
							<div
								class="px-3 py-2 text-sm font-mono text-text-primary border-b border-border-subtle last:border-b-0"
							>
								{item}
							</div>
						{/each}
					</div>
				</div>
			{/if}

			<!-- Warning notice -->
			<div
				class="flex items-center gap-2 p-3 bg-status-blocked/10 border border-status-blocked/20 rounded-md mb-6"
			>
				<svg class="w-4 h-4 text-status-blocked shrink-0" viewBox="0 0 24 24" fill="currentColor">
					<path
						d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z"
					/>
				</svg>
				<span class="text-xs text-status-blocked">This action cannot be undone</span>
			</div>

			<!-- Actions -->
			<div class="flex items-center justify-end gap-3">
				<button class="btn btn-ghost" onclick={oncancel} disabled={loading} type="button">
					{cancelLabel}
				</button>
				<button class="btn btn-danger" onclick={onconfirm} disabled={loading} type="button">
					{#if loading}
						<svg class="w-4 h-4 animate-spin" viewBox="0 0 24 24" fill="none">
							<circle
								class="opacity-25"
								cx="12"
								cy="12"
								r="10"
								stroke="currentColor"
								stroke-width="4"
							></circle>
							<path
								class="opacity-75"
								fill="currentColor"
								d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
							></path>
						</svg>
						Deleting...
					{:else}
						{confirmLabel}
					{/if}
				</button>
			</div>
		</div>
	{/if}
</dialog>
