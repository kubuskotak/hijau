<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type MtConfig } from '$lib/api';
	import { Button } from '$lib/components/ui/button/index';
	import { Input } from '$lib/components/ui/input/index';
	import { Label } from '$lib/components/ui/label/index';
	import { Badge } from '$lib/components/ui/badge/index';

	let { data } = $props();
	const pid = $derived(data.project.id);
	const selectClass = 'flex h-9 w-full rounded-md border border-input bg-transparent px-3 text-sm';

	let cfg = $state<MtConfig | null>(null);
	let loading = $state(true);
	let err = $state('');
	let saving = $state(false);
	let saved = $state(false);

	let provider = $state('claude');
	let model = $state('');
	let apiKey = $state('');
	let enabled = $state(true);

	async function load() {
		loading = true;
		err = '';
		try {
			cfg = await api.mtConfig(pid);
			if (cfg.provider) {
				provider = cfg.provider;
				model = cfg.model;
				enabled = cfg.enabled;
			}
		} catch (e) {
			err = (e as Error).message;
		} finally {
			loading = false;
		}
	}
	onMount(load);

	async function save(e: SubmitEvent) {
		e.preventDefault();
		saving = true;
		err = '';
		saved = false;
		try {
			cfg = await api.configureMT(pid, {
				provider,
				model: model || undefined,
				apiKey: apiKey || undefined,
				enabled
			});
			apiKey = '';
			saved = true;
		} catch (e) {
			err = (e as Error).message;
		} finally {
			saving = false;
		}
	}
</script>

<div class="max-w-xl space-y-6">
	<div>
		<h2 class="text-lg font-medium">Machine translation</h2>
		<p class="text-sm text-muted-foreground">
			Configure the MT provider used for suggestions and auto-translate. Stays off until enabled.
		</p>
	</div>

	{#if loading}
		<p class="text-sm text-muted-foreground">Loading…</p>
	{:else}
		<form class="space-y-4 rounded-xl border bg-card p-4 shadow-sm" onsubmit={save}>
			<div class="flex items-center gap-2 text-sm">
				Status:
				{#if cfg?.enabled}
					<Badge variant="default" class="text-emerald-600 dark:text-emerald-400">enabled</Badge>
				{:else}
					<Badge variant="secondary">disabled</Badge>
				{/if}
				{#if cfg?.hasCredentials}<Badge variant="secondary">key set</Badge>{/if}
			</div>

			<div class="space-y-1.5">
				<Label for="mt-provider">Provider</Label>
				<select id="mt-provider" class={selectClass} bind:value={provider}>
					<option value="claude">Claude (Anthropic)</option>
					<option value="mock">Mock (no key — testing)</option>
				</select>
			</div>

			<div class="space-y-1.5">
				<Label for="mt-model">Model</Label>
				<Input
					id="mt-model"
					bind:value={model}
					placeholder="claude-haiku-4-5 (bulk) · claude-sonnet-4-6 (review)"
				/>
			</div>

			{#if provider === 'claude'}
				<div class="space-y-1.5">
					<Label for="mt-key">API key</Label>
					<Input
						id="mt-key"
						type="password"
						bind:value={apiKey}
						placeholder={cfg?.hasCredentials ? '•••••• (leave blank to keep current)' : 'sk-ant-…'}
						autocomplete="off"
					/>
					<p class="text-xs text-muted-foreground">Stored encrypted (AES-256-GCM); never shown again.</p>
				</div>
			{/if}

			<label class="flex items-center gap-2 text-sm">
				<input type="checkbox" bind:checked={enabled} class="h-4 w-4 rounded border-input" />
				Enabled
			</label>

			{#if err}<p class="text-sm text-destructive">{err}</p>{/if}
			{#if saved}<p class="text-sm text-emerald-600 dark:text-emerald-400">Saved ✓</p>{/if}
			<Button type="submit" disabled={saving}>{saving ? 'Saving…' : 'Save'}</Button>
		</form>
	{/if}
</div>
