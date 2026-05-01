<script lang="ts">
	import { untrack } from 'svelte';
	import { updateConfig, type Config, type ConfigResponse } from '$lib/api';
	import SettingsSection from '$lib/components/settings/SettingsSection.svelte';
	import SettingsField from '$lib/components/settings/SettingsField.svelte';
	import ChannelPicker from '$lib/components/settings/ChannelPicker.svelte';

	let { data }: { data: { config: ConfigResponse | null; available: boolean; error?: string } } = $props();

	function stripMeta(r: ConfigResponse): Config {
		const { meta: _meta, ...rest } = r as Config & { meta?: unknown };
		return rest as Config;
	}

	const initial = $derived(data.config);
	// Use untrack to explicitly capture the one-time initial value — this is intentional
	// (working/snapshot are the editable copy; they don't track data reactively)
	let working = $state<Config | null>(untrack(() => data.config ? structuredClone(stripMeta(data.config)) : null));
	let snapshot = $state<Config | null>(untrack(() => data.config ? structuredClone(stripMeta(data.config)) : null));
	let errors = $state<Record<string, string>>({});
	let saving = $state(false);
	let toast = $state<string | null>(null);

	const restartKeys = $derived(initial?.meta.requires_restart ?? []);
	const dirty = $derived(working && snapshot && JSON.stringify(working) !== JSON.stringify(snapshot));

	async function save() {
		if (!working) return;
		saving = true;
		errors = {};
		try {
			const updated = await updateConfig(working);
			snapshot = structuredClone(stripMeta(updated));
			working = structuredClone(stripMeta(updated));
			toast = 'Saved';
			setTimeout(() => (toast = null), 2000);
		} catch (err) {
			const fe = (err as { fieldErrors?: Record<string, string> }).fieldErrors;
			if (fe) errors = fe;
			else toast = 'Save failed';
		} finally {
			saving = false;
		}
	}

	function discard() {
		if (snapshot) working = structuredClone(snapshot);
		errors = {};
	}
</script>

<svelte:head><title>Settings · Arc</title></svelte:head>

<div class="max-w-2xl mx-auto p-8 space-y-6">
	<header class="flex items-center justify-between">
		<h1 class="text-xl font-semibold text-text-primary">Settings</h1>
		{#if initial}
			<span class="text-xs text-text-muted">{initial.meta.path}</span>
		{/if}
	</header>

	{#if !data.available || !working}
		<div class="card p-8 text-center">
			<p class="text-sm text-text-secondary">Settings are unavailable on this deployment.</p>
		</div>
	{:else}
		<SettingsSection title="CLI">
			<SettingsField label="Server URL" help="Where the CLI calls home." error={errors['cli.server']}>
				<input class="input w-full" bind:value={working.cli.server} />
			</SettingsField>
		</SettingsSection>

		<SettingsSection title="Server" warn="Changes here require restarting arc-server.">
			<SettingsField label="Port" error={errors['server.port']} requiresRestart={restartKeys.includes('server.port')}>
				<input class="input w-full" type="number" min="1" max="65535" bind:value={working.server.port} />
			</SettingsField>
			<SettingsField label="Database path" error={errors['server.db_path']} requiresRestart={restartKeys.includes('server.db_path')}>
				<input class="input w-full" bind:value={working.server.db_path} />
			</SettingsField>
		</SettingsSection>

		<SettingsSection title="Share">
			<SettingsField label="Default author" error={errors['share.author']}>
				<input class="input w-full" bind:value={working.share.author} />
			</SettingsField>
			<SettingsField label="Remote server" error={errors['share.server']}>
				<input class="input w-full" bind:value={working.share.server} />
			</SettingsField>
		</SettingsSection>

		<SettingsSection title="Updates">
			<SettingsField label="Channel" error={errors['updates.channel']}>
				<ChannelPicker
					value={(working.updates.channel as 'stable' | 'rc' | 'nightly') ?? 'stable'}
					onChange={(v) => (working!.updates.channel = v)}
				/>
			</SettingsField>
		</SettingsSection>

		<footer class="flex justify-between">
			<button class="btn btn-ghost" onclick={discard} disabled={!dirty || saving}>Discard changes</button>
			<button class="btn btn-primary" onclick={save} disabled={!dirty || saving}>
				{saving ? 'Saving…' : 'Save'}
			</button>
		</footer>

		{#if toast}
			<div class="fixed bottom-6 right-6 card px-4 py-2 text-sm">{toast}</div>
		{/if}
	{/if}
</div>
