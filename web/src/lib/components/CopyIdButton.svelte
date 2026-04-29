<script lang="ts">
	interface Props {
		value: string;
		/** 'hover' fades in on parent hover (for cards), 'visible' is always shown (for detail page) */
		reveal?: 'hover' | 'visible';
		/** Whether the parent group is hovered (pass from card's group hover state) */
		groupHovered?: boolean;
	}

	let { value, reveal = 'hover', groupHovered = false }: Props = $props();

	let copied = $state(false);
	let hovered = $state(false);
	let timeoutId: ReturnType<typeof setTimeout> | undefined;

	const opacity = $derived(
		copied ? 1 : hovered ? 1 : reveal === 'visible' ? 0.5 : groupHovered ? 0.5 : 0
	);

	function copyToClipboard(text: string) {
		try {
			navigator.clipboard?.writeText(text);
		} catch {
			// Fallback for non-secure contexts (e.g. http:// on non-localhost)
			const textarea = document.createElement('textarea');
			textarea.value = text;
			textarea.style.position = 'fixed';
			textarea.style.opacity = '0';
			document.body.appendChild(textarea);
			textarea.select();
			document.execCommand('copy');
			document.body.removeChild(textarea);
		}
	}

	function handleClick(e: MouseEvent) {
		e.preventDefault();
		e.stopPropagation();
		copyToClipboard(value);
		copied = true;
		clearTimeout(timeoutId);
		timeoutId = setTimeout(() => {
			copied = false;
		}, 1200);
	}
</script>

<button
	type="button"
	class="copy-id-btn"
	class:copied
	style="opacity: {opacity};"
	onmouseenter={() => (hovered = true)}
	onmouseleave={() => (hovered = false)}
	onclick={handleClick}
	aria-label="Copy issue ID {value}"
>
	{#if copied}
		<svg class="copy-id-icon" viewBox="0 0 24 24" fill="currentColor">
			<path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z" />
		</svg>
	{:else}
		<svg class="copy-id-icon" viewBox="0 0 24 24" fill="currentColor">
			<path
				d="M16 1H4c-1.1 0-2 .9-2 2v14h2V3h12V1zm3 4H8c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h11c1.1 0 2-.9 2-2V7c0-1.1-.9-2-2-2zm0 16H8V7h11v14z"
			/>
		</svg>
	{/if}

	{#if hovered || copied}
		<span class="copy-id-tooltip" class:copy-id-tooltip-copied={copied}>
			{copied ? 'Copied!' : 'Copy ID'}
		</span>
	{/if}
</button>

<style>
	.copy-id-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		background: none;
		border: none;
		cursor: pointer;
		color: var(--color-text-muted);
		padding: 0;
		line-height: 1;
		position: relative;
		transition:
			opacity 150ms ease,
			color 150ms ease;
	}

	.copy-id-btn:hover {
		color: var(--color-primary-400);
	}

	.copy-id-btn:active {
		transform: scale(0.9);
	}

	.copy-id-btn.copied {
		color: var(--color-status-open);
	}

	.copy-id-icon {
		display: block;
		width: 14px;
		height: 14px;
	}

	.copy-id-tooltip {
		position: absolute;
		bottom: calc(100% + 6px);
		left: 50%;
		transform: translateX(-50%);
		font-family: var(--font-sans);
		font-size: 11px;
		font-weight: 500;
		color: var(--color-text-primary);
		background: var(--color-surface-600);
		border: 1px solid var(--color-border);
		border-radius: 4px;
		padding: 3px 8px;
		white-space: nowrap;
		pointer-events: none;
		animation: copy-tooltip-in 150ms ease-out;
		z-index: 10;
	}

	.copy-id-tooltip-copied {
		color: var(--color-status-open);
	}

	@keyframes copy-tooltip-in {
		from {
			opacity: 0;
			transform: translateX(-50%) translateY(2px);
		}
		to {
			opacity: 1;
			transform: translateX(-50%) translateY(0);
		}
	}
</style>
