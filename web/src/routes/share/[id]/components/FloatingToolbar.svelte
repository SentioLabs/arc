<script lang="ts">
	import { onMount, onDestroy, tick } from 'svelte';
	import { clampedAnchorLeft } from './positioning.ts';

	export type ToolbarAction = 'praise' | 'comment' | 'delete' | 'suggest' | 'quick-label';

	const {
		anchorRect,
		onAction,
		onDismiss,
		reviewerName,
		onSetName
	}: {
		anchorRect: DOMRect;
		onAction: (a: ToolbarAction) => void;
		onDismiss: () => void;
		reviewerName: string | null;
		onSetName: (name: string) => void;
	} = $props();

	// Inline name capture — only rendered when the reviewer has no name yet.
	// Replaces the legacy modal flow where typing a comment first then being
	// asked for a name led to people answering with their comment text.
	// Here the name input lives in the toolbar itself, sibling to the action
	// icons; actions are visibly inert until the field has content.
	let nameDraft = $state('');
	let nameError = $state(false);
	let nameInput: HTMLInputElement | undefined = $state();
	const needsName = $derived(!reviewerName);
	const nameReady = $derived(nameDraft.trim().length > 0);

	function commitName() {
		const trimmed = nameDraft.trim();
		if (!trimmed) {
			flashRequired();
			return;
		}
		onSetName(trimmed);
	}

	function flashRequired() {
		nameError = true;
		nameInput?.focus();
		setTimeout(() => {
			nameError = false;
		}, 700);
	}

	function handleNameKey(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
			commitName();
		}
	}

	function tryAction(a: ToolbarAction) {
		if (needsName) {
			// Persist what they've typed if it's substantive — they may have
			// just typed a name and reached for an action. Otherwise flash.
			if (nameReady) {
				commitName();
				// Don't auto-perform: let them deliberately click again now
				// that the toolbar has woken up. Avoids accidentally posting
				// "Praise" the moment the name input clears.
				return;
			}
			flashRequired();
			return;
		}
		onAction(a);
	}

	let toolbar: HTMLDivElement | undefined = $state();
	// Measured after the toolbar mounts (its width is content-driven, so we
	// can't know it ahead of time). We seed with a conservative upper bound
	// (220px) so the first paint is already clamped sensibly; the measured
	// value then refines it on the next tick.
	let measuredWidth = $state(220);

	function computePosition(rect: DOMRect, width: number): { top: number; left: number } {
		const TOOLBAR_HEIGHT = 44;
		const GAP = 12;
		const top = rect.top + window.scrollY - TOOLBAR_HEIGHT - GAP;
		const rawLeft = rect.left + window.scrollX + rect.width / 2;
		const left = clampedAnchorLeft(rawLeft, width, window.innerWidth);
		return { top: Math.max(8 + window.scrollY, top), left };
	}

	const position = $derived(computePosition(anchorRect, measuredWidth));

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			e.preventDefault();
			onDismiss();
		}
	}

	function handleDocumentClick(e: MouseEvent) {
		if (!toolbar) return;
		if (toolbar.contains(e.target as Node)) return;
		const sel = window.getSelection();
		if (sel && sel.toString().trim().length > 0) return;
		onDismiss();
	}

	onMount(async () => {
		document.addEventListener('keydown', handleKeydown);
		document.addEventListener('mousedown', handleDocumentClick);
		// Wait one tick so the toolbar has rendered, then measure it. This
		// refines `measuredWidth` from the seed (220) to the true rendered
		// width — typically ~200px depending on the icon set.
		await tick();
		if (toolbar) measuredWidth = toolbar.offsetWidth;
		// Focus the name field on first render so the user can start typing
		// without an extra click. Skipped once a name is already on file.
		if (needsName) nameInput?.focus();
	});

	onDestroy(() => {
		document.removeEventListener('keydown', handleKeydown);
		document.removeEventListener('mousedown', handleDocumentClick);
	});

	type Tone = 'praise' | 'comment' | 'delete' | 'muted';
	const toneClass: Record<Tone, string> = {
		praise: 'text-[var(--ink-praise)] hover:bg-[var(--ink-praise-bg)]',
		comment: 'text-[var(--ink-comment)] hover:bg-[var(--ink-comment-bg)]',
		delete: 'text-[var(--ink-delete)] hover:bg-[var(--ink-delete-bg)]',
		muted: 'text-[var(--ink-text-muted)] hover:bg-[var(--ink-paper)]'
	};

	// While the field is empty, action buttons are functionally locked. Their
	// hover tooltip swaps from the action's normal label to a single directive
	// — the gentle middle tier between "passively dimmed" and "shake on click".
	function actionTitle(active: string): string {
		return needsName ? 'Enter your name first' : active;
	}
</script>

<div
	bind:this={toolbar}
	class="floating-toolbar fixed z-[100] {needsName
		? 'needs-name flex-col items-stretch gap-1 p-1.5'
		: 'flex-row items-center gap-0.5 p-1'} ui-sans flex"
	style="top: {position.top}px; left: {position.left}px; transform: translateX(-50%);"
	role="toolbar"
	aria-label="Annotation actions"
>
	{#if needsName}
		<!-- Name capture row. Renders only when reviewerName is null.
			 Visual: editorial proof-sheet input (baseline rule, not a box)
			 by default; on validation fail, snaps red + brief shake. The
			 dimmed action icons below carry the "locked" signal; their
			 hover tooltip swaps to "Enter your name first" on hover, and
			 the shake fires when a locked action is clicked. -->
		<div class="name-row" class:is-error={nameError}>
			<input
				bind:this={nameInput}
				bind:value={nameDraft}
				onkeydown={handleNameKey}
				type="text"
				class="name-field"
				placeholder="your name…"
				aria-label="Your name (required to post annotations)"
				aria-invalid={nameError}
				autocomplete="off"
				spellcheck="false"
			/>
			<button
				type="button"
				class="name-commit"
				class:is-ready={nameReady}
				onclick={commitName}
				aria-label="Save name"
				title="Press Enter or click to save"
			>
				<svg viewBox="0 0 12 12" width="11" height="11" aria-hidden="true">
					<path
						d="M2 6 H10 M7 3 L10 6 L7 9"
						fill="none"
						stroke="currentColor"
						stroke-width="1.4"
						stroke-linecap="round"
						stroke-linejoin="round"
					/>
				</svg>
			</button>
		</div>
	{/if}

	<div
		class="action-row flex flex-row items-center gap-0.5 {needsName ? 'is-locked' : ''}"
		aria-disabled={needsName}
	>
	<button
		type="button"
		class="rounded-md p-2 transition-colors {toneClass.praise}"
		title={actionTitle('Mark as praise (looks good)')}
		onclick={() => tryAction('praise')}
	>
		<svg
			class="h-4 w-4"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			stroke-width="2"
			aria-hidden="true"
		>
			<path
				stroke-linecap="round"
				stroke-linejoin="round"
				d="M7 11v10H3V11h4zm14-2.5a2 2 0 00-2-2h-5l1.4-4.4a1 1 0 00-1.5-1.1L8 6v15l8.5-1a3 3 0 002.7-2.4l1.7-7.5a2 2 0 00.1-1.6z"
			/>
		</svg>
	</button>

	<button
		type="button"
		class="rounded-md p-2 transition-colors {toneClass.comment}"
		title={actionTitle('Add a comment')}
		onclick={() => tryAction('comment')}
	>
		<svg
			class="h-4 w-4"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			stroke-width="2"
			aria-hidden="true"
		>
			<path
				stroke-linecap="round"
				stroke-linejoin="round"
				d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"
			/>
		</svg>
	</button>

	<div class="mx-0.5 h-5 w-px bg-[var(--ink-rule)]"></div>

	<button
		type="button"
		class="rounded-md p-2 transition-colors {toneClass.delete}"
		title={actionTitle('Propose removing this text')}
		onclick={() => tryAction('delete')}
	>
		<svg
			class="h-4 w-4"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			stroke-width="2"
			aria-hidden="true"
		>
			<path
				stroke-linecap="round"
				stroke-linejoin="round"
				d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
			/>
		</svg>
	</button>

	<button
		type="button"
		class="rounded-md p-2 transition-colors {toneClass.comment}"
		title={actionTitle('Propose replacement text')}
		onclick={() => tryAction('suggest')}
	>
		<svg
			class="h-4 w-4"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			stroke-width="2"
			aria-hidden="true"
		>
			<path
				stroke-linecap="round"
				stroke-linejoin="round"
				d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z"
			/>
		</svg>
	</button>

	<button
		type="button"
		class="rounded-md p-2 transition-colors {toneClass.praise}"
		title={actionTitle('Pick a label')}
		onclick={() => tryAction('quick-label')}
	>
		<svg
			class="h-4 w-4"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			stroke-width="2"
			aria-hidden="true"
		>
			<path stroke-linecap="round" stroke-linejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z" />
		</svg>
	</button>

	<div class="mx-0.5 h-5 w-px bg-[var(--ink-rule)]"></div>

	<button
		type="button"
		class="rounded-md p-2 transition-colors {toneClass.muted}"
		title="Cancel (Esc)"
		onclick={onDismiss}
	>
		<svg
			class="h-4 w-4"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			stroke-width="2"
			aria-hidden="true"
		>
			<path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
		</svg>
	</button>
	</div>

</div>
