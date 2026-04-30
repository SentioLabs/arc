<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { setReviewerName } from '$lib/paste/identity';

	const {
		onSave,
		initialName = ''
	}: { onSave: (name: string) => void; initialName?: string } = $props();
	// Seed inside onMount so we don't snapshot the prop at the
	// reactive-graph top level (svelte/state_referenced_locally). The
	// modal is destroyed/recreated by `{#if showNamePrompt}`, so reading
	// `initialName` once on mount captures the correct value each open.
	let name = $state('');
	let input: HTMLInputElement | undefined = $state();

	function save() {
		const trimmed = name.trim();
		if (!trimmed) return;
		setReviewerName(trimmed);
		onSave(trimmed);
	}

	function handleKey(e: KeyboardEvent) {
		if (e.key === 'Enter' && name.trim()) {
			e.preventDefault();
			save();
		}
	}

	onMount(async () => {
		name = initialName;
		await tick();
		input?.focus();
		input?.select();
	});
</script>

<div
	class="ui-sans fixed inset-0 z-[200] flex items-center justify-center bg-[oklch(0_0_0_/_0.65)] backdrop-blur-sm"
	role="dialog"
	aria-modal="true"
>
	<div
		class="anno-card w-[400px] p-6"
		style="animation: anno-card-in 250ms cubic-bezier(0.16, 1, 0.3, 1);"
	>
		<div class="mb-1 text-[10px] uppercase tracking-[0.12em] text-[var(--ink-text-faint)]">
			Sign your review
		</div>
		<h2 class="mb-2 text-lg font-semibold text-[var(--ink-text)]">What's your name?</h2>
		<p class="mb-4 text-sm text-[var(--ink-text-muted)]">
			Shown alongside your annotations. Stored in this browser only — never sent in plaintext to the
			server.
		</p>
		<input
			bind:this={input}
			bind:value={name}
			onkeydown={handleKey}
			placeholder="Your name"
			class="mb-4 w-full rounded-md border border-[var(--ink-rule)] bg-[var(--ink-paper)] px-3 py-2 text-sm text-[var(--ink-text)] focus:border-[var(--ink-comment-edge)] focus:outline-none"
		/>
		<button
			type="button"
			onclick={save}
			disabled={!name.trim()}
			class="w-full rounded-md border border-[var(--ink-comment-edge)] bg-[var(--ink-comment-bg)] px-3 py-2 text-sm font-medium text-[var(--ink-comment)] transition-colors hover:bg-[oklch(from_var(--ink-comment)_l_c_h_/_0.24)] disabled:cursor-not-allowed disabled:opacity-50"
		>
			Continue
		</button>
	</div>
</div>
