<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type Activity } from '$lib/api';
	import { Button } from '$lib/components/ui/button/index';
	import { Badge } from '$lib/components/ui/badge/index';

	let { data } = $props();
	const pid = $derived(data.project.id);

	let items = $state<Activity[]>([]);
	let loading = $state(true);
	let err = $state('');
	let live = $state(true);

	async function load() {
		try {
			items = await api.listActivity(pid, 100);
			err = '';
		} catch (e) {
			err = (e as Error).message;
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		void load();
		// Light "live" feel: refetch periodically while the toggle is on.
		const timer = setInterval(() => {
			if (live) void load();
		}, 8000);
		return () => clearInterval(timer);
	});

	function label(type: string): string {
		return type.replace(/_/g, ' ');
	}
	function who(a: Activity): string {
		if (a.actorEmail) return a.actorEmail;
		return a.actorKind === 'mt' ? 'machine translation' : a.actorKind;
	}
	function fmt(ts: string): string {
		return new Date(ts).toLocaleString();
	}
</script>

<div class="flex items-center justify-between">
	<div>
		<h2 class="text-lg font-medium">Activity</h2>
		<p class="text-sm text-muted-foreground">Recent changes across this project.</p>
	</div>
	<div class="flex items-center gap-2">
		<label class="flex items-center gap-2 text-sm text-muted-foreground">
			<input type="checkbox" bind:checked={live} class="h-4 w-4 rounded border-input" />
			Live
		</label>
		<Button variant="outline" size="sm" onclick={() => load()}>Refresh</Button>
	</div>
</div>

{#if err}<p class="mt-3 text-sm text-destructive">{err}</p>{/if}

{#if loading && items.length === 0}
	<p class="mt-6 text-sm text-muted-foreground">Loading…</p>
{:else if items.length === 0}
	<div class="mt-6 rounded-xl border border-dashed p-10 text-center">
		<p class="text-sm text-muted-foreground">No activity yet.</p>
	</div>
{:else}
	<ol class="mt-4 divide-y rounded-xl border">
		{#each items as a (a.id)}
			<li class="flex items-start justify-between gap-3 px-4 py-3 text-sm">
				<div class="min-w-0">
					<div class="flex items-center gap-2">
						<Badge variant="secondary">{label(a.type)}</Badge>
						{#if a.keyName}
							<span class="truncate font-medium">{a.keyName}</span>
						{/if}
						{#if a.languageTag}
							<span class="text-muted-foreground">({a.languageTag})</span>
						{/if}
					</div>
					<div class="mt-1 text-xs text-muted-foreground">by {who(a)}</div>
				</div>
				<span class="shrink-0 text-xs text-muted-foreground">{fmt(a.createdAt)}</span>
			</li>
		{/each}
	</ol>
{/if}
