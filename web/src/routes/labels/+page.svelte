<script lang="ts">
	import { listLabels, createLabel, updateLabel, deleteLabel, type Label } from '$lib/api';
	import { ColorPicker, ConfirmDialog } from '$lib/components';

	let labels = $state<Label[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Form state
	let showForm = $state(false);
	let editingLabel = $state<Label | null>(null);
	let formName = $state('');
	let formColor = $state('');
	let formDescription = $state('');
	let saving = $state(false);

	// Delete state
	let deletingLabel = $state<Label | null>(null);
	let deleteLoading = $state(false);

	$effect(() => {
		loadLabels();
	});

	async function loadLabels() {
		loading = true;
		error = null;
		try {
			labels = await listLabels();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load labels';
		} finally {
			loading = false;
		}
	}

	function openCreate() {
		editingLabel = null;
		formName = '';
		formColor = '#3b82f6';
		formDescription = '';
		showForm = true;
	}

	function openEdit(label: Label) {
		editingLabel = label;
		formName = label.name;
		formColor = label.color || '';
		formDescription = label.description || '';
		showForm = true;
	}

	function cancelForm() {
		showForm = false;
		editingLabel = null;
	}

	async function handleSave() {
		if (!formName.trim()) return;
		saving = true;
		try {
			if (editingLabel) {
				await updateLabel(editingLabel.name, formColor, formDescription);
			} else {
				await createLabel(formName.trim(), formColor, formDescription);
			}
			showForm = false;
			editingLabel = null;
			await loadLabels();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to save label';
		} finally {
			saving = false;
		}
	}

	async function handleDelete() {
		if (!deletingLabel) return;
		deleteLoading = true;
		try {
			await deleteLabel(deletingLabel.name);
			deletingLabel = null;
			await loadLabels();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to delete label';
		} finally {
			deleteLoading = false;
		}
	}
</script>

<div class="flex-1 p-6 animate-fade-in">
	<header class="mb-6 flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold text-text-primary mb-1">Labels</h1>
			<p class="text-text-secondary">
				{labels.length} global label{labels.length !== 1 ? 's' : ''}
			</p>
		</div>
		{#if !showForm}
			<button class="btn btn-primary" onclick={openCreate}>
				<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
					<path d="M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6v2z" />
				</svg>
				New Label
			</button>
		{/if}
	</header>

	<!-- Create/Edit form -->
	{#if showForm}
		<div class="card p-6 mb-6 animate-fade-in">
			<h2 class="text-lg font-semibold text-text-primary mb-4">
				{editingLabel ? 'Edit Label' : 'Create Label'}
			</h2>
			<form
				onsubmit={(e) => {
					e.preventDefault();
					handleSave();
				}}
				class="space-y-4"
			>
				<div>
					<label for="label-name" class="block text-sm font-medium text-text-secondary mb-1"
						>Name</label
					>
					<input
						id="label-name"
						type="text"
						bind:value={formName}
						class="input w-full"
						placeholder="e.g. bug, enhancement, urgent"
						disabled={!!editingLabel}
						required
					/>
				</div>

				<div>
					<label for="label-color" class="block text-sm font-medium text-text-secondary mb-1"
						>Color</label
					>
					<ColorPicker value={formColor} onchange={(c) => (formColor = c)} />
				</div>

				<div>
					<label for="label-desc" class="block text-sm font-medium text-text-secondary mb-1"
						>Description</label
					>
					<textarea
						id="label-desc"
						bind:value={formDescription}
						class="input w-full"
						rows="2"
						placeholder="Optional description"
					></textarea>
				</div>

				<div class="flex items-center gap-3 pt-2">
					<button type="submit" class="btn btn-primary" disabled={saving || !formName.trim()}>
						{#if saving}
							Saving...
						{:else}
							{editingLabel ? 'Update' : 'Create'}
						{/if}
					</button>
					<button type="button" class="btn btn-ghost" onclick={cancelForm} disabled={saving}>
						Cancel
					</button>
				</div>
			</form>
		</div>
	{/if}

	{#if loading}
		<div class="flex items-center justify-center py-12">
			<div class="text-text-muted animate-pulse">Loading...</div>
		</div>
	{:else if error}
		<div class="card p-8 text-center">
			<p class="text-status-blocked mb-4">{error}</p>
			<button class="btn btn-primary" onclick={loadLabels}>Retry</button>
		</div>
	{:else if labels.length === 0 && !showForm}
		<div class="card p-12 text-center">
			<div
				class="w-16 h-16 bg-surface-700 rounded-2xl flex items-center justify-center mx-auto mb-4"
			>
				<svg class="w-8 h-8 text-text-muted" viewBox="0 0 24 24" fill="currentColor">
					<path
						d="M17.63 5.84C17.27 5.33 16.67 5 16 5L5 5.01C3.9 5.01 3 5.9 3 7v10c0 1.1.9 1.99 2 1.99L16 19c.67 0 1.27-.33 1.63-.84L22 12l-4.37-6.16z"
					/>
				</svg>
			</div>
			<h2 class="text-xl font-semibold text-text-primary mb-2">No labels yet</h2>
			<p class="text-text-secondary mb-4">Create your first label to start organizing issues</p>
			<button class="btn btn-primary" onclick={openCreate}>
				<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
					<path d="M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6v2z" />
				</svg>
				Create Label
			</button>
		</div>
	{:else}
		<div class="grid sm:grid-cols-2 lg:grid-cols-3 gap-4">
			{#each labels as label (label.name)}
				<div class="card p-4 group">
					<div class="flex items-center justify-between mb-2">
						<div class="flex items-center gap-3">
							<div
								class="w-4 h-4 rounded-full border border-white/10"
								style="background-color: {label.color || '#6b7280'}"
							></div>
							<h3 class="font-medium text-text-primary">{label.name}</h3>
						</div>
						<div
							class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity"
						>
							<button
								type="button"
								class="p-1 rounded hover:bg-surface-600 text-text-muted hover:text-text-primary transition-colors"
								onclick={() => openEdit(label)}
								title="Edit"
							>
								<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
									<path
										d="M3 17.25V21h3.75L17.81 9.94l-3.75-3.75L3 17.25zM20.71 7.04c.39-.39.39-1.02 0-1.41l-2.34-2.34c-.39-.39-1.02-.39-1.41 0l-1.83 1.83 3.75 3.75 1.83-1.83z"
									/>
								</svg>
							</button>
							<button
								type="button"
								class="p-1 rounded hover:bg-surface-600 text-text-muted hover:text-status-blocked transition-colors"
								onclick={() => (deletingLabel = label)}
								title="Delete"
							>
								<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
									<path
										d="M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12zM19 4h-3.5l-1-1h-5l-1 1H5v2h14V4z"
									/>
								</svg>
							</button>
						</div>
					</div>
					{#if label.description}
						<p class="text-sm text-text-secondary">{label.description}</p>
					{:else}
						<p class="text-sm text-text-muted">No description</p>
					{/if}
					{#if label.color}
						<p class="text-xs text-text-muted mt-2 font-mono">{label.color}</p>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</div>

<!-- Delete confirmation -->
<ConfirmDialog
	open={!!deletingLabel}
	title="Delete Label"
	message="Are you sure you want to delete the label '{deletingLabel?.name}'? It will be removed from all issues."
	confirmLabel="Delete"
	loading={deleteLoading}
	onconfirm={handleDelete}
	oncancel={() => (deletingLabel = null)}
/>
