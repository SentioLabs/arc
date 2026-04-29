<script lang="ts">
	import { setReviewerName } from '$lib/paste/identity';

	const { onSave } = $props<{ onSave: (name: string) => void }>();
	let name = $state('');

	function save() {
		const trimmed = name.trim();
		if (!trimmed) return;
		setReviewerName(trimmed);
		onSave(trimmed);
	}
</script>

<div class="fixed inset-0 bg-black/50 flex items-center justify-center" role="dialog">
	<div class="bg-white p-4 rounded shadow space-y-3 w-80">
		<h2 class="font-semibold">What's your name?</h2>
		<p class="text-sm text-gray-500">
			This is shown alongside your review comments. Stored in this browser only.
		</p>
		<input bind:value={name} placeholder="Your name" class="w-full border p-2 rounded" />
		<button
			onclick={save}
			class="w-full bg-blue-600 text-white p-2 rounded"
			disabled={!name.trim()}>Save</button
		>
	</div>
</div>
