<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { listPlans, uploadPlan, revisionURL, type Plan } from '$lib/api/plans';
	let plans = $state<Plan[]>([]);
	let archived = $state(false);
	let offset = $state(0);
	let error = $state('');
	let title = $state('');
	let content = $state('');
	let busy = $state(false);
	let attempt: { title: string; content: string; key: string } | null = null;
	$effect(() => {
		if ($page.params.projectId) load($page.params.projectId, archived, offset);
	});
	async function load(project: string, archived: boolean, offset: number) {
		try {
			plans = await listPlans(project, archived, offset);
			error = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Could not load plans';
		}
	}
	async function upload() {
		if (!$page.params.projectId || busy) return;
		busy = true;
		if (!attempt || attempt.title !== title || attempt.content !== content)
			attempt = { title, content, key: crypto.randomUUID() };
		try {
			const result = await uploadPlan(
				$page.params.projectId,
				{ title: attempt.title, content: attempt.content },
				attempt.key
			);
			await goto(revisionURL(result.plan.project_id, result.plan.id, result.revision.revision));
		} catch (err) {
			error = err instanceof Error ? err.message : 'Could not upload plan';
		} finally {
			busy = false;
		}
	}
</script>

<section class="p-8 space-y-6">
	<h1 class="text-2xl">Plans</h1>
	<label
		><input type="checkbox" bind:checked={archived} onchange={() => (offset = 0)} /> Archived plans</label
	>
	{#if error}<p role="alert">{error}</p>{/if}
	<ul class="space-y-3">
		{#each plans as plan (plan.id)}<li class="card p-3">
				<a class="underline" href={revisionURL(plan.project_id, plan.id, plan.head_revision)}
					>{plan.title || plan.id}</a
				>
				· Revision {plan.head_revision} · {plan.lifecycle}
			</li>
		{:else}<li>No plans in this view.</li>{/each}
	</ul>
	<div class="flex gap-3">
		<button disabled={offset === 0} onclick={() => (offset -= 50)}>Previous</button><button
			disabled={plans.length < 50}
			onclick={() => (offset += 50)}>Next</button
		>
	</div>
	<details>
		<summary>Upload a new plan</summary>
		<div class="space-y-3 mt-3">
			<label class="block">Title<input class="input w-full" bind:value={title} /></label>
			<label class="block"
				>Markdown file<input
					type="file"
					accept=".md,.markdown,text/markdown,text/plain"
					onchange={async (e) => {
						const file = e.currentTarget.files?.[0];
						if (file) content = await file.text();
					}}
				/></label
			>
			<label class="block"
				>Plan content<textarea class="input w-full h-64" bind:value={content}></textarea></label
			>
			<button class="btn btn-primary" disabled={busy} onclick={upload}>Upload plan</button>
		</div>
	</details>
</section>
