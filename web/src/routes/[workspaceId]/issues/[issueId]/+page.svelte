<script lang="ts">
	import { Header, StatusBadge, PriorityBadge, TypeBadge, Markdown, InlineTextEdit, InlineSelect, InlineMarkdownEdit, CommentForm, LabelPicker } from '$lib/components';
	import { formatDateTime, formatRelativeTime, dependencyTypeLabels } from '$lib/utils';
	import { page } from '$app/stores';
	import { getContext } from 'svelte';
	import type { Writable } from 'svelte/store';
	import {
		getIssue,
		getComments,
		getEvents,
		getDependencies,
		updateIssue,
		createComment,
		addLabelToIssue,
		removeLabelFromIssue,
		listLabels,
		type Workspace,
		type Issue,
		type Comment,
		type Event,
		type DependencyGraph,
		type Label,
		type UpdateIssueRequest
	} from '$lib/api';

	const workspaces = getContext<Writable<Workspace[]>>('workspaces');
	const workspaceId = $derived($page.params.workspaceId);
	const issueId = $derived($page.params.issueId);
	const workspace = $derived($workspaces.find((ws) => ws.id === workspaceId));

	let issue = $state<Issue | null>(null);
	let comments = $state<Comment[]>([]);
	let events = $state<Event[]>([]);
	let dependencies = $state<DependencyGraph>({ dependencies: [], dependents: [] });
	let loading = $state(true);
	let error = $state<string | null>(null);
	let allLabels = $state<Label[]>([]);

	$effect(() => {
		if (workspaceId && issueId) loadData();
	});

	async function loadData() {
		if (!workspaceId || !issueId) return;
		loading = true;
		error = null;
		try {
			const [issueData, commentsData, eventsData, depsData, labelsData] = await Promise.all([
				getIssue(workspaceId, issueId, true),
				getComments(workspaceId, issueId),
				getEvents(workspaceId, issueId, 50),
				getDependencies(workspaceId, issueId),
				listLabels()
			]);
			issue = issueData;
			comments = commentsData;
			events = eventsData;
			dependencies = depsData;
			allLabels = labelsData;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load issue';
		} finally {
			loading = false;
		}
	}

	async function handleUpdateField(field: keyof UpdateIssueRequest, value: string | number) {
		if (!workspaceId || !issueId) return;
		const updated = await updateIssue(workspaceId, issueId, { [field]: value } as UpdateIssueRequest);
		issue = updated;
	}

	async function handleAddLabel(labelName: string) {
		if (!workspaceId || !issueId) return;
		await addLabelToIssue(workspaceId, issueId, labelName);
		await loadData();
	}

	async function handleRemoveLabel(labelName: string) {
		if (!workspaceId || !issueId) return;
		await removeLabelFromIssue(workspaceId, issueId, labelName);
		await loadData();
	}

	async function handleAddComment(text: string) {
		if (!workspaceId || !issueId) return;
		await createComment(workspaceId, issueId, text);
		await loadData();
	}
</script>

<svelte:head>
	<title>{issue?.title ?? 'Issue'} - {workspace?.name ?? 'Arc'} - Arc</title>
</svelte:head>

{#if workspace}
	{#if loading}
		<Header {workspace} title="Loading..." />
		<div class="flex-1 flex items-center justify-center">
			<div class="text-text-muted animate-pulse">Loading issue...</div>
		</div>
	{:else if error}
		<Header {workspace} title="Error" />
		<div class="flex-1 flex items-center justify-center p-8">
			<div class="card p-8 text-center">
				<p class="text-status-blocked mb-4">{error}</p>
				<button class="btn btn-primary" onclick={loadData}>Retry</button>
			</div>
		</div>
	{:else if issue}
		<Header {workspace} title={issue.id} />

		<div class="flex-1 p-6 animate-fade-in">
			<div class="max-w-4xl mx-auto">
				<!-- Issue Header -->
				<header class="mb-8">
					<div class="flex items-center gap-3 mb-4">
						<span class="font-mono text-sm text-text-muted">{issue.id}</span>
						<InlineSelect
							value={issue.issue_type}
							options={[
								{ value: 'bug', label: 'Bug' },
								{ value: 'feature', label: 'Feature' },
								{ value: 'task', label: 'Task' },
								{ value: 'epic', label: 'Epic' },
								{ value: 'chore', label: 'Chore' }
							]}
							onSave={(v) => handleUpdateField('issue_type', v)}
						>
							<TypeBadge type={issue.issue_type} />
						</InlineSelect>
						<InlineSelect
							value={issue.status}
							options={[
								{ value: 'open', label: 'Open' },
								{ value: 'in_progress', label: 'In Progress' },
								{ value: 'blocked', label: 'Blocked' },
								{ value: 'deferred', label: 'Deferred' },
								{ value: 'closed', label: 'Closed' }
							]}
							onSave={(v) => handleUpdateField('status', v)}
						>
							<StatusBadge status={issue.status} />
						</InlineSelect>
						<InlineSelect
							value={issue.priority.toString()}
							options={[
								{ value: '0', label: 'Critical (P0)' },
								{ value: '1', label: 'High (P1)' },
								{ value: '2', label: 'Medium (P2)' },
								{ value: '3', label: 'Low (P3)' },
								{ value: '4', label: 'Backlog (P4)' }
							]}
							onSave={(v) => handleUpdateField('priority', parseInt(v))}
						>
							<PriorityBadge priority={issue.priority} />
						</InlineSelect>
					</div>

					<InlineTextEdit
						value={issue.title}
						onSave={(v) => handleUpdateField('title', v)}
						class="text-2xl font-bold text-text-primary mb-4"
					/>

					<div class="flex items-center gap-4 text-sm text-text-muted">
						<span class="flex items-center gap-1">
							Assigned to
							<InlineTextEdit
								value={issue.assignee ?? ''}
								onSave={(v) => handleUpdateField('assignee', v)}
								placeholder="Unassigned"
								class="font-semibold text-text-secondary"
							/>
						</span>
						<span>Created {formatRelativeTime(issue.created_at)}</span>
						<span>Updated {formatRelativeTime(issue.updated_at)}</span>
					</div>
				</header>

				<div class="grid lg:grid-cols-3 gap-6">
					<!-- Main content -->
					<div class="lg:col-span-2 space-y-6">
						<!-- Description -->
						<section class="card p-6">
							<h2 class="text-sm font-medium text-text-muted uppercase tracking-wider mb-3">Description</h2>
							<InlineMarkdownEdit
								value={issue.description ?? ''}
								onSave={(v) => handleUpdateField('description', v)}
								placeholder="Add a description..."
							/>
						</section>

						<!-- Comments -->
						<section class="card p-6">
							<h2 class="text-sm font-medium text-text-muted uppercase tracking-wider mb-4">
								Comments ({comments.length})
							</h2>

							{#if comments.length === 0}
								<p class="text-sm text-text-muted">No comments yet</p>
							{:else}
								<div class="space-y-4">
									{#each comments as comment (comment.id)}
										<div class="border-b border-border pb-4 last:border-0 last:pb-0">
											<div class="flex items-center justify-between mb-2">
												<span class="font-medium text-text-primary text-sm">{comment.author}</span>
												<span
													class="text-xs text-text-muted"
													title={formatDateTime(comment.created_at)}
												>
													{formatRelativeTime(comment.created_at)}
												</span>
											</div>
											<Markdown content={comment.text} />
										</div>
									{/each}
								</div>
							{/if}
							<div class="mt-4 pt-4 border-t border-border">
								<CommentForm onSubmit={handleAddComment} />
							</div>
						</section>

						<!-- Activity -->
						<section class="card p-6">
							<h2 class="text-sm font-medium text-text-muted uppercase tracking-wider mb-4">
								Activity
							</h2>

							{#if events.length === 0}
								<p class="text-sm text-text-muted">No activity yet</p>
							{:else}
								<div class="space-y-3">
									{#each events as event (event.id)}
										<div class="flex items-start gap-3 text-sm">
											<div class="w-2 h-2 rounded-full bg-surface-500 mt-1.5"></div>
											<div class="flex-1">
												<span class="text-text-secondary">
													<strong class="text-text-primary">{event.actor}</strong>
													{#if event.event_type === 'created'}
														created this issue
													{:else if event.event_type === 'status_changed'}
														changed status from
														<span class="font-mono text-xs">{event.old_value}</span>
														to <span class="font-mono text-xs">{event.new_value}</span>
													{:else if event.event_type === 'commented'}
														added a comment
													{:else if event.event_type === 'closed'}
														closed this issue
													{:else if event.event_type === 'reopened'}
														reopened this issue
													{:else if event.event_type === 'label_added'}
														added label <span class="font-mono text-xs">{event.new_value}</span>
													{:else if event.event_type === 'label_removed'}
														removed label <span class="font-mono text-xs">{event.old_value}</span>
													{:else if event.event_type === 'dependency_added'}
														added dependency to
														<span class="font-mono text-xs">{event.new_value}</span>
													{:else if event.event_type === 'dependency_removed'}
														removed dependency to
														<span class="font-mono text-xs">{event.old_value}</span>
													{:else}
														{event.event_type}
													{/if}
												</span>
												<span class="text-text-muted ml-2" title={formatDateTime(event.created_at)}>
													{formatRelativeTime(event.created_at)}
												</span>
											</div>
										</div>
									{/each}
								</div>
							{/if}
						</section>
					</div>

					<!-- Sidebar -->
					<div class="space-y-6">
						<!-- Labels -->
						<section class="card p-4">
							<h3 class="text-xs font-medium text-text-muted uppercase tracking-wider mb-3">Labels</h3>
							<LabelPicker
								currentLabels={issue.labels ?? []}
								{allLabels}
								onAdd={handleAddLabel}
								onRemove={handleRemoveLabel}
							/>
						</section>

						<!-- Dependencies -->
						<section class="card p-4">
							<h3 class="text-xs font-medium text-text-muted uppercase tracking-wider mb-3">
								Dependencies ({dependencies.dependencies.length})
							</h3>
							{#if dependencies.dependencies.length > 0}
								<div class="space-y-2">
									{#each dependencies.dependencies as dep (dep.depends_on_id)}
										<a
											href="/{workspaceId}/issues/{dep.depends_on_id}"
											class="block p-2 bg-surface-700 rounded hover:bg-surface-600 transition-colors"
										>
											<div class="flex items-center justify-between">
												<span class="font-mono text-xs text-primary-400">{dep.depends_on_id}</span>
												<span class="text-[10px] text-text-muted uppercase">
													{dependencyTypeLabels[dep.type]}
												</span>
											</div>
										</a>
									{/each}
								</div>
							{:else}
								<p class="text-sm text-text-muted">No dependencies</p>
							{/if}
						</section>

						<!-- Dependents -->
						<section class="card p-4">
							<h3 class="text-xs font-medium text-text-muted uppercase tracking-wider mb-3">
								Dependents ({dependencies.dependents.length})
							</h3>
							{#if dependencies.dependents.length > 0}
								<div class="space-y-2">
									{#each dependencies.dependents as dep (dep.issue_id)}
										<a
											href="/{workspaceId}/issues/{dep.issue_id}"
											class="block p-2 bg-surface-700 rounded hover:bg-surface-600 transition-colors"
										>
											<div class="flex items-center justify-between">
												<span class="font-mono text-xs text-primary-400">{dep.issue_id}</span>
												<span class="text-[10px] text-text-muted uppercase">
													{dependencyTypeLabels[dep.type]}
												</span>
											</div>
										</a>
									{/each}
								</div>
							{:else}
								<p class="text-sm text-text-muted">No dependents</p>
							{/if}
						</section>

						<!-- External Reference -->
						{#if issue.external_ref}
							<section class="card p-4">
								<h3 class="text-xs font-medium text-text-muted uppercase tracking-wider mb-2">
									External Ref
								</h3>
								<span class="font-mono text-sm text-text-secondary">{issue.external_ref}</span>
							</section>
						{/if}

						<!-- Close Info -->
						{#if issue.closed_at}
							<section class="card p-4">
								<h3 class="text-xs font-medium text-text-muted uppercase tracking-wider mb-2">
									Closed
								</h3>
								<p class="text-sm text-text-secondary">{formatDateTime(issue.closed_at)}</p>
								{#if issue.close_reason}
									<div class="mt-2"><Markdown content={issue.close_reason} /></div>
								{/if}
							</section>
						{/if}
					</div>
				</div>
			</div>
		</div>
	{/if}
{/if}
