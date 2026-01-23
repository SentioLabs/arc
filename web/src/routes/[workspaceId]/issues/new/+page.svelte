<script lang="ts">
	import { Header } from '$lib/components';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { getContext } from 'svelte';
	import type { Writable } from 'svelte/store';
	import { createIssue, type Workspace, type CreateIssueRequest } from '$lib/api';

	const workspaces = getContext<Writable<Workspace[]>>('workspaces');
	const workspaceId = $derived($page.params.workspaceId);
	const workspace = $derived($workspaces.find((ws) => ws.id === workspaceId));

	let title = $state('');
	let description = $state('');
	let issueType = $state<'bug' | 'feature' | 'task' | 'epic' | 'chore'>('task');
	let priority = $state(2);
	let assignee = $state('');
	let submitting = $state(false);
	let error = $state<string | null>(null);

	const issueTypes = [
		{ value: 'bug', label: 'Bug' },
		{ value: 'feature', label: 'Feature' },
		{ value: 'task', label: 'Task' },
		{ value: 'epic', label: 'Epic' },
		{ value: 'chore', label: 'Chore' }
	];

	const priorities = [
		{ value: 0, label: 'P0 - Critical' },
		{ value: 1, label: 'P1 - High' },
		{ value: 2, label: 'P2 - Medium' },
		{ value: 3, label: 'P3 - Low' },
		{ value: 4, label: 'P4 - Backlog' }
	];

	async function handleSubmit(e: Event) {
		e.preventDefault();
		if (!workspaceId || !title.trim()) return;

		submitting = true;
		error = null;

		try {
			const request: CreateIssueRequest = {
				title: title.trim(),
				description: description.trim() || undefined,
				issue_type: issueType,
				priority,
				assignee: assignee.trim() || undefined
			};

			const issue = await createIssue(workspaceId, request);
			goto(`/${workspaceId}/issues/${issue.id}`);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create issue';
		} finally {
			submitting = false;
		}
	}

	function handleCancel() {
		goto(`/${workspaceId}/issues`);
	}
</script>

<svelte:head>
	<title>New Issue - {workspace?.name ?? 'Arc'}</title>
</svelte:head>

{#if workspace}
	<Header {workspace} title="New Issue" />

	<div class="flex-1 p-6 animate-fade-in">
		<div class="max-w-2xl mx-auto">
			<header class="mb-8">
				<h1 class="text-2xl font-bold text-text-primary mb-2">Create New Issue</h1>
				<p class="text-text-secondary">Add a new issue to {workspace.name}</p>
			</header>

			{#if error}
				<div class="card p-4 mb-6 border-status-blocked/50 bg-status-blocked/10">
					<p class="text-status-blocked">{error}</p>
				</div>
			{/if}

			<form onsubmit={handleSubmit} class="space-y-6">
				<!-- Title -->
				<div>
					<label for="title" class="block text-sm font-medium text-text-secondary mb-2">
						Title <span class="text-status-blocked">*</span>
					</label>
					<input
						id="title"
						type="text"
						bind:value={title}
						class="input w-full"
						placeholder="Brief description of the issue"
						required
						disabled={submitting}
					/>
				</div>

				<!-- Description -->
				<div>
					<label for="description" class="block text-sm font-medium text-text-secondary mb-2">
						Description
					</label>
					<textarea
						id="description"
						bind:value={description}
						class="input w-full min-h-32"
						placeholder="Detailed description, context, or requirements..."
						disabled={submitting}
					></textarea>
				</div>

				<!-- Type and Priority -->
				<div class="grid grid-cols-2 gap-4">
					<div>
						<label for="type" class="block text-sm font-medium text-text-secondary mb-2">
							Type
						</label>
						<select id="type" bind:value={issueType} class="input w-full" disabled={submitting}>
							{#each issueTypes as type (type.value)}
								<option value={type.value}>{type.label}</option>
							{/each}
						</select>
					</div>

					<div>
						<label for="priority" class="block text-sm font-medium text-text-secondary mb-2">
							Priority
						</label>
						<select id="priority" bind:value={priority} class="input w-full" disabled={submitting}>
							{#each priorities as p (p.value)}
								<option value={p.value}>{p.label}</option>
							{/each}
						</select>
					</div>
				</div>

				<!-- Assignee -->
				<div>
					<label for="assignee" class="block text-sm font-medium text-text-secondary mb-2">
						Assignee
					</label>
					<input
						id="assignee"
						type="text"
						bind:value={assignee}
						class="input w-full"
						placeholder="Username or leave empty"
						disabled={submitting}
					/>
				</div>

				<!-- Actions -->
				<div class="flex items-center justify-end gap-3 pt-4 border-t border-border">
					<button type="button" class="btn btn-ghost" onclick={handleCancel} disabled={submitting}>
						Cancel
					</button>
					<button type="submit" class="btn btn-primary" disabled={submitting || !title.trim()}>
						{#if submitting}
							Creating...
						{:else}
							Create Issue
						{/if}
					</button>
				</div>
			</form>
		</div>
	</div>
{/if}
