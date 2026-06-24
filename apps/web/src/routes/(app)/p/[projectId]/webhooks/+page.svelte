<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type Webhook, type WebhookDelivery } from '$lib/api';
	import { Button } from '$lib/components/ui/button/index';
	import { Input } from '$lib/components/ui/input/index';
	import { Label } from '$lib/components/ui/label/index';
	import { Badge } from '$lib/components/ui/badge/index';

	let { data } = $props();
	const pid = $derived(data.project.id);

	const ALL_EVENTS = ['translation.updated', 'translation.reviewed', 'translation.needs_work'];

	let webhooks = $state<Webhook[]>([]);
	let loading = $state(true);
	let err = $state('');

	let url = $state('');
	let selected = $state<Record<string, boolean>>({});
	let busy = $state('');

	// The one-time secret shown after a successful create.
	let newSecret = $state('');
	let copied = $state(false);

	// Per-webhook delivery state, keyed by webhook id.
	let openId = $state('');
	let deliveries = $state<Record<string, WebhookDelivery[]>>({});
	let delLoading = $state('');
	let delErr = $state<Record<string, string>>({});

	function fmtTime(iso: string): string {
		const d = new Date(iso);
		return Number.isNaN(d.getTime()) ? iso : d.toLocaleString();
	}

	async function load() {
		loading = true;
		err = '';
		try {
			webhooks = await api.listWebhooks(pid);
		} catch (e) {
			err = (e as Error).message;
		} finally {
			loading = false;
		}
	}

	onMount(load);

	async function addWebhook(e: SubmitEvent) {
		e.preventDefault();
		if (!url.trim()) return;
		busy = 'add';
		err = '';
		newSecret = '';
		copied = false;
		try {
			const events = ALL_EVENTS.filter((ev) => selected[ev]);
			const wh = await api.createWebhook(pid, {
				url: url.trim(),
				events: events.length ? events : undefined
			});
			if (wh.secret) newSecret = wh.secret;
			url = '';
			selected = {};
			await load();
		} catch (e) {
			err = (e as Error).message;
		} finally {
			busy = '';
		}
	}

	async function copySecret() {
		try {
			await navigator.clipboard.writeText(newSecret);
			copied = true;
		} catch {
			copied = false;
		}
	}

	async function removeWebhook(wid: string) {
		busy = wid;
		err = '';
		try {
			await api.deleteWebhook(pid, wid);
			if (openId === wid) openId = '';
			await load();
		} catch (e) {
			err = (e as Error).message;
		} finally {
			busy = '';
		}
	}

	async function toggleDeliveries(wid: string) {
		if (openId === wid) {
			openId = '';
			return;
		}
		openId = wid;
		if (deliveries[wid]) return;
		delLoading = wid;
		delErr = { ...delErr, [wid]: '' };
		try {
			deliveries = { ...deliveries, [wid]: await api.listWebhookDeliveries(pid, wid) };
		} catch (e) {
			delErr = { ...delErr, [wid]: (e as Error).message };
		} finally {
			delLoading = '';
		}
	}
</script>

<div class="grid gap-8 lg:grid-cols-2">
	<div>
		<h2 class="text-lg font-medium">Webhooks</h2>
		<p class="text-sm text-muted-foreground">
			Receive an HTTP POST when translations change. Each webhook is signed with a secret shown once
			at creation.
		</p>

		{#if loading}
			<p class="mt-4 text-sm text-muted-foreground">Loading…</p>
		{:else if webhooks.length === 0}
			<p class="mt-4 text-sm text-muted-foreground">No webhooks yet.</p>
		{:else}
			<ul class="mt-4 space-y-3">
				{#each webhooks as w (w.id)}
					<li class="rounded-xl border bg-card p-4 shadow-sm">
						<div class="flex items-start justify-between gap-3">
							<div class="min-w-0">
								<div class="flex items-center gap-2">
									<span class="truncate font-medium">{w.url}</span>
									<Badge variant={w.active ? 'default' : 'secondary'}>
										{w.active ? 'Active' : 'Inactive'}
									</Badge>
								</div>
								<p class="mt-1 text-sm text-muted-foreground">
									{w.events.length ? w.events.join(', ') : 'all events'}
								</p>
							</div>
							<div class="flex shrink-0 gap-2">
								<Button
									variant="outline"
									size="sm"
									onclick={() => toggleDeliveries(w.id)}
								>
									{openId === w.id ? 'Hide' : 'Deliveries'}
								</Button>
								<Button
									variant="outline"
									size="sm"
									onclick={() => removeWebhook(w.id)}
									disabled={busy === w.id}
								>
									Delete
								</Button>
							</div>
						</div>

						{#if openId === w.id}
							<div class="mt-3 border-t pt-3">
								{#if delLoading === w.id}
									<p class="text-sm text-muted-foreground">Loading deliveries…</p>
								{:else if delErr[w.id]}
									<p class="text-sm text-destructive">{delErr[w.id]}</p>
								{:else if (deliveries[w.id] ?? []).length === 0}
									<p class="text-sm text-muted-foreground">No deliveries yet.</p>
								{:else}
									<ul class="space-y-2">
										{#each deliveries[w.id] as d (d.id)}
											<li class="flex items-start justify-between gap-3 text-sm">
												<div class="min-w-0">
													<div class="flex items-center gap-2">
														<span class="font-medium">{d.event}</span>
														<Badge variant={d.success ? 'default' : 'destructive'}>
															{d.success ? 'Success' : 'Failed'}
														</Badge>
														<span class="text-muted-foreground">{d.statusCode}</span>
													</div>
													{#if d.error}
														<p class="text-sm text-destructive">{d.error}</p>
													{/if}
												</div>
												<span class="shrink-0 text-xs text-muted-foreground">{fmtTime(d.createdAt)}</span>
											</li>
										{/each}
									</ul>
								{/if}
							</div>
						{/if}
					</li>
				{/each}
			</ul>
		{/if}
		{#if err}<p class="mt-3 text-sm text-destructive">{err}</p>{/if}
	</div>

	<form class="h-fit rounded-xl border bg-card p-4 shadow-sm" onsubmit={addWebhook}>
		<h3 class="font-medium">Add a webhook</h3>
		<div class="mt-3 space-y-3">
			<div class="space-y-1.5">
				<Label for="w-url">Payload URL</Label>
				<Input id="w-url" type="url" bind:value={url} placeholder="https://example.com/hooks" required />
			</div>
			<div class="space-y-1.5">
				<Label>Events</Label>
				<p class="text-sm text-muted-foreground">Leave all unchecked to receive every event.</p>
				<div class="space-y-1.5">
					{#each ALL_EVENTS as ev (ev)}
						<label class="flex items-center gap-2 text-sm">
							<input
								type="checkbox"
								bind:checked={selected[ev]}
								class="h-4 w-4 rounded border-input"
							/>
							{ev}
						</label>
					{/each}
				</div>
			</div>
			<Button type="submit" disabled={busy === 'add'}>
				{busy === 'add' ? 'Adding…' : 'Add webhook'}
			</Button>
		</div>

		{#if newSecret}
			<div class="mt-4 rounded-xl border border-primary bg-primary/5 p-4">
				<p class="text-sm font-medium">Signing secret</p>
				<p class="mt-1 text-sm text-muted-foreground">
					This is shown only once. Copy it now — you will not be able to see it again.
				</p>
				<code class="mt-2 block break-all rounded-md border bg-background px-2 py-1.5 font-mono text-sm">
					{newSecret}
				</code>
				<Button class="mt-2" variant="outline" size="sm" onclick={copySecret}>
					{copied ? 'Copied' : 'Copy secret'}
				</Button>
			</div>
		{/if}
	</form>
</div>
