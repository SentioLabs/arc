<script lang="ts">
	import { page } from '$app/stores';
	import { listLegacyPlans, getMetadata, revisionURL } from '$lib/api/plans';
	let destination = $state<string | null>(null);
	let message = $state('Loading legacy inventory…');
	$effect(() => {
		if ($page.params.planId) resolveLegacy($page.params.planId);
	});
	async function resolveLegacy(id: string) {
		destination = null;
		try {
			for (let offset = 0; ; offset += 50) {
				const records = await listLegacyPlans(offset);
				const legacy = records.find((record) => record.id === id);
				if (legacy) {
					if (legacy.imported_project_id) {
						const plan = await getMetadata(legacy.imported_project_id, id);
						destination = revisionURL(plan.project_id, id, plan.head_revision);
						message =
							'This legacy plan was imported. Open its project revision history to review retained content.';
					} else
						message =
							'Legacy plan is unimported. An operator must migrate it with an explicit project and source manifest. Legacy approval is unverified.';
					return;
				}
				if (records.length < 50) break;
			}
			message =
				'Legacy plan is missing or unimported. An operator can migrate retained legacy records with an explicit project and source manifest.';
		} catch (error) {
			message = error instanceof Error ? error.message : 'Could not load legacy inventory';
		}
	}
</script>

<section class="p-8 space-y-4">
	<h1 class="text-xl">Legacy planner</h1>
	<p>{message}</p>
	{#if destination}<a class="underline" href={destination}>Open imported plan</a>{/if}
	<p>
		Use arc server plans migrate with an operator-reviewed manifest. This page does not open client
		file paths.
	</p>
</section>
