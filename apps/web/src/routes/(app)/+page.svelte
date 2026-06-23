<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type Project, type Org } from '$lib/api';
	import { Button }  from '$lib/components/ui/button/index';
	import { Input } from '$lib/components/ui/input/index';
	import { Label } from '$lib/components/ui/label/index';

	let projects = $state<Project[]>([]);
	let orgs = $state<Org[]>([]);
	let loading = $state(true);
	let error = $state('');

	let showNew = $state(false);
	let newName = $state('');
	let newOrg = $state('');
	let creating = $state(false);

	async function load() {
		loading = true;
		error = '';
		try {
			[projects, orgs] = await Promise.all([api.listProjects(), api.listOrgs()]);
			if (orgs.length && !newOrg) newOrg = orgs[0].id;
		} catch (e) {
			error = (e as Error).message;
		} finally {
			loading = false;
		}
	}

	onMount(load);

	async function create(e: SubmitEvent) {
		e.preventDefault();
		if (!newName.trim() || !newOrg) return;
		creating = true;
		error = '';
		try {
			const p = await api.createProject({ orgId: newOrg, name: newName.trim() });
			projects = [p, ...projects];
			newName = '';
			showNew = false;
		} catch (e) {
			error = (e as Error).message;
		} finally {
			creating = false;
		}
	}
</script>

<div class="flex items-center justify-between">
	<div>
		<h1 class="text-2xl font-semibold tracking-tight">Projects</h1>
		<p class="text-sm text-muted-foreground">Your localization projects.</p>
	</div>
	<Button onclick={() => (showNew = !showNew)}>{showNew ? 'Cancel' : 'New project'}</Button>
</div>

{#if showNew}
	<form class="mt-4 rounded-xl border bg-card p-4 shadow-sm sm:max-w-md" onsubmit={create}>
		<div class="space-y-3">
			<div class="space-y-1.5">
				<Label for="np-name">Project name</Label>
				<Input id="np-name" bind:value={newName} placeholder="Acme Web" required />
			</div>
			{#if orgs.length > 1}
				<div class="space-y-1.5">
					<Label for="np-org">Organization</Label>
					<select
						id="np-org"
						bind:value={newOrg}
						class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 text-sm"
					>
						{#each orgs as o (o.id)}
							<option value={o.id}>{o.name}</option>
						{/each}
					</select>
				</div>
			{/if}
			<Button type="submit" disabled={creating}>{creating ? 'Creating…' : 'Create'}</Button>
		</div>
	</form>
{/if}

{#if error}<p class="mt-4 text-sm text-destructive">{error}</p>{/if}

{#if loading}
	<p class="mt-8 text-sm text-muted-foreground">Loading…</p>
{:else if projects.length === 0}
	<div class="mt-8 rounded-xl border border-dashed p-10 text-center">
		<p class="text-sm text-muted-foreground">No projects yet. Create your first one.</p>
	</div>
{:else}
	<div class="mt-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
		{#each projects as p (p.id)}
			<a
				href={`/p/${p.id}`}
				class="rounded-xl border bg-card p-4 shadow-sm transition-colors hover:border-foreground/20 hover:bg-accent/40"
			>
				<div class="font-medium">{p.name}</div>
				<div class="mt-0.5 text-xs text-muted-foreground">{p.slug}</div>
				{#if p.description}<p class="mt-2 text-sm text-muted-foreground">{p.description}</p>{/if}
			</a>
		{/each}
	</div>
{/if}
