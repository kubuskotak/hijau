<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type EditorRow, type Language, type Translation } from '$lib/api';
	import { Button, buttonVariants } from '$lib/components/ui/button/index';
	import {Input} from '$lib/components/ui/input/index';
	import {Label} from '$lib/components/ui/label/index';
	import EditorCell from '$lib/components/editor-cell.svelte';
	import SidePanel from '$lib/components/side-panel.svelte';

	let { data } = $props();
	const pid = $derived(data.project.id);
	const baseId = $derived(data.project.baseLanguageId);
	const orderedLangs = $derived.by(() => {
		const langs = data.languages as Language[];
		return [...langs.filter((l) => l.id === baseId), ...langs.filter((l) => l.id !== baseId)];
	});

	let rows = $state<EditorRow[]>([]);
	let loading = $state(true);
	let err = $state('');
	let search = $state('');

	let showAdd = $state(false);
	let kName = $state('');
	let kNamespace = $state('');
	let kDesc = $state('');
	let adding = $state(false);
	let panel = $state<{ row: EditorRow; lang: Language } | null>(null);

	async function loadFeed() {
		loading = true;
		err = '';
		try {
			const feed = await api.editorFeed(pid, { search: search || undefined, limit: 200 });
			rows = feed.keys;
		} catch (e) {
			err = (e as Error).message;
		} finally {
			loading = false;
		}
	}

	onMount(loadFeed);

	let searchTimer: ReturnType<typeof setTimeout> | undefined;
	function onSearchInput() {
		clearTimeout(searchTimer);
		searchTimer = setTimeout(loadFeed, 250);
	}

	function onCellSaved(rowIdx: number, langId: string, updated: Translation, wasBase: boolean) {
		rows[rowIdx].translations[langId] = updated;
		if (wasBase) void loadFeed(); // refetch so OUTDATED siblings show
	}

	async function addKey(e: SubmitEvent) {
		e.preventDefault();
		if (!kName.trim()) return;
		adding = true;
		err = '';
		try {
			await api.createKey(pid, {
				name: kName.trim(),
				namespace: kNamespace.trim() || undefined,
				description: kDesc.trim() || undefined
			});
			kName = '';
			kNamespace = '';
			kDesc = '';
			showAdd = false;
			await loadFeed();
		} catch (e) {
			err = (e as Error).message;
		} finally {
			adding = false;
		}
	}
</script>

{#if orderedLangs.length === 0}
	<div class="rounded-xl border border-dashed p-10 text-center">
		<p class="text-sm text-muted-foreground">Add a language before you can manage translations.</p>
		<a href={`/p/${pid}/languages`} class={buttonVariants({ variant: 'default' }) + ' mt-4'}>
			Manage languages
		</a>
	</div>
{:else}
	<div class="flex items-center justify-between gap-3">
		<Input placeholder="Search keys…" bind:value={search} oninput={onSearchInput} class="max-w-xs" />
		<Button onclick={() => (showAdd = !showAdd)}>{showAdd ? 'Cancel' : 'Add key'}</Button>
	</div>

	{#if showAdd}
		<form class="rounded-xl border bg-card p-4 shadow-sm" onsubmit={addKey}>
			<div class="grid gap-3 sm:grid-cols-3">
				<div class="space-y-1.5">
					<Label for="k-name">Key name</Label>
					<Input id="k-name" bind:value={kName} placeholder="cart.checkout.button" required />
				</div>
				<div class="space-y-1.5">
					<Label for="k-ns">Namespace</Label>
					<Input id="k-ns" bind:value={kNamespace} placeholder="(optional)" />
				</div>
				<div class="space-y-1.5">
					<Label for="k-desc">Description</Label>
					<Input id="k-desc" bind:value={kDesc} placeholder="Context for translators" />
				</div>
			</div>
			<div class="mt-3">
				<Button type="submit" disabled={adding}>{adding ? 'Adding…' : 'Add key'}</Button>
			</div>
		</form>
	{/if}

	{#if err}<p class="text-sm text-destructive">{err}</p>{/if}

	<div class="overflow-x-auto rounded-xl border">
		<table class="w-full border-collapse text-sm">
			<thead>
				<tr class="border-b bg-muted/40 text-left">
					<th class="sticky left-0 z-10 bg-muted/40 px-3 py-2 font-medium">Key</th>
					{#each orderedLangs as l (l.id)}
						<th class="min-w-64 px-3 py-2 font-medium">
							{l.name}
							<span class="font-normal text-muted-foreground">({l.tag})</span>
							{#if l.id === baseId}<span class="text-muted-foreground"> · base</span>{/if}
						</th>
					{/each}
				</tr>
			</thead>
			<tbody>
				{#each rows as row, i (row.id)}
					<tr class="border-b align-top">
						<td class="sticky left-0 z-10 max-w-56 bg-background px-3 py-2">
							<div class="font-medium break-words">{row.name}</div>
							{#if row.description}
								<div class="text-xs text-muted-foreground">{row.description}</div>
							{/if}
						</td>
						{#each orderedLangs as l (l.id)}
							<td class="min-w-64 px-3 py-2">
								{#if row.translations[l.id]}
									<EditorCell
										{pid}
										keyId={row.id}
										lang={l}
										tr={row.translations[l.id]}
										isBase={l.id === baseId}
										onsaved={(t, wasBase) => onCellSaved(i, l.id, t, wasBase)}
										onopen={() => (panel = { row, lang: l })}
									/>
								{/if}
							</td>
						{/each}
					</tr>
				{/each}
			</tbody>
		</table>
	</div>

	{#if loading}
		<p class="text-sm text-muted-foreground">Loading…</p>
	{:else if rows.length === 0}
		<p class="text-sm text-muted-foreground">No keys yet. Add your first one.</p>
	{/if}
{/if}

{#if panel}
	<SidePanel
		{pid}
		keyRow={panel.row}
		lang={panel.lang}
		baseText={panel.row.translations[baseId]?.text ?? ''}
		onclose={() => (panel = null)}
	/>
{/if}
