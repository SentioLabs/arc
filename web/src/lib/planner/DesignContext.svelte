<script lang="ts">
	import {
		getMetadata,
		getRevision,
		resolveGovernance,
		revisionURL,
		PlanAPIError,
		type Plan,
		type Revision,
		type Governance
	} from '$lib/api/plans';
	import type { components } from '$lib/api/types';
	let {
		projectId,
		issueId,
		explicitPin
	}: { projectId: string; issueId: string; explicitPin?: components['schemas']['PlanReference'] } =
		$props();
	let governing = $state<Governance | null>(null);
	let sources = $state<
		{ source: components['schemas']['GoverningPlanContext']; plan: Plan; revision: Revision }[]
	>([]);
	let loading = $state(true);
	let error = $state('');
	$effect(() => {
		load(projectId, issueId);
	});
	async function load(project: string, issue: string) {
		loading = true;
		error = '';
		sources = [];
		try {
			governing = await resolveGovernance(project, issue);
			if (governing) {
				sources = await Promise.all(
					[...governing.context, governing].map(async (source) => {
						const [plan, revision] = await Promise.all([
							getMetadata(project, source.reference.plan_id),
							getRevision(project, source.reference.plan_id, source.reference.revision)
						]);
						return { source, plan, revision };
					})
				);
			}
		} catch (err) {
			error =
				err instanceof PlanAPIError
					? `${err.code ?? 'governance_unavailable'}: ${err.message}`
					: err instanceof Error
						? err.message
						: 'Could not load governing design';
		} finally {
			loading = false;
		}
	}
</script>

<section class="card p-5 space-y-3" aria-label="Design context">
	<h2 class="font-semibold">Design context</h2>
	<p>
		Explicit container pin: {#if explicitPin}<a
				class="underline"
				href={revisionURL(projectId, explicitPin.plan_id, explicitPin.revision)}
				>Pinned revision {explicitPin.revision}</a
			>
			· {explicitPin.plan_id}{:else}None{/if}
	</p>
	{#if loading}<p>Loading governing designs…</p>
	{:else if error}<p role="alert">{error}</p>
		<p>
			Reconcile the parent hierarchy and governing container pins before using this issue's design
			context. Inspect parent dependencies to resolve ambiguity or cycles.
		</p>
		<button onclick={() => load(projectId, issueId)}>Retry design context</button>
	{:else if !governing}<p>No governing plan. This issue remains available for unlinked work.</p>
	{:else}
		<ol class="space-y-3">
			{#each sources as entry (entry.source.container_id)}
				<li>
					<p>
						{entry.source.container_id === governing.container_id
							? 'Primary design'
							: 'Higher-level context'} ·
						<a class="underline" href="/{projectId}/issues/{entry.source.container_id}"
							>{entry.source.container_type} {entry.source.container_id}</a
						>
					</p>
					<a class="underline" href={revisionURL(projectId, entry.plan.id, entry.revision.revision)}
						>{entry.plan.title} — Revision {entry.revision.revision}</a
					>
					<p>{entry.revision.review_status} · {entry.plan.lifecycle}</p>
				</li>
			{/each}
		</ol>
	{/if}
	<p class="text-sm text-text-muted">
		Changing a pin requires a reviewed reconciliation through plan adoption.
	</p>
</section>
