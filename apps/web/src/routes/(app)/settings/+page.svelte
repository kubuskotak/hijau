<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type Pat } from '$lib/api';
	import { Button } from '$lib/components/ui/button/index';
	import { Input } from '$lib/components/ui/input/index';
	import { Label } from '$lib/components/ui/label/index';

	let tokens = $state<Pat[]>([]);
	let loading = $state(true);
	let err = $state('');
	let name = $state('');
	let creating = $state(false);
	let fresh = $state(''); // raw token, shown once after create

	async function load() {
		loading = true;
		err = '';
		try {
			tokens = await api.listMyTokens();
		} catch (e) {
			err = (e as Error).message;
		} finally {
			loading = false;
		}
	}
	onMount(load);

	async function create(e: SubmitEvent) {
		e.preventDefault();
		creating = true;
		err = '';
		fresh = '';
		try {
			const res = await api.createToken(name.trim() || 'token');
			fresh = res.token;
			name = '';
			await load();
		} catch (e) {
			err = (e as Error).message;
		} finally {
			creating = false;
		}
	}

	async function revoke(t: Pat) {
		try {
			await api.revokeToken(t.id);
			tokens = tokens.filter((x) => x.id !== t.id);
		} catch (e) {
			err = (e as Error).message;
		}
	}
	function copy(text: string) {
		navigator.clipboard?.writeText(text).catch(() => {});
	}
	function fmt(ts: string | undefined): string {
		return ts ? new Date(ts).toLocaleString() : '—';
	}
</script>

<div class="space-y-6">
	<div>
		<a href="/" class="text-xs text-muted-foreground hover:underline">← Projects</a>
		<h1 class="mt-1 text-2xl font-semibold tracking-tight">Settings</h1>
		<p class="text-sm text-muted-foreground">Personal access tokens for the CLI and MCP server.</p>
	</div>

	{#if err}<p class="text-sm text-destructive">{err}</p>{/if}

	{#if fresh}
		<div class="rounded-xl border border-emerald-500/40 bg-emerald-500/5 p-4">
			<div class="text-sm font-medium">Your new token — copy it now, it won't be shown again.</div>
			<div class="mt-2 flex items-center gap-2">
				<code class="flex-1 truncate rounded-md bg-muted px-2 py-1 text-xs">{fresh}</code>
				<Button variant="outline" size="sm" onclick={() => copy(fresh)}>Copy</Button>
			</div>
			<p class="mt-2 text-xs text-muted-foreground">
				Use it as <code class="text-xs">HIJAU_TOKEN</code> (with
				<code class="text-xs">HIJAU_API_URL</code>) for the MCP server, or
				<code class="text-xs">hijau login --token …</code> for the CLI.
			</p>
		</div>
	{/if}

	<div class="grid gap-8 lg:grid-cols-[1fr_18rem]">
		<div>
			<h2 class="text-lg font-medium">Your tokens</h2>
			{#if loading}
				<p class="mt-4 text-sm text-muted-foreground">Loading…</p>
			{:else if tokens.length === 0}
				<div class="mt-4 rounded-xl border border-dashed p-8 text-center">
					<p class="text-sm text-muted-foreground">No tokens yet.</p>
				</div>
			{:else}
				<ul class="mt-4 divide-y rounded-xl border">
					{#each tokens as t (t.id)}
						<li class="flex items-center justify-between gap-3 px-4 py-3">
							<div class="min-w-0">
								<div class="truncate text-sm font-medium">{t.name}</div>
								<div class="text-xs text-muted-foreground">
									<code>{t.prefix}…</code> · created {fmt(t.createdAt)} · last used {fmt(t.lastUsedAt)}
								</div>
							</div>
							<Button variant="outline" size="sm" onclick={() => revoke(t)}>Revoke</Button>
						</li>
					{/each}
				</ul>
			{/if}
		</div>

		<form class="h-fit rounded-xl border bg-card p-4 shadow-sm" onsubmit={create}>
			<h3 class="font-medium">New token</h3>
			<p class="mt-1 text-xs text-muted-foreground">Acts with your permissions.</p>
			<div class="mt-3 space-y-3">
				<div class="space-y-1.5">
					<Label for="t-name">Name</Label>
					<Input id="t-name" bind:value={name} placeholder="my-laptop CLI" />
				</div>
				<Button type="submit" disabled={creating}>{creating ? 'Creating…' : 'Generate token'}</Button>
			</div>
		</form>
	</div>
</div>
